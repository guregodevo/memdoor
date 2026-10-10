package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"memdoor/pkg/secrets"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"memdoor/gateway/config"
	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
	"memdoor/pkg/mcp"
	"memdoor/pkg/sandbox"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// agent_runtime_tools: tool execution and profile-based tool filtering.
// Split out of agent_adapter.go (2026-08-28) to keep one concern per file;
// Pattern: OpenClaw one-file-per-concern organization. Same package, same
// behavior — pure code movement.

// executeTool runs a tool and returns execution info
// Pattern: Tool execution wrapper (OpenClaw's tool handling pattern)
// toolOutcomeEvent turns one finished tool call into the event the log store
// can be asked questions of.
//
// It is a separate function so the CHOICE — which type, which fields, success
// or not — is testable without standing up a whole AgentRuntime. The emit that
// calls it is one unbranching line in executeTool, verified by a live receipt
// rather than a unit test.
func toolOutcomeEvent(info ToolExecutionInfo, sessionID, runID string, d time.Duration) *logs.Event {
	if info.Error != "" {
		ev := logs.NewEvent(logs.EventToolFailed, "Agent",
			fmt.Sprintf("tool %s failed: %s", info.Name, info.Error)).
			WithSession(sessionID).WithRun(runID).
			WithLevel(logs.LevelWarn).
			WithData("tool", info.Name).
			WithData("error", info.Error).
			WithDuration(d)
		ev.Success = false
		return ev
	}
	return logs.NewEvent(logs.EventToolCompleted, "Agent",
		fmt.Sprintf("tool %s completed", info.Name)).
		WithSession(sessionID).WithRun(runID).
		WithData("tool", info.Name).
		WithDuration(d)
}

// applyPatchIsATool is what a bash call that ran apply_patch as a command
// is told.
const applyPatchIsATool = "apply_patch is a TOOL, not a shell command — bash cannot run it. Call the apply_patch tool directly, with the FULL patch text (*** Begin Patch ... *** End Patch) as its input. Run builds/tests in a separate bash call"

// shellRanApplyPatch matches bash's own "command not found" for apply_patch,
// wherever it sits in the result: the formatter puts the first output line
// after its "Output:" label, so the message is not always at a line start.
var shellRanApplyPatch = regexp.MustCompile(`(?m)(?:^|\s)bash: (line \d+: )?apply_patch: command not found`)

// A TOOL'S OUTPUT NEVER CARRIES A SECRET (live 2026-10-10): asked for a free
// model, the coder ran `env | grep openrouter` and `cat
// ~/.memdoor/credentials.json`, and the person's OpenRouter key and two
// gateway tokens went to the model, the transcript on disk and the window.
// The shapes /share already redacts (pkg/secrets RedactText) are redacted
// here, once, for every tool, before the result reaches any of the three.
func (ar *AgentRuntime) executeTool(ctx context.Context, toolUse *llm.ToolUseBlock, runID string, session *Session) (info ToolExecutionInfo, result llm.ContentBlockParamUnion) {
	info, result = ar.executeToolUnredacted(ctx, toolUse, runID, session)
	return redactToolResult(info, result)
}

// redactToolResult replaces every secret-shaped value in a tool's output and
// error, and in the result block the model reads.
func redactToolResult(info ToolExecutionInfo, result llm.ContentBlockParamUnion) (ToolExecutionInfo, llm.ContentBlockParamUnion) {
	info.Output, info.Error = secrets.RedactText(info.Output), secrets.RedactText(info.Error)
	if r := result.OfToolResult; r != nil {
		for i := range r.Content {
			if t := r.Content[i].OfText; t != nil {
				t.Text = secrets.RedactText(t.Text)
			}
		}
	}
	return info, result
}

func (ar *AgentRuntime) executeToolUnredacted(ctx context.Context, toolUse *llm.ToolUseBlock, runID string, session *Session) (info ToolExecutionInfo, result llm.ContentBlockParamUnion) {
	// Returning is progress: the turn backstop counts from here (sharedctx.WithIdleTimeout).
	defer sharedctx.Progress(ctx)
	// Stamp the tool's wall time on every return path. A call blocked before it
	// runs (allowlist, guard, destructive-bash) records ~0ms, which is honest —
	// it didn't execute. Fixes the metering that reported DurationMs: 0 for all.
	start := time.Now()

	sessionID := session.ID
	log := logs.New("Agent").WithSession(sessionID).WithRun(runID)

	// A CALL WITH NO ARGUMENTS IS NOTHING, NOT A FAILURE.
	//
	// The API brain emits {"arguments":{},"name":"notes"} and then the real
	// call — a sampling artifact, not a decision. Counted as a failed
	// call, three of them tripped the loop-breaker and ended a turn with no
	// film (run 20, 2026-09-20: notes; run 7: todo_write). The call answers
	// with what it needs and costs the turn nothing, so the retry that
	// follows is the one that matters. A tool that legitimately takes no
	// input (todo_read, notes read) is unaffected: it is not in the list of
	// tools that need one.
	if emptyToolInput(toolUse.Input) && toolNeedsInput(toolUse.Name) {
		info.Name, info.Output = toolUse.Name, toolUse.Name+": no arguments were given — call it again with them (nothing was done, nothing failed)"
		log.Warn("Tool called with no arguments — answered, not failed", slog.String("tool", toolUse.Name))
		return info, llm.NewToolResultBlock(toolUse.ID, info.Output, false)
	}

	// A TOOL THAT PANICS MUST NOT TAKE THE GATEWAY WITH IT.
	//
	// Live 2026-09-17: one caption card with no lit word indexed words[-1],
	// and "index out of range" ended the PROCESS mid-turn — the film, the
	// session, the websocket and every other agent in it, for a bad index
	// in a picture. Nothing between the tool and main() caught anything.
	//
	// A panic here is still a bug and still gets its stack in the log; what
	// changes is who pays for it. The turn gets a failed tool call, which
	// it already knows how to handle — retry, work around, or tell the
	// user — and everything else keeps running.
	defer func() {
		if r := recover(); r != nil {
			// THE STACK GOES IN THE MESSAGE, not only in an attribute.
			// `memdoor logs query` renders the message and drops slog
			// attrs, so the first crash logged this way was unreadable from
			// the CLI: "PANICKED: nil pointer dereference" and nothing to
			// act on (2026-09-17). The first frames from OUR packages are
			// the whole diagnosis — the empty-word crash was one line.
			stack := string(debug.Stack())
			log.Error(fmt.Sprintf("Tool %s PANICKED: %v — at %s", toolUse.Name, r, ourFrames(stack, 4)),
				slog.String("tool", toolUse.Name),
				slog.String("stack", truncateForLog(stack, 4000)))
			info.Name, info.Input = toolUse.Name, string(toolUse.Input)
			info.Error = fmt.Sprintf("%s crashed on this input (%v). This is a bug in the tool, not in your call — "+
				"try a different input, or use another tool; repeating the same call will crash again.", toolUse.Name, r)
			info.DurationMs = time.Since(start).Milliseconds()
			result = llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		}
	}()

	// Stamp the tool's wall time on every return path, and EMIT the outcome.
	//
	// A call blocked before it runs (allowlist, guard, destructive-bash)
	// records ~0ms, which is honest — it didn't execute.
	//
	// The emit is why this is one defer and not two. Every failure path
	// already populated info.Error and every one of them threw it away:
	// EventToolFailed was declared in logs/event.go and emitted by nothing, so
	// `memdoor logs query` could not answer "what is my tool failure rate".
	// That is not just an observability gap. Production traces of 761M coding
	// agent calls (Liu et al.) put tool failures at 9% of turns and show them
	// amplifying compute ~4x through retry loops — 36 LLM calls in a turn
	// instead of 9 — so it is the largest cost line the harness itself
	// controls, and it was unmeasured.
	defer func() {
		info.DurationMs = time.Since(start).Milliseconds()
		log.EmitEvent(toolOutcomeEvent(info, sessionID, runID, time.Since(start)))
	}()

	// MISROUTED CALL: the model names one tool but its ARGUMENTS address another
	// (live: name=apply_patch, input {"command":"bash","parameters":{"command":
	// "go test ./..."}} — three identical "patch contains no file sections"
	// refusals). When the arguments' "command" is EXACTLY a registered tool name
	// and "parameters" carries that tool's args, dispatch there. A normal bash
	// call ({"command":"go test"}) never matches — a shell string is not a tool
	// name.
	var misrouted struct {
		Command    string          `json:"command"`
		Parameters json.RawMessage `json:"parameters"`
	}
	if json.Unmarshal(toolUse.Input, &misrouted) == nil &&
		misrouted.Command != "" && misrouted.Command != toolUse.Name &&
		len(misrouted.Parameters) > 0 && ar.hasTool(misrouted.Command) {
		log.Info("Re-routing misaddressed tool call",
			slog.String("named", toolUse.Name),
			slog.String("actual", misrouted.Command))
		toolUse.Name = misrouted.Command
		toolUse.Input = misrouted.Parameters
	}

	info = ToolExecutionInfo{
		Name:  toolUse.Name,
		Input: string(toolUse.Input),
	}

	// AUTHORIZATION: Verify tool is in the allowed list
	if allowedTools, ok := ctx.Value(ctxAllowedTools).(map[string]bool); ok {
		if !allowedTools[toolUse.Name] {
			info.Error = fmt.Sprintf("tool '%s' not allowed for this agent", toolUse.Name)
			log.Warn("Tool execution blocked by allowlist",
				slog.String("tool", toolUse.Name))
			return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		}
	}

	// Safety net: block obviously catastrophic bash commands (wipe root, format a
	// disk, fork bomb) in ANY mode — a misfiring model must not be able to
	// brick the machine on one command. Defense in depth, not a sandbox.
	if toolUse.Name == "bash" {
		var in struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(toolUse.Input, &in)
		if reason, blocked := destructiveBashReason(in.Command); blocked {
			info.Error = "blocked for safety (" + reason + "): if this is truly intended, run it yourself"
			log.Warn("Destructive bash command blocked",
				slog.String("reason", reason),
				slog.String("command", truncateForLog(in.Command, 200)))
			return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		}
		// TOOLS-AS-SHELL-COMMANDS: the model narrates its own tools into bash
		// ('apply_patch "*** Begin Patch / ..." \n bash "go run x.go"' — live,
		// twice, exit 127). The bare "command not found" teaches nothing; name
		// the mistake and the correct move. Read from what bash itself said,
		// not guessed from the text: a line starting "apply_patch" inside a
		// heredoc refused `cat > README.md` (live 2026-09-30).
		defer func() {
			if shellRanApplyPatch.MatchString(info.Error + "\n" + info.Output) {
				info.Error = applyPatchIsATool + "\n\n" + strings.TrimSpace(info.Error+"\n"+info.Output)
				result = llm.NewToolResultBlock(toolUse.ID, info.Error, true)
			}
		}()
	}

	// WORKSPACE TOOL GUARDS: admin-configured block rules (workspace setting
	// `tool_guards`) — the configurable layer above the hardcoded net.
	if ar.workspaceSetting != nil {
		if rules := parseToolGuards(ar.workspaceSetting("tool_guards")); len(rules) > 0 {
			if msg := guardBlocks(rules, toolUse.Name, toolUse.Input); msg != "" {
				info.Error = msg
				log.Warn("Tool call blocked by workspace guard",
					slog.String("tool", toolUse.Name),
					slog.String("input", truncateForLog(string(toolUse.Input), 200)))
				return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
			}
		}
	}

	// WHEN VERIFIABLE, DON'T ASK (ask_gate.go): a question the agent could
	// answer by reading or running is sent back to it instead of to the person.
	if toolUse.Name == tools.AskUserQuestionDefinition.Name && ar.agentWorksOnCodebase(ctx) {
		task, _ := ctx.Value(turnTaskKey{}).(string)
		if refusal := ar.turnVerdict.askGate(ctx, session, task, toolUse.Input); refusal != "" {
			info.Error = refusal
			log.Info("Ask gate held a question back", slog.String("input", truncateForLog(string(toolUse.Input), 200)))
			ar.decisionNote(runID, session.ID, "held a question back as verifiable: "+askedQuestion(toolUse.Input))
			return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		}
	}

	// APPROVAL MODE: a tool that changes or runs something waits for the
	// person's yes (approval.go) — the mode a company that forbids "yolo"
	// requires. Off by default; read per call.
	if err := ar.askApproval(ctx, toolUse, runID, session, log); err != nil {
		info.Error = err.Error()
		return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
	}

	toolUse.Input = withTurnTask(ctx, toolUse.Name, toolUse.Input)
	switch toolUse.Name {
	case "notes":
		toolUse.Input = withHarness(toolUse.Input, map[string]any{
			"conversation": conversationOf(ctx).String(), "rules_file": projectRulesFile(turnWorkdir(ctx))})
	case "recall":
		toolUse.Input = withHarness(toolUse.Input, map[string]any{"transcript": transcriptKeyOf(session)})
	}

	// Confine a codebase agent's file/bash work to a dedicated workspace so a
	// relative path (e.g. write_file "main.go") can't clobber the gateway's cwd —
	// which during dev IS this repo. An absolute path OUTSIDE the workdir is
	// refused with the same voice as the bash cd fence (see coderPathEscape).
	if ar.agentWorksOnCodebase(ctx) {
		wd := turnWorkdir(ctx)
		if msg := coderPathEscape(toolUse.Name, toolUse.Input, wd); msg != "" {
			info.Error = msg
			log.Warn("Tool call blocked: absolute path outside coder workdir",
				slog.String("tool", toolUse.Name),
				slog.String("input", truncateForLog(string(toolUse.Input), 200)))
			return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		}
		toolUse.Input = confineToCoderWorkdir(toolUse.Name, toolUse.Input, wd)
		// Give apply_patch the set of files this turn has actually READ/written, so
		// it can refuse a blind rewrite-from-memory of an existing file.
		if toolUse.Name == "apply_patch" {
			if reads, ok := ctx.Value(turnReadsKey{}).(map[string]bool); ok {
				var in map[string]any
				if json.Unmarshal(toolUse.Input, &in) == nil {
					names := make([]string, 0, len(reads))
					for n := range reads {
						names = append(names, n)
					}
					in["turn_reads"] = names
					if b, err := json.Marshal(in); err == nil {
						toolUse.Input = b
					}
				}
			}
		}
		info.Input = string(toolUse.Input)
	}

	// Extract sandbox context from Go context (set by the message handler upstream)
	var sandboxCtx sandbox.SandboxContext
	if sandboxCtxValue := ctx.Value(sharedctx.SandboxContextKey); sandboxCtxValue != nil {
		sandboxCtx = sandboxCtxValue.(sandbox.SandboxContext)
	}

	// Interactive ask_user_question: instead of reading the SERVER's stdin (which
	// isn't the user's terminal), emit the question as an event and BLOCK on the
	// broker until the client (TUI) sends the answer back over the WS. This turns
	// the tool into a request/response round-trip so the answer becomes the tool
	// result the agent reads.
	if toolUse.Name == "ask_user_question" {
		var q struct {
			Question string   `json:"question"`
			Options  []string `json:"options"`
			Default  string   `json:"default"`
		}
		if err := json.Unmarshal(toolUse.Input, &q); err != nil || q.Question == "" || len(q.Options) < 2 {
			info.Error = "ask_user_question needs a question and 2-5 options"
			return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		}
		qid := generateID("q")
		ch, cancel := ar.questions.register(qid)
		defer cancel()
		ar.events.EmitEvent(runID, "question", sessionID, map[string]interface{}{
			"question_id": qid,
			"question":    q.Question,
			"options":     q.Options,
			"default":     q.Default,
		})
		log.Info("Asked user question", slog.String("question", truncateForLog(q.Question, 120)))
		select {
		case ans := <-ch:
			out, _ := json.Marshal(map[string]string{"question": q.Question, "selected": ans})
			info.Output = string(out)
			return info, llm.NewToolResultBlock(toolUse.ID, string(out), false)
		case <-ctx.Done():
			info.Error = "question cancelled"
			return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		case <-time.After(5 * time.Minute):
			info.Error = "no answer received within 5 minutes"
			return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		}
	}

	// exit_plan_mode: the model signals its plan is ready. The tool's own function
	// returns a JSON blob (status/plan/note) — feeding that back as the tool result
	// dumps raw JSON into the chat. Instead emit the plan as a "plan" event the TUI
	// renders as a proposal, and hand the MODEL a short mode-aware ack (not the JSON,
	// not the plan echoed back — it just wrote it). Mirrors the ask_user_question
	// special-case above.
	if toolUse.Name == "exit_plan_mode" {
		var p struct {
			Plan string `json:"plan"`
		}
		_ = json.Unmarshal(toolUse.Input, &p)
		if strings.TrimSpace(p.Plan) == "" {
			info.Error = "exit_plan_mode needs a plan (the detailed plan to present)"
			return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
		}
		ar.events.EmitEvent(runID, "plan", sessionID, map[string]interface{}{
			"plan": p.Plan,
		})
		log.Info("Plan presented", slog.Int("plan_chars", len(p.Plan)))
		ack := "Plan presented to the user for approval. Stop here and wait — do not implement until the user approves."
		if permissionModeFromContext(ctx) != permissionModePlan {
			// The model called exit_plan_mode while not actually in plan mode; steer
			// it to just proceed rather than stall waiting for an approval that the
			// current mode doesn't gate on.
			ack = "Plan noted. You are not in plan mode — proceed to implement it now."
		}
		info.Output = ack
		return info, llm.NewToolResultBlock(toolUse.ID, ack, false)
	}

	// todo_write / todo_read: the DURABLE, channel-shared task list — the
	// coordination spine of the plan → execute → follow-up loop. Keyed by CHANNEL
	// (not session) so the planner and coder — different agents in the same
	// channel — read and write the SAME live list across turns. todo_write
	// persists the passed list; todo_read returns it. Both echo the list + the
	// follow-up directive (the next step, or verify+done), so no turn ends with
	// work still pending.
	if toolUse.Name == "todo_write" || toolUse.Name == "todo_read" {
		_, channel := ar.extractWorkspaceAndChannel(session.ID)
		key := channel
		if key == "" {
			key = session.ID
		}
		if toolUse.Name == "todo_write" {
			// AN EMPTY todo_write IS NOTHING, NOT A FAILURE. The API brain sends
			// {"arguments":{},"name":"todo_write"} and then the real one; each
			// empty one counted as a failed call, and three of them ended a
			// turn on the loop-breaker before any edit (run 7, 2026-09-20).
			if emptyToolInput(toolUse.Input) {
				info.Output = "no todos given — nothing changed; call todo_write with a todos list when you have one"
				return info, llm.NewToolResultBlock(toolUse.ID, info.Output, false)
			}
			var p tools.TodoWriteInput
			if err := json.Unmarshal(toolUse.Input, &p); err != nil || len(p.Todos) == 0 {
				// Absorb the shapes a small model actually emits instead of
				// rejecting them. Asked for a checklist, a model commonly sends
				// {"content":"step one, step two"} or {"todos":"a\nb"} — the
				// intent is unmistakable, and an error here costs a whole round
				// trip to teach a schema it already half-knows.
				if todos := tools.CoerceTodos(toolUse.Input); len(todos) > 0 {
					p.Todos = todos
				} else {
					info.Error = `todo_write needs a todos list, e.g. {"todos":[{"content":"Read the file","status":"in_progress","active_form":"Reading the file"}]}`
					return info, llm.NewToolResultBlock(toolUse.ID, info.Error, true)
				}
			}
			// Status-theater guard: items NEWLY flipped to completed in a turn where
			// no file-mutating tool has succeeded are boxes ticked without work (the
			// model "completes" plan items by bookkeeping alone). Mechanical, same
			// class as the mutation-claim note: the write still lands, the result
			// carries an honest flag the model AND user both see.
			before := tools.GetTasks(key)
			newlyDone := newlyCompletedCount(before, p.Todos)
			tools.NotePlanWrite(key, runID, before, p.Todos)
			tools.SetTasks(key, p.Todos)
			out := tools.FormatTodosFor(tools.GetTasks(key), tools.PlanFromEarlierRun(key, runID))
			if newlyDone > 0 {
				if mutated, ok := ctx.Value(turnMutatedKey{}).(*bool); ok && !*mutated {
					out += fmt.Sprintf("\n\n(note: %d item(s) marked completed, but NO file was changed this turn — if the work was not done in an earlier turn, it is NOT done. Do the work before marking it completed.)", newlyDone)
				}
			}
			info.Output = out
			return info, llm.NewToolResultBlock(toolUse.ID, out, false)
		}
		out := tools.FormatTodosFor(tools.GetTasks(key), tools.PlanFromEarlierRun(key, runID))
		info.Output = out
		return info, llm.NewToolResultBlock(toolUse.ID, out, false)
	}

	// Special handling for sessions_spawn tool (requires runtime context)
	if toolUse.Name == "sessions_spawn" && ar.sessionsSpawn != nil {
		// Get workspace directory from agent config
		workspace := ar.agentConfig.GetWorkspace()
		if workspace == "" {
			workspace = "."
		}

		// The REQUESTER is the agent currently running (set by the message service),
		// not the runtime's static config id — otherwise a spawn records the wrong
		// requester and the announce-back returns to the wrong agent (e.g. the
		// planner spawns the coder but the result never comes back to the planner).
		agentID, _ := ctx.Value("buddy_agent_name").(string)
		if agentID == "" {
			agentID = ar.agentConfig.ID
		}
		if agentID == "" {
			agentID = "main"
		}

		// CRITICAL FIX: Pass the session object directly to avoid metadata loss
		// Pattern: Thread-safe execution by using the session object directly
		// instead of looking up from SessionManager (which would create a race condition
		// if multiple agents are working concurrently in the same session)
		//
		// The problem we solved:
		// 1. Message service creates simpleSessionContext with parent_message_id in metadata
		// 2. AgentExecutorAdapter converts it to gateway.Session (ephemeral, not in SessionManager)
		// 3. Old approach: sessions_spawn called GetOrCreateSession() → created NEW empty session
		// 4. New approach: Pass session object directly → parent_message_id preserved
		log.Debug("Passing session object directly to sessions_spawn tool",
			slog.String("session_id", sessionID),
			slog.Int("metadata_count", len(session.Metadata)))

		// Execute with session object directly (thread-safe, no SessionManager lookup)
		toolResult, toolError := ar.sessionsSpawn.ExecuteWithSession(toolUse.Input, session, agentID, workspace, turnWorkdir(ctx))

		// Store result
		if toolError != nil {
			info.Error = toolError.Error()
			log.WithError(toolError).Error("Tool sessions_spawn failed")
			return info, llm.NewToolResultBlock(toolUse.ID, toolError.Error(), true)
		}

		info.Output = toolResult
		log.Debug("Tool sessions_spawn succeeded", slog.Int("bytes", len(toolResult)))
		return info, llm.NewToolResultBlock(toolUse.ID, toolResult, false)
	}

	// Special handling for task_flow tool (starts a deterministic managed flow
	// owned by the calling session; the flow registry drives the steps).
	if toolUse.Name == "task_flow" && ar.taskFlow != nil {
		toolResult, toolError := ar.taskFlow.ExecuteWithSession(toolUse.Input, session)
		if toolError != nil {
			info.Error = toolError.Error()
			log.WithError(toolError).Error("Tool task_flow failed")
			return info, llm.NewToolResultBlock(toolUse.ID, toolError.Error(), true)
		}
		info.Output = toolResult
		log.Debug("Tool task_flow succeeded", slog.Int("bytes", len(toolResult)))
		return info, llm.NewToolResultBlock(toolUse.ID, toolResult, false)
	}

	// Special handling for sessions_send tool (A2A messaging)
	if toolUse.Name == "sessions_send" && ar.sessionsSend != nil {
		// Cast to SessionsSendTool interface (defined at execution time)
		type sessionsSendExecutor interface {
			Execute(paramsJSON string) (string, error)
		}
		tool := ar.sessionsSend.(sessionsSendExecutor)

		// Execute A2A message send
		toolResult, toolError := tool.Execute(string(toolUse.Input))

		// Store result
		if toolError != nil {
			info.Error = toolError.Error()
			log.WithError(toolError).Error("Tool sessions_send failed (A2A)")
			return info, llm.NewToolResultBlock(toolUse.ID, toolError.Error(), true)
		}

		info.Output = toolResult
		log.Debug("Tool sessions_send succeeded (A2A)", slog.Int("bytes", len(toolResult)))
		return info, llm.NewToolResultBlock(toolUse.ID, toolResult, false)
	}

	// Special handling for agent_log tool (log reading)
	if toolUse.Name == "agent_log" && ar.agentLog != nil {
		// Execute log reading
		toolResult, toolError := ar.agentLog.Execute(string(toolUse.Input))

		// Store result
		if toolError != nil {
			info.Error = toolError.Error()
			log.WithError(toolError).Warn("Tool agent_log failed")
			return info, llm.NewToolResultBlock(toolUse.ID, toolError.Error(), true)
		}

		info.Output = toolResult
		log.Debug("Tool agent_log succeeded", slog.Int("bytes", len(toolResult)))
		return info, llm.NewToolResultBlock(toolUse.ID, toolResult, false)
	}

	// Special handling for memory tool (per-agent memory database)
	// Pattern: OpenClaw ~/.openclaw/memory/{agentId}.sqlite
	if toolUse.Name == "memory" {
		// Use buddy name from context (set by message service), fall back to config ID
		agentID, _ := ctx.Value("buddy_agent_name").(string)
		if agentID == "" {
			agentID = ar.agentConfig.ID
		}
		if agentID == "" {
			agentID = "main"
		}

		// Execute with repo-backed memory store (Raft-replicated)
		store := ar.GetMemoryStore(agentID)
		if store == nil {
			info.Error = "memory store not available"
			return info, llm.NewToolResultBlock(toolUse.ID, "memory store not available", true)
		}
		toolResult, toolError := tools.MemoryWithStore(toolUse.Input, store)

		// Store result
		if toolError != nil {
			info.Error = toolError.Error()
			log.WithError(toolError).Error("Tool memory failed")
			return info, llm.NewToolResultBlock(toolUse.ID, toolError.Error(), true)
		}

		info.Output = toolResult
		log.Debug("Tool memory succeeded",
			slog.String("agent_id", agentID),
			slog.Int("bytes", len(toolResult)))
		return info, llm.NewToolResultBlock(toolUse.ID, toolResult, false)
	}

	// Special handling for todo_write tool (broadcast todos to UI)
	// Pattern: Broadcast tool results for UI visibility
	if toolUse.Name == "todo_write" {
		// Execute the standard TodoWrite function
		toolResult, toolError := tools.TodoWrite(toolUse.Input)

		// Store result
		if toolError != nil {
			info.Error = toolError.Error()
			log.WithError(toolError).Error("Tool todo_write failed")
			return info, llm.NewToolResultBlock(toolUse.ID, toolError.Error(), true)
		}

		info.Output = toolResult

		// Emit event for todo updates (for TUI/Web UI)
		// Parse the input to extract todos for broadcasting
		var todoInput tools.TodoWriteInput
		if err := json.Unmarshal(toolUse.Input, &todoInput); err == nil {
			// Emit event with todo data using infra.EventEmitter
			ar.events.EmitEvent(runID, infra.EventStreamTool, sessionID, map[string]interface{}{
				"type":  "todo",
				"todos": todoInput.Todos,
			})
		}

		log.Debug("Tool todo_write succeeded", slog.Int("todos", len(todoInput.Todos)))
		return info, llm.NewToolResultBlock(toolUse.ID, toolResult, false)
	}

	// Special handling for get_secret tool (per-agent scoped secrets)
	if toolUse.Name == "get_secret" && ar.secretRepo != nil {
		agentID := sandboxCtx.CurrentAgentID
		getter := &repoSecretGetter{repo: ar.secretRepo}
		toolResult, toolError := tools.GetSecretWithRepo(toolUse.Input, getter, agentID)

		if toolError != nil {
			info.Error = toolError.Error()
			log.WithError(toolError).Error("Tool get_secret failed")
			return info, llm.NewToolResultBlock(toolUse.ID, toolError.Error(), true)
		}

		info.Output = "[secret retrieved]"
		log.Debug("Tool get_secret succeeded", slog.String("agent_id", agentID))
		return info, llm.NewToolResultBlock(toolUse.ID, toolResult, false)
	}

	// A tool that streams (bash today) runs through its streamingExecutor so the
	// TUI can render a live tail while it works, and so a non-terminating command
	// is bounded instead of hanging the turn. Each chunk is emitted as a "delta"
	// tool event tagged with the tool's own name; the executor returns the final
	// tool result (already formatted, e.g. bash's FAILED message). The safety and
	// tools-as-shell guards above already ran, and confineToCoderWorkdir already
	// rewrote paths, so what streams here is exactly what would otherwise run.
	if streamer := streamerFor(toolUse.Name); streamer != nil {
		// Progress is reported as the tool stream's "update" event carrying the
		// output SO FAR — the shape the consumer/assembler already understands
		// (ToolEventData.Event == "update" updates the tool's Output). A bespoke
		// event name would be dropped: the TUI's live path is
		// OnAgentEvent -> consumer.ProcessEvent -> handleToolUpdate.
		var sofar strings.Builder
		out, err := streamer.Stream(ctx, toolUse.Input, func(chunk string) {
			sofar.WriteString(chunk)
			ar.events.EmitEvent(runID, infra.EventStreamTool, sessionID, map[string]interface{}{
				"event":  "update",
				"tool":   toolUse.Name,
				"output": sofar.String(),
			})
		})
		if err != nil {
			info.Error = err.Error()
			log.WithError(err).Error("Streaming tool failed", slog.String("tool", toolUse.Name))
			return info, llm.NewToolResultBlock(toolUse.ID, err.Error(), true)
		}
		info.Output = out
		return info, llm.NewToolResultBlock(toolUse.ID, out, false)
	}

	// Standard tool execution (for all other tools)
	var toolResult string
	var toolError error
	var toolFound bool

	// A codebase agent (coder, planner) works directly on the real project
	// filesystem — the same cwd bash/edit_file/grep/glob already operate in. The
	// sandbox-aware file tools (read_file/write_file/list_files) otherwise remap
	// paths under ~/.memdoor/sandbox (and, with no AgentScope set, deny them
	// outright), so a file the coder wrote via write_file would be invisible to a
	// later `bash go build`, and the planner's read_file couldn't see the project
	// at all. Route those specific tools through the legacy real-fs Function so
	// the whole palette agrees on ONE filesystem. No new reach: bash/edit_file
	// (coder) and grep/glob (both) already grant real-fs access; this only removes
	// an inconsistency.
	doerRealFS := ar.agentWorksOnCodebase(ctx)

	for _, tool := range ar.turnTools(ctx) {
		if tool.Name == toolUse.Name {
			toolFound = true
			realFS := doerRealFS && doerFilesystemTools[tool.Name] && tool.Function != nil
			toolUse.Input = coerceInputToSchema(toolUse.Input, tool.InputSchema)
			switch {
			case tool.FunctionWithContext != nil && !realFS:
				// Prefer FunctionWithContext (sandbox-aware) if available.
				toolResult, toolError = tool.FunctionWithContext(toolUse.Input, sandboxCtx)
			case tool.Function != nil:
				// Legacy Function: no sandbox remap — real cwd.
				toolResult, toolError = tool.Function(toolUse.Input)
			default:
				toolError = fmt.Errorf("tool '%s' has no execution function", toolUse.Name)
			}
			break
		}
	}

	if !toolFound {
		toolError = fmt.Errorf("tool '%s' not found", toolUse.Name)
	}

	// Store result
	if toolError != nil {
		info.Error = toolError.Error()
		log.WithError(toolError).Error("Tool execution failed", slog.String("tool", toolUse.Name))
		return info, llm.NewToolResultBlock(toolUse.ID, toolError.Error(), true)
	}

	info.Output = toolResult
	// INFO, not Debug. A failing tool logged at Warn/Error and a succeeding one
	// at Debug meant `memdoor logs query` could show you every tool that broke
	// and never one that worked — so "which tools does a turn actually use, and
	// how long do they take" was unanswerable without scraping the TUI pane.
	// That is the measurement the harness work depends on, and one line per
	// tool call is a few dozen a turn.
	log.Info("Tool execution succeeded",
		slog.String("tool", toolUse.Name),
		slog.Int("bytes", len(toolResult)),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()))
	return info, llm.NewToolResultBlock(toolUse.ID, toolResult, false)
}

// runInference makes an API call to the LLM
// Pattern: API wrapper (clean separation like OpenClaw)
// getFilteredTools returns tools filtered based on agent configuration
// Pattern: OpenClaw tool filtering with profiles and smart defaults
// Now accepts context to support buddy-specific tool configuration from database
func (ar *AgentRuntime) getFilteredTools(ctx context.Context) []tools.ToolDefinition {
	return ar.filteredToolsFor(ctx)
}

func (ar *AgentRuntime) filteredToolsFor(ctx context.Context) []tools.ToolDefinition {
	log := logs.New("Agent")

	// Check for buddy_tools in context (from database configuration)
	// This allows per-agent tool configuration to override config files
	if buddyToolsValue := ctx.Value(sharedctx.BuddyToolsKey); buddyToolsValue != nil {
		if buddyTools, ok := buddyToolsValue.([]string); ok && len(buddyTools) > 0 {
			log.Debug("Using buddy.Tools from database as allowlist",
				slog.Int("tool_count", len(buddyTools)),
				slog.Any("tools", buddyTools))

			// Filter tools to only include those in buddy.Tools
			filtered := make([]tools.ToolDefinition, 0, len(buddyTools))
			buddyToolSet := make(map[string]bool)
			for _, toolName := range buddyTools {
				buddyToolSet[toolName] = true
			}

			for _, tool := range ar.turnTools(ctx) {
				if buddyToolSet[tool.Name] || (buddyToolSet[mcpPaletteEntry] && strings.HasPrefix(tool.Name, mcp.ToolPrefix)) {
					filtered = append(filtered, tool)
				}
			}

			// Include memory tool for learning — but skip for agents
			// that have only read-oriented tools, to keep the tool count
			// low and avoid confusing the model.
			if !buddyToolSet["memory"] && len(buddyToolSet) > 1 {
				for _, tool := range ar.tools {
					if tool.Name == "memory" {
						filtered = append(filtered, tool)
						break
					}
				}
			}

			filteredNames := make([]string, 0, len(filtered))
			for _, t := range filtered {
				filteredNames = append(filteredNames, t.Name)
			}
			log.Debug(fmt.Sprintf("Buddy tool filtering applied: filtered=[%s] palette=[%s]",
				strings.Join(filteredNames, ","), strings.Join(buddyTools, ",")))

			return filtered
		}
	}

	// Apply default profile when no tool configuration is specified
	needsDefault := ar.agentConfig == nil ||
		ar.agentConfig.Tools == nil ||
		(!ar.agentConfig.HasToolsProfile() && !ar.agentConfig.HasToolsAllowList() && !ar.agentConfig.HasToolsDenyList())

	if needsDefault {
		log.Debug("Applying default tool profile",
			slog.String("profile", string(config.DefaultToolProfile)))
		return ar.filterToolsByProfile(string(config.DefaultToolProfile))
	}

	// Filter tools based on agent configuration (profile + allow/deny)
	filtered := make([]tools.ToolDefinition, 0, len(ar.tools))
	for _, tool := range ar.tools {
		if ar.agentConfig.IsToolAllowed(tool.Name) {
			filtered = append(filtered, tool)
		}
	}

	log.Debug("Tool filtering applied",
		slog.String("agent_id", ar.agentConfig.ID),
		slog.String("profile", ar.agentConfig.Tools.Profile),
		slog.Int("allowed_tools", len(filtered)),
		slog.Int("total_tools", len(ar.tools)))

	return filtered
}

// filterToolsByProfile filters tools using a specific profile
// Helper method for applying default profiles
func (ar *AgentRuntime) filterToolsByProfile(profileID string) []tools.ToolDefinition {
	// Determine agent ID (default to "main" if not available)
	agentID := "main"
	if ar.agentConfig != nil {
		agentID = ar.agentConfig.ID
	}

	// Create a temporary agent config with the profile
	tempConfig := &config.AgentConfig{
		ID: agentID,
		Tools: &config.AgentToolsConfig{
			Profile: profileID,
		},
	}

	// Filter tools
	filtered := make([]tools.ToolDefinition, 0, len(ar.tools))
	for _, tool := range ar.tools {
		if tempConfig.IsToolAllowed(tool.Name) {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// ourFrames picks the first n frames from memdoor's own packages out of a
// stack, as "file.go:line". The runtime and library frames above them say
// where it blew up; these say what we did.
func ourFrames(stack string, n int) string {
	var out []string
	for _, line := range strings.Split(stack, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "/Dev/aktapus/") && !strings.Contains(line, "memdoor/") {
			continue
		}
		i := strings.LastIndex(line, "/")
		if i < 0 {
			continue
		}
		frame := line[i+1:]
		if j := strings.Index(frame, " "); j > 0 {
			frame = frame[:j]
		}
		if !strings.Contains(frame, ".go:") {
			continue
		}
		out = append(out, frame)
		if len(out) == n {
			break
		}
	}
	if len(out) == 0 {
		return "an unreadable stack"
	}
	return strings.Join(out, " ← ")
}

// emptyToolInput says whether a call carried no arguments at all: an
// empty object, or nothing.
func emptyToolInput(in json.RawMessage) bool {
	return strings.Trim(string(in), "{} \t\r\n") == ""
}

// toolNeedsInput says whether a tool cannot do anything without
// arguments. todo_read and the like are absent on purpose: an empty call
// is a real call for them.
func toolNeedsInput(name string) bool {
	switch name {
	case "todo_read", "ask_user_question", "exit_plan_mode":
		return false
	}
	return name == "notes" || name == "todo_write" || name == "bash" ||
		name == "read_file" || name == "skill" || name == "web_fetch"
}
