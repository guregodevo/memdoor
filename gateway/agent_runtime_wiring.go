package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
	"memdoor/gateway/subagents"
	"memdoor/pkg/repository"
	"memdoor/tools"
)

// agent_runtime_wiring: dependency wiring (Wire*/Set*), warmup, and lifecycle.
// Split out of agent_adapter.go (2026-08-28) to keep one concern per file;
// Pattern: OpenClaw one-file-per-concern organization. Same package, same
// behavior — pure code movement.

// WireSessionsSpawnTool wires the sessions_spawn tool into the runtime
// Pattern: Post-construction dependency injection for tools requiring runtime dependencies
// This is called after AgentRuntime is created but before the server starts
func (ar *AgentRuntime) WireSessionsSpawnTool(registry *subagents.SubagentRegistry, queueEnqueuer tools.QueueEnqueuer, sessionGetter tools.SessionGetter, resolveConfig func(agentID string) ([]string, string)) error {
	log := logs.New("Agent")
	log.Debug("Wiring sessions_spawn tool with runtime dependencies")

	// Create and store SessionsSpawnTool instance
	ar.sessionsSpawn = tools.NewSessionsSpawnTool(registry, queueEnqueuer, sessionGetter, resolveConfig, ar.verbose)

	// Add tool definition to available tools
	// Note: The Function field is nil - execution is handled specially in executeTool()
	toolDef := tools.ToolDefinition{
		Name:        "sessions_spawn",
		Description: tools.SessionsSpawnDefinition.Description,
		InputSchema: tools.SessionsSpawnInputSchema,
		Function:    nil, // Execution handled specially in executeTool()
	}

	ar.tools = append(ar.tools, toolDef)

	log.Debug("sessions_spawn tool wired successfully", slog.Int("total_tools", len(ar.tools)))

	return nil
}

// WireTaskFlowTool wires the task_flow tool into the runtime.
// Pattern: same post-construction injection as sessions_spawn — task_flow needs
// the flow registry (a runtime dependency), so its ToolDefinition.Function is nil
// and execution is dispatched by name in executeTool().
func (ar *AgentRuntime) WireTaskFlowTool(starter tools.FlowStarter) error {
	log := logs.New("Agent")
	ar.taskFlow = tools.NewTaskFlowTool(starter)
	ar.tools = append(ar.tools, tools.ToolDefinition{
		Name:        "task_flow",
		Description: tools.TaskFlowDefinition.Description,
		InputSchema: tools.TaskFlowInputSchema,
		Function:    nil, // Execution handled specially in executeTool()
	})
	log.Debug("task_flow tool wired successfully", slog.Int("total_tools", len(ar.tools)))
	return nil
}

// SetSpawnA2AChecker wires A2A policy into sessions_spawn for cross-agent authorization
func (ar *AgentRuntime) SetSpawnA2AChecker(checker tools.A2AChecker) {
	if ar.sessionsSpawn != nil {
		ar.sessionsSpawn.SetA2AChecker(checker)
	}
}

// SetToolRouter installs the decision-model contributor to per-turn tool
// narrowing (tool_routing.go). Nil leaves every turn's tools unchanged.
func (ar *AgentRuntime) SetToolRouter(r *toolRouter) { ar.toolRouter = r }

// SetTurnVerdict installs the decision-model check for a turn that ends on an
// announcement the phrase list does not know (turn_verdict.go).
func (ar *AgentRuntime) SetTurnVerdict(v *turnVerdict) { ar.turnVerdict = v }

// SetWorkspaceSettingReader injects a workspace-settings getter (duck-typed to
// avoid a repository import here). Enables admin-configured tool guards; nil
// leaves guards off.
// settingEditFormat: "hashline" numbers reads and describes line-anchored
// edits first (docs/features/HASHLINE.md); anything else is the patch format.
const settingEditFormat = "edit_format"

func (ar *AgentRuntime) SetWorkspaceSettingReader(read func(key string) string) {
	ar.workspaceSetting = read
	tools.SetEditFormat(func() string { return read(settingEditFormat) })
}

// WireSecretTool wires the get_secret tool with the agent secrets repository
func (ar *AgentRuntime) WireSecretTool(secretRepo repository.AgentSecretRepository) {
	log := logs.New("Agent")
	ar.secretRepo = secretRepo

	ar.tools = append(ar.tools, tools.ToolDefinition{
		Name:        "get_secret",
		Description: tools.GetSecretDefinition.Description,
		InputSchema: tools.GetSecretInputSchema,
		Function:    nil,
	})

	log.Debug("get_secret tool wired successfully", slog.Int("total_tools", len(ar.tools)))
}

// EventEmitter returns the agent's event emitter
// Allows external code (e.g., WebSocket handlers) to subscribe to events
func (ar *AgentRuntime) EventEmitter() *infra.EventEmitter {
	return ar.events
}

// repoSecretGetter adapts AgentSecretRepository to tools.SecretGetter
type repoSecretGetter struct {
	repo repository.AgentSecretRepository
}

func (g *repoSecretGetter) Get(agentID, name string) (string, error) {
	return g.repo.Get(context.Background(), agentID, name)
}

// Close gracefully shuts down the agent runtime and its dependencies
// Pattern: Graceful shutdown - ensures all resources are properly released
func (ar *AgentRuntime) Close() error {
	log := logs.New("Agent")
	log.Info("Agent runtime shutting down gracefully")

	// Close session persistence (flushes and closes any resources)
	if ar.persistence != nil {
		if err := ar.persistence.Close(); err != nil {
			log.Warn("Failed to close session persistence",
				slog.String("error", err.Error()))
		}
	}

	// Note: EventEmitter doesn't require cleanup - it's just an in-memory event bus
	// No file handles or network connections to close

	log.Info("Agent runtime closed successfully")
	return nil
}

// Helper: Convert AgentResponse to JSON for WebSocket
func (r *AgentResponse) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// DeletePersistedSession wipes the on-disk conversation history file
// for a given session ID. The /sessions/clear HTTP handler calls this
// after Session.Clear so a "memory wipe" actually removes the
// persisted history that LoadRecentMessages will reload on the next
// turn — without this, a chat session keeps re-loading its old
// conversation from disk and the agent never sees the wipe.
//
// No-op (returns nil) when persistence isn't wired (test harnesses
// without a configured store).
func (ar *AgentRuntime) DeletePersistedSession(sessionID string) error {
	if ar.persistence == nil {
		return nil
	}
	return ar.persistence.DeleteSession(sessionID)
}

// RewindPersistedSession drops the last N turns from the persisted transcript
// (see SessionPersistence.RewindSession). The caller clears the in-memory
// session so the next turn reloads the truncated file.
func (ar *AgentRuntime) RewindPersistedSession(sessionID string, turns int) (int, error) {
	if ar.persistence == nil {
		return 0, fmt.Errorf("session persistence not enabled")
	}
	return ar.persistence.RewindSession(sessionID, turns)
}

// truncateForLog caps a string at maxLen runes for safe inclusion in
// log lines. Tool inputs/outputs can be megabytes (a whole file read, a
// fetched page); shoving them into structured logs
// floods the gateway log file and slows the log query. Truncation
// keeps the trace useful — first N chars are usually enough to see
// what the call was — without polluting the log store.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// hasTool reports whether a tool name is registered in this runtime.
func (ar *AgentRuntime) hasTool(name string) bool {
	for _, t := range ar.tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

// answeringModel is the model a turn is measured against — its window, its
// output cap, the request log — and the remote engine serving it: the model
// that answers (providers.RemoteEngine.AnsweringModel), else the local one.
func (ar *AgentRuntime) answeringModel(ctx context.Context) (string, *providers.RemoteEngine) {
	re := providers.ActiveRemoteEngine()
	if re == nil {
		// No engine chosen at start: the model that answers is the pin, else
		// the connected provider's (providers.GetClientFor's order), and the
		// turn is sized by ITS window.
		if m := providers.ModelFromContext(ctx); m != "" {
			return m, nil
		}
		if m := providers.ConnectedModel(ctx); m != "" {
			return m, nil
		}
		return ar.resolvedModel, nil
	}
	agent, _ := ctx.Value("buddy_agent_name").(string)
	return re.AnsweringModel(ctx, agent), re
}

// maxOutputTokens is the per-inference generation cap: the answering model's
// own (ctxmgmt.GetModelLimits), 8192 when no figure is known. The window reserves exactly that much, so a prompt under the
// effective limit always leaves room for the reply: sized from the engine's
// fixed 262K instead, a 32K model was asked for 16,384 tokens on top of a
// 21K prompt and refused the request (HTTP 400, live 2026-09-29).
func maxOutputTokens(model string) int64 {
	if l, err := ctxmgmt.GetModelLimits(model); err == nil && l.MaxOutputTokens > 0 {
		return int64(l.MaxOutputTokens)
	}
	return 8192
}

// fitReplyToWindow caps a reply at the room the prompt leaves in the
// answering model's window. The normal cap always fits (the window reserves
// it); the escalated retry of a cut-off reply asks for half the window, and
// on a 32K model with a 20K prompt that is a request the provider refuses.
func fitReplyToWindow(want int64, model string, check *ctxmgmt.CheckResult) int64 {
	if check == nil {
		return want
	}
	limits, err := ctxmgmt.GetModelLimits(model)
	if err != nil {
		return want
	}
	if room := int64(limits.ContextWindow - check.TotalTokens); room > 0 && room < want {
		return room
	}
	return want
}

// maxOutputTokensOverrideKey carries the ESCALATED cap for a single retry of a
// turn that was cut off. Context-scoped so it applies to exactly that retry and
// nothing else.
type maxOutputTokensOverrideKey struct{}

// withMaxOutputOverride scopes an escalated cap to a single retry.
func withMaxOutputOverride(ctx context.Context, n int64) context.Context {
	return context.WithValue(ctx, maxOutputTokensOverrideKey{}, n)
}

// maxOutputTokensFor is maxOutputTokens unless this inference is the escalated
// retry of a truncated one.
func maxOutputTokensFor(ctx context.Context, model string) int64 {
	if n, ok := ctx.Value(maxOutputTokensOverrideKey{}).(int64); ok && n > 0 {
		return n
	}
	return maxOutputTokens(model)
}

// escalatedMaxOutputTokens is the LARGER cap a truncated turn is retried at,
// before falling back to multi-turn recovery.
//
// Claude Code's shape (query.ts: tengu_otk_slot_v1 → ESCALATED_MAX_TOKENS): a
// reply cut off at the default cap is retried ONCE at a much bigger one, on the
// theory that the model simply needed more room. Only if that also hits the cap
// does the expensive multi-turn dance begin.
//
// Twice the normal cap, and never more than half the serving window. Half
// the window alone was the rule while every window was 262K (131K); with
// windows following the model, GLM 5.3 Flash (1.31M) was retried at 655,360
// tokens, and a reply that had spent its 16K thinking thought on for minutes
// with nothing written (live, 2026-09-29, a Tetris turn).
func escalatedMaxOutputTokens(model string) int64 {
	out := maxOutputTokens(model) * 2
	if l, err := ctxmgmt.GetModelLimits(model); err == nil && l.ContextWindow > 0 {
		out = min(out, int64(l.ContextWindow/2))
		if l.MaxOutputCeiling > 0 {
			// The model's own cap, as the reference states it: asking
			// past it is a refusal, not a longer reply.
			out = min(out, int64(l.MaxOutputCeiling))
		}
	}
	return out
}
