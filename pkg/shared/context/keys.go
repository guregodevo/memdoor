package context

// ContextKey is a type for context keys used across the application
// This shared definition prevents type mismatches when passing values through context
type ContextKey string

const (
	// ActorIDKey is the context key for the authenticated actor ID
	ActorIDKey ContextKey = "actor_id"

	// AgentIDKey names the agent a turn runs as (the buddy's id or name), so
	// a metered turn can be attributed to who asked and who answered.
	AgentIDKey ContextKey = "agent_id"

	// SessionIDKey is the session a turn belongs to.
	SessionIDKey ContextKey = "turn_session_id"

	// TierKey is the rung of the agent's model ladder the turn runs on (an
	// int; 0 or absent = the cheap default). Read from the session's
	// "route_tier" metadata.
	TierKey ContextKey = "route_tier"

	// ModelKey (string) is a model the person pinned by id for the
	// conversation, off the agent's ladder, served exactly.
	ModelKey ContextKey = "route_model"

	// SortKey and OrderKey (strings) are the person's host preference for a
	// model pinned by id: a sort (price | throughput | latency | default) and
	// their own host order (providers/byok.go).
	SortKey  ContextKey = "route_sort"
	OrderKey ContextKey = "route_order"

	// EffortKey (string) is the turn's reasoning effort: low | medium | high,
	// sent as reasoning.effort to a model that takes it.
	EffortKey ContextKey = "route_effort"

	// WorkspaceIDKey is the context key for the authenticated user's workspace ID
	WorkspaceIDKey ContextKey = "workspace_id"

	// SandboxContextKey is the context key for the sandbox execution context
	SandboxContextKey ContextKey = "sandbox_context"

	// IsBuddyChatKey marks the request as a buddy-driven chat (vs.
	// a config-file agent). Used to pick the lean chat prompt mode.
	// Replaces the old BuddyProviderKey signal — provider/model are
	// no longer per-buddy, they're workspace-wide via byok, but we
	// still need a flag to distinguish buddy vs. config-agent runs.
	IsBuddyChatKey ContextKey = "is_buddy_chat"

	// BuddyLearningKey is the context key for the per-agent learning enabled flag
	BuddyLearningKey ContextKey = "buddy_learning_enabled"

	// BuddyToolsKey is the context key for the per-agent tool list
	BuddyToolsKey ContextKey = "buddy_tools"

	// BuddyTemperatureKey is the context key for the per-agent sampling
	// temperature. The buddy row stores it (pkg/domain.Buddy.Temperature);
	// the agent runtime reads it here to pass through to the LLM provider.
	// Zero means "use provider default", same as llm.ChatRequest.
	BuddyTemperatureKey ContextKey = "buddy_temperature"

	// PermissionModeKey carries the user's current interaction mode for the turn:
	// "default", "acceptEdits", or "plan" (Claude-style, cycled with Shift+Tab).
	// The agent runtime gates tool execution on it — plan mode is read-only, so
	// the single coding agent researches and calls exit_plan_mode instead of
	// mutating; acceptEdits/default allow edits. Empty means "default".
	PermissionModeKey ContextKey = "permission_mode"

	// WorkdirKey carries the client's working directory for the turn — the
	// directory the TUI was LAUNCHED from (Claude-CLI semantics: cd into your
	// project, run memdoor tui, the coder works on THAT project). The coder's
	// file/bash/locate/verify confinement roots here; empty falls back to
	// ~/memdoor-coder.
	WorkdirKey ContextKey = "workdir"
)
