package streaming

import "time"

// AgentEventStream represents different event streams
type AgentEventStream string

const (
	StreamLifecycle AgentEventStream = "lifecycle"
	StreamAssistant AgentEventStream = "assistant"
	StreamTool      AgentEventStream = "tool"
	StreamContext   AgentEventStream = "context"
)

// AgentEvent represents a streaming event
// Pattern: OpenClaw emitAgentEvent
type AgentEvent struct {
	RunID     string                 `json:"run_id"`
	Stream    AgentEventStream       `json:"stream"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// LifecycleEventType represents lifecycle event types
type LifecycleEventType string

const (
	LifecycleRunStart    LifecycleEventType = "run_start"
	LifecycleRunComplete LifecycleEventType = "run_complete"
	LifecycleRunError    LifecycleEventType = "run_error"
)

// AssistantEventType represents assistant message event types
// Pattern: OpenClaw text_delta, text_start, text_end
type AssistantEventType string

const (
	AssistantTextStart AssistantEventType = "text_start"
	AssistantTextDelta AssistantEventType = "text_delta"
	AssistantTextEnd   AssistantEventType = "text_end"
)

// ToolEventType represents tool execution event types
type ToolEventType string

const (
	ToolStart    ToolEventType = "tool_start"
	ToolProgress ToolEventType = "tool_progress"
	ToolComplete ToolEventType = "tool_complete"
	ToolError    ToolEventType = "tool_error"
)

// ContextEventType represents context usage event types
// Pattern: OpenClaw context tracking for TUI display
type ContextEventType string

const (
	ContextUpdate   ContextEventType = "context_update"   // Regular context usage update
	ContextWarning  ContextEventType = "context_warning"  // Context approaching limit (>60%)
	ContextOverflow ContextEventType = "context_overflow" // Context over limit
)

// BlockChunkingConfig configures block chunking behavior
// Pattern: OpenClaw BlockReplyChunking
type BlockChunkingConfig struct {
	MinChars        int    `json:"min_chars"`
	MaxChars        int    `json:"max_chars"`
	BreakPreference string `json:"break_preference"` // "paragraph", "newline", "sentence"
}
