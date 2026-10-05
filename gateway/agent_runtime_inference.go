package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"memdoor/gateway/compaction"
	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/gateway/memory"
	"memdoor/gateway/prompts"
	"memdoor/gateway/providers"
	"memdoor/pkg/llm"
	"memdoor/pkg/sandbox"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// agent_runtime_inference: runInference — assemble the LLM request and stream the response.
// Split out of agent_adapter.go (2026-08-28) to keep one concern per file;
// Pattern: OpenClaw one-file-per-concern organization. Same package, same
// behavior — pure code movement.

// runInference makes one LLM call. It returns the assistant message AND the
// conversation it actually ran on — which may have been COMPACTED in place. The
// caller MUST adopt the returned conversation (the tool loop threads it back in),
// otherwise compaction is computed and discarded every turn and the working set
// grows unbounded (the persisted session bloats and the KV prefix never settles).
func (ar *AgentRuntime) runInference(ctx context.Context, conversation []llm.MessageParam, extraSystemPrompt string) (*llm.Message, []llm.MessageParam, error) {
	// Returning is progress: the turn backstop counts from here (sharedctx.WithIdleTimeout).
	defer sharedctx.Progress(ctx)
	// Get run ID and session ID from context if available (needed for logging)
	runID := ""
	sessionID := ""
	skipCompaction := false
	if rid, ok := ctx.Value(ctxRunID).(string); ok {
		runID = rid
	}
	if sid, ok := ctx.Value(ctxSessionID).(string); ok {
		sessionID = sid
	}
	if skip, ok := ctx.Value(ctxSkipCompaction).(bool); ok {
		skipCompaction = skip
	}

	log := logs.New("Agent").WithSession(sessionID).WithRun(runID)

	windowModel, re := ar.answeringModel(ctx)

	// Get filtered tools based on agent configuration (with context for buddy_tools)
	allowedTools := ar.getFilteredTools(ctx)

	// Store allowed tool names in context for executeTool verification
	allowedToolNames := make(map[string]bool, len(allowedTools))
	for _, tool := range allowedTools {
		allowedToolNames[tool.Name] = true
	}
	ctx = context.WithValue(ctx, ctxAllowedTools, allowedToolNames)

	// Convert tools to Anthropic format. tools.BuildToolUnionParams is the
	// single source of truth so the gateway's KV-cache warmup (which calls
	// the same helper) produces byte-identical schemas — divergence would
	// silently break the prefix-cache match on the tool-schema portion.
	// Submit the turn's narrowed surface (turn_tools.go); enforcement above
	// keeps the full palette, so narrowing never blocks a tool the agent owns.
	// Without a decision model jgrep and jread are not offered; that is not a
	// saving the decision model made, so the receipt starts after it.
	offered := tools.WithoutIdleJudges(allowedTools)
	submitted := submittedTools(ctx, offered)
	// The receipt (pkg/savings): schemas travel on every call, so the saving
	// from a narrowed palette is per call, not per turn.
	agentForSaving, _ := ctx.Value("buddy_agent_name").(string)
	recordToolboxSaving(offered, submitted, providers.ServedModel(ctx, agentForSaving))
	anthropicTools := tools.BuildToolUnionParams(submitted)
	anthropicToolParams := make([]llm.ToolParam, 0, len(submitted))
	for _, tu := range anthropicTools {
		if tu.OfTool != nil {
			anthropicToolParams = append(anthropicToolParams, *tu.OfTool)
		}
	}

	// Build system prompt
	workspace := ar.agentConfig.GetWorkspace()
	if workspace == "" {
		workspace = "."
	}

	model := ar.resolvedModel
	systemPromptBuilder := prompts.NewSystemPromptBuilder(ar.agentConfig, submitted, workspace, model)
	// The project's AGENTS.md, from the directory this turn works
	// in (the TUI's cwd, a spawned run's workdir), not the agent's own home.
	systemPromptBuilder.SetProjectDir(turnWorkdir(ctx))
	systemPromptBuilder.SetProjectInstructions(projectInstructionsFor(ctx, turnWorkdir(ctx)))
	// Identify the agent by the buddy actually running (e.g. "coder"), not the shared
	// runtime's static config id — so the identity line doesn't contradict the buddy's
	// own prompt ("You are the coder" vs "You are agent 'main'").
	if name, _ := ctx.Value("buddy_agent_name").(string); name != "" {
		systemPromptBuilder.SetAgentName(name)
	}

	// Buddy agents are defined by their seeded system_prompt + palette, not by
	// hardcoded prompt sections. A codebase task agent (coder or planner) gets an
	// identity-only prompt so its OWN seed is the whole instruction — no chat
	// "stay silent / REPLY_SKIP" collaboration block bolted on (which makes a
	// small model skip the task). Conversational buddies keep chat mode.
	if isBuddy, _ := ctx.Value(sharedctx.IsBuddyChatKey).(bool); isBuddy {
		if ar.agentWorksOnCodebase(ctx) {
			systemPromptBuilder.SetPromptMode(prompts.PromptModeNone)
		} else {
			systemPromptBuilder.SetPromptMode(prompts.PromptModeChat)
		}
	}

	// Add skills to the system prompt (OpenClaw pattern)
	if len(ar.loadedSkills) > 0 {
		systemPromptBuilder.SetSkills(ar.loadedSkills)
	}

	// Add secret names to system prompt if agent has secrets
	if ar.secretRepo != nil {
		if sandboxCtxValue := ctx.Value(sharedctx.SandboxContextKey); sandboxCtxValue != nil {
			if sctx, ok := sandboxCtxValue.(sandbox.SandboxContext); ok && sctx.CurrentAgentID != "" {
				names, err := ar.secretRepo.List(ctx, sctx.CurrentAgentID)
				if err == nil && len(names) > 0 {
					systemPromptBuilder.SetSecretNames(names)
					log.Info("Injected secret names into system prompt",
						slog.String("agent_id", sctx.CurrentAgentID),
						slog.Int("secret_count", len(names)),
						slog.Any("secrets", names))
				} else {
					log.Debug("No secrets found for agent",
						slog.String("agent_id", sctx.CurrentAgentID))
				}
			} else {
				log.Info("No CurrentAgentID in sandbox context for secrets")
			}
		} else {
			log.Info("No sandbox_context in context for secrets")
		}
	}

	systemPrompt, err := systemPromptBuilder.Build()
	if err != nil {
		log.WithError(err).Error("Failed to build system prompt")
		// Continue without system prompt rather than failing
		systemPrompt = ""
	}
	// The prompt as it is SENT, extraSystemPrompt first (ping-pong context,
	// and the coder's whole personality, skills and project instructions).
	// Every measurement below uses it: the window check, compaction and
	// /context measured the builder's part alone — one line for the coder —
	// and every conversation read ~2.3K tokens short (2026-09-29).
	if extraSystemPrompt != "" {
		if systemPrompt != "" {
			systemPrompt = extraSystemPrompt + "\n\n" + systemPrompt
		} else {
			systemPrompt = extraSystemPrompt
		}
	}

	// Populate ContextToolState for context inspection tool and post-execution context tracking
	// Pattern: OpenClaw context tool state injection
	tools.ContextToolState.Model = windowModel
	tools.ContextToolState.SystemPrompt = systemPrompt
	tools.ContextToolState.ToolSchemas = anthropicToolParams
	tools.ContextToolState.Messages = conversation

	// Check context before API call.
	var checkResult *ctxmgmt.CheckResult
	if ar.preflight != nil {
		var err error
		checkResult, err = ar.preflight.Check(runID, sessionID, windowModel, systemPrompt, anthropicToolParams, conversation)
		if err != nil {
			log.WithError(err).Debug("Pre-flight check failed")
		}
	}

	// The decision is a pure function of the check, the agent's kind, the
	// brain and the turn position (see compactionDecision); the log line
	// keeps the old shape so `logs query --regex "Compaction check"` works.
	onCodebase := ar.agentWorksOnCodebase(ctx)
	compactionType, why := compactionDecision(checkResult, isMidTurn(ctx))
	shouldTriggerCompaction := compactionType != compaction.CompactionNone
	if checkResult != nil {
		msg := fmt.Sprintf("Compaction check: codebase=%v util=%.1f%% tokens=%d should_compact=%v (%s)",
			onCodebase, checkResult.Utilization, checkResult.TotalTokens, shouldTriggerCompaction, why)
		if shouldTriggerCompaction {
			log.Info(msg, slog.Float64("utilization", checkResult.Utilization), slog.Bool("should_compact", shouldTriggerCompaction))
		} else {
			log.Debug(msg)
		}
	} else {
		log.Debug("Compaction check result is nil")
	}

	if skipCompaction {
		compactionType, shouldTriggerCompaction = compaction.CompactionNone, false
	}
	// compaction.enabled: false stops the threshold; a turn over the window
	// is still fitted, or the request could not be sent.
	if compactionType == compaction.CompactionThreshold && !ar.compactionConfig.On() {
		compactionType, shouldTriggerCompaction = compaction.CompactionNone, false
	}

	log.Debug("Compaction trigger decision",
		slog.Bool("should_trigger", shouldTriggerCompaction),
		slog.String("compaction_type", fmt.Sprintf("%v", compactionType)),
		slog.Bool("skip_compaction", skipCompaction))

	if shouldTriggerCompaction && ar.preflight != nil {
		steps := fitLadder
		var fitErr error
		conversation, checkResult, fitErr = ar.fitConversation(ctx, conversation, fitRequest{
			runID: runID, sessionID: sessionID, why: compactionType, steps: steps,
			check: func(c []llm.MessageParam) *ctxmgmt.CheckResult {
				chk, _ := ar.preflight.Check(runID, sessionID, windowModel, systemPrompt, anthropicToolParams, c)
				return chk
			},
			flush: func(c []llm.MessageParam) { ar.flushNotes(ctx, c) },
		})
		if fitErr != nil {
			return nil, conversation, fitErr
		}
	}

	// Inject context tool state for /context command
	tools.ContextToolState.Model = windowModel
	tools.ContextToolState.SystemPrompt = systemPrompt
	tools.ContextToolState.ToolSchemas = anthropicToolParams
	tools.ContextToolState.Messages = conversation

	// Calculate context size before API call for debugging
	conversationTokens := ar.compactor.CountConversationTokens(conversation)
	systemTokens := ar.compactor.CountSystemPromptTokens(systemPrompt)
	toolSchemaTokens := ar.compactor.CountToolSchemaTokens(anthropicToolParams)
	totalTokens := conversationTokens + systemTokens + toolSchemaTokens

	log.Info("Calling LLM API",
		slog.Int("messages", len(conversation)),
		slog.Int("tools", len(anthropicTools)),
		slog.Int("conversation_tokens", conversationTokens),
		slog.Int("system_tokens", systemTokens),
		slog.Int("tool_schema_tokens", toolSchemaTokens),
		slog.Int("total_input_tokens", totalTokens),
		slog.Int("system_prompt_chars", len(systemPrompt)))

	// Make API call with system prompt. Temperature is per-buddy — the
	// buddy row stores it (pkg/domain.Buddy.Temperature); agent_handlers
	// pushes it onto ctx under BuddyTemperatureKey, server_jobs forwards
	// it, and we read it here. Zero = omit from request = provider default
	// (which for Groq is OpenAI-style 1.0 — too hot for tool-calling).
	//
	// Agent is the fine-grained buddy name (set by message/service.go on
	// ctx as "buddy_agent_name"). Plumbed onto MessageNewParams.Agent
	// so the central LLM log can attribute tokens per agent — backs
	// `memdoor tokens --by agent` cost attribution.
	agentName, _ := ctx.Value("buddy_agent_name").(string)
	params := llm.MessageNewParams{
		Model: llm.Model(model),
		// Output cap follows the answering model: a big window admits
		// bigger single generations (live: an 8192 cap truncated a 27B's
		// whole-file patch after 7 minutes of generation).
		MaxTokens: fitReplyToWindow(maxOutputTokensFor(ctx, windowModel), windowModel, checkResult),
		Messages:  conversation,
		Tools:     anthropicTools,
		Agent:     agentName,
	}
	if temp, ok := ctx.Value(sharedctx.BuddyTemperatureKey).(float64); ok && temp > 0 {
		params.Temperature = temp
	}
	// Empty-step retry: same context, hotter sampling. A deterministic re-sample
	// (KV-cached) reproduces the same empty output — the retry only helps if it
	// can land somewhere else.
	if temp, ok := ctx.Value(retryTemperatureKey{}).(float64); ok && temp > 0 {
		params.Temperature = temp
	}

	finalSystemPrompt := systemPrompt

	if finalSystemPrompt != "" {
		params.System = []llm.TextBlockParam{
			{
				Text: finalSystemPrompt,
				Type: "text",
			},
		}
	}

	// Always log the LLM request summary so we can correlate calls. `model` is
	// the placeholder label ("default"), so the receipt names the model the
	// engine actually serves: a receipt that names the wrong model is worse
	// than no receipt.
	effectiveModel := model
	if re != nil {
		effectiveModel = fmt.Sprintf("%s (remote %s)", windowModel, re.Name)
	}
	log.Info(fmt.Sprintf("LLM request: model=%s msgs=%d tools=%d prompt_len=%d",
		effectiveModel, len(conversation), len(anthropicTools), len(finalSystemPrompt)))

	// Log the EXACT system prompt the agent is built with, and the tool names it was
	// given — so we diagnose "has tools but doesn't act" from facts, not guesses.
	builtToolNames := make([]string, 0, len(allowedTools))
	for _, td := range allowedTools {
		builtToolNames = append(builtToolNames, td.Name)
	}
	log.Debug(fmt.Sprintf("AGENT BUILD [%s] tools=[%s]\n--- system prompt ---\n%s\n--- end ---",
		agentName, strings.Join(builtToolNames, ","), truncateForLog(finalSystemPrompt, 6000)))
	// Log every message the agent actually sees, so we can compare a spawned run to a
	// direct run message-for-message.
	for i, m := range conversation {
		log.Debug(fmt.Sprintf("AGENT MSG [%s] #%d role=%s: %s",
			agentName, i, m.Role, truncateForLog(ar.extractTextFromMessage(m), 300)))
	}

	// Stats-only: length of system prompt and per-message byte/block counts.
	// We don't dump the bodies — they're huge — but the stats let us see at
	// a glance how much context the LLM is actually receiving.
	if finalSystemPrompt != "" {
		log.Debug("LLM system prompt", slog.Int("bytes", len(finalSystemPrompt)))
	}

	// ONE line per call, not one per message per call. The old loop was
	// quadratic in a turn — a 12-message conversation over 10 tool rounds wrote
	// 120 near-identical INFO lines and buried everything else in the log. The
	// question it answers ("how much context is the model actually getting?")
	// is answered better by the totals; per-message detail stays at Debug.
	var totalBytes, userMsgs, asstMsgs int
	for _, msg := range conversation {
		raw, _ := json.Marshal(msg)
		totalBytes += len(raw)
		if msg.Role == "user" {
			userMsgs++
		} else {
			asstMsgs++
		}
		log.Debug("LLM message",
			slog.String("role", string(msg.Role)),
			slog.Int("bytes", len(raw)),
			slog.Int("blocks", len(msg.Content)))
	}
	log.Info("LLM conversation",
		slog.Int("messages", len(conversation)),
		slog.Int("user", userMsgs),
		slog.Int("assistant", asstMsgs),
		slog.Int("bytes", totalBytes))

	toolNames := make([]string, 0, len(anthropicTools))
	for _, tool := range anthropicTools {
		if tool.OfTool != nil {
			toolNames = append(toolNames, tool.OfTool.Name)
		}
	}
	log.Debug("LLM tools", slog.Any("tool_names", toolNames))

	// Resolve the LLM client for this agent through its provider
	// (providers/factory.go).
	llmClient, clientErr := ar.clientFactory.GetClientFor(ctx, agentName)
	if clientErr != nil {
		log.Error("Failed to get LLM client",
			slog.String("error", clientErr.Error()))
		return nil, conversation, clientErr
	}
	params.Model = llm.Model(providers.ModelName)

	// Plant a per-delta callback on ctx. The OAI client uses SSE under
	// the hood; this is the hook that turns each chunk into a real-time
	// WebSocket event so chat clients see "..." → "the model is
	// actually writing" while the request is still in flight, instead
	// of one giant block at the end. Path:
	//   provider SSE loop    → llm.StreamCallback(delta)
	//   here                 → ar.events.EmitEvent(EventStreamAssistant, text_delta)
	//   infra.EventEmitter   → synchronous, in-order delivery per listener
	//   ConnectInfraEventEmitter (server.go:383) → broadcast.SubscriptionManager
	//   websocket            → CLI / web client renders the delta
	// No new wiring — the bridge has been in place since OpenClaw; we
	// just had nothing emitting per-token deltas onto it.
	//
	// Duck-typed contract: llm.WithStreamCallback / llm.StreamCallback
	// live in pkg/llm, not gateway/providers. The agent runtime never
	// reaches into the provider package for this; the provider reads
	// the same callback off ctx without knowing or caring who planted
	// it (or whether it was planted at all).
	//
	// Skip the wrap when runID is empty (memory-flush sub-call, boot
	// warmup, etc.) so background inference doesn't leak partial
	// tokens onto chat-client streams.
	if runID != "" {
		ctx = llm.WithStreamCallback(ctx, func(delta string) {
			ar.events.EmitEvent(runID, infra.EventStreamAssistant, sessionID, map[string]interface{}{
				"event": "text_delta",
				"delta": delta,
			})
		})
		ctx = llm.WithNotice(ctx, func(text string) {
			ar.events.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
				"event":   "provider_notice",
				"message": text,
			})
		})
		// The prose breaker, coder turns only: stop a reply that has run past
		// the threshold as tag-free prose — a runaway burns its whole 16k cap
		// (~7 minutes, measured three times 2026-08-31) before any
		// after-the-fact guard can see it. A stopped reply comes back as
		// StopReasonStreamGuard and the turn driver retries it, the same way
		// it retries a reasoning-only reply. A chat agent is excluded: its
		// long prose IS the answer.
		if ar.agentWorksOnCodebase(ctx) {
			ctx = llm.WithStreamGuard(ctx, providers.ProseRunawayGuard)
		}
	}

	// Name the conversation so a pooled engine can send this turn back to the
	// machine already holding its cached prefix. Same duck-typed contract as
	// the stream callback above: planted here, read in the provider, ignored by
	// engines that do not pool.
	//
	// The key is the TRANSCRIPT, not the session: two agents in one channel
	// share a session but not a system prompt, so they are separate prefixes
	// and there is nothing to gain by pinning them together.
	ctx = llm.WithRouteKey(ctx, transcriptKey(sessionID, agentName))

	message, err := llmClient.Messages().New(ctx, params)

	// THE SERVER'S COUNT IS THE COUNT. Our estimate is chars/4 and the
	// serving tokenizer is not: French prose and JSON tool output came to
	// 61,444 tokens where we had counted under the 57,344 limit, so the
	// shed ladder above never fired and the turn died on the refusal
	// instead (2026-09-18 14:36). The refusal names the real count: learn
	// the factor from it, shed tool output until the corrected estimate
	// fits, and ask once more. An accepted answer teaches the factor too
	// (usage.input_tokens), so the ladder sees what the server will see.
	if err != nil {
		if _, _, real, ok := ctxmgmt.ParseContextLengthError(err.Error()); ok && !skipCompaction {
			ctxmgmt.CalibrateTokensHard(real, totalTokens)
			log.Warn("the server counts more tokens than we do — recalibrated, shedding to fit and asking again",
				slog.Int("server_input_tokens", real), slog.Int("our_estimate", totalTokens),
				slog.Float64("token_scale", ctxmgmt.TokenScale()))
			refit, _, fitErr := ar.fitConversation(ctx, conversation, fitRequest{
				runID: runID, sessionID: sessionID, why: compaction.CompactionOverLimit, steps: refusalLadder,
				check: func(c []llm.MessageParam) *ctxmgmt.CheckResult {
					chk, _ := ar.preflight.Check(runID, sessionID, windowModel, systemPrompt, anthropicToolParams, c)
					return chk
				},
			})
			fitted := fitErr == nil
			if fitted {
				conversation = refit
			}
			if fitted {
				params.Messages = conversation
				message, err = llmClient.Messages().New(ctx, params)
			}
		}
	}
	if err != nil {
		log.Error("LLM API call failed",
			slog.String("error", err.Error()),
			slog.String("hint", "check network connectivity to the managed LLM tier; re-run 'memdoor setup' if credentials look corrupted"))
		return nil, conversation, err
	}
	if message != nil && message.Usage.InputTokens > 0 {
		ctxmgmt.CalibrateTokens(int(message.Usage.InputTokens), totalTokens)
	}

	log.Debug("LLM API call succeeded")

	// Log detailed response payload (if verbose mode)
	if ar.verbose && message != nil {
		log.Info("LLM API Response",
			slog.String("id", message.ID),
			slog.String("model", string(message.Model)),
			slog.String("role", string(message.Role)),
			slog.String("stop_reason", string(message.StopReason)),
			slog.Int("content_blocks", len(message.Content)),
			slog.Int("input_tokens", int(message.Usage.InputTokens)),
			slog.Int("output_tokens", int(message.Usage.OutputTokens)))

		// Log each content block
		for i, content := range message.Content {
			switch content.Type {
			case "text":
				textPreview := content.Text
				if len(textPreview) > 200 {
					textPreview = textPreview[:200] + "..."
				}
				log.Info("Response content block",
					slog.Int("index", i),
					slog.String("type", "text"),
					slog.String("text_preview", textPreview))
			case "tool_use":
				toolUse := content.AsToolUse()
				log.Info("Response content block",
					slog.Int("index", i),
					slog.String("type", "tool_use"),
					slog.String("tool_name", toolUse.Name),
					slog.String("tool_id", toolUse.ID))
			}
		}
	}

	return message, conversation, nil
}

// midTurnKey marks a context as belonging to a turn's follow-up inferences
// (after the first tool call), where the live tool results are the working set.
type midTurnKey struct{}

func withMidTurn(ctx context.Context) context.Context {
	return context.WithValue(ctx, midTurnKey{}, true)
}

func isMidTurn(ctx context.Context) bool {
	v, _ := ctx.Value(midTurnKey{}).(bool)
	return v
}

// compactionDecision says whether to compact before this inference and how.
//
//   - Over the hard limit: emergency (AI) compaction, any time.
//   - A threshold compaction never fires MID-TURN. Between tool calls the
//     conversation IS the working set: pruning it dropped the cut's and
//     tighten's receipts and the model re-derived the numbers wrong, then
//     re-verified files it had just verified (live 2026-09-03 12:12 on the
//     27B). It waits for the next turn's first inference.
func compactionDecision(check *ctxmgmt.CheckResult, midTurn bool) (compaction.CompactionType, string) {
	if check == nil {
		return compaction.CompactionNone, "no check"
	}
	if !check.CanProceed {
		return compaction.CompactionOverLimit, "over the hard limit"
	}
	if !check.ShouldCompact {
		return compaction.CompactionNone, "under threshold"
	}
	if midTurn {
		return compaction.CompactionNone, "deferred: mid-turn, the tool results are the working set"
	}
	return compaction.CompactionThreshold, "threshold"
}

// Rough bytes-per-token for the window estimate, and the share of the
// window past which pruning starts.
const ()

// flushReplyText is what a memory flush said worth keeping, or "" when it said
// nothing (the NO_REPLY token, or no text at all).
func flushReplyText(m *llm.Message) string {
	var b strings.Builder
	for _, c := range m.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	t := strings.TrimSpace(b.String())
	if t == "" || strings.EqualFold(strings.Trim(t, " .`*_"), memory.SilentReplyToken) {
		return ""
	}
	return t
}
