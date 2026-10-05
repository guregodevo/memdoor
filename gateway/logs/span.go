package logs

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Span represents a unit of work with timing and causality
// Think of it like OpenTelemetry spans but integrated with our event system
type Span interface {
	// Logging within span
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(err error, fields ...Field)

	// Event emission
	EmitEvent(event *Event)

	// Tags and metadata
	SetTag(key string, value any)
	SetSuccess(success bool)

	// Lifecycle
	Finish()
	FinishWithError(err error)

	// Nesting
	StartChild(name string, eventType EventType) Span

	// Access
	ID() string
	ParentID() string
	RootID() string
}

// span is the internal implementation
type span struct {
	id        string
	parentID  string
	rootID    string
	name      string
	eventType EventType
	component string

	// Context
	session string
	runID   string

	// Timing
	startTime time.Time
	endTime   time.Time

	// State
	tags    map[string]any
	success bool
	mu      sync.Mutex

	// Logger reference
	logger *EventLogger
}

// NewSpan creates a new root span
func NewSpan(name string, eventType EventType, component string, logger *EventLogger) Span {
	spanID := uuid.New().String()
	return &span{
		id:        spanID,
		rootID:    spanID, // Root span's root is itself
		name:      name,
		eventType: eventType,
		component: component,
		startTime: time.Now(),
		tags:      make(map[string]any),
		success:   true,
		logger:    logger,
	}
}

func (s *span) ID() string {
	return s.id
}

func (s *span) ParentID() string {
	return s.parentID
}

func (s *span) RootID() string {
	return s.rootID
}

// WithSession sets session context
func (s *span) WithSession(sessionID string) Span {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.session = sessionID
	return s
}

// WithRun sets run context
func (s *span) WithRun(runID string) Span {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runID = runID
	return s
}

func (s *span) Debug(msg string, fields ...Field) {
	s.logWithLevel(LevelDebug, msg, fields...)
}

func (s *span) Info(msg string, fields ...Field) {
	s.logWithLevel(LevelInfo, msg, fields...)
}

func (s *span) Warn(msg string, fields ...Field) {
	s.logWithLevel(LevelWarn, msg, fields...)
}

func (s *span) Error(err error, fields ...Field) {
	s.logWithLevel(LevelError, err.Error(), fields...)
	s.SetSuccess(false)
}

func (s *span) logWithLevel(level Level, msg string, fields ...Field) {
	event := NewEvent(s.eventType, s.component, msg).
		WithLevel(level).
		WithSpan(s.id).
		WithParent(s.parentID).
		WithRoot(s.rootID)

	if s.session != "" {
		event.WithSession(s.session)
	}
	if s.runID != "" {
		event.WithRun(s.runID)
	}

	// Add fields as data
	for _, field := range fields {
		event.WithData(field.Key, field.Value)
	}

	// Add span tags as data
	s.mu.Lock()
	for k, v := range s.tags {
		event.WithData(k, v)
	}
	s.mu.Unlock()

	s.logger.EmitEvent(event)
}

func (s *span) EmitEvent(event *Event) {
	// Enrich event with span context
	event.SpanID = s.id
	event.ParentID = s.parentID
	event.RootID = s.rootID

	if s.session != "" && event.Session == "" {
		event.Session = s.session
	}
	if s.runID != "" && event.RunID == "" {
		event.RunID = s.runID
	}

	s.logger.EmitEvent(event)
}

func (s *span) SetTag(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags[key] = value
}

func (s *span) SetSuccess(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.success = success
}

func (s *span) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.endTime = time.Now()
	duration := s.endTime.Sub(s.startTime)

	// Emit span completion event
	event := NewEvent(s.eventType, s.component, s.name+" completed").
		WithSpan(s.id).
		WithParent(s.parentID).
		WithRoot(s.rootID).
		WithDuration(duration)

	if s.session != "" {
		event.WithSession(s.session)
	}
	if s.runID != "" {
		event.WithRun(s.runID)
	}

	event.Success = s.success

	// Add all tags as data
	for k, v := range s.tags {
		event.WithData(k, v)
	}

	s.logger.EmitEvent(event)
}

func (s *span) FinishWithError(err error) {
	s.SetSuccess(false)
	s.Error(err)
	s.Finish()
}

func (s *span) StartChild(name string, eventType EventType) Span {
	childID := uuid.New().String()

	child := &span{
		id:        childID,
		parentID:  s.id,
		rootID:    s.rootID, // Inherit root from parent
		name:      name,
		eventType: eventType,
		component: s.component,
		session:   s.session,
		runID:     s.runID,
		startTime: time.Now(),
		tags:      make(map[string]any),
		success:   true,
		logger:    s.logger,
	}

	// Emit child span start event
	event := NewEvent(eventType, s.component, name+" started").
		WithSpan(childID).
		WithParent(s.id).
		WithRoot(s.rootID)

	if s.session != "" {
		event.WithSession(s.session)
	}
	if s.runID != "" {
		event.WithRun(s.runID)
	}

	s.logger.EmitEvent(event)

	return child
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value any
}
