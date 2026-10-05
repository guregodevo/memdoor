package logs

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event
type EventType string

const (
	// Lifecycle Events
	EventAgentStarted   EventType = "agent_started"
	EventAgentStopped   EventType = "agent_stopped"
	EventSessionStarted EventType = "session_started"
	EventSessionEnded   EventType = "session_ended"
	EventRunStarted     EventType = "run_started"
	EventRunCompleted   EventType = "run_completed"

	// Message Events
	EventMessageReceived EventType = "message_received"
	EventMessageSent     EventType = "message_sent"

	// Tool Events
	EventToolCalled    EventType = "tool_called"
	EventToolCompleted EventType = "tool_completed"
	EventToolFailed    EventType = "tool_failed"

	// Processing Events
	EventThinkingStarted     EventType = "thinking_started"
	EventThinkingFinished    EventType = "thinking_finished"
	EventCompactionStarted   EventType = "compaction_started"
	EventCompactionCompleted EventType = "compaction_completed"

	// State Events
	EventStateChange EventType = "state_change"
	EventError       EventType = "error"
	EventMetric      EventType = "metric"

	// Legacy/Compatibility Events
	EventCompaction EventType = "compaction"
	EventWebSocket  EventType = "websocket"
	EventQueue      EventType = "queue"
)

// Level represents log severity
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Event is the core unit of the logging system
// Each event represents something that happened, with full context
type Event struct {
	// Identity
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Event metadata
	Type      EventType `json:"type"`
	Component string    `json:"component"`
	Level     Level     `json:"level"`

	// Context
	Session string `json:"session,omitempty"`
	RunID   string `json:"run_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"` // For distributed tracing

	// Causality - this is what makes it agent-friendly!
	ParentID string `json:"parent_id,omitempty"` // What triggered this
	RootID   string `json:"root_id,omitempty"`   // Root cause event

	// Content
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
	Error   *ErrorInfo     `json:"error,omitempty"`

	// Performance
	Duration time.Duration `json:"duration,omitempty"` // How long this took

	// Outcome
	Success bool `json:"success"`
}

// ErrorInfo captures rich error context
type ErrorInfo struct {
	Type      string `json:"type"`                 // connection_error, timeout, etc.
	Message   string `json:"message"`              // Error message
	Stack     string `json:"stack,omitempty"`      // Stack trace
	Retryable bool   `json:"retryable"`            // Can this be retried?
	RootCause string `json:"root_cause,omitempty"` // ID of causal event
}

// NewEvent creates a new event with defaults
func NewEvent(eventType EventType, component string, message string) *Event {
	return &Event{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Type:      eventType,
		Component: component,
		Level:     LevelInfo,
		Message:   message,
		Data:      make(map[string]any),
		Success:   true,
	}
}

// WithLevel sets the log level
func (e *Event) WithLevel(level Level) *Event {
	e.Level = level
	return e
}

// WithSession sets the session ID
func (e *Event) WithSession(sessionID string) *Event {
	e.Session = sessionID
	return e
}

// WithRun sets the run ID
func (e *Event) WithRun(runID string) *Event {
	e.RunID = runID
	return e
}

// WithParent sets the parent event ID (causality)
func (e *Event) WithParent(parentID string) *Event {
	e.ParentID = parentID
	return e
}

// WithRoot sets the root cause event ID
func (e *Event) WithRoot(rootID string) *Event {
	e.RootID = rootID
	return e
}

// WithSpan sets the span ID (for distributed tracing)
func (e *Event) WithSpan(spanID string) *Event {
	e.SpanID = spanID
	return e
}

// WithData adds custom data
func (e *Event) WithData(key string, value any) *Event {
	if e.Data == nil {
		e.Data = make(map[string]any)
	}
	e.Data[key] = value
	return e
}

// WithDuration sets the duration
func (e *Event) WithDuration(d time.Duration) *Event {
	e.Duration = d
	return e
}

// WithError sets error information
func (e *Event) WithError(err error, errorType string, retryable bool) *Event {
	e.Level = LevelError
	e.Success = false
	e.Error = &ErrorInfo{
		Type:      errorType,
		Message:   err.Error(),
		Retryable: retryable,
	}
	return e
}

// WithErrorInfo sets detailed error information
func (e *Event) WithErrorInfo(info *ErrorInfo) *Event {
	e.Level = LevelError
	e.Success = false
	e.Error = info
	return e
}

// ToJSON converts event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}
