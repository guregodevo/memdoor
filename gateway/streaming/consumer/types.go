package consumer

import (
	"time"

	"memdoor/gateway/infra"
)

// ProcessedEvent represents a consumer-processed event ready for UI consumption
// This is the output of the consumer layer, converted from raw AgentEvent
type ProcessedEvent struct {
	RunID         string                 // Agent run identifier
	SessionID     string                 // Session identifier (for multi-session UIs)
	EventType     string                 // Type of event: "thinking", "text_delta", "text_end", "tool_start", "tool_complete", "lifecycle_start", "lifecycle_complete", "lifecycle_error"
	TextDelta     string                 // Incremental text chunk (for streaming)
	FullText      string                 // Complete text (for final events)
	TextAppend    bool                   // FullText is only a tail: add it under what streamed
	ToolUpdate    *ToolState             // Tool execution state update
	ThinkingState bool                   // Whether assistant is in thinking state
	Finalized     bool                   // Whether this run is finalized
	Timestamp     time.Time              // Event timestamp
	Error         string                 // Error message if any
	Metadata      map[string]interface{} // Additional metadata
}

// ToolState represents the state of a tool execution
type ToolState struct {
	Name      string     // Tool name (e.g., "Read", "Edit", "Bash")
	Status    string     // "running", "complete", "error"
	Input     string     // Tool input (JSON or string)
	Output    string     // Tool output/result
	Error     string     // Error message if status is "error"
	StartTime time.Time  // When tool execution started
	EndTime   *time.Time // When tool execution ended (nil if still running)
}

// EventHandler is the callback function that UIs register to receive processed events
// This is the main interface between the consumer and UI layers
type EventHandler func(event *ProcessedEvent)

// RunState tracks the accumulated state for a single agent run
// Used internally by StreamAssembler to build text incrementally
type RunState struct {
	RunID       string                // Agent run ID
	SessionID   string                // Session ID
	Text        string                // Accumulated text so far
	ToolEvents  map[string]*ToolState // Active tool executions (keyed by tool name)
	IsThinking  bool                  // Whether currently in thinking state
	LastUpdated time.Time             // Last update timestamp
	Finalized   bool                  // Whether run is complete
}

// UpdateResult represents what changed in a run state after processing an event
// Returned by StreamAssembler.ProcessEvent to inform consumers what to update
type UpdateResult struct {
	RunID         string     // Run that was updated
	TextDelta     string     // New text added (empty if no text change)
	FullText      string     // Complete accumulated text
	ToolUpdate    *ToolState // Tool state change (nil if no tool update)
	ThinkingState bool       // Current thinking state
	Finalized     bool       // Whether run is now finalized
	EventType     string     // Type of event that caused this update
	TextAppend    bool       // the "text" event carried only a tail to append
}

// Typed event data structures - parsed from infra.AgentEvent.Data based on Stream type

// LifecycleEventData represents parsed lifecycle event data
// Stream: "lifecycle"
type LifecycleEventData struct {
	Event   string `json:"event"`   // "start", "complete", "end", "error"
	Message string `json:"message"` // Optional message
	Error   string `json:"error"`   // Error message if event is "error"
	Model   string `json:"model"`   // The model that answered, on "complete"
}

// ToolEventData represents parsed tool event data
// Stream: "tool"
type ToolEventData struct {
	Event  string `json:"event"`  // "start", "update", "result", "complete", "progress", "subagent"
	Tool   string `json:"tool"`   // Tool name
	Input  string `json:"input"`  // Tool input (JSON or string)
	Output string `json:"output"` // Tool output/result
	Error  string `json:"error"`  // Error message if tool failed
	// The status beats (gateway/agent_runtime_progress.go): "progress" is a
	// running tool saying how long it has run; "subagent" is a spawned run
	// telling the screen that asked for it what it is doing. Neither carries
	// a result, so they build no ToolState — the screen reads these fields.
	Seconds int    `json:"seconds"` // how long the tool (or the spawned run) has been at it
	Agent   string `json:"agent"`   // subagent: the agent doing the work
	Session string `json:"session"` // subagent: the spawned run's session
	State   string `json:"state"`   // subagent: "running" or "done"
}

// AssistantEventData represents parsed assistant event data
// Stream: "assistant"
type AssistantEventData struct {
	Event string `json:"event"` // "thinking", "text_start", "text_delta", "text_end", "text"
	Text  string `json:"text"`  // Text content (for text_end / text / thinking)
	// Delta is the chunk on a text_delta event. The emitter names it "delta"
	// (agent_runtime_inference.go), and this struct had only Text — so every
	// streamed token decoded to "" and the TUI rendered nothing while the engine
	// generated for minutes. An unmatched JSON field is not an error, it is a
	// zero value, which is exactly why this was invisible.
	Delta string `json:"delta"`
	// Append marks a final "text" event that carries only what did NOT
	// stream (the receipt line, a note): it goes UNDER the streamed text,
	// never in its place.
	Append bool `json:"append"`
}

// Chunk is the streamed text on this event, whichever key carried it.
func (d AssistantEventData) Chunk() string {
	if d.Delta != "" {
		return d.Delta
	}
	return d.Text
}

// ErrorEventData represents parsed error event data
// Stream: "error"
type ErrorEventData struct {
	Event   string `json:"event"`   // "error"
	Message string `json:"message"` // Error message
	Error   string `json:"error"`   // The runtime's key for the same thing (agent_runtime_process.go)
	Code    string `json:"code"`    // Error code (optional)
}

// ContextEventData represents parsed context event data
// Stream: "context"
type ContextEventData struct {
	Event   string  `json:"event"`   // "context_update", "context_warning", "context_overflow"
	Tokens  int     `json:"tokens"`  // Current token count
	Limit   int     `json:"limit"`   // Context window limit
	Percent float64 `json:"percent"` // Utilization percentage
	// What is eating the window, and where compaction starts (/context).
	System       int    `json:"system"`
	Tools        int    `json:"tools"`
	Conversation int    `json:"conversation"`
	ToolOutput   int    `json:"tool_output"` // part of Conversation
	CompactAt    int    `json:"compact_at"`
	CompactRule  string `json:"compact_rule"` // where CompactAt comes from
	KeepRecent   int    `json:"keep_recent"`  // what /compact keeps word for word
}

// ParsedEventData is a union type for all parsed event data structures
// Discriminator pattern: use type assertion to get specific event type
type ParsedEventData interface {
	isEventData() // Marker method
}

func (LifecycleEventData) isEventData() {}
func (ToolEventData) isEventData()      {}
func (AssistantEventData) isEventData() {}
func (ErrorEventData) isEventData()     {}
func (ContextEventData) isEventData()   {}

// Consumer options for configuration
type ConsumerOptions struct {
	// DeduplicationTTL is how long to remember finalized runs (default: 10 minutes)
	DeduplicationTTL time.Duration

	// EnableLogging enables debug logging for event processing
	EnableLogging bool

	// MaxRunStates limits how many concurrent run states to track (default: 100)
	MaxRunStates int

	// PruneInterval is how often to prune old run states (default: 1 minute)
	PruneInterval time.Duration
}

// DefaultConsumerOptions returns sensible defaults
func DefaultConsumerOptions() ConsumerOptions {
	return ConsumerOptions{
		DeduplicationTTL: 10 * time.Minute,
		EnableLogging:    false,
		MaxRunStates:     100,
		PruneInterval:    1 * time.Minute,
	}
}

// StreamType represents the type of event stream
type StreamType string

const (
	StreamLifecycle StreamType = "lifecycle"
	StreamTool      StreamType = "tool"
	StreamAssistant StreamType = "assistant"
	StreamError     StreamType = "error"
	StreamContext   StreamType = "context"
)

// ConvertInfraStream converts infra.AgentEventStream to consumer.StreamType
func ConvertInfraStream(stream infra.AgentEventStream) StreamType {
	return StreamType(string(stream))
}
