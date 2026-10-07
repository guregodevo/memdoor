package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
	"memdoor/pkg/savings"
	sharedctx "memdoor/pkg/shared/context"
)

// agent_runtime_process: ProcessMessage — the turn driver: assemble the turn, run the tool loop, apply the honesty guards, persist.
// Split out of agent_adapter.go (2026-08-28) to keep one concern per file;
// Pattern: OpenClaw one-file-per-concern organization. Same package, same
// behavior — pure code movement.

// ProcessMessage handles a single user message and returns agent response
// Pattern: Single responsibility - one message in, one response out (OpenClaw style)
// Added extraSystemPrompt parameter for ping-pong conversations
// retiredToolResult is what a tool gets instead of running, once it has failed
// enough times this turn to be considered unavailable rather than unlucky.
//
// The message has to do two things the plain error did not: say the tool is
// gone for this turn (so the model stops treating each failure as a fresh
// attempt), and name what to do instead. A bare repeat of "no workspace
// available" invites exactly the retry it is trying to stop — measured at five
// identical failures in one live turn.
func retiredToolResult(name, key, input, id string, fails int, why string) (ToolExecutionInfo, llm.ContentBlockParamUnion) {
	// SAY WHAT WENT WRONG, NOT JUST THAT SOMETHING DID. "Do not call it
	// again" was the whole message, and live (2026-09-16) the same call came
	// back four more times — because nothing in the refusal said what to do
	// instead. The last real reason is the actionable half: a file name that
	// does not exist can be fixed on the next call, a missing workspace
	// cannot.
	msg := fmt.Sprintf("tool '%s' failed %d times this turn, so it is not being run again here.", name, fails)
	if on, ok := strings.CutPrefix(key, name+" on "); ok {
		msg = fmt.Sprintf("tool '%s' failed %d times this turn on %s, so it is not run again on that here; other files are not affected.", name, fails, on)
	}
	if strings.TrimSpace(why) != "" {
		msg += " The last reason was: " + truncateForLog(strings.TrimSpace(why), 400) +
			" — fix THAT if it is fixable (a wrong file name, a missing argument) and the tool works again next turn."
	}
	msg += " Calling it again returns this same line; use another tool or finish with what you have."
	info := ToolExecutionInfo{Name: name, Input: input, Error: msg}
	return info, llm.NewToolResultBlock(id, info.Error, true)
}

// abandonRest decides whether a tool_use block should be refused unrun because
// the rest of THIS assistant message is not worth executing, and returns the
// result text to hand back.
//
// It exists because every other limit bounds ITERATIONS, not the blocks inside
// one reply, and a degenerate generation is mostly tool calls. Measured
// 2026-08-30 on a 27B asked to fix a file that did not exist: one
// message carried 583 read_file blocks. The loop-breaker set `stuck` on the
// 4th, but it is only read after the whole message is processed, so 579 more
// ran. Healthy turns in the same session carried 2 and 8.
//
// n is this block's 1-based position among the message's tool calls.
func abandonRest(stuck bool, n, max int) (string, bool) {
	const tail = " STOP issuing tool calls and answer with what you have."
	if stuck {
		return "not run: the loop-breaker tripped earlier in this same reply." + tail, true
	}
	if n > max {
		return fmt.Sprintf("not run: this reply exceeded %d tool calls.", max) + tail, true
	}
	return "", false
}

// reasoningOnlyReply reports whether a reply is nothing but the model's own
// reasoning — it thought, announced what it would do, and never did it.
//
// replyContent strips <think>…</think> so the ANSWER reaches the transcript,
// but keeps the RAW text when stripping would leave nothing (a reply that is
// only reasoning). A surviving think marker is therefore the signal: it means
// the strip found nothing else to keep.
//
// Measured 2026-08-30 across a four-task suite on a 27B: two of four
// turns ended this way. The whole stored reply was "Let's start by reading the
// stats.go file first to see what's in it.\n</think>" — 393 seconds of
// generation, zero tool calls, nothing edited, reported as a task failure. The
// existing empty-step retry cannot catch it: that one requires EMPTY text and
// at least one tool already executed, and this has text and none.
func reasoningOnlyReply(text string) bool {
	return strings.Contains(text, "</think>") || strings.Contains(text, "<think>") ||
		announcesUnmadeAction(text)
}

// announcesUnmadeAction is the SECOND signal, and the one the marker rule
// misses: the model writes its reasoning as ordinary prose — no think tags at
// all — and finishes by saying what it is about to do, having done nothing.
// Measured live 2026-08-30 23:03, a full page of planning ending:
//
//	I'll make the calls.
//	I'll start by loading the relevant skills and checking the environment in parallel.
//
// No tool call anywhere in it. The turn ended silently, nothing was built, and
// the marker rule could not see it because no marker survived.
//
// Only the TAIL is examined. Mid-reply an announcement is just narration
// ("let me check the version, then..."), and a finished answer that happens to
// contain one earlier is not asking to be retried. What identifies this
// failure is a reply that STOPS on the announcement.
//
// This is only ever consulted where the reply made no tool call at all (the
// !hasToolUse branch), so an announcement here is by definition unfulfilled.
// The cost of being wrong is one extra inference, once per turn: the reply
// that already streamed stays in the person's answer.
func announcesUnmadeAction(text string) bool {
	t := []rune(strings.ToLower(strings.TrimSpace(text)))
	if len(t) == 0 {
		return false
	}
	const tail = 220
	if len(t) > tail {
		t = t[len(t)-tail:]
	}
	s := string(t)

	// "let me know if…" is an offer to the reader, not an action the model was
	// about to take. It is the one phrase that would otherwise make every
	// polite finished answer look unfinished.
	s = strings.ReplaceAll(s, "let me know", "")
	s = strings.ReplaceAll(s, "let us know", "")

	for _, p := range []string{
		"i'll ", "i will ", "i am going to", "i'm going to",
		"let me ", "let's ", "lets ",
		"start by", "begin by", "first, i", "next, i",
		"make the call", "making the call",
		// The editor's: "Fixing both: extend part 1 to 257 s, start part 9
		// at 901 s" ended a turn with the fix described and no stitch
		// (Sara's brief, 2026-09-19).
		"fixing ", "re-stitch", "restitch", "stitching now", "rendering now", "re-render", "rebuilding",
		// "Now assembling both films — every part is the whole shot…" ended
		// the duckling song's turn with a plan written and no stitch
		// (2026-09-21): the present progressive, said as the last thing.
		"now assembling", "assembling now", "assembling both", "assembling the",
		"now stitching", "now rendering", "now generating", "generating now", "now cutting", "now tightening",
	} {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// ProcessMessage wraps the turn driver so EVERY error exit emits a lifecycle
// "error" event. The turn's death was already broadcast as execution.failed on
// the channel hub, but the TUI demonstrably receives the AGENT-EVENT stream
// (its deltas arrive on it) and that stream said nothing — so the screen kept
// a spinner counting over a turn the gateway knew was dead (live, 2026-08-31
// 15:21: "connection refused" in the log, "Puzzling… 4m 31s" on screen).
// One wrapper beats an emit at each of the driver's error returns, which is a
// rule that only holds until someone adds the next return.
func (ar *AgentRuntime) ProcessMessage(ctx context.Context, userMessage string, session *Session, runID string, extraSystemPrompt string) (*AgentResponse, error) {
	// A spawned run is invisible to the screen that asked for it — different
	// session, different run, and events are broadcast per session. This
	// mirrors "your subagent is working" onto that screen for the whole turn
	// (agent_runtime_progress.go); it does nothing for an ordinary turn.
	stopMirror := ar.watchTurnForRequester(session, runID)
	defer stopMirror()

	resp, err := ar.processMessage(ctx, userMessage, session, runID, extraSystemPrompt)
	if err != nil && runID != "" {
		ar.events.EmitEvent(runID, infra.EventStreamLifecycle, session.ID, map[string]interface{}{
			"event": "error",
			"error": err.Error(),
		})
	}
	return resp, err
}

func (ar *AgentRuntime) processMessage(ctx context.Context, userMessage string, session *Session, runID string, extraSystemPrompt string) (out *AgentResponse, err error) {
	log := logs.New("Agent").WithSession(session.ID).WithRun(runID)
	log.Debug("Processing message", slog.String("message", userMessage))

	// Emit lifecycle start event (OpenClaw pattern)
	ar.events.EmitEvent(runID, infra.EventStreamLifecycle, session.ID, map[string]interface{}{
		"event":   "start",
		"message": userMessage,
	})

	// Start tracking execution flow
	ar.flowTracker.Start(runID, session.ID)

	// Build conversation history from session. Pass ctx so the builder can
	// size the sliding window from the agent's palette (a codebase agent
	// carries the whole conversation; a chat agent keeps ten messages).
	conversation := ar.buildConversation(ctx, session)

	// Validate user message is not empty or whitespace-only
	trimmedMessage := strings.TrimSpace(userMessage)
	if trimmedMessage == "" {
		return nil, fmt.Errorf("user message cannot be empty or contain only whitespace")
	}

	// SLASH SKILLS: "/name args" becomes the named skill's content injected as
	// the task (see slash_skills.go). Deterministic — no reliance on the model
	// choosing to call the skill tool. Unknown names pass through unchanged.
	if expanded, ok := expandSlashSkill(trimmedMessage, turnWorkdir(ctx)); ok {
		log.Info("Slash skill expanded", slog.String("command", truncateForLog(trimmedMessage, 80)))
		userMessage = expanded
	}

	// LEARNING: Inject relevant past memories before the user message
	agentName, _ := ctx.Value("buddy_agent_name").(string)
	logs.New("Agent").Info(fmt.Sprintf("AGENT TASK [%s]: %s", agentName, truncateForLog(userMessage, 600)))

	// The turn's request rides on the context so tools that judge their output
	// against the task (read_file, notes) get it per call (withTurnTask).
	ctx = context.WithValue(ctx, turnTaskKey{}, userMessage)

	// The person's MCP servers' tools join the turn before routing sees it
	// (mcp_tools.go).
	ctx = withMCPTools(ctx)

	// before_prompt_build: contributors may narrow the tools submitted for this
	// turn (turn_tools.go). The decision-model router is one; it fails open.
	if allow := ar.toolRouter.route(ctx, agentName, userMessage); allow != nil {
		ctx = withTurnToolsAllow(ctx, allow)
	}
	// Add new user message
	userMsg := llm.NewUserMessage(llm.NewTextBlock(userMessage))
	conversation = append(conversation, userMsg)

	// Track response
	response := &AgentResponse{
		ToolsExecuted: make([]ToolExecutionInfo, 0),
	}

	// Hang-backstop timeout, NOT a pacer — same contract as the outer
	// agent-execution timeout in pkg/message/service.go (ff04d7f). A shorter
	// inner deadline silently ended slow-but-healthy turns one layer deeper,
	// as "empty final text" at exactly 10:00.
	backstop := agentBackstopTimeout()
	// A hung turn is one that stopped moving: the backstop counts from the
	// last model answer or tool result (sharedctx.Progress in runInference
	// and executeTool), not from the start. As a total it ended a coder turn
	// at exactly 30:00 with the model answering every few seconds (live
	// 2026-09-30, 19:49:52 → 20:19:52, reported as "oai request failed …
	// context deadline exceeded").
	ctx, cancel := sharedctx.WithIdleTimeout(ctx, backstop)
	defer cancel()
	// A turn the backstop ended says so, not what the call in flight saw.
	turnCtx := ctx
	defer func() {
		if cause := context.Cause(turnCtx); err != nil && errors.Is(cause, sharedctx.ErrNoProgress) {
			err = fmt.Errorf("the turn stopped: %w (the call in flight: %v)", cause, err)
			if out != nil {
				out.Error = err.Error()
			}
		}
	}()

	// Add run ID and session ID to context for pre-flight checking
	// Add session object to context for memory flush tracking
	ctx = context.WithValue(ctx, ctxRunID, runID)
	ctx = context.WithValue(ctx, ctxSessionID, session.ID)
	ctx = context.WithValue(ctx, sharedctx.SessionIDKey, session.ID)
	if t := sessionTier(session); t > 0 {
		ctx = context.WithValue(ctx, sharedctx.TierKey, t)
	}
	if m := sessionModel(session); m != "" {
		ctx = context.WithValue(ctx, sharedctx.ModelKey, m) // a model pinned by id
		ctx = context.WithValue(ctx, sharedctx.SortKey, sessionString(session, sessionSortKey))
		ctx = context.WithValue(ctx, sharedctx.OrderKey, sessionString(session, sessionOrderKey))
	}
	effort, effortFrom := ar.turnEffort(ctx, session, agentName, userMessage)
	ctx = context.WithValue(ctx, sharedctx.EffortKey, effort)
	reportRoute(ctx, sessionEffortUsed, effort)
	reportRoute(ctx, sessionEffortFrom, effortFrom)
	logs.New("Agent").Info("turn effort", slog.String("effort", effort), slog.String("from", effortFrom), slog.String("session", session.ID))
	if t := sessionTier(session); t > 0 || sessionRouted(session) {
		// Legible in the log too: the rung a turn runs on and why.
		logs.New("Agent").Info("turn route", slog.Int("tier", t), slog.Bool("pinned", sessionPinned(session)),
			slog.String("model", sessionModel(session)), slog.String("reason", sessionReason(session)), slog.String("session", session.ID))
	}
	ctx = context.WithValue(ctx, ctxSession, session)

	// Emit thinking event before API call
	ar.events.EmitEvent(runID, infra.EventStreamAssistant, session.ID, map[string]interface{}{
		"event": "thinking",
	})

	// Get the model's first response. No forcing — the coder is just its skills and
	// hard-to-fail tools: the model decides to call a tool on its own, the tool-call
	// parser accepts whatever envelope it emits, and a tool that can't do the job
	// returns an explicit, actionable error the model reads and fixes. That error
	// text is the feedback loop; nothing constrains decoding.
	// A TURN WITH A BUDGET RUNS UNATTENDED (turn_done.go): the model is told
	// so, and told to ask its questions first.
	turnBudget := ar.turnVerdict.sessionBudget(session)
	if turnBudget > 0 && ar.agentWorksOnCodebase(ctx) {
		if extraSystemPrompt != "" {
			extraSystemPrompt += "\n\n"
		}
		extraSystemPrompt += unattendedPrompt(turnBudget)
	}
	message, conversation, err := ar.runInference(ctx, conversation, extraSystemPrompt)
	// Everything after the first inference is mid-turn: a threshold compaction
	// waits for the next turn (see compactionDecision).
	ctx = withMidTurn(ctx)
	if err != nil {
		log.WithError(err).Error("Inference failed")
		response.Error = err.Error()
		// Emit error event
		ar.events.EmitEvent(runID, infra.EventStreamError, session.ID, map[string]interface{}{
			"error": err.Error(),
		})
		return response, err
	}
	conversation = append(conversation, message.ToParam())

	// Emit context update immediately after first API response (for real-time token display)
	// This ensures token counts appear during tool execution, not just at the end
	ar.emitContextUpdate(runID, session.ID, conversation)

	// Process response and handle tool loops
	// Pattern: Tool execution loop (like OpenClaw's tool handling)
	// The cap that used to sit here (50 API calls, calibrated from
	// multi-instrument research runs) is gone (Greg, 2026-09-26: "no hard
	// limit please"): a coder building an app from
	// scratch used all 50 on a HEALTHY run and was cut off while tidying. What
	// stops a spin is the loop-breakers below, the streamed-bash cap, the
	// turn verdict and the shadow score (route_shadow.go) — and the person,
	// who sees the bar and can press Esc.
	apiCallCount := 1 // Already made 1 call in runInference above

	// Loop-breaker. Two ways a small model spins without progress, both end the turn:
	//   1. It re-emits the EXACT SAME failing tool call (an apply_patch whose anchor will
	//      never match) — count identical calls, stop at maxRepeatedFail.
	//   2. It FLAILS: several tool calls fail in a row with DIFFERENT inputs and no
	//      success between (e.g. after finishing, it loads the wrong skill and tries to
	//      apply_patch that skill's markdown, each attempt malformed differently). The
	//      exact-repeat counter never trips on these, so also count consecutive failures
	//      across any tools and stop at maxConsecutiveFail. A successful tool call means
	//      progress and resets it, so a normal fix loop (patch fails, read, patch works)
	//      is unaffected.
	repeatedFail := map[string]int{}
	const maxRepeatedFail = 3
	consecutiveFail := 0
	const maxConsecutiveFail = 4
	// A GUARD'S FIRST TRIP IS A WORD, NOT THE END (Greg, 2026-10-05: "like
	// omp" — oh-my-pi's loop guard injects a redirect and lets the turn go
	// on). The loop-breaker and the exploring check nudge once, in the
	// model's next input; the second trip ends the turn as before.
	loopNudged, exploreNudged := false, false

	//   3. A tool fails for a STRUCTURAL reason — it is in the agent's palette but
	//      cannot work in this context at all. Neither counter above catches it:
	//      repeatedFail is keyed on name AND input, so varying one argument resets
	//      it, and consecutiveFail resets on any successful call in between.
	//
	//      Measured 2026-08-29 on a live coder turn: one tool failed FIVE
	//      times with "no workspace available", interleaved with successful
	//      read_file and context calls that
	//      reset the flailing counter each time. Five of eleven tool calls in that
	//      turn — a 45% failure rate from one tool that could never have worked.
	//
	//      "No workspace available" will not become true later in the same turn, so
	//      the third failure of a tool BY NAME retires it for the rest of the turn.
	//      It does not end the turn — the model keeps its other tools and its work —
	//      it just stops paying a round trip to be told the same thing again. This
	//      is the retry amplification the production traces measure at ~4x compute.
	failsByTool := toolFailureLedger{}
	const maxFailsPerTool = 3

	// maxCallsPerMessage bounds tool calls inside ONE assistant message. See the
	// abandon check in the loop below: 583 in a single reply, measured.
	const maxCallsPerMessage = 24
	abandoned := 0
	emptyStepRetried := false
	emptyStepClimbed := false
	reasoningOnlyRetried := false
	recapRetried := false
	doneRounds := 0
	var turnTokens, doneFrom int64 // tokens this turn, and the count when continuation began
	streamed := ""                 // the rounds' text as the window received it (unstreamed.go)

	// TRUNCATION RECOVERY — Claude Code's shape (query.ts:1185).
	//
	// A reply cut off at max_tokens did not finish: whatever it was doing is
	// incomplete, and the tool call it was about to emit does not exist. Ending
	// the turn there throws the work away, and CAPPING LOWER only reaches that
	// point sooner while truncating legitimate long generations (a whole-file
	// patch is thousands of tokens).
	//
	// So recover, in two stages. First escalate: retry the SAME request once at
	// a larger cap, on the theory that it simply needed more room. If that is
	// also cut off, keep the truncated text and ask the model to RESUME
	// mid-thought — up to maxTruncationRecoveries times.
	//
	// Measured 2026-08-30 on a 27B: turns of exactly 16,384 output tokens
	// taking 369s at 44 tok/s, ending with no tool call and no work done, while
	// healthy turns were 65-90 tokens. Nothing recovered and nothing even said
	// it had happened.
	answering, _ := ar.answeringModel(ctx)
	recovery, err := NewTruncationRecovery(escalatedMaxOutputTokens(answering))
	if err != nil {
		return nil, err
	}
	repeatedOK := map[string]int{}
	// askedBefore is every call's input, by tool, for the repeat that the two
	// counters above cannot see: the SAME ASK reworded, whose result also
	// changes each time. Live 2026-09-18: forty-one notes appends of one
	// rule, each phrased a little differently, each answered with a new byte
	// count. Neither name+input nor name+output ever matched.
	askedBefore := map[string][]string{}

	// Track whether any file-mutating tool succeeded THIS turn, shared with the
	// todo_write special-case via ctx: a checklist item flipped to completed in a
	// turn that changed no file is status-theater (boxes ticked, work not done)
	// and gets an honest note appended to the tool result.
	turnFileMutated := false
	ctx = context.WithValue(ctx, turnMutatedKey{}, &turnFileMutated)

	// Files READ or WRITTEN this turn (base names). apply_patch uses this to refuse
	// a blind whole-file rewrite: overwriting an existing file the model has NOT
	// read (or just created) this turn is a rewrite-from-MEMORY — where a model drifts
	// details (arguments, values) it didn't just see.
	turnReads := map[string]bool{}
	ctx = context.WithValue(ctx, turnReadsKey{}, turnReads)

	// Shadow routing: the turn is scored for steps without progress and
	// what an escalation rule would have done is logged; nothing changes
	// (route_shadow.go).
	shadow := newRouteShadow(ctx, agentName)
	defer func() { shadow.finish(response) }()

	for {
		if message != nil {
			turnTokens += message.Usage.InputTokens + message.Usage.OutputTokens
		}
		var toolResults []llm.ContentBlockParamUnion
		var hasToolUse bool
		var stuck bool // the same tool call has failed maxRepeatedFail times
		var responseText string
		callsThisMessage := 0
		// The same call several times in ONE reply is a decoding artifact —
		// the 27B emitted one call three times over after a long
		// generation (2026-09-13 16:38) — not a loop. It runs once; every
		// copy gets the one result, and none of them count as a repeat.
		seenCalls := map[string]ToolExecutionInfo{}

		// Process all content blocks
		for _, content := range message.Content {
			switch content.Type {
			case "text":
				responseText += content.Text

			case "tool_use":
				hasToolUse = true
				toolUse := content.AsToolUse()
				if prev, dup := seenCalls[llm.ToolCallKey(toolUse.Name, toolUse.Input)]; dup {
					log.Info("duplicate tool call in one reply collapsed", slog.String("tool", toolUse.Name))
					toolResults = append(toolResults, llm.NewToolResultBlock(toolUse.ID, firstNonEmptyString(prev.Output, prev.Error), prev.Error != ""))
					continue
				}
				callsThisMessage++

				// ABANDON THE REST OF A MESSAGE THE MODEL PRODUCED WHILE FLAILING.
				//
				// Every limit above bounds ITERATIONS round the outer loop, and
				// `stuck` is only read after this inner
				// loop has finished. Nothing bounded the number of tool_use blocks
				// inside ONE assistant message, and a degenerate generation is
				// mostly tool calls.
				//
				// Measured 2026-08-30 on a 27B: asked to fix a file that did
				// not exist (the harness had misnamed the fixture), the model
				// emitted a SINGLE message carrying 583 read_file blocks. `stuck`
				// was set on the 4th and 579 more ran anyway, because it is not
				// checked until every block is processed: the whole burn was one
				// iteration. Healthy turns in the
				// same session carried 2 and 8 blocks.
				//
				// Both conditions must still emit a tool_result: an unanswered
				// tool_use is a malformed conversation, and the next inference
				// rejects it.
				if why, skip := abandonRest(stuck, callsThisMessage, maxCallsPerMessage); skip {
					toolResults = append(toolResults, llm.NewToolResultBlock(toolUse.ID, why, true))
					abandoned++
					continue
				}

				// Promoted from Debug to Info so tool calls show up in
				// normal log output — when a heartbeat fires we want to see
				// exactly which tools it ran (and their inputs) without
				// having to flip a verbose flag.
				// The tool name is embedded in the message string so it
				// shows up in `memdoor logs query` output, which doesn't
				// render structured slog attrs.
				log.Info(fmt.Sprintf("Tool call started: %s", toolUse.Name),
					slog.String("tool", toolUse.Name),
					slog.String("input", truncateForLog(string(toolUse.Input), 500)))

				// Emit tool start event
				// "id" is what lets a RESULT find the frame that asked for it.
				// Without it the client matches on tool NAME, and three bash
				// calls in one reply are indistinguishable — measured
				// 2026-08-30: three parallel bash frames each rendered another
				// call's output, so `Bash(ls -la …)` displayed "bash: python:
				// command not found".
				ar.events.EmitEvent(runID, infra.EventStreamTool, session.ID, map[string]interface{}{
					"event": "start",
					"id":    toolUse.ID,
					"tool":  toolUse.Name,
					"input": string(toolUse.Input),
				})

				// Track tool start in execution flow
				ar.flowTracker.RecordToolStart(runID, toolUse.Name, len(response.ToolsExecuted)+1, string(toolUse.Input))

				// A long tool is silent between "start" and "complete", which
				// on screen is indistinguishable from a hang. Beat while it
				// runs (agent_runtime_progress.go).
				stopProgress := ar.watchToolProgress(session, runID, toolUse.ID, toolUse.Name)

				// A tool retired this turn is refused without running. See
				// maxFailsPerTool above: the reason it kept failing (no workspace,
				// missing binary, unreachable service) does not change mid-turn, so
				// the only thing another attempt buys is another round trip.
				var toolInfo ToolExecutionInfo
				var toolResult llm.ContentBlockParamUnion
				failKey := failureKey(toolUse.Name, toolUse.Input)
				refused := failsByTool.retired(failKey, maxFailsPerTool)
				if refused {
					toolInfo, toolResult = retiredToolResult(toolUse.Name, failKey, string(toolUse.Input),
						toolUse.ID, failsByTool.count(failKey), failsByTool.reason(failKey))
					log.Warn("Tool retired for this turn after repeated failures",
						slog.String("tool", failKey),
						slog.Int("failures", failsByTool.count(failKey)))
				} else {
					toolInfo, toolResult = ar.executeTool(ctx, &toolUse, runID, session)
				}
				stopProgress()
				// REPEATED NO-OP READS: a model can spin on a SUCCESSFUL read — live:
				// six identical todo_read calls in one turn, no work between. The
				// failure loop-breaker never trips (they succeed), so the RESULT
				// says it: keyed on name+input+OUTPUT, a re-run whose output
				// changed (go build after a patch) is never flagged.
				if toolInfo.Error == "" {
					okKey := toolUse.Name + "\x00" + string(toolUse.Input) + "\x00" + toolInfo.Output
					repeatedOK[okKey]++

					// SAME RESULT, DIFFERENT WORDING, IS STILL NO PROGRESS.
					//
					// The key above includes the INPUT, so a model that rephrases
					// the command each time never trips it. Measured 2026-08-30:
					// EIGHTEEN bash calls in one turn re-fetching one URL —
					// python3 -c, then curl -s, then curl -sS, then
					// ls && curl — every one returning the same 117,729 bytes,
					// and the guard silent because no two inputs matched.
					//
					// What says "no progress" is the RESULT. Keyed on name+output
					// alone, verifying the same fact a third way is caught. A
					// re-run whose output CHANGED (go build after a patch) still
					// never matches, which is the property that makes this safe.
					sameResult := toolUse.Name + "\x00" + toolInfo.Output
					repeatedOK[sameResult]++
					if n := repeatedOK[sameResult]; n >= 3 && repeatedOK[okKey] < 3 {
						toolInfo.Output += fmt.Sprintf("\n\n(you have run %s %d times this turn and got THIS EXACT result every time, "+
							"with different commands. The answer is not going to change — stop verifying it and do the next real step)", toolUse.Name, n)
						toolResult = llm.NewToolResultBlock(toolUse.ID, toolInfo.Output, false)
					}

					if n := repeatedOK[okKey]; n >= 2 && toolUse.Name == "skill" {
						// A re-loaded skill re-injects its FULL text — context bloat
						// that fuels the very loop it rides (live: three identical
						// `skill new-program` loads in 21s). From the second load on,
						// the result is a short redirect, not the content again.
						toolInfo.Output = "You ALREADY loaded this skill this turn — its steps are above. Do NOT load it again; execute step 1 now with a concrete tool call (apply_patch / bash)."
						toolResult = llm.NewToolResultBlock(toolUse.ID, toolInfo.Output, false)
					} else if n >= 3 {
						toolInfo.Output += fmt.Sprintf("\n\n(repeat #%d — this exact call already returned this exact result this turn. STOP re-reading; take the next concrete action now)", n)
						toolResult = llm.NewToolResultBlock(toolUse.ID, toolInfo.Output, false)
					}
					// STRUCTURAL, not advisory. The note above was ignored 22 times
					// in a row (ffprobe | grep title, 2026-09-02 morning),
					// and a 27B alternated preview/todo_write for 50 rounds the same
					// evening because the todo tool kept telling it to "verify".
					// A fourth identical call, or a fifth identical result, is not
					// work — the turn ends here, the way repeated failures end it.
					if repeatedOK[okKey] >= 4 || repeatedOK[sameResult] >= 5 {
						stuck = true
					}
				}
				// THE SAME ASK, REWORDED, IS THE SAME ASK. Three near-identical
				// inputs to one tool get told so; six end the turn — a model
				// that has asked the same thing six ways is not working.
				if n := nearSameAsks(askedBefore[toolUse.Name], string(toolUse.Input)); n >= 3 {
					toolInfo.Output += fmt.Sprintf("\n\n(this is the %s time you have made nearly this same %s call this turn — "+
						"the earlier answers are above. Do not ask it again; do the next real step)", ordinal(n+1), toolUse.Name)
					toolResult = llm.NewToolResultBlock(toolUse.ID, toolInfo.Output, toolInfo.Error != "")
					if n >= 6 {
						stuck = true
					}
				}
				askedBefore[toolUse.Name] = append(askedBefore[toolUse.Name], string(toolUse.Input))
				response.ToolsExecuted = append(response.ToolsExecuted, toolInfo)
				toolResults = append(toolResults, ar.capToolResult(toolResult))
				seenCalls[llm.ToolCallKey(toolUse.Name, toolUse.Input)] = toolInfo
				shadow.observe(toolUse.Name, string(toolUse.Input), toolInfo)

				// Track tool complete in execution flow
				ar.flowTracker.RecordToolComplete(runID, toolUse.Name, len(response.ToolsExecuted), toolInfo.Output, toolInfo.Error)

				// Companion to the start log above — outcome (ok / error)
				// + truncated output, so a single grep finds the full
				// "agent X called tool Y with Z, got W" trace. Tool name
				// is in the message string for log-query visibility.
				if toolInfo.Error != "" {
					// Count identical failing calls (name+input): trip when the SAME one has
					// failed maxRepeatedFail times. ALSO count consecutive failures across
					// any tools (flailing with different malformed inputs), tripping at
					// maxConsecutiveFail.
					fk := toolUse.Name + "\x00" + string(toolUse.Input)
					repeatedFail[fk]++
					consecutiveFail++
					// A REFUSAL IS NOT A NEW FAILURE. Counting it made the
					// number climb with every repeat ("failed 5 times",
					// "6 times") as if the tool were breaking again, when
					// nothing had run at all.
					if !refused {
						failsByTool.failed(failKey, toolInfo.Error)
					}
					if repeatedFail[fk] >= maxRepeatedFail || consecutiveFail >= maxConsecutiveFail {
						stuck = true
					}
					log.Warn(fmt.Sprintf("Tool call failed: %s — %s", toolUse.Name, truncateForLog(toolInfo.Error, 200)),
						slog.String("tool", toolUse.Name),
						slog.String("error", truncateForLog(toolInfo.Error, 500)))
				} else {
					// A successful tool call is progress — reset the flailing counter so a
					// normal fix loop (patch fails, read, patch works) isn't cut short.
					// The same evidence clears THIS tool's own failures: it has just
					// proved it can work, so its earlier failures must not retire it.
					consecutiveFail = 0
					failsByTool.succeeded(failKey)
					switch toolUse.Name {
					case "apply_patch", "write_file", "edit_file", "search_replace":
						turnFileMutated = true
					}
					recordTurnReads(turnReads, toolUse.Name, toolUse.Input, toolInfo.Output)
					log.Info(fmt.Sprintf("Tool call completed: %s", toolUse.Name),
						slog.String("tool", toolUse.Name),
						slog.String("output", truncateForLog(toolInfo.Output, 500)))
				}

				// Emit tool complete event
				// Full output sent - Gorilla WebSocket handles compression automatically for large messages
				ar.events.EmitEvent(runID, infra.EventStreamTool, session.ID, map[string]interface{}{
					"event":  "complete",
					"id":     toolUse.ID,
					"tool":   toolUse.Name,
					"output": toolInfo.Output,
					"error":  toolInfo.Error,
				})
			}
		}

		// SHOW THE REASONING. The "thinking" event above is only a spinner
		// trigger — it carries no content — and stripThink removed the reasoning
		// before anything could display it, so a reasoning model's work was
		// invisible in the TUI. It is emitted, never persisted: ToParam drops it,
		// because the model family's own chat template does not replay prior-turn
		// thinking and keeping it would bloat every later request.
		if message != nil && strings.TrimSpace(message.Thinking) != "" {
			ar.events.EmitEvent(runID, infra.EventStreamAssistant, session.ID, map[string]interface{}{
				"event": "thinking",
				"text":  message.Thinking,
			})
		}

		// Store text response
		if responseText != "" {
			if response.Text != "" {
				response.Text += "\n"
			}
			response.Text += responseText

			// Emit text event (OpenClaw pattern)
			ar.events.EmitEvent(runID, infra.EventStreamAssistant, session.ID, map[string]interface{}{
				"event": "text",
				"text":  responseText,
			})
			if streamed != "" {
				streamed += "\n"
			}
			streamed += responseText
		}

		// TRUNCATED: do not end the turn on an unfinished reply. The POLICY —
		// escalate once, then resume a bounded number of times, then give up —
		// lives in TruncationRecovery; this loop only carries it out.
		if message != nil && message.StopReason == llm.StopReasonMaxTokens {
			shadow.observeTruncation()
			step := recovery.Next()
			switch step.Kind {
			case RecoveryEscalate:
				log.Warn("Reply hit the output cap — retrying once with a larger one",
					slog.Int64("escalated_to", step.MaxTokens))
				conversation = conversation[:len(conversation)-1] // drop the cut-off reply
				escCtx := withMaxOutputOverride(ctx, step.MaxTokens)
				message, conversation, err = ar.runInference(escCtx, conversation, extraSystemPrompt)
			case RecoveryResume:
				log.Warn("Reply truncated again — resuming mid-thought",
					slog.Int("attempt", step.Attempt))
				conversation = append(conversation, llm.NewUserMessage(
					llm.ContentBlockParamUnion{OfText: &llm.TextBlockParam{Text: step.Prompt}}))
				message, conversation, err = ar.runInference(ctx, conversation, extraSystemPrompt)
			default: // RecoveryExhausted
				log.Warn("Truncation recovery exhausted")
				if response.Text != "" {
					response.Text += "\n\n"
				}
				response.Text += "Stopped: the reply kept hitting the output limit. The work is incomplete — ask for a smaller piece of it."
			}
			if step.Kind == RecoveryExhausted {
				break
			}
			if err != nil {
				log.WithError(err).Error("Truncation recovery inference failed")
				break
			}
			conversation = append(conversation, message.ToParam())
			continue
		}

		// If no tool use, we're done. Verification discipline (build/run before
		// reporting done) lives in the coder's skill, not in an injected per-turn
		// prompt here — reinjecting directives degrades a small model.
		if !hasToolUse {
			// EMPTY-STEP RETRY (codebase agents): a model sometimes emits a bare
			// EOS mid-task — no text, no tool call — right after a successful tool
			// step (live: read_file succeeded, next step empty, task abandoned with
			// main.go never written); a bare-JSON reply the parser scrubbed
			// arrives the same way. An empty step is a sampling artifact
			// or a format slip, not a decision: retry the SAME inference once
			// with nothing injected, whether or not a tool ran this turn
			// (empty_reply.go). A chat agent with no filesystem palette is
			// excluded — its prose IS the answer.
			if emptyReplyRetryWanted(responseText, emptyStepRetried, ar.agentWorksOnCodebase(ctx)) {
				emptyStepRetried = true
				log.Warn("Empty completion mid-task — retrying inference once",
					slog.Int("tools_executed", len(response.ToolsExecuted)))
				conversation = conversation[:len(conversation)-1] // drop the empty assistant msg
				retryCtx := context.WithValue(ctx, retryTemperatureKey{}, 0.7)
				retryPrompt := extraSystemPrompt
				if message != nil {
					retryPrompt = emptyRetryPrompt(extraSystemPrompt, message.StopReason)
				}
				message, conversation, err = ar.runInference(retryCtx, conversation, retryPrompt)
				if err != nil {
					log.WithError(err).Error("Empty-step retry inference failed")
					break
				}
				conversation = append(conversation, message.ToParam())
				continue
			}
			// The retry came back empty too. The window gets a sentence and
			// the session keeps that sentence in place of the blank, so the
			// next reply has nothing empty to imitate.
			if strings.TrimSpace(responseText) == "" && emptyStepRetried && ar.agentWorksOnCodebase(ctx) {
				// The same model blank twice is that model's answer for this
				// prompt; the ladder has a next rung for exactly this. One
				// more try there, then the notice. Live 2026-10-07: the
				// first rung wrote a workflow step's start stamp, answered
				// nothing twice, and the turn ended with the target missing
				// — a run a person had scheduled for the night, failed on
				// its first step with two rungs never asked.
				if agent, _ := ctx.Value("buddy_agent_name").(string); !emptyStepClimbed {
					if rung, model := nextRungAfterEmpty(ctx, agent); model != "" {
						emptyStepClimbed = true
						log.Warn("Empty completion twice — climbing the ladder for one more try",
							slog.Int("rung", rung+1), slog.String("model", model),
							slog.Int("tools_executed", len(response.ToolsExecuted)))
						conversation = conversation[:len(conversation)-1]
						climbCtx := context.WithValue(context.WithValue(ctx, retryTemperatureKey{}, 0.7), sharedctx.TierKey, rung)
						message, conversation, err = ar.runInference(climbCtx, conversation, extraSystemPrompt)
						if err != nil {
							log.WithError(err).Error("Empty-step climb inference failed")
							break
						}
						conversation = append(conversation, message.ToParam())
						continue
					}
				}
				log.Warn("Empty completion twice — ending the turn with a visible notice",
					slog.Int("tools_executed", len(response.ToolsExecuted)))
				response.Text = emptyReplyNotice
				conversation[len(conversation)-1] = llm.NewAssistantMessage(llm.NewTextBlock(emptyReplyNotice))
				break
			}

			// REASONING-ONLY RETRY: the model thought, said what it was about to
			// do, and stopped without doing it. Same class as the empty step —
			// a sampling artifact rather than a decision — but it arrives with
			// TEXT and with no tool executed yet, so the branch above cannot see
			// it. Retry the same inference once; nothing is injected, because
			// re-injecting directives degrades a small model.
			// NOT conditioned on how many tools already ran. The first version of
			// this required ToolsExecuted == 0 and therefore missed the real
			// failure entirely: measured 2026-08-30, the model called read_file
			// successfully, then produced 9,234 characters of reasoning ending
			// "Then run `go run stats.go` with bash.</think>" — and stopped
			// without emitting the call it had just planned. One tool had run, so
			// the guard skipped it and the turn was abandoned.
			guardStopped := message != nil && message.StopReason == llm.StopReasonStreamGuard
			if guardStopped {
				log.Warn("Prose breaker stopped the reply — tag-free past the threshold with no tool call; retrying",
					slog.Int("chars", len(responseText)))
			}
			announced := !reasoningOnlyRetried && !guardStopped && ar.agentWorksOnCodebase(ctx) &&
				(reasoningOnlyReply(responseText) || ar.turnVerdict.announcesUndone(ctx, agentName, userMessage, responseText))
			if announced {
				shadow.observeAnnounced()
			}
			if !reasoningOnlyRetried && (announced || guardStopped) && ar.agentWorksOnCodebase(ctx) {
				reasoningOnlyRetried = true
				log.Warn("Reasoning-only reply with no tool call — retrying inference once",
					slog.String("reply", truncateForLog(responseText, 200)))
				conversation = conversation[:len(conversation)-1]
				// The person keeps what already streamed: a finished answer
				// misread as an announcement (a review ending on its "next
				// action") was wiped from the reply, and the window replaced it
				// with the receipt line (live 2026-10-04, "it cuts the window
				// again"). Raw <think> reasoning is the one thing dropped.
				shown := responseText
				if strings.Contains(responseText, "<think>") || strings.Contains(responseText, "</think>") {
					response.Text = strings.TrimSpace(strings.TrimSuffix(response.Text, responseText))
					shown = ""
				}
				// Words did not move the 27B (2026-09-13: two 25-minute
				// monologues on one brief, nudge and handback included), nor
				// GLM 5.3 Flash on the coder (2026-09-29: "I'll find which
				// commands were recently added" twice, then the turn ended).
				// The retry's reply opens on a tool call.
				retryCtx := llm.WithForcedToolCall(context.WithValue(ctx, retryTemperatureKey{}, 0.7))
				// The retry says WHY it is a retry. A hotter temperature and
				// nothing else got the same deliberation again on the 27B
				// (2026-09-07: the cut card, then four minutes weighing file
				// names, to the 16k cap); one sentence at the top of the
				// system prompt is what turns the next reply into a call.
				nudge := "Your previous reply ran long without acting. Call the next tool now; keep any explanation to one sentence."
				if extraSystemPrompt != "" {
					nudge = extraSystemPrompt + "\n\n" + nudge
				}
				message, conversation, err = ar.runInference(retryCtx, conversation, nudge)
				if err != nil {
					log.WithError(err).Error("Reasoning-only retry inference failed")
					break
				}
				conversation = append(conversation, withLeadingText(message.ToParam(), shown))
				continue
			}
			// A RECAP AFTER A LOT OF WORK (recap.go): a long turn that ends on a
			// line is asked, once, to end on what was made and what is next.
			if !recapRetried && recapWanted(len(response.ToolsExecuted), response.Text) && ar.agentWorksOnCodebase(ctx) {
				recapRetried = true
				log.Warn("Long turn ended on a line — asking for the recap once",
					slog.Int("tools_executed", len(response.ToolsExecuted)), slog.Int("chars", len(response.Text)))
				conversation = conversation[:len(conversation)-1]
				response.Text = strings.TrimSpace(strings.TrimSuffix(response.Text, responseText))
				nudge := recapNudge(len(response.ToolsExecuted))
				if extraSystemPrompt != "" {
					nudge = extraSystemPrompt + "\n\n" + nudge
				}
				message, conversation, err = ar.runInference(ctx, conversation, nudge)
				if err != nil {
					log.WithError(err).Error("Recap retry inference failed")
					break
				}
				conversation = append(conversation, message.ToParam())
				continue
			}
			// DONE MEANS THE RECEIPTS SHOW IT (turn_done.go). Once per turn, the
			// decision model reads what the turn actually did — files changed,
			// checks run and what they returned — against the request and the
			// final message. Unfinished sends the model back once, with the
			// receipts' own reasons at the top of its prompt.
			// With a budget (turn_token_budget) it keeps going, its own "next"
			// as the prompt, while unfinished and the budget is not spent.
			if ar.agentWorksOnCodebase(ctx) && len(response.ToolsExecuted) > 0 {
				receipts := ar.turnVerdict.judgeExitZeroChecks(ctx, receiptsOf(response.ToolsExecuted))
				// A DECLARED DONE (a workflow task's target) is checked, not
				// judged: the artifact is there or it is not.
				declaredTarget, _ := session.GetMetadataValue(doneWhenKey)
				label, exists, declared := doneWhen(ctx, turnWorkdir(ctx), fmt.Sprint(declaredTarget))
				var p float64
				retry := false
				if declared {
					retry = !exists
					log.Info("Turn done-when", slog.String("target", label), slog.Bool("exists", exists))
				} else {
					p, retry = ar.turnVerdict.unfinished(ctx, agentName, userMessage, responseText, receipts)
				}
				if retry {
					if !continueAllowed(doneRounds, turnTokens-doneFrom, turnBudget) {
						log.Warn("Turn still unfinished, budget spent", slog.Int("rounds", doneRounds), slog.Int64("spent", turnTokens-doneFrom), slog.Int64("budget", turnBudget))
						response.Text += fmt.Sprintf("\n\n⚠ Not shown done after %d more round%s and %s tokens (turn_token_budget %s). Say continue, or what to do instead.",
							doneRounds, plural(doneRounds), approxTokens(turnTokens-doneFrom), approxTokens(turnBudget))
						break
					}
					if doneRounds == 0 {
						doneFrom = turnTokens
					}
					doneRounds++
					log.Warn("Turn ended unfinished by its receipts — continuing",
						slog.Int("round", doneRounds), slog.Float64("p_unfinished", p), slog.Int("changed", len(receipts.changed)), slog.Int("checks", len(receipts.checks)))
					ar.decisionNote(runID, session.ID, fmt.Sprintf("not shown done by the receipts (P=%.2f) — continuing, round %d", p, doneRounds))
					// The model continues without its premature conclusion, but
					// the person keeps it: it already streamed to the window, and
					// cutting it from the reply made the final text stop starting
					// with what streamed, so the window replaced a whole review
					// with the receipt line (live 2026-10-04, "it cuts the
					// window again").
					conversation = conversation[:len(conversation)-1]
					shown := responseText
					nudge := continueNudge(responseText, receipts)
					if declared {
						nudge = doneWhenNudge(label)
						ar.decisionNote(runID, session.ID, "the "+label+" is missing — continuing, round "+strconv.Itoa(doneRounds))
					}
					if extraSystemPrompt != "" {
						nudge = extraSystemPrompt + "\n\n" + nudge
					}
					message, conversation, err = ar.runInference(llm.WithForcedToolCall(ctx), conversation, nudge)
					if err != nil {
						log.WithError(err).Error("Done retry inference failed")
						break
					}
					conversation = append(conversation, withLeadingText(message.ToParam(), shown))
					continue
				}
			}
			break
		}

		// Abandoning calls is never silent: it means the model produced a reply
		// that was mostly degenerate tool calls, which is worth seeing.
		if abandoned > 0 {
			log.Warn("Abandoned tool calls in one reply",
				slog.Int("abandoned", abandoned),
				slog.Int("calls_in_message", callsThisMessage),
				slog.Bool("after_loop_breaker", stuck))
			abandoned = 0
		}

		// Loop-breaker tripped: either the same tool call failed maxRepeatedFail times, or
		// tool calls failed maxConsecutiveFail times in a row with no progress. Stop the
		// turn rather than spin — the error text isn't moving the model. End with an honest
		// note so the failure is visible (and the outer flow / user can react) instead of a
		// silent 50-call burn.
		if stuck && !loopNudged {
			loopNudged = true
			log.Warn("Loop-breaker: repeated calls or consecutive failures — nudging once",
				slog.Int("tools_executed", len(response.ToolsExecuted)),
				slog.Int("consecutive_fail", consecutiveFail))
			toolResults = append(toolResults, llm.NewTextBlock(loopNudge(response.ToolsExecuted)))
		} else if stuck {
			log.Warn("Loop-breaker: repeated calls or consecutive failures — ending turn",
				slog.Int("tools_executed", len(response.ToolsExecuted)),
				slog.Int("consecutive_fail", consecutiveFail))
			if response.Text != "" {
				response.Text += "\n\n"
			}
			// THE END SAYS WHY. "Repeated tool failures aren't converging —
			// the change could not be applied" was the coder's sentence and
			// it named nothing: the editor's turn ended on three refused
			// transcript reads and the person saw a line about a change
			// (2026-09-18 19:17, "i cant see the end"). The last failure is
			// the reason; put it in the window.
			response.Text += "Stopped: the same tool kept failing the same way" + lastToolFailure(response.ToolsExecuted) +
				". Say what to do differently, or name the exact file and lines."
			break
		}

		// The context gauge is what tells the user how close this turn is to
		// compaction. Emitting only at the start and end of a run means it sits
		// still through the tool loop — which is exactly the stretch that grows
		// the conversation, and exactly when someone watches it.
		ar.emitContextUpdate(runID, session.ID, conversation)

		// Check API call limit to prevent infinite retry loops
		apiCallCount++
		// THE DECISION MODEL ENDS A RUN THAT IS GOING NOWHERE. The first time
		// the shadow score says the last steps made no progress, one yes/no
		// over those steps asks whether the run is stuck; a yes ends the turn
		// with a wrap-up round. No count does this (Greg, 2026-09-26).
		if shadow.justFired() {
			if p, stop := ar.turnVerdict.stuck(ctx, agentName, userMessage, shadow.windowText()); stop {
				log.Warn("Turn stopped: no progress", slog.Float64("p_stuck", p), slog.Int("calls", apiCallCount))
				savings.Record(savings.Entry{Kind: savings.KindEarlyStop}) // counted, never priced

				conversation, streamed = ar.wrapUpTurn(ctx, conversation, toolResults, stuckWrapUp, extraSystemPrompt, response, streamed)
				response.Text += fmt.Sprintf("⚠️ Stopped: the last steps made no progress (decision model, P=%.2f). Say what to try instead, or ask to continue.", p)
				break
			}
		}
		// A turn that only reads never trips the score above. Every 30 reads
		// with no edit, the decision model is asked whether it should stop
		// exploring and say what it would change (turn_verdict.go exploring).
		if reads, edited, due := shadow.exploreDue(); due {
			if p, stop := ar.turnVerdict.exploring(ctx, agentName, userMessage, shadow.windowText(), reads, edited); stop && !exploreNudged {
				exploreNudged = true
				log.Warn("Exploring without a change — nudging once", slog.Float64("p", p), slog.Int("reads", reads))
				ar.decisionNote(runID, session.ID, fmt.Sprintf("%d reads and no change (P=%.2f) — told to stop exploring", reads, p))
				toolResults = append(toolResults, llm.NewTextBlock(exploreNudge))
			} else if stop {
				log.Warn("Turn stopped: exploring without a change", slog.Float64("p", p), slog.Int("reads", reads))
				savings.Record(savings.Entry{Kind: savings.KindEarlyStop}) // counted, never priced

				conversation, streamed = ar.wrapUpTurn(ctx, conversation, toolResults, exploreWrapUp, extraSystemPrompt, response, streamed)
				since := "and no change yet"
				if edited {
					since = "since the last change, going in circles"
				}
				response.Text += fmt.Sprintf("⚠️ Stopped: %d reads %s (decision model, P=%.2f). Say go to make the change above, or say what to do instead.", reads, since, p)
				break
			}
		}
		// Send tool results back
		toolResultMessage := llm.NewUserMessage(toolResults...)
		conversation = append(conversation, toolResultMessage)

		// Save conversation with tool results BEFORE making API call
		// This ensures tool_result blocks are persisted even if the API call fails
		// Prevents orphaned tool_use blocks when rate limits or other errors occur
		ar.updateSession(session, conversation)

		// Get followup response. Adopt the returned conversation: runInference may
		// have compacted it in place, and the loop must continue from the compacted
		// form (else the pruned history grows back every turn). The followup is a
		// plain inference — the coder's workflow (keep acting until it builds) lives
		// in its skill, not in adapter-level retry forcing.
		message, conversation, err = ar.runInference(ctx, conversation, extraSystemPrompt)
		if err != nil {
			log.WithError(err).Error("Followup inference failed")
			response.Error = err.Error()
			// Session already saved with tool results above, safe to return
			return response, err
		}
		conversation = append(conversation, message.ToParam())
	}

	// Update session with conversation
	ar.updateSession(session, conversation)

	// Complete execution flow tracking
	ar.flowTracker.Complete(runID)

	// The model that answered, on the response and on the complete event,
	// so the screen can say so.
	if message != nil && message.Model != "" {
		response.Model = string(message.Model)
	}

	// THE NOTES AND THE RECEIPT GO OUT BEFORE "COMPLETE": the window closes
	// the message on the complete event, so a text event after it was never
	// shown (live 2026-10-03: the receipt line reached the log and not the
	// screen).
	// HONESTY NOTE (mutation claim): a small model that cannot land an edit
	// sometimes narrates success anyway — "the file has been updated" — with ZERO
	// mutating tool calls in the turn (seen live: claimed a roof was added while
	// showing the unchanged code). Mechanical check, same class as the citation
	// guard: if a codebase agent's reply claims a change but no file-mutating tool
	// succeeded this turn, append an honest note so a false "done" can't pass
	// silently. No inference, no retry, no gating — the text stays, flagged.
	if ar.agentWorksOnCodebase(ctx) && strings.TrimSpace(response.Text) != "" {
		switch {
		case len(response.ToolsExecuted) == 0:
			// ZERO-TOOL TURN: a coding agent that ran no tools read nothing, wrote
			// nothing, and verified nothing — whatever the reply shows (even a bare
			// "ALPHA2" mimicking expected output) is unbacked. Subsumes the two
			// checks below for this case.
			response.Text += "\n\n(note: this turn used no tools — nothing was read, written, or verified)"
		case claimsFileChange(response.Text) && !fileMutationSucceeded(response.ToolsExecuted):
			response.Text += "\n\n(note: no file was actually modified in this turn — the change described above was NOT applied)"
		case containsNarratedCode(response.Text) && !fileMutationSucceeded(response.ToolsExecuted):
			// NARRATED CODE: the program is SHOWN (fenced block or pasted diff
			// lines) instead of written — it exists nowhere on disk.
			response.Text += "\n\n(note: the code above was only DISPLAYED — nothing was written to disk. It must be written with apply_patch.)"
		}
	}

	// THE RECEIPT LINE: what this turn changed and what it checked, from the
	// tool records, under every codebase turn that did either (turn_done.go).
	if ar.agentWorksOnCodebase(ctx) && len(response.ToolsExecuted) > 0 {
		line := ar.turnVerdict.judgeExitZeroChecks(ctx, receiptsOf(response.ToolsExecuted)).line()
		if declaredTarget, ok := session.GetMetadataValue(doneWhenKey); ok {
			if label, exists, declared := doneWhen(ctx, turnWorkdir(ctx), fmt.Sprint(declaredTarget)); declared {
				present, absent := " exists", " missing"
				if strings.HasPrefix(label, "target `") { // a command target passes or fails
					present, absent = " passes", " fails"
				}
				mark := "✓ " + label + present
				if !exists {
					mark = "⚠ " + label + absent
				}
				if line != "" {
					rest := strings.TrimLeft(strings.TrimLeft(line, "✓⚠"), " ")
					// A command can write the target (a heredoc, a script):
					// the receipts cannot count that as a change, the target
					// check proves it, so "nothing changed" would contradict it.
					rest = strings.TrimPrefix(rest, "nothing changed · ")
					// The target's state decides a declared task; "checked:" or
					// "unverified:" beside "✓ target exists" read as a
					// contradiction (live 2026-10-03). A failing check stays.
					rest = strings.TrimPrefix(strings.TrimPrefix(rest, "checked: "), "unverified: ")
					line = mark + " · " + rest
				} else {
					line = mark
				}
			}
		}
		if line != "" {
			response.Text = strings.TrimRight(response.Text, "\n") + "\n\n" + line
		}
	}

	// What the rounds did not stream — a notice, a stop, a note — is sent
	// as one more text event, or the window never shows it (unstreamed.go).
	if tail, isTail := unstreamedTail(response.Text, streamed); tail != "" {
		ar.events.EmitEvent(runID, infra.EventStreamAssistant, session.ID, map[string]interface{}{
			"event":  "text",
			"text":   tail,
			"append": isTail,
		})
	}

	// Emit lifecycle complete event
	ar.events.EmitEvent(runID, infra.EventStreamLifecycle, session.ID, map[string]interface{}{
		"event":          "complete",
		"success":        true,
		"tools_executed": len(response.ToolsExecuted),
		"model":          response.Model,
	})

	// Emit final context update (for TUI context bar display)
	ar.emitContextUpdate(runID, session.ID, conversation)

	// Clean up run tracking (but keep flow for potential visualization)
	ar.events.ClearRun(runID)

	// LEARNING: Post-execution reflection for learning agents
	if isLearning, ok := ctx.Value(sharedctx.BuddyLearningKey).(bool); ok && isLearning && agentName != "" {
		count := ar.incrementInteractionCount(agentName)
		if ar.shouldReflect(count, userMessage, response) {
			go ar.triggerReflection(agentName, userMessage, response)
		}
	}

	log.Debug("Message processed successfully", slog.Int("tools_executed", len(response.ToolsExecuted)))

	return response, nil
}

func firstNonEmptyString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// wrapUpTurn ends a turn that must stop mid-work the way a person needs it
// ended: the last tool results and a brief go into the conversation, then
// ONE round with no tools asks for what was built and verified, how to run
// it and what is left — streamed like any answer. It returns the extended
// conversation and streamed text; the caller adds its notice after.
func (ar *AgentRuntime) wrapUpTurn(ctx context.Context, conversation []llm.MessageParam, toolResults []llm.ContentBlockParamUnion,
	brief, extraSystemPrompt string, response *AgentResponse, streamed string) ([]llm.MessageParam, string) {
	if len(toolResults) > 0 {
		conversation = append(conversation, llm.NewUserMessage(toolResults...))
	}
	conversation = append(conversation, llm.NewUserMessage(llm.NewTextBlock(brief)))
	wrap, _, err := ar.runInference(withTurnToolsAllow(ctx, nil), conversation, extraSystemPrompt)
	if err != nil || wrap == nil {
		if err != nil {
			logs.New("Agent").Warn("Wrap-up round failed", slog.String("error", err.Error()))
		}
	} else {
		var text string
		for _, c := range wrap.Content {
			if c.Type == "text" {
				text += c.Text
			}
		}
		if strings.TrimSpace(text) != "" {
			if response.Text != "" {
				response.Text += "\n\n"
			}
			response.Text += strings.TrimSpace(text)
			streamed += text
			conversation = append(conversation, wrap.ToParam())
		}
		if wrap.Model != "" {
			response.Model = string(wrap.Model)
		}
	}
	if response.Text != "" {
		response.Text += "\n\n"
	}
	return conversation, streamed
}

// loopNudge is the first word to a turn repeating a failing call: the
// failure, and that repeating it is over. It rides in the model's next input
// beside the tool results.
func loopNudge(executed []ToolExecutionInfo) string {
	return "⚠ The same tool call keeps failing the same way" + lastToolFailure(executed) +
		". Do not repeat it. Change your approach, or tell the person what is blocking you and end your turn."
}

// exploreNudge is the first word to a turn that reads and reads: make the
// change, or say what it would be, and stop reading.
const exploreNudge = "⚠ You have read and searched for many steps without changing anything. Stop exploring now: " +
	"make the change you have in mind, or tell the person what you found and exactly what you would change, and end your turn. Do not read more."

// stuckWrapUp is the brief for a run the decision model judged stuck.
const stuckWrapUp = "Your last steps repeated themselves or failed the same way without progress, so this turn stops here. " +
	"Make no tool call. In a few lines tell the person: what is done and verified (files, commands you ran), " +
	"what you were trying that did not work and why you think it failed, and what you would try next. Claim nothing you did not verify."

// exploreWrapUp ends a turn that read and read and changed nothing: what it
// found, and the change it would make, so the person can say go.
const exploreWrapUp = "You have read and searched for many steps without making a change, so this turn stops here. " +
	"Make no tool call. In a few lines tell the person: what you found (the files and lines that matter), " +
	"and exactly the change you would make next, file by file. Claim nothing you did not verify."

// withLeadingText puts the text a turn already showed at the head of the
// assistant message that follows its retry. The retry runs without that text
// (it must not anchor the next step), but the conversation keeps it: dropped,
// the person's next turn asked about an answer the model no longer had
// (2026-10-04). One assistant message — text, then the call — is valid for
// every provider, where two in a row are not.
func withLeadingText(p llm.MessageParam, text string) llm.MessageParam {
	if strings.TrimSpace(text) == "" {
		return p
	}
	p.Content = append([]llm.ContentBlockParamUnion{llm.NewTextBlock(text)}, p.Content...)
	return p
}
