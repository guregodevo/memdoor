package consumer

import (
	"sync"
	"testing"
	"time"

	"memdoor/gateway/infra"
)

// TestDiscriminator tests event data parsing
func TestDiscriminator(t *testing.T) {
	d := NewDiscriminator(false)

	t.Run("Parse Lifecycle Event", func(t *testing.T) {
		event := infra.AgentEvent{
			RunID:  "run-1",
			Stream: infra.EventStreamLifecycle,
			Data: map[string]interface{}{
				"event":   "start",
				"message": "Starting run",
			},
		}

		parsed, err := d.ParseEventData(event)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		lifecycleData, ok := parsed.(LifecycleEventData)
		if !ok {
			t.Fatalf("Expected LifecycleEventData, got: %T", parsed)
		}

		if lifecycleData.Event != "start" {
			t.Errorf("Expected event 'start', got: %s", lifecycleData.Event)
		}

		if lifecycleData.Message != "Starting run" {
			t.Errorf("Expected message 'Starting run', got: %s", lifecycleData.Message)
		}
	})

	t.Run("Parse Tool Event", func(t *testing.T) {
		event := infra.AgentEvent{
			RunID:  "run-1",
			Stream: infra.EventStreamTool,
			Data: map[string]interface{}{
				"event": "start",
				"tool":  "Read",
				"input": `{"file_path": "test.go"}`,
			},
		}

		parsed, err := d.ParseEventData(event)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		toolData, ok := parsed.(ToolEventData)
		if !ok {
			t.Fatalf("Expected ToolEventData, got: %T", parsed)
		}

		if toolData.Event != "start" {
			t.Errorf("Expected event 'start', got: %s", toolData.Event)
		}

		if toolData.Tool != "Read" {
			t.Errorf("Expected tool 'Read', got: %s", toolData.Tool)
		}
	})

	t.Run("Parse Assistant Event", func(t *testing.T) {
		event := infra.AgentEvent{
			RunID:  "run-1",
			Stream: infra.EventStreamAssistant,
			Data: map[string]interface{}{
				"event": "text_delta",
				"text":  "Hello world",
			},
		}

		parsed, err := d.ParseEventData(event)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		assistantData, ok := parsed.(AssistantEventData)
		if !ok {
			t.Fatalf("Expected AssistantEventData, got: %T", parsed)
		}

		if assistantData.Event != "text_delta" {
			t.Errorf("Expected event 'text_delta', got: %s", assistantData.Event)
		}

		if assistantData.Text != "Hello world" {
			t.Errorf("Expected text 'Hello world', got: %s", assistantData.Text)
		}
	})
}

// TestStreamAssembler tests stream assembly and text building
func TestStreamAssembler(t *testing.T) {
	sa := NewStreamAssembler(10, false)

	runID := "run-test-1"
	sessionID := "session-1"

	t.Run("Thinking Event", func(t *testing.T) {
		event := infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Stream:    infra.EventStreamAssistant,
			Timestamp: time.Now().UnixMilli(),
		}

		parsedData := AssistantEventData{
			Event: "thinking",
		}

		result, err := sa.ProcessEvent(event, parsedData)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.ThinkingState {
			t.Error("Expected thinking state to be true")
		}
	})

	t.Run("Text Delta Events", func(t *testing.T) {
		deltas := []string{"Hello", " ", "world", "!"}
		expectedFull := ""

		for _, delta := range deltas {
			event := infra.AgentEvent{
				RunID:     runID,
				SessionID: sessionID,
				Stream:    infra.EventStreamAssistant,
				Timestamp: time.Now().UnixMilli(),
			}

			parsedData := AssistantEventData{
				Event: "text_delta",
				Text:  delta,
			}

			result, err := sa.ProcessEvent(event, parsedData)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if result.TextDelta != delta {
				t.Errorf("Expected delta '%s', got: '%s'", delta, result.TextDelta)
			}

			expectedFull += delta
			if result.FullText != expectedFull {
				t.Errorf("Expected full text '%s', got: '%s'", expectedFull, result.FullText)
			}

			if result.ThinkingState {
				t.Error("Expected thinking state to be false during text streaming")
			}
		}

		// Final text should be "Hello world!"
		state := sa.GetRunState(runID)
		if state.Text != "Hello world!" {
			t.Errorf("Expected final text 'Hello world!', got: '%s'", state.Text)
		}
	})

	t.Run("Tool Events", func(t *testing.T) {
		// Tool start
		event := infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Stream:    infra.EventStreamTool,
			Timestamp: time.Now().UnixMilli(),
		}

		parsedData := ToolEventData{
			Event: "start",
			Tool:  "Read",
			Input: `{"file_path": "test.go"}`,
		}

		result, err := sa.ProcessEvent(event, parsedData)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if result.ToolUpdate == nil {
			t.Fatal("Expected tool update, got nil")
		}

		if result.ToolUpdate.Name != "Read" {
			t.Errorf("Expected tool name 'Read', got: %s", result.ToolUpdate.Name)
		}

		if result.ToolUpdate.Status != "running" {
			t.Errorf("Expected status 'running', got: %s", result.ToolUpdate.Status)
		}

		// Tool complete
		parsedData = ToolEventData{
			Event:  "complete",
			Tool:   "Read",
			Output: "file contents here",
		}

		result, err = sa.ProcessEvent(event, parsedData)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if result.ToolUpdate.Status != "complete" {
			t.Errorf("Expected status 'complete', got: %s", result.ToolUpdate.Status)
		}

		if result.ToolUpdate.Output != "file contents here" {
			t.Errorf("Expected output 'file contents here', got: %s", result.ToolUpdate.Output)
		}
	})

	t.Run("Lifecycle Complete", func(t *testing.T) {
		event := infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Stream:    infra.EventStreamLifecycle,
			Timestamp: time.Now().UnixMilli(),
		}

		parsedData := LifecycleEventData{
			Event: "complete",
		}

		result, err := sa.ProcessEvent(event, parsedData)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !result.Finalized {
			t.Error("Expected run to be finalized")
		}
	})
}

// TestDeduplicator tests run deduplication
func TestDeduplicator(t *testing.T) {
	d := NewDeduplicator(1*time.Second, false) // 1 second TTL for testing

	t.Run("Mark and Check Finalized", func(t *testing.T) {
		runID := "run-1"

		if d.IsFinalized(runID) {
			t.Error("Expected run to not be finalized initially")
		}

		d.MarkFinalized(runID)

		if !d.IsFinalized(runID) {
			t.Error("Expected run to be finalized after marking")
		}
	})

	t.Run("Session Run Tracking", func(t *testing.T) {
		sessionID := "session-1"
		runID1 := "run-1"
		runID2 := "run-2"

		d.AddSessionRun(sessionID, runID1)
		d.AddSessionRun(sessionID, runID2)

		runs := d.GetSessionRuns(sessionID)
		if len(runs) != 2 {
			t.Errorf("Expected 2 runs, got: %d", len(runs))
		}

		if !d.IsRunInSession(sessionID, runID1) {
			t.Error("Expected run-1 to be in session")
		}

		if !d.IsRunInSession(sessionID, runID2) {
			t.Error("Expected run-2 to be in session")
		}
	})

	t.Run("Pruning", func(t *testing.T) {
		d.ClearAllFinalized()
		d.ClearAllSessions()

		runID := "run-prune"
		d.MarkFinalized(runID)

		if !d.IsFinalized(runID) {
			t.Error("Expected run to be finalized")
		}

		// Wait for TTL to expire
		time.Sleep(1100 * time.Millisecond)

		pruned := d.Prune()
		if pruned == 0 {
			t.Error("Expected at least 1 run to be pruned")
		}

		if d.IsFinalized(runID) {
			t.Error("Expected run to be pruned after TTL")
		}
	})
}

// TestConsumer tests the full consumer workflow
func TestConsumer(t *testing.T) {
	opts := DefaultConsumerOptions()
	opts.EnableLogging = false
	opts.PruneInterval = 0 // Disable auto-pruning for tests

	var receivedEvents []*ProcessedEvent
	var mu sync.Mutex

	handler := func(event *ProcessedEvent) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, event)
	}

	consumer := NewConsumer(handler, opts)
	defer consumer.Stop()

	runID := "run-consumer-test"
	sessionID := "session-consumer"

	t.Run("Full Event Sequence", func(t *testing.T) {
		// Lifecycle start
		consumer.ProcessEvent(infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Seq:       1,
			Stream:    infra.EventStreamLifecycle,
			Timestamp: time.Now().UnixMilli(),
			Data: map[string]interface{}{
				"event": "start",
			},
		})

		// Thinking
		consumer.ProcessEvent(infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Seq:       2,
			Stream:    infra.EventStreamAssistant,
			Timestamp: time.Now().UnixMilli(),
			Data: map[string]interface{}{
				"event": "thinking",
			},
		})

		// Text deltas
		consumer.ProcessEvent(infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Seq:       3,
			Stream:    infra.EventStreamAssistant,
			Timestamp: time.Now().UnixMilli(),
			Data: map[string]interface{}{
				"event": "text_delta",
				"text":  "Hello ",
			},
		})

		consumer.ProcessEvent(infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Seq:       4,
			Stream:    infra.EventStreamAssistant,
			Timestamp: time.Now().UnixMilli(),
			Data: map[string]interface{}{
				"event": "text_delta",
				"text":  "world!",
			},
		})

		// Tool call
		consumer.ProcessEvent(infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Seq:       5,
			Stream:    infra.EventStreamTool,
			Timestamp: time.Now().UnixMilli(),
			Data: map[string]interface{}{
				"event": "start",
				"tool":  "Read",
				"input": `{"file_path": "test.go"}`,
			},
		})

		consumer.ProcessEvent(infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Seq:       6,
			Stream:    infra.EventStreamTool,
			Timestamp: time.Now().UnixMilli(),
			Data: map[string]interface{}{
				"event":  "complete",
				"tool":   "Read",
				"output": "file contents",
			},
		})

		// Lifecycle complete
		consumer.ProcessEvent(infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Seq:       7,
			Stream:    infra.EventStreamLifecycle,
			Timestamp: time.Now().UnixMilli(),
			Data: map[string]interface{}{
				"event": "complete",
			},
		})

		// Wait for async handlers to complete
		time.Sleep(100 * time.Millisecond)

		// Verify events received
		mu.Lock()
		defer mu.Unlock()

		if len(receivedEvents) != 7 {
			t.Errorf("Expected 7 events, got: %d", len(receivedEvents))
		}

		// Check final run state
		state := consumer.GetRunState(runID)
		if state == nil {
			t.Fatal("Expected run state to exist")
		}

		if state.Text != "Hello world!" {
			t.Errorf("Expected text 'Hello world!', got: '%s'", state.Text)
		}

		if !state.Finalized {
			t.Error("Expected run to be finalized")
		}

		// Verify deduplication - processing same event again should be skipped
		initialCount := len(receivedEvents)
		consumer.ProcessEvent(infra.AgentEvent{
			RunID:     runID,
			SessionID: sessionID,
			Seq:       8,
			Stream:    infra.EventStreamAssistant,
			Timestamp: time.Now().UnixMilli(),
			Data: map[string]interface{}{
				"event": "text_delta",
				"text":  " extra",
			},
		})

		time.Sleep(50 * time.Millisecond)

		if len(receivedEvents) != initialCount {
			t.Error("Expected finalized run to be deduplicated")
		}
	})
}
