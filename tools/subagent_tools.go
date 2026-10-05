package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/subagents"
	"memdoor/pkg/shared"

	"github.com/google/uuid"
)

// =============================================================================
// sessions_spawn tool - Spawn a background subagent for parallel task execution
// =============================================================================

// A2AChecker checks if agent-to-agent communication is allowed
// Duck-typed interface to avoid importing pkg/authorization
type A2AChecker interface {
	IsAllowed(fromAgentID, toAgentID string) bool
}

// SessionsSpawnTool holds dependencies for the sessions_spawn tool
// Pattern: OpenClaw src/agents/tools/sessions-spawn-tool.ts
type SessionsSpawnTool struct {
	registry      *subagents.SubagentRegistry
	queueEnqueuer QueueEnqueuer // Interface to enqueue jobs
	sessionGetter SessionGetter // Interface to get/create sessions
	a2aChecker    A2AChecker    // Optional A2A policy checker
	// resolveConfig returns the target agent's palette + system prompt, resolved
	// HERE at spawn time (on the requester's turn) so the subagent is built WITH its
	// config and carries it on the job — rather than the executor re-fetching it
	// under single-connection DB contention (which intermittently gave the coder no
	// palette). Optional; nil means "no config carried" (executor falls back).
	resolveConfig func(agentID string) (tools []string, systemPrompt string)
	log           *logs.EventLogger
}

// QueueEnqueuer is an interface for enqueueing subagent jobs
// This abstracts the QueueManager to avoid circular dependencies. tools + prompt
// are the target agent's resolved config, carried on the job so the subagent runs
// AS that agent without a DB re-fetch at execution time.
type QueueEnqueuer interface {
	// workdir is the REQUESTER's working directory; the subagent runs in it
	// (empty = the server default).
	EnqueueSubagentJob(runID, sessionKey, agentID, message string, tools []string, systemPrompt, workdir string, timeout time.Duration) error
}

// SessionGetter is an interface for getting/creating sessions
// This abstracts session management to avoid circular dependencies
type SessionGetter interface {
	GetOrCreateSession(sessionKey, kind string) (Session, error)
}

// RequesterWorkdirKey is the child session's record of where its requester
// works, read back when the run's announcement is built.
const RequesterWorkdirKey = "requester_workdir"

// RequesterSessionKey is the child session's record of WHO is waiting for it.
// A spawned run gets its own run id and its own session key, and events are
// broadcast per session — so everything a sub-session did was invisible to the
// screen that asked for it: a ten-minute render showed a spinner still saying
// "Writing a Captions call" (live, 2026-09-16). With the requester's key on
// the child, the runtime can mirror the child's progress onto the screen that
// is waiting.
const RequesterSessionKey = "requester_session_key"

// Session represents a minimal session interface
type Session interface {
	GetKey() string
	SetSystemPrompt(prompt string) error
	SetMetadata(key string, value interface{})
	GetMetadata() map[string]interface{} // Return all metadata
}

// NewSessionsSpawnTool creates a new sessions_spawn tool
func NewSessionsSpawnTool(
	registry *subagents.SubagentRegistry,
	queueEnqueuer QueueEnqueuer,
	sessionGetter SessionGetter,
	resolveConfig func(agentID string) (tools []string, systemPrompt string),
	verbose bool,
) *SessionsSpawnTool {
	return &SessionsSpawnTool{
		registry:      registry,
		queueEnqueuer: queueEnqueuer,
		sessionGetter: sessionGetter,
		resolveConfig: resolveConfig,
		log:           logs.New("Agent"),
	}
}

// SetA2AChecker sets the A2A policy checker for cross-agent spawn authorization
func (t *SessionsSpawnTool) SetA2AChecker(checker A2AChecker) {
	t.a2aChecker = checker
}

// SessionsSpawnInput defines the parameters for spawning a subagent
type SessionsSpawnInput struct {
	Task              string `json:"task" jsonschema_description:"The specific task for the subagent to complete. Be clear and focused."`
	Label             string `json:"label,omitempty" jsonschema_description:"Optional short label for this subagent run (for tracking)."`
	AgentID           string `json:"agentId,omitempty" jsonschema_description:"Agent ID to use (default: the agent that spawns it). Cross-agent spawning requires permissions."`
	Model             string `json:"model,omitempty" jsonschema_description:"Override model for this subagent (e.g., 'claude-sonnet-4', 'claude-haiku-4')."`
	Thinking          string `json:"thinking,omitempty" jsonschema_description:"Thinking mode: 'enabled', 'disabled', or 'auto' (default: disabled for subagents)."`
	RunTimeoutSeconds int    `json:"runTimeoutSeconds,omitempty" jsonschema_description:"Maximum runtime in seconds (default: 300). Subagent will be terminated if exceeded."`
	Cleanup           string `json:"cleanup,omitempty" jsonschema_description:"Cleanup mode: 'delete' (remove session after completion) or 'keep' (preserve for inspection). Default: 'delete'."`
}

// GenerateSessionsSpawnInputSchema generates the JSON schema for the input
var SessionsSpawnInputSchema = GenerateSchema[SessionsSpawnInput]()

// SessionsSpawnDefinition is the tool definition for sessions_spawn
var SessionsSpawnDefinition = ToolDefinition{
	Name: "sessions_spawn",
	Description: `Spawn a background subagent to handle a specific task in parallel.

Use this when you need to:
- Execute a task in the background while continuing to respond to the user
- Perform parallel operations (e.g., search docs while analyzing code)
- Delegate a well-defined subtask to a focused subagent

The subagent will:
- Execute in isolation with its own session
- Focus solely on the assigned task
- Report results back when complete
- Be ephemeral (can be auto-deleted after completion)

Important:
- The subagent will NOT have access to this conversation's context
- Provide a clear, self-contained task description
- Results will be announced back to you when the subagent completes
- You can continue working while the subagent runs in the background

Example uses:
- "Search documentation for error handling best practices"
- "Run the test suite and report any failures"
- "Analyze code complexity in the auth module"`,
	InputSchema: SessionsSpawnInputSchema,
	Function:    nil, // Will be set by binding to a specific instance
}

// Execute implements the sessions_spawn tool logic
func (t *SessionsSpawnTool) Execute(input json.RawMessage, requesterSessionKey, requesterAgentID, workspaceDir, workdir string) (string, error) {
	// Fallback to session lookup if no session object is provided
	// This happens when called from old code paths
	requesterSession, err := t.sessionGetter.GetOrCreateSession(requesterSessionKey, "")
	if err != nil {
		return "", fmt.Errorf("failed to get requester session: %w", err)
	}
	return t.ExecuteWithSession(input, requesterSession, requesterAgentID, workspaceDir, workdir)
}

// ExecuteWithSession implements the sessions_spawn tool logic with direct session access
// Pattern: Thread-safe execution by using the session object directly instead of SessionManager lookup
// This avoids race conditions when multiple agents work concurrently
// workdir is the requester's turn directory: a coder spawned by another agent
// must write where that agent works, not in the coder's scratch dir
// (2026-09-02: EnqueueSubagentJob started every subagent on an empty context,
// so a spawned coder always fell back to ~/memdoor-coder).
func (t *SessionsSpawnTool) ExecuteWithSession(input json.RawMessage, requesterSession Session, requesterAgentID, workspaceDir, workdir string) (string, error) {
	var params SessionsSpawnInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	requesterSessionKey := requesterSession.GetKey()

	t.log.Info("Spawning subagent",
		slog.String("task", params.Task),
		slog.String("label", params.Label),
		slog.String("agent_id", params.AgentID),
		slog.String("requester_session_key", requesterSessionKey))

	// Validate parameters
	if err := t.validateParams(params); err != nil {
		return "", err
	}

	// Set defaults
	agentID := params.AgentID
	if agentID == "" {
		agentID = requesterAgentID
	}

	cleanup := params.Cleanup
	if cleanup == "" {
		cleanup = "delete" // Default to delete for cleanliness
	}

	runTimeoutSeconds := params.RunTimeoutSeconds
	if runTimeoutSeconds == 0 {
		runTimeoutSeconds = 300 // Default 5 minutes
	}

	// Check cross-agent permissions (if spawning to different agent)
	if agentID != requesterAgentID {
		if err := t.checkCrossAgentPermissions(requesterAgentID, agentID); err != nil {
			return "", err
		}
	}

	// Generate run ID and child session key
	runID := generateRunID()
	childSessionKey := fmt.Sprintf("agent:%s:subagent:%s", agentID, runID)

	// Extract parent message ID from requester session metadata (for threading responses)
	// Pattern: Thread-safe - uses the session object passed directly from agent execution
	// This avoids the race condition of looking up session from SessionManager
	var parentMessageID int64
	meta := requesterSession.GetMetadata()
	t.log.Debug("Retrieved requester session metadata",
		slog.Any("metadata", meta),
		slog.String("session_key", requesterSessionKey))

	if meta != nil {
		if msgID, ok := meta["parent_message_id"].(int64); ok {
			parentMessageID = msgID
			t.log.Info("✓ Captured parent message ID for subagent threading",
				slog.Int64("parent_message_id", parentMessageID),
				slog.String("run_id", runID))
		} else {
			t.log.Warn("⚠ parent_message_id not found in session metadata",
				slog.String("session_key", requesterSessionKey))
		}
	} else {
		t.log.Warn("⚠ Session metadata is nil",
			slog.String("session_key", requesterSessionKey))
	}

	// Create subagent run record
	record := &subagents.SubagentRunRecord{
		RunID:               runID,
		ChildSessionKey:     childSessionKey,
		RequesterSessionKey: requesterSessionKey,
		RequesterDisplayKey: fmt.Sprintf("Agent %s", requesterAgentID),
		Task:                params.Task,
		Cleanup:             cleanup,
		Label:               params.Label,
		ParentMessageID:     parentMessageID, // Store for threading subagent responses
		CreatedAt:           time.Now(),
		ArchiveAtMs:         time.Now().Add(24 * time.Hour).UnixMilli(), // Auto-archive after 24h
	}

	// Register run in registry
	if err := t.registry.Register(record); err != nil {
		return "", fmt.Errorf("failed to register subagent run: %w", err)
	}

	// Create subagent session with special system prompt
	session, err := t.sessionGetter.GetOrCreateSession(childSessionKey, "subagent")
	if err != nil {
		return "", fmt.Errorf("failed to create subagent session: %w", err)
	}

	// Extract parent channel ID from requester session and store in subagent metadata
	// This allows todo events from subagents to be broadcast to the correct channel
	parsedSessionID := shared.ParseSessionID(requesterSessionKey)
	if parentChannelID := parsedSessionID.GetChannelID(); parentChannelID != "" {
		session.SetMetadata("parent_channel_id", parentChannelID)
	}
	// Who is waiting: the screen subscribed to the requester's session is the
	// one that must hear this run's progress.
	if requesterSessionKey != "" {
		session.SetMetadata(RequesterSessionKey, requesterSessionKey)
	}
	if workdir != "" {
		// The announcement that wakes the requester when this run ends must
		// run where the requester works: without it the requester's retry ran
		// in the coder's scratch directory (2026-09-15 11:58, a Valve trailer
		// fetched into ~/memdoor-coder while the film lived in ~/zapping).
		session.SetMetadata(RequesterWorkdirKey, workdir)
	}

	// Build and set subagent system prompt
	promptParams := subagents.SubagentPromptParams{
		Task:         params.Task,
		AgentID:      agentID,
		WorkspaceDir: workspaceDir,
		Label:        params.Label,
	}
	systemPrompt := subagents.BuildSubagentSystemPrompt(promptParams)
	if err := session.SetSystemPrompt(systemPrompt); err != nil {
		return "", fmt.Errorf("failed to set subagent system prompt: %w", err)
	}

	// Resolve the target agent's config HERE, at spawn time, so the subagent is
	// built WITH its palette + prompt and carries them on the job — no DB re-fetch
	// (and no contention race) when it runs. FAIL FAST if it can't be built: a
	// spawned agent with no palette is a bug, not a degraded mode, so we refuse to
	// launch a blank default agent.
	var subTools []string
	var subPrompt string
	if t.resolveConfig != nil {
		subTools, subPrompt = t.resolveConfig(agentID)
	}
	if len(subTools) == 0 {
		return "", fmt.Errorf("cannot spawn agent %q: its tool palette could not be resolved (agent missing or has no tools)", agentID)
	}

	// Enqueue job in subagent lane with task as initial message
	// runTimeoutSeconds was accepted, echoed back and never enforced: a
	// sub-session that read transcripts and planned in prose ran 18 minutes
	// past its 900 s (2026-09-14 22:0x). The limit rides on the job; the
	// executor cancels the turn when it expires and tells the requester.
	if err := t.queueEnqueuer.EnqueueSubagentJob(runID, childSessionKey, agentID, params.Task, subTools, subPrompt, workdir, time.Duration(runTimeoutSeconds)*time.Second); err != nil {
		return "", fmt.Errorf("failed to enqueue subagent job: %w", err)
	}

	t.log.Info("Successfully spawned subagent",
		slog.String("run_id", runID),
		slog.String("session_key", childSessionKey))

	return spawnedLine(runID, params.Label, agentID, childSessionKey, runTimeoutSeconds), nil
}

// spawnedLine is what the spawning agent (and the window) reads back: one
// line, like the cron tool's, not a JSON record (Greg, 2026-10-05). The
// run id and the session are in it for anyone who needs them.
func spawnedLine(runID, label, agentID, session string, timeout int) string {
	name := runID
	if label != "" {
		name = fmt.Sprintf("%s %q", runID, label)
	}
	return fmt.Sprintf("▶ %s spawned as %s (session %s, up to %ds). It runs in the background and reports here when done — "+
		"do not wait or poll; end your turn.", name, agentID, session, timeout)
}

// validateParams validates spawn parameters
func (t *SessionsSpawnTool) validateParams(params SessionsSpawnInput) error {
	if params.Task == "" {
		return fmt.Errorf("task is required")
	}

	if len(params.Task) > 10000 {
		return fmt.Errorf("task is too long (max 10000 characters)")
	}

	if params.Cleanup != "" && params.Cleanup != "delete" && params.Cleanup != "keep" {
		return fmt.Errorf("cleanup must be 'delete' or 'keep'")
	}

	if params.RunTimeoutSeconds < 0 || params.RunTimeoutSeconds > 3600 {
		return fmt.Errorf("runTimeoutSeconds must be between 0 and 3600 (1 hour)")
	}

	if params.Thinking != "" && params.Thinking != "enabled" && params.Thinking != "disabled" && params.Thinking != "auto" {
		return fmt.Errorf("thinking must be 'enabled', 'disabled', or 'auto'")
	}

	return nil
}

// checkCrossAgentPermissions checks if cross-agent spawning is allowed
func (t *SessionsSpawnTool) checkCrossAgentPermissions(requesterAgentID, targetAgentID string) error {
	if t.a2aChecker == nil {
		// No policy configured — allow (backward compatible)
		t.log.Debug("Cross-agent spawn allowed (no policy)",
			slog.String("requester", requesterAgentID),
			slog.String("target", targetAgentID))
		return nil
	}

	if !t.a2aChecker.IsAllowed(requesterAgentID, targetAgentID) {
		t.log.Warn("Cross-agent spawn denied by A2A policy",
			slog.String("requester", requesterAgentID),
			slog.String("target", targetAgentID))
		return fmt.Errorf("cross-agent spawn denied: agent '%s' is not allowed to spawn agent '%s'", requesterAgentID, targetAgentID)
	}

	t.log.Debug("Cross-agent spawn allowed by A2A policy",
		slog.String("requester", requesterAgentID),
		slog.String("target", targetAgentID))
	return nil
}

// generateRunID generates a unique run ID for the subagent
func generateRunID() string {
	return fmt.Sprintf("run-%s", uuid.New().String()[:8])
}

// BindToContext creates a bound function for use in ToolDefinition
// This allows the tool to be used with the standard tool execution pattern
func (t *SessionsSpawnTool) BindToContext(requesterSessionKey, requesterAgentID, workspaceDir string) func(json.RawMessage) (string, error) {
	return func(input json.RawMessage) (string, error) {
		// Legacy binding without a turn context: no requester directory to carry.
		return t.Execute(input, requesterSessionKey, requesterAgentID, workspaceDir, "")
	}
}
