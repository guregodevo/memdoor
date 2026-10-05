package logs

import (
	"context"
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var (
	// Global default logger instance
	globalLogger *EventLogger
	globalMu     sync.RWMutex
)

// sharedState contains state that is shared between parent and child loggers
type sharedState struct {
	storage    Storage
	verbose    bool
	buffer     []*Event
	bufferSize int
	bufferMu   *sync.Mutex
	flushTimer *time.Ticker
	stopChan   chan struct{}
	wg         *sync.WaitGroup
}

// EventLogger is the main logger interface for applications
type EventLogger struct {
	component string
	shared    *sharedState

	// Context that gets inherited by all events
	session  string
	runID    string
	parentID string
}

// NewEventLogger creates a new event logger
func NewEventLogger(component string, storage Storage, verbose bool) *EventLogger {
	var wg sync.WaitGroup
	var bufferMu sync.Mutex

	shared := &sharedState{
		storage:    storage,
		verbose:    verbose,
		bufferSize: 100, // Flush every 100 events
		buffer:     make([]*Event, 0, 100),
		bufferMu:   &bufferMu,
		stopChan:   make(chan struct{}),
		wg:         &wg,
	}

	// Start background flusher (every 1 second)
	shared.flushTimer = time.NewTicker(1 * time.Second)
	shared.wg.Add(1)

	logger := &EventLogger{
		component: component,
		shared:    shared,
	}

	go logger.flushLoop()

	return logger
}

// NewEventLoggerWithSQLite creates a new event logger with SQLite storage
func NewEventLoggerWithSQLite(component string, logsDir string, verbose bool) (*EventLogger, error) {
	// Create SQLite storage
	dbPath := filepath.Join(logsDir, "events.db")
	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("create sqlite storage: %w", err)
	}

	return NewEventLogger(component, storage, verbose), nil
}

// WithSession creates a child logger with session context
func (l *EventLogger) WithSession(sessionID string) *EventLogger {
	return &EventLogger{
		component: l.component,
		shared:    l.shared, // Share the same state pointer
		session:   sessionID,
		runID:     l.runID,
		parentID:  l.parentID,
	}
}

// WithRun creates a child logger with run context
func (l *EventLogger) WithRun(runID string) *EventLogger {
	return &EventLogger{
		component: l.component,
		shared:    l.shared, // Share the same state pointer
		session:   l.session,
		runID:     runID,
		parentID:  l.parentID,
	}
}

// WithParent creates a child logger with parent event ID (causality)
func (l *EventLogger) WithParent(parentID string) *EventLogger {
	return &EventLogger{
		component: l.component,
		shared:    l.shared, // Share the same state pointer
		session:   l.session,
		runID:     l.runID,
		parentID:  parentID,
	}
}

// WithError creates a child logger with error context (compatibility with old logger API)
// This returns a logger-like object that captures the error and includes it when logging
func (l *EventLogger) WithError(err error) *errorLogger {
	return &errorLogger{
		parent: l,
		err:    err,
	}
}

// errorLogger is a helper for the WithError() compatibility pattern
type errorLogger struct {
	parent *EventLogger
	err    error
}

// Error logs an error message with the captured error
func (el *errorLogger) Error(msg string, args ...any) {
	// Convert error to string and prepend to args
	newArgs := make([]any, 0, len(args)+2)
	newArgs = append(newArgs, "error", el.err.Error())
	newArgs = append(newArgs, args...)
	el.parent.Error(msg, newArgs...)
}

// Warn logs a warning message with the captured error
func (el *errorLogger) Warn(msg string, args ...any) {
	newArgs := make([]any, 0, len(args)+2)
	newArgs = append(newArgs, "error", el.err.Error())
	newArgs = append(newArgs, args...)
	el.parent.Warn(msg, newArgs...)
}

// Info logs an info message with the captured error
func (el *errorLogger) Info(msg string, args ...any) {
	newArgs := make([]any, 0, len(args)+2)
	newArgs = append(newArgs, "error", el.err.Error())
	newArgs = append(newArgs, args...)
	el.parent.Info(msg, newArgs...)
}

// Debug logs a debug message with the captured error
func (el *errorLogger) Debug(msg string, args ...any) {
	newArgs := make([]any, 0, len(args)+2)
	newArgs = append(newArgs, "error", el.err.Error())
	newArgs = append(newArgs, args...)
	el.parent.Debug(msg, newArgs...)
}

// Standard logging methods with slog-compatible API
// These accept variadic any args in the form: key1, value1, key2, value2, ...
// Also supports slog.Attr types like slog.String("key", "value")

func (l *EventLogger) Debug(msg string, args ...any) {
	if !l.shared.verbose {
		return
	}
	event := NewEvent(EventStateChange, l.component, msg).
		WithLevel(LevelDebug)
	l.enrichAndEmitSlog(event, args...)
}

func (l *EventLogger) Info(msg string, args ...any) {
	event := NewEvent(EventStateChange, l.component, msg).
		WithLevel(LevelInfo)
	l.enrichAndEmitSlog(event, args...)
}

func (l *EventLogger) Warn(msg string, args ...any) {
	event := NewEvent(EventStateChange, l.component, msg).
		WithLevel(LevelWarn)
	l.enrichAndEmitSlog(event, args...)
}

func (l *EventLogger) Error(msg string, args ...any) {
	event := NewEvent(EventError, l.component, msg).
		WithLevel(LevelError)
	l.enrichAndEmitSlog(event, args...)
}

// EmitEvent emits a custom event
func (l *EventLogger) EmitEvent(event *Event) {
	l.enrichAndEmit(event)
}

// enrichAndEmit adds context and emits the event
func (l *EventLogger) enrichAndEmit(event *Event, fields ...Field) {
	// Add inherited context
	if l.session != "" && event.Session == "" {
		event.Session = l.session
	}
	if l.runID != "" && event.RunID == "" {
		event.RunID = l.runID
	}
	if l.parentID != "" && event.ParentID == "" {
		event.ParentID = l.parentID
	}

	// Add fields
	for _, field := range fields {
		event.WithData(field.Key, field.Value)
	}

	// Buffer for storage
	l.bufferEvent(event)
}

// enrichAndEmitSlog adds context and emits the event with slog-style args
func (l *EventLogger) enrichAndEmitSlog(event *Event, args ...any) {
	// Add inherited context
	if l.session != "" && event.Session == "" {
		event.Session = l.session
	}
	if l.runID != "" && event.RunID == "" {
		event.RunID = l.runID
	}
	if l.parentID != "" && event.ParentID == "" {
		event.ParentID = l.parentID
	}

	// Convert slog-style args to event data
	// Supports: key, value pairs OR slog.Attr types
	for i := 0; i < len(args); i++ {
		// Check if it's an slog.Attr-like type with String() method
		if attr, ok := args[i].(interface{ String() string }); ok {
			// Try to extract key-value from slog.Attr
			// slog.String("key", "value") implements fmt.Stringer
			str := attr.String()
			// Format is typically "key=value", parse it
			if idx := stringIndexByte(str, '='); idx > 0 {
				key := str[:idx]
				value := str[idx+1:]
				event.WithData(key, value)
			}
		} else if i+1 < len(args) {
			// Treat as key-value pair
			if key, ok := args[i].(string); ok {
				event.WithData(key, args[i+1])
				i++ // Skip next element as we used it as value
			}
		}
	}

	// Buffer for storage
	l.bufferEvent(event)
}

// stringIndexByte is a simple helper to find first occurrence of byte in string
func stringIndexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// StartSpan creates a new span for tracking operations
func (l *EventLogger) StartSpan(name string, eventType EventType) Span {
	s := NewSpan(name, eventType, l.component, l).(*span)

	// Inherit context
	if l.session != "" {
		s.session = l.session
	}
	if l.runID != "" {
		s.runID = l.runID
	}
	if l.parentID != "" {
		s.parentID = l.parentID
		s.rootID = l.parentID // Parent becomes root for now
	}

	return s
}

// Buffer management
func (l *EventLogger) bufferEvent(event *Event) {
	l.shared.bufferMu.Lock()
	defer l.shared.bufferMu.Unlock()

	l.shared.buffer = append(l.shared.buffer, event)

	// Flush if buffer is full
	if len(l.shared.buffer) >= l.shared.bufferSize {
		l.flushUnlocked()
	}
}

func (l *EventLogger) flushLoop() {
	defer l.shared.wg.Done()

	for {
		select {
		case <-l.shared.flushTimer.C:
			l.flush()
		case <-l.shared.stopChan:
			// Final flush
			l.flush()
			return
		}
	}
}

func (l *EventLogger) flush() {
	l.shared.bufferMu.Lock()
	defer l.shared.bufferMu.Unlock()
	l.flushUnlocked()
}

func (l *EventLogger) flushUnlocked() {
	if len(l.shared.buffer) == 0 {
		return
	}

	// Write to storage
	events := l.shared.buffer
	if err := l.shared.storage.WriteEvents(context.Background(), events); err != nil {
		// Fallback: Write to stderr when SQLite fails (preserves observability)
		// This ensures events are never lost even if database is unavailable
		for _, event := range events {
			fmt.Fprintf(os.Stderr, "[FALLBACK] %s [%s] %s: %s",
				event.Timestamp.Format(time.RFC3339), event.Level, event.Component, event.Message)
			if event.Error != nil {
				fmt.Fprintf(os.Stderr, " | error=%s", event.Error.Message)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
		// Also log the storage error itself
		fmt.Fprintf(os.Stderr, "[FALLBACK] Failed to write %d events to storage: %v\n",
			len(events), err)
	}

	// Clear buffer
	l.shared.buffer = l.shared.buffer[:0]
}

// Close flushes remaining events and stops the logger
func (l *EventLogger) Close() error {
	close(l.shared.stopChan)
	l.shared.wg.Wait()

	// Close storage to ensure all writes are committed
	if l.shared.storage != nil {
		return l.shared.storage.Close()
	}
	return nil
}

// Global logger initialization and access functions
// These provide compatibility with the old gateway/logger package API

// StorageWrapper is an optional decorator applied to the SQLite
// Storage at InitGlobalLogger time. External packages (specifically
// gateway/telemetry) set this var BEFORE the gateway boots its logger,
// so events flow through the wrapper on their way to the inner
// sqlite. Default nil = no wrap (test paths and CLI scripts that
// don't care about telemetry are unaffected).
//
// Plumbed as a package-level var rather than a parameter on
// InitGlobalLogger so callers that don't care don't need to update.
// Set once at process startup before InitGlobalLogger; later
// mutations have no effect.
var StorageWrapper func(Storage) Storage

// InitGlobalLogger initializes the global logger with SQLite storage
// This should be called once at application startup
func InitGlobalLogger(logsDir string, verbose bool) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("create logs directory: %w", err)
	}

	// Create SQLite storage, optionally pass through the
	// StorageWrapper hook (e.g. telemetry.Install) set at process
	// startup. When unset, the storage passes through unchanged and
	// the EventLogger writes straight to sqlite — identical to the
	// pre-hook behavior.
	dbPath := filepath.Join(logsDir, "events.db")
	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		return fmt.Errorf("create sqlite storage: %w", err)
	}
	var wrapped Storage = storage
	if StorageWrapper != nil {
		wrapped = StorageWrapper(storage)
	}
	globalLogger = NewEventLogger("Gateway", wrapped, verbose)
	return nil
}

// InitGlobalLoggerDefault initializes the global logger in ~/.memdoor/logs,
// the store `memdoor logs query` reads. Under `go test` it writes to a temp
// dir instead: a test that logs into the live store reads as the running
// gateway misbehaving (2026-09-28: TestWebSocket_ConcurrentCloseRace put 100
// "session manager not initialized" warnings in the live logs per run).
func InitGlobalLoggerDefault(verbose bool) error {
	if testing.Testing() {
		dir, err := os.MkdirTemp("", "memdoor-test-logs-*")
		if err != nil {
			return err
		}
		return InitGlobalLogger(dir, verbose)
	}

	logsDir := shared.MemdoorHome("logs")
	return InitGlobalLogger(logsDir, verbose)
}

// GetGlobalStorage returns the storage backend from the global logger.
// Returns nil if the global logger is not initialized.
func GetGlobalStorage() Storage {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalLogger == nil {
		return nil
	}
	return globalLogger.shared.storage
}

// FlushGlobal synchronously drains the global logger's buffer to storage.
// Intended for tests that emit an event and then immediately query it
// back: without this they race the 1s background flush loop, which
// intermittently times out on a loaded CI runner. No-op if the global
// logger isn't initialized.
func FlushGlobal() {
	globalMu.RLock()
	l := globalLogger
	globalMu.RUnlock()
	if l != nil {
		l.flush()
	}
}

// New creates a component-scoped logger from the global logger.
//
// Fail-fast: if the global logger isn't initialized this panics. The gateway
// initializes it eagerly at startup (and refuses to start otherwise), so by the
// time any component calls New the logger is guaranteed present. A nil here
// means a real ordering bug — surface it loudly rather than silently degrade.
func New(component string) *EventLogger {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalLogger == nil {
		panic("global logger not initialized - call InitGlobalLogger first")
	}

	// Create a new logger with the same shared state but different component
	return &EventLogger{
		component: component,
		shared:    globalLogger.shared, // Share storage and state
	}
}

// CloseGlobalLogger flushes and closes the global logger
// This should be called at application shutdown
func CloseGlobalLogger() error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalLogger != nil {
		err := globalLogger.Close()
		globalLogger = nil
		return err
	}
	return nil
}
