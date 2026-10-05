package logs

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestEventCreation tests basic event creation
func TestEventCreation(t *testing.T) {
	event := NewEvent(EventMessageReceived, "TestComponent", "Test message")

	if event.ID == "" {
		t.Error("Event ID should not be empty")
	}
	if event.Type != EventMessageReceived {
		t.Errorf("Expected type %s, got %s", EventMessageReceived, event.Type)
	}
	if event.Component != "TestComponent" {
		t.Errorf("Expected component 'TestComponent', got %s", event.Component)
	}
	if event.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got %s", event.Message)
	}
	if event.Level != LevelInfo {
		t.Errorf("Expected default level INFO, got %s", event.Level)
	}
	if !event.Success {
		t.Error("Expected default success to be true")
	}
}

// TestEventChaining tests event causality tracking
func TestEventChaining(t *testing.T) {
	root := NewEvent(EventMessageReceived, "Test", "Root event").
		WithSession("session1").
		WithRun("run1")

	child := NewEvent(EventToolCalled, "Test", "Child event").
		WithParent(root.ID).
		WithRoot(root.ID).
		WithSession("session1").
		WithRun("run1")

	if child.ParentID != root.ID {
		t.Errorf("Expected parent ID %s, got %s", root.ID, child.ParentID)
	}
	if child.RootID != root.ID {
		t.Errorf("Expected root ID %s, got %s", root.ID, child.RootID)
	}
}

// TestSQLiteStorage tests SQLite storage operations
func TestSQLiteStorage(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create test events
	events := []*Event{
		NewEvent(EventMessageReceived, "Test", "Message 1").
			WithLevel(LevelInfo).
			WithSession("session1"),
		NewEvent(EventToolCalled, "Test", "Tool call").
			WithLevel(LevelDebug).
			WithSession("session1"),
		NewEvent(EventError, "Test", "Error occurred").
			WithLevel(LevelError).
			WithSession("session1").
			WithErrorInfo(&ErrorInfo{
				Type:      "test_error",
				Message:   "Test error message",
				Retryable: true,
			}),
	}

	// Write events
	ctx := context.Background()
	if err := storage.WriteEvents(ctx, events); err != nil {
		t.Fatalf("Failed to write events: %v", err)
	}

	// Query all events
	query := &Query{
		Descending: false,
	}
	result, err := storage.QueryEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(result.Events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(result.Events))
	}

	// Verify all expected messages are present
	messages := make(map[string]bool)
	for _, e := range result.Events {
		messages[e.Message] = true
	}

	expectedMessages := []string{"Message 1", "Tool call", "Error occurred"}
	for _, expected := range expectedMessages {
		if !messages[expected] {
			t.Errorf("Expected to find message '%s' in results", expected)
		}
	}
}

// TestQueryByLevel tests filtering by log level
func TestQueryByLevel(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create events with different levels
	events := []*Event{
		NewEvent(EventStateChange, "Test", "Debug message").WithLevel(LevelDebug),
		NewEvent(EventStateChange, "Test", "Info message").WithLevel(LevelInfo),
		NewEvent(EventError, "Test", "Error message").WithLevel(LevelError),
		NewEvent(EventStateChange, "Test", "Warn message").WithLevel(LevelWarn),
	}

	ctx := context.Background()
	if err := storage.WriteEvents(ctx, events); err != nil {
		t.Fatalf("Failed to write events: %v", err)
	}

	// Query only errors
	query := &Query{
		Levels: []Level{LevelError},
	}
	result, err := storage.QueryEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(result.Events) != 1 {
		t.Errorf("Expected 1 error event, got %d", len(result.Events))
	}
	if result.Events[0].Level != LevelError {
		t.Errorf("Expected ERROR level, got %s", result.Events[0].Level)
	}
}

// TestQueryBySession tests filtering by session
func TestQueryBySession(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create events for different sessions
	events := []*Event{
		NewEvent(EventMessageReceived, "Test", "Session 1 msg").WithSession("session1"),
		NewEvent(EventMessageReceived, "Test", "Session 2 msg").WithSession("session2"),
		NewEvent(EventMessageReceived, "Test", "Session 1 msg 2").WithSession("session1"),
	}

	ctx := context.Background()
	if err := storage.WriteEvents(ctx, events); err != nil {
		t.Fatalf("Failed to write events: %v", err)
	}

	// Query session1 events
	query := &Query{
		Session: "session1",
	}
	result, err := storage.QueryEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(result.Events) != 2 {
		t.Errorf("Expected 2 session1 events, got %d", len(result.Events))
	}
}

// TestCausalChain tests causal chain reconstruction
func TestCausalChain(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a chain of events
	root := NewEvent(EventMessageReceived, "Test", "Root event")
	child1 := NewEvent(EventToolCalled, "Test", "Child 1").
		WithParent(root.ID).
		WithRoot(root.ID)
	child2 := NewEvent(EventToolCompleted, "Test", "Child 2").
		WithParent(child1.ID).
		WithRoot(root.ID)

	events := []*Event{root, child1, child2}

	ctx := context.Background()
	if err := storage.WriteEvents(ctx, events); err != nil {
		t.Fatalf("Failed to write events: %v", err)
	}

	// Reconstruct chain
	chain, err := storage.TraceChain(ctx, child2.ID)
	if err != nil {
		t.Fatalf("Failed to trace chain: %v", err)
	}

	if chain.RootEvent.ID != root.ID {
		t.Errorf("Expected root event ID %s, got %s", root.ID, chain.RootEvent.ID)
	}
	if chain.TotalEvents != 3 {
		t.Errorf("Expected 3 events in chain, got %d", chain.TotalEvents)
	}
	if chain.Depth != 2 {
		t.Errorf("Expected depth 2, got %d", chain.Depth)
	}
}

// TestSessionReconstruction tests session timeline reconstruction
func TestSessionReconstruction(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create session events with specific timestamps for ordering
	sessionID := "test-session"
	now := time.Now()

	event1 := NewEvent(EventMessageReceived, "Test", "User message").WithSession(sessionID)
	event1.Timestamp = now.Add(-2 * time.Minute)

	event2 := NewEvent(EventToolCalled, "Test", "Tool execution").WithSession(sessionID)
	event2.Timestamp = now.Add(-1 * time.Minute)

	event3 := NewEvent(EventMessageSent, "Test", "Assistant response").WithSession(sessionID)
	event3.Timestamp = now

	events := []*Event{event1, event2, event3}

	ctx := context.Background()
	if err := storage.WriteEvents(ctx, events); err != nil {
		t.Fatalf("Failed to write events: %v", err)
	}

	// Reconstruct session
	timeline, err := storage.ReconstructSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to reconstruct session: %v", err)
	}

	if len(timeline) != 3 {
		t.Errorf("Expected 3 events in timeline, got %d", len(timeline))
	}

	// Verify chronological order
	if timeline[0].Type != EventMessageReceived {
		t.Errorf("Expected first event to be message received, got %s", timeline[0].Type)
	}
	if timeline[2].Type != EventMessageSent {
		t.Errorf("Expected last event to be message sent, got %s", timeline[2].Type)
	}
}

// TestEventLogger tests the logger with SQLite storage
func TestEventLogger(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewEventLoggerWithSQLite("TestComponent", tempDir, true)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log some events
	logger.Info("Test info message")
	logger.Warn("Test warning message")
	logger.Debug("Test debug message")

	// Give time for buffer flush
	time.Sleep(1500 * time.Millisecond)

	// Query the storage directly
	dbPath := filepath.Join(tempDir, "events.db")
	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}
	defer storage.Close()

	query := &Query{
		Components: []string{"TestComponent"},
	}
	result, err := storage.QueryEvents(context.Background(), query)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(result.Events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(result.Events))
	}
}

// TestSpan tests span tracking
func TestSpan(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewEventLoggerWithSQLite("TestComponent", tempDir, true)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Create a span
	span := logger.StartSpan("test-operation", EventToolCalled)
	span.SetTag("test-key", "test-value")
	span.Info("Operation in progress")
	span.Finish()

	// Give time for buffer flush
	time.Sleep(1500 * time.Millisecond)

	// Verify events were written
	dbPath := filepath.Join(tempDir, "events.db")
	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}
	defer storage.Close()

	query := &Query{
		EventTypes: []EventType{EventToolCalled},
	}
	result, err := storage.QueryEvents(context.Background(), query)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(result.Events) < 2 {
		t.Errorf("Expected at least 2 events (start + complete), got %d", len(result.Events))
	}
}

// TestQueryBuilder tests the fluent query builder interface
func TestQueryBuilder(t *testing.T) {
	qb := NewQueryBuilder().
		Since(1*time.Hour).
		Levels(LevelError, LevelWarn).
		Components("WebSocket", "Agent").
		ErrorsOnly().
		Limit(50)

	query := qb.Build()

	if query.Since != 1*time.Hour {
		t.Errorf("Expected since 1h, got %v", query.Since)
	}
	if len(query.Levels) != 2 {
		t.Errorf("Expected 2 levels, got %d", len(query.Levels))
	}
	if len(query.Components) != 2 {
		t.Errorf("Expected 2 components, got %d", len(query.Components))
	}
	if query.SuccessOnly == nil || *query.SuccessOnly != false {
		t.Error("Expected ErrorsOnly to set SuccessOnly to false")
	}
	if query.Limit != 50 {
		t.Errorf("Expected limit 50, got %d", query.Limit)
	}
}

// TestStorageStats tests storage statistics
func TestStorageStats(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create events
	events := []*Event{
		NewEvent(EventMessageReceived, "Test", "Message 1").WithLevel(LevelInfo),
		NewEvent(EventError, "Test", "Error 1").WithLevel(LevelError),
		NewEvent(EventStateChange, "Test", "Debug 1").WithLevel(LevelDebug),
	}

	ctx := context.Background()
	if err := storage.WriteEvents(ctx, events); err != nil {
		t.Fatalf("Failed to write events: %v", err)
	}

	// Get stats
	stats, err := storage.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalEvents != 3 {
		t.Errorf("Expected 3 total events, got %d", stats.TotalEvents)
	}
	if stats.EventsByLevel[LevelInfo] != 1 {
		t.Errorf("Expected 1 INFO event, got %d", stats.EventsByLevel[LevelInfo])
	}
	if stats.EventsByLevel[LevelError] != 1 {
		t.Errorf("Expected 1 ERROR event, got %d", stats.EventsByLevel[LevelError])
	}
	if stats.DatabaseSize <= 0 {
		t.Error("Expected database size to be > 0")
	}
}

// TestFindSimilar tests finding similar events
func TestFindSimilar(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create similar events
	events := []*Event{
		NewEvent(EventError, "WebSocket", "Connection failed").WithLevel(LevelError),
		NewEvent(EventError, "WebSocket", "Write error").WithLevel(LevelError),
		NewEvent(EventError, "WebSocket", "Read timeout").WithLevel(LevelError),
		NewEvent(EventStateChange, "WebSocket", "Connected").WithLevel(LevelInfo),
	}

	ctx := context.Background()
	if err := storage.WriteEvents(ctx, events); err != nil {
		t.Fatalf("Failed to write events: %v", err)
	}

	// Find similar to first error
	similar, err := storage.FindSimilar(ctx, events[0].ID, 10)
	if err != nil {
		t.Fatalf("Failed to find similar: %v", err)
	}

	// Should find 2 similar error events (excluding the original)
	if len(similar) != 2 {
		t.Errorf("Expected 2 similar events, got %d", len(similar))
	}
}

// BenchmarkWriteEvents benchmarks event writing performance
func BenchmarkWriteEvents(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	event := NewEvent(EventMessageReceived, "Benchmark", "Test message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := storage.WriteEvents(ctx, []*Event{event}); err != nil {
			b.Fatalf("Failed to write event: %v", err)
		}
	}
}

// BenchmarkQueryEvents benchmarks query performance
func BenchmarkQueryEvents(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Pre-populate with events
	ctx := context.Background()
	events := make([]*Event, 1000)
	for i := 0; i < 1000; i++ {
		events[i] = NewEvent(EventMessageReceived, "Benchmark", "Test message").
			WithLevel(LevelInfo)
	}
	if err := storage.WriteEvents(ctx, events); err != nil {
		b.Fatalf("Failed to write events: %v", err)
	}

	query := &Query{
		Levels: []Level{LevelInfo},
		Limit:  100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := storage.QueryEvents(ctx, query); err != nil {
			b.Fatalf("Failed to query events: %v", err)
		}
	}
}
