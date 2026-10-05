package shared

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EventCategory classifies events into broad groups for UI rendering and filtering
// SHARED KERNEL: Used by both Team Collaboration (persistent) and Multi-Agent Platform (transient)
type EventCategory string

const (
	EventCategoryLifecycle EventCategory = "lifecycle" // Start, stop, state transitions
	EventCategoryThinking  EventCategory = "thinking"  // Reasoning, planning, analysis
	EventCategoryTool      EventCategory = "tool"      // Tool invocations and results
	EventCategoryResponse  EventCategory = "response"  // Content generation
	EventCategoryError     EventCategory = "error"     // Failures and exceptions
)

// EventType identifies the specific event within a category
// SHARED KERNEL: Unified event taxonomy across both contexts
type EventType string

// Lifecycle Events - Execution state transitions
const (
	EventExecutionQueued    EventType = "execution.queued"
	EventExecutionStarted   EventType = "execution.started"
	EventExecutionCompleted EventType = "execution.completed"
	EventExecutionFailed    EventType = "execution.failed"
	EventExecutionTimeout   EventType = "execution.timeout"
	EventExecutionCancelled EventType = "execution.cancelled"
)

// Thinking Events - AI reasoning and planning
const (
	EventThinkingStarted   EventType = "thinking.started"
	EventThinkingCompleted EventType = "thinking.completed"
)

// Tool Events - Tool invocation lifecycle
const (
	EventToolCallStarted   EventType = "tool.call.started"
	EventToolCallCompleted EventType = "tool.call.completed"
	EventToolCallFailed    EventType = "tool.call.failed"
)

// Response Events - Content generation
const (
	EventResponseGenerated EventType = "response.generated"
	EventResponseStreaming EventType = "response.streaming" // Streaming chunks
)

// Error Events - Failures
const (
	EventErrorOccurred EventType = "error.occurred"
)

// Event represents a single event in an execution timeline
// SHARED KERNEL: Core event structure used by both bounded contexts
//
// Team Collaboration uses this for:
//   - Persistent ExecutionEvent (stored in database)
//   - Audit trail and debugging
//   - Billing and token tracking
//
// Multi-Agent Platform uses this for:
//   - Transient AgentEvent (WebSocket streaming)
//   - Real-time UI updates
//   - Progress indicators
type Event struct {
	// Type is the specific event (e.g., "execution.started", "thinking.started")
	Type EventType

	// Category is the broad classification for UI rendering
	Category EventCategory

	// Data contains the event payload (flexible map for extensibility)
	// Common fields:
	//   - "thoughts" (string) - for thinking events
	//   - "tool" (string) - for tool events
	//   - "arguments" (map) - for tool events
	//   - "result" (any) - for tool events
	//   - "content" (string) - for response events
	//   - "error" (string) - for error events
	Data map[string]interface{}

	// Timestamp is when the event occurred (UTC)
	Timestamp time.Time
}

// GetCategory returns the category for this event type
// Parses the type prefix: "execution.started" → "lifecycle"
func (et EventType) GetCategory() EventCategory {
	parts := strings.SplitN(string(et), ".", 2)
	if len(parts) < 2 {
		return EventCategoryLifecycle // Default
	}

	prefix := parts[0]
	switch prefix {
	case "execution":
		return EventCategoryLifecycle
	case "thinking":
		return EventCategoryThinking
	case "tool":
		return EventCategoryTool
	case "response":
		return EventCategoryResponse
	case "error":
		return EventCategoryError
	default:
		return EventCategoryLifecycle
	}
}

// String returns the string representation of the event type
func (et EventType) String() string {
	return string(et)
}

// String returns the string representation of the category
func (ec EventCategory) String() string {
	return string(ec)
}

// Validate checks if the event is valid
func (e *Event) Validate() error {
	if e.Type == "" {
		return fmt.Errorf("event type cannot be empty")
	}
	if e.Category == "" {
		return fmt.Errorf("event category cannot be empty")
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("event timestamp cannot be zero")
	}
	return nil
}

// GetString extracts a string value from event data
func (e *Event) GetString(key string) (string, bool) {
	val, ok := e.Data[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt extracts an int value from event data
func (e *Event) GetInt(key string) (int, bool) {
	val, ok := e.Data[key]
	if !ok {
		return 0, false
	}

	// Handle both int and float64 (JSON unmarshaling default)
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetMap extracts a map value from event data
func (e *Event) GetMap(key string) (map[string]interface{}, bool) {
	val, ok := e.Data[key]
	if !ok {
		return nil, false
	}
	m, ok := val.(map[string]interface{})
	return m, ok
}

// MarshalJSON customizes JSON serialization
func (e *Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type      string                 `json:"type"`
		Category  string                 `json:"category"`
		Data      map[string]interface{} `json:"data"`
		Timestamp int64                  `json:"timestamp"` // Unix milliseconds for JavaScript
	}{
		Type:      string(e.Type),
		Category:  string(e.Category),
		Data:      e.Data,
		Timestamp: e.Timestamp.UnixMilli(),
	})
}

// UnmarshalJSON customizes JSON deserialization
func (e *Event) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type      string                 `json:"type"`
		Category  string                 `json:"category"`
		Data      map[string]interface{} `json:"data"`
		Timestamp int64                  `json:"timestamp"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	e.Type = EventType(raw.Type)
	e.Category = EventCategory(raw.Category)
	e.Data = raw.Data
	e.Timestamp = time.UnixMilli(raw.Timestamp).UTC()

	return nil
}

// IsLifecycle returns true if this is a lifecycle event
func (e *Event) IsLifecycle() bool {
	return e.Category == EventCategoryLifecycle
}

// IsThinking returns true if this is a thinking event
func (e *Event) IsThinking() bool {
	return e.Category == EventCategoryThinking
}

// IsTool returns true if this is a tool event
func (e *Event) IsTool() bool {
	return e.Category == EventCategoryTool
}

// IsResponse returns true if this is a response event
func (e *Event) IsResponse() bool {
	return e.Category == EventCategoryResponse
}

// IsError returns true if this is an error event
func (e *Event) IsError() bool {
	return e.Category == EventCategoryError
}
