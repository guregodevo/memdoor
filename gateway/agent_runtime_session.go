package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/gateway/memory"
	"memdoor/gateway/providers"
	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// agent_runtime_session: conversation assembly, per-agent memory, reflection, and session mutation.
// Split out of agent_adapter.go (2026-08-28) to keep one concern per file;
// Pattern: OpenClaw one-file-per-concern organization. Same package, same
// behavior — pure code movement.

// wholeTranscript loads every message of a transcript.
const wholeTranscript = 100000

// recentMessageLimitForAgent picks the sliding-window size based on the
// buddy's tool palette (passed through ctx as BuddyToolsKey by the
// message service). The multi-message window is inherited from OpenClaw's
// chat-agent design, where history carries conversational continuity.
//
// Chat-style agents (chief, etc.) keep the 10-message default — they rely on
// conversation history for continuity. Codebase agents carry the whole
// conversation. Returns 10 when ctx has no BuddyToolsKey.
func recentMessageLimitForAgent(ctx context.Context) int {
	buddyTools, _ := ctx.Value(sharedctx.BuddyToolsKey).([]string)
	for _, t := range buddyTools {
		switch t {
		case "bash", "edit_file", "write_file", "read_file", "grep", "glob":
			// A codebase agent carries the WHOLE conversation, as Claude Code
			// does: a follow-up is about what it just read and said. It used to
			// load zero, and on
			// the person's key "is this Go?" after reading a Go file got "No."
			// (live 2026-09-29). The window is bounded by compaction at 60% of
			// the answering model's window, never past 200K
			// (ctxmgmt.CompactionCeiling); /fresh and /clear start it over.
			return wholeTranscript
		}
	}
	return 10
}

// subagentHistoryLimit bounds a focused subagent session to its most recent whole
// turn (task → tool_use → tool_result → short report ≈ 4 messages) plus the new
// instruction. This is OpenClaw's "trim by whole turns" (rule 7), NOT the chat
// sliding-window+RAG reinjection: no unrelated messages are pulled in, and the prior
// step's tool_use stays in view so the established pattern the model continues is
// TOOL-CALLING. Keeping only ~one prior turn also avoids piling up several "done"
// report turns, which makes a model read the task as finished and reply empty.
// The file on disk carries full state across steps; the coder read_files what it needs.
const subagentHistoryLimit = 4

// buildConversation converts session messages to Anthropic conversation format
// Pattern: Sliding window + RAG for memory-efficient context
// This replaces the old compaction-based approach with RAG semantic retrieval
// transcriptKey returns the key a turn's REASONING transcript is loaded/saved under.
// For a channel session (shared by every agent mentioned there) it appends
// ":agent:<name>" so each agent gets its OWN isolated transcript — OpenClaw parity.
// The session.ID itself is unchanged (it routes WS events + the client subscription),
// so isolating the transcript never breaks the TUI stream. Subagent/agent keys (which
// are already isolated) pass through untouched.
func transcriptKey(sessionID, agentName string) string {
	if agentName != "" &&
		strings.HasPrefix(sessionID, "workspace:") &&
		strings.Contains(sessionID, ":channel:") &&
		!strings.Contains(sessionID, ":agent:") {
		return sessionID + ":agent:" + agentName
	}
	return sessionID
}

func (ar *AgentRuntime) buildConversation(ctx context.Context, session *Session) []llm.MessageParam {
	log := logs.New("Agent").WithSession(session.ID)

	// Session isolation: derive a PER-AGENT transcript key from the (shared) channel
	// key + the agent, so each agent has its OWN transcript instead of the shared
	// channel store. session.ID itself stays the channel key (event routing / client
	// subscription depend on it — see NewChannelSessionID). The agent name is stashed
	// in session metadata so updateSession (no ctx) saves to the same key.
	agentName, _ := ctx.Value("buddy_agent_name").(string)
	if agentName == "" {
		// Some re-invocations (the verify+repair loop) don't carry buddy_agent_name in
		// ctx. Fall back to what a prior turn stashed, so EVERY turn on this session
		// uses the SAME transcript key — otherwise one turn loads the isolated store and
		// the next loads the channel store, mismatching conversation_length (a panic).
		if an, _ := session.GetMetadataValue("transcript_agent"); an != nil {
			agentName, _ = an.(string)
		}
	}
	tKey := transcriptKey(session.ID, agentName)
	if tKey != session.ID {
		session.SetMetadata("transcript_agent", agentName)
	}
	log.Info(fmt.Sprintf("SESSION_KEY key=%q transcript_key=%q", session.ID, tKey))

	// Focused subagent task session (e.g. a task_flow coder running step by step):
	// apply OpenClaw's session pattern — load the FULL own transcript, with NO
	// sliding window and NO RAG reinjection. The short-window + RAG machinery below
	// exists to protect chat agents from busy-channel scrollback; a subagent's
	// history is only its own task turns. Trimming it to a tiny window (which
	// over-weights the last narration turn) and RAG-injecting unrelated messages is
	// exactly the "reinjection" that made a model mimic prior prose instead of
	// continuing to call tools. The replayed history's structured tool_use/tool_result
	// pairs are the established pattern the model should continue.
	if strings.Contains(session.ID, ":subagent:") {
		msgs, err := ar.persistence.LoadRecentMessages(session.ID, subagentHistoryLimit)
		if err != nil {
			log.WithError(err).Error("Failed to load subagent history")
			msgs = make([]llm.MessageParam, 0)
		}
		// Drop a trailing orphaned tool_use so history stays well-formed (pairing).
		msgs = ar.stripOrphanedToolUse(msgs)
		// updateSession slices conversation[conversation_length:] to persist only new
		// messages — this metadata MUST be set on every buildConversation path or that
		// slice panics with a stale length. (The chat path sets it below.)
		session.SetMetadata("conversation_length", len(msgs))
		log.Debug(fmt.Sprintf("CONVERSATION session.ID=%q loaded=%d (bounded subagent transcript)", session.ID, len(msgs)))
		return msgs
	}

	// Configuration: Sliding window size
	// Default 10 messages (previously REDUCED from 20→10 to prevent API
	// quota exhaustion). Per-agent override below.
	recentMessageLimit := recentMessageLimitForAgent(ctx)

	// STEP 1: Load recent messages (sliding window approach)
	recentMessages, err := ar.persistence.LoadRecentMessages(tKey, recentMessageLimit)
	if err != nil {
		log.WithError(err).Error("Failed to load recent messages")
		return make([]llm.MessageParam, 0)
	}

	log.Debug("Loaded recent messages (sliding window)",
		slog.Int("count", len(recentMessages)),
		slog.Int("limit", recentMessageLimit))
	log.Debug(fmt.Sprintf("CONVERSATION session.ID=%q loaded=%d recent messages", session.ID, len(recentMessages)))

	conversation := append([]llm.MessageParam(nil), recentMessages...)

	// STEP 4: Remove orphaned tool_use blocks at the end of conversation
	// This can happen when a previous run crashed after saving tool_use but before saving tool_result
	conversation = ar.stripOrphanedToolUse(conversation)

	// Initialize metadata with loaded conversation length
	session.SetMetadata("conversation_length", len(conversation))

	log.Debug("Built conversation context",
		slog.Int("total", len(conversation)),
		slog.Int("recent", len(recentMessages)))

	return conversation
}

// GetMemoryStore returns a cached memory store for the given agent name.
// Uses the main DB AgentMemoryRepository (Raft-replicated) instead of separate SQLite files.
func (ar *AgentRuntime) GetMemoryStore(agentName string) memory.MemoryStore {
	if ar.agentMemRepo == nil || agentName == "" {
		return nil
	}

	if store, ok := ar.memoryStores.Load(agentName); ok {
		return store.(memory.MemoryStore)
	}

	store := memory.NewRepoMemoryStore(ar.agentMemRepo, agentName)
	ar.memoryStores.Store(agentName, store)
	return store
}

// incrementInteractionCount increments and returns the per-agent interaction counter
func (ar *AgentRuntime) incrementInteractionCount(agentName string) int64 {
	val, _ := ar.interactionCounters.LoadOrStore(agentName, &atomic.Int64{})
	return val.(*atomic.Int64).Add(1)
}

// shouldReflect decides whether to trigger reflection for this interaction
func (ar *AgentRuntime) shouldReflect(count int64, userMessage string, response *AgentResponse) bool {
	// Always reflect on errors or tool failures
	if response.Error != "" {
		return true
	}
	for _, tool := range response.ToolsExecuted {
		if tool.Error != "" {
			return true
		}
	}

	// Always reflect on user corrections
	if looksLikeCorrection(userMessage) {
		return true
	}

	// Periodic reflection every 5th interaction
	if count%5 == 0 {
		return true
	}

	return false
}

// looksLikeCorrection detects if a user message is correcting the agent
func looksLikeCorrection(msg string) bool {
	lower := strings.ToLower(msg)
	patterns := []string{"no,", "no ", "wrong", "actually,", "actually ", "that's not", "incorrect", "don't ", "stop ", "i said", "not what i"}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// reflectionModelName returns the same model the agent uses for
// chat — no separate config knob for reflection.
func (ar *AgentRuntime) reflectionModelName() string {
	if ar == nil || ar.clientFactory == nil {
		return ""
	}
	_ = ar.clientFactory
	return providers.ModelName
}

// triggerReflection calls Sonnet to analyze the interaction and store structured memories
func (ar *AgentRuntime) triggerReflection(agentName, userMessage string, response *AgentResponse) {
	log := logs.New("Learning")

	store := ar.GetMemoryStore(agentName)
	if store == nil {
		log.Warn("Memory store unavailable, skipping reflection",
			slog.String("agent", agentName))
		return
	}

	// Build tool outcomes summary
	var toolOutcomes strings.Builder
	hasErrors := response.Error != ""
	hasToolFailures := false
	for _, tool := range response.ToolsExecuted {
		status := "success"
		if tool.Error != "" {
			status = fmt.Sprintf("FAILED: %s", tool.Error)
			hasToolFailures = true
		}
		toolOutcomes.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, status))
	}

	isCorrection := looksLikeCorrection(userMessage)
	errorsStr := "none"
	if hasErrors {
		errorsStr = response.Error
	}

	// Build reflection prompt
	reflectionPrompt := fmt.Sprintf(`You just completed an interaction. Evaluate using EXTERNAL evidence only.

INTERACTION:
User message: %s
Agent response: %s
Tools used:
%s
Errors: %s
User correction: %v

RULES:
- If user corrected the agent: store a semantic memory with tag "correction", confidence 1.0
- If tools succeeded and task completed: store a procedural memory with success_count 1
- If tools failed: store a procedural memory with failure_count 1 and a lesson
- If nothing notable happened: store nothing. Do not create noise.
- Be extremely brief. One memory per interaction maximum.
- Use the memory tool to store your findings.`,
		truncateStr(userMessage, 300),
		truncateStr(response.Text, 500),
		toolOutcomes.String(),
		truncateStr(errorsStr, 200),
		isCorrection)

	// Build memory tool for Sonnet
	memoryTool := llm.ToolParam{
		Name:        "memory",
		Description: llm.String("Store or update a memory. Actions: 'store' (new memory) or 'update' (existing memory by ID)."),
		InputSchema: tools.MemoryInputSchema,
	}

	// Call Sonnet for reflection. Agent is namespaced with a "/reflect"
	// suffix so per-agent cost attribution can split the user-facing
	// agent run from its post-hoc reflection pass.
	params := llm.MessageNewParams{
		Model:     llm.Model(ar.reflectionModelName()),
		MaxTokens: int64(1024),
		Agent:     agentName + "/reflect",
		Messages: []llm.MessageParam{
			llm.NewUserMessage(llm.NewTextBlock(reflectionPrompt)),
		},
		Tools: []llm.ToolUnionParam{
			{OfTool: &memoryTool},
		},
		System: []llm.TextBlockParam{
			{
				Text: "You are a reflection engine. Analyze interactions and store structured lessons using the memory tool. Be brief and precise.",
				Type: "text",
			},
		},
	}

	// The reflection is the agent's own, on its own model: named, so its
	// ladder picks it.
	ctx := context.WithValue(context.Background(), "buddy_agent_name", agentName)
	reflectionClient, clientErr := ar.clientFactory.GetClientFor(ctx, agentName)
	if clientErr != nil {
		log.Warn("Failed to get LLM client for reflection",
			slog.String("error", clientErr.Error()))
		return
	}
	msg, err := reflectionClient.Messages().New(ctx, params)
	if err != nil {
		log.Warn(fmt.Sprintf("Reflection API call failed: %v", err),
			slog.String("agent", agentName))
		return
	}

	// Execute any memory tool calls from Sonnet's response
	memoriesStored := 0
	for _, block := range msg.Content {
		if block.Type != "tool_use" {
			continue
		}
		toolUse := block.AsToolUse()
		if toolUse.Name != "memory" {
			continue
		}

		// Parse and execute the memory action
		result, execErr := ar.executeReflectionMemory(store, agentName, toolUse.Input)
		if execErr != nil {
			log.Warn(fmt.Sprintf("Reflection memory action failed: %v (input: %s)", execErr, string(toolUse.Input)),
				slog.String("agent", agentName))
			continue
		}
		memoriesStored++
		log.Debug("Reflection stored memory",
			slog.String("agent", agentName),
			slog.String("result", result))
	}

	log.Info("Reflection completed",
		slog.String("agent", agentName),
		slog.Int("memories_stored", memoriesStored),
		slog.Bool("has_errors", hasErrors),
		slog.Bool("has_tool_failures", hasToolFailures),
		slog.Bool("is_correction", isCorrection))
}

// executeReflectionMemory executes a memory tool call from the reflection model
func (ar *AgentRuntime) executeReflectionMemory(store memory.MemoryStore, agentName string, input json.RawMessage) (string, error) {
	var memInput tools.MemoryInput
	if err := json.Unmarshal(input, &memInput); err != nil {
		return "", fmt.Errorf("failed to parse memory input: %w", err)
	}

	var tags []string
	if memInput.Tags != "" {
		tags = strings.Split(memInput.Tags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}
	// Always tag as auto-reflection
	tags = append(tags, "auto_reflection")

	var metadata map[string]interface{}
	if memInput.Metadata != "" {
		json.Unmarshal([]byte(memInput.Metadata), &metadata)
	}

	switch memInput.Action {
	case "store":
		memID, err := store.Store(memInput.Content, tags, metadata)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("stored memory %s", memID), nil

	case "update":
		if memInput.ID == "" {
			return "", fmt.Errorf("update requires an ID")
		}
		var contentPtr *string
		if memInput.Content != "" {
			contentPtr = &memInput.Content
		}
		if err := store.Update(memInput.ID, contentPtr, tags, metadata); err != nil {
			return "", err
		}
		return fmt.Sprintf("updated memory %s", memInput.ID), nil

	default:
		return "", fmt.Errorf("reflection only supports store and update actions, got: %s", memInput.Action)
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// stripOrphanedToolUse removes tool_use blocks that lack matching tool_result blocks.
// This can happen when a previous run crashed after saving tool_use but before saving tool_result.
// Strips individual tool_use blocks from messages, removing the entire message if nothing remains.
func (ar *AgentRuntime) stripOrphanedToolUse(conversation []llm.MessageParam) []llm.MessageParam {
	log := logs.New("Agent")

	// Collect all tool_result IDs
	toolResultIDs := make(map[string]bool)
	for _, msg := range conversation {
		for _, content := range msg.Content {
			if content.OfToolResult != nil {
				toolResultIDs[content.OfToolResult.ToolUseID] = true
			}
		}
	}

	// Strip orphaned tool_use blocks from each message
	result := make([]llm.MessageParam, 0, len(conversation))
	for _, msg := range conversation {
		var cleanContent []llm.ContentBlockParamUnion
		stripped := 0

		for _, content := range msg.Content {
			if content.OfToolUse != nil && !toolResultIDs[content.OfToolUse.ID] {
				stripped++
				continue
			}
			cleanContent = append(cleanContent, content)
		}

		if stripped > 0 {
			log.Warn("Stripped orphaned tool_use blocks from conversation",
				slog.Int("count", stripped))
		}

		if len(cleanContent) > 0 {
			msg.Content = cleanContent
			result = append(result, msg)
		} else {
			log.Warn("Removed empty message after stripping orphaned tool_use blocks")
		}
	}

	return result
}

// extractTextFromMessage extracts text content from a MessageParam
func (ar *AgentRuntime) extractTextFromMessage(msg llm.MessageParam) string {
	for _, content := range msg.Content {
		// ContentBlockParamUnion uses inline union types with Of* fields
		if content.OfText != nil {
			return content.OfText.Text
		}
	}
	return ""
}

// extractWorkspaceAndChannel parses session ID to extract workspace and channel IDs
// Format: "workspace:{workspaceID}:channel:{channelID}"
// Returns: (workspaceID, channelID) or (ar.workspaceID, "") if not a channel session
func (ar *AgentRuntime) extractWorkspaceAndChannel(sessionID string) (string, string) {
	if !strings.HasPrefix(sessionID, "workspace:") {
		return ar.workspaceID, ""
	}

	parts := strings.Split(sessionID, ":")
	if len(parts) == 4 && parts[2] == "channel" {
		return parts[1], parts[3]
	}

	return ar.workspaceID, ""
}

// updateSession stores the conversation back in the session
// Pattern: State management (like OpenClaw's session persistence)
func (ar *AgentRuntime) updateSession(session *Session, conversation []llm.MessageParam) {
	log := logs.New("Agent").WithSession(session.ID)

	// What the transcript does not hold yet. The cursor is clamped: a stale
	// one must never panic the gateway with an out-of-range slice.
	initialLength := sessionCursor(session, len(conversation))
	newMessages := conversation[initialLength:]
	if len(newMessages) > 0 {
		// One batch: two turns saving message by message could interleave
		// and leave a tool call without its result.
		if err := ar.persistence.SaveMessages(transcriptKeyOf(session), newMessages); err != nil {
			log.WithError(err).Error("Failed to save message batch")
		}
	}

	// Update session metadata
	// Pattern: Metadata storage (OpenClaw approach)
	// Length must count what the TRANSCRIPT holds (grounding filtered out), or
	// the next turn's save slices past its own first new message.
	session.SetMetadata("conversation_length", initialLength+len(newMessages))
	session.SetMetadata("last_updated", time.Now())

	log.Debug("Saved new messages to disk atomically", slog.Int("count", len(newMessages)))
}

// emitContextUpdate calculates and emits context usage information
// Pattern: Extracted from ProcessMessage to allow real-time emission during tool execution
func (ar *AgentRuntime) emitContextUpdate(runID, sessionID string, conversation []llm.MessageParam) {
	log := logs.New("Agent").WithSession(sessionID).WithRun(runID)

	inspector, err := ctxmgmt.NewContextInspector(
		tools.ContextToolState.Model,
		tools.ContextToolState.SystemPrompt,
		tools.ContextToolState.ToolSchemas,
		conversation,
		false, // verbose
	)
	if err == nil {
		summary := inspector.GetSummary()

		data := map[string]interface{}{
			"event":   "context_update",
			"tokens":  summary.TotalTokens,
			"limit":   summary.EffectiveLimit,
			"percent": summary.Utilization,
			// What is eating the window (/context).
			"system":       summary.SystemPromptTokens,
			"tools":        summary.ToolSchemaTokens,
			"conversation": summary.ConversationTokens,
			"tool_output":  summary.ToolOutputTokens,
		}
		if ar.preflight != nil {
			data["compact_at"] = ar.preflight.CompactAt(tools.ContextToolState.Model)
			data["compact_rule"] = ar.preflight.CompactRule()
			if !ar.compactionConfig.On() {
				data["compact_rule"] = "off (compaction.enabled: false)"
			}
		}
		data["keep_recent"] = ar.keepRecentTokens()
		ar.events.EmitEvent(runID, infra.EventStreamContext, sessionID, data)
	} else {
		log.WithError(err).Debug("Failed to calculate context")
	}
}

// AnswerQuestion delivers a client's answer to a pending interactive
// ask_user_question, unblocking the tool call. Returns false if no such question
// is waiting. Called by the WebSocket handler when the TUI sends the selection.
func (ar *AgentRuntime) AnswerQuestion(questionID, answer string) bool {
	return ar.questions.answer(questionID, answer)
}

// EmitContextBaseline reports what every request already carries — the system
// prompt, the tool schemas and the answering model's window — with no
// conversation behind it. Called when a TUI subscribes to a session: before
// the first turn /context showed "0 / 200.0k", the 200K the compaction ceiling
// happened to be, not the window, and "0" hid the prompt and tool tokens the
// request was already paying for. Best effort: if the tool state is not yet
// populated (no turn has run, warmup not wired), nothing is emitted and the
// TUI's "no report yet" stands.
func (ar *AgentRuntime) EmitContextBaseline(sessionID string) {
	if ar == nil || ar.events == nil {
		return
	}
	if tools.ContextToolState.Model == "" || tools.ContextToolState.SystemPrompt == "" {
		return
	}
	ar.emitContextUpdate("baseline", sessionID, nil)
}
