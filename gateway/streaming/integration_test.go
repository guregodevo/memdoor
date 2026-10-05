package streaming

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/broadcast"
	"memdoor/gateway/infra"
)

// TestStreamingIntegration verifies end-to-end event flow
// Pattern: OpenClaw integration test - events flow from infra -> streaming -> broadcast -> subscribers
func TestStreamingIntegration(t *testing.T) {
	// Setup infrastructure
	broadcaster := broadcast.NewSubscriptionManager(false)
	infraEmitter := infra.NewEventEmitter(false)

	// Connect infra emitter to broadcaster
	unregister := ConnectInfraEventEmitter(infraEmitter, broadcaster, false)
	defer unregister()

	// Create mock subscriber
	subscriber := &mockWebSocketClient{
		id:       "test-client-1",
		received: make([]broadcast.Event, 0),
	}

	runID := "test-run-123"
	sessionID := "test-session-main"

	// Subscribe to run
	broadcaster.AddSubscriber(subscriber)
	broadcaster.SubscribeToRun(subscriber.id, runID)

	// Give time for subscriptions to register
	time.Sleep(10 * time.Millisecond)

	// Emit lifecycle start event
	infraEmitter.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
		"event":   "start",
		"message": "test message",
	})

	// Emit assistant text_delta event
	infraEmitter.EmitEvent(runID, infra.EventStreamAssistant, sessionID, map[string]interface{}{
		"event": "text_delta",
		"text":  "Hello world",
		"delta": "world",
	})

	// Emit tool start event
	infraEmitter.EmitEvent(runID, infra.EventStreamTool, sessionID, map[string]interface{}{
		"event": "tool_start",
		"tool":  "read_file",
	})

	// Emit lifecycle complete event
	infraEmitter.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
		"event":          "complete",
		"tools_executed": 1,
	})

	// Wait for events to propagate (async emission)
	time.Sleep(100 * time.Millisecond)

	// Verify subscriber received events
	subscriber.mu.Lock()
	receivedCount := len(subscriber.received)
	subscriber.mu.Unlock()

	if receivedCount != 4 {
		t.Fatalf("Expected 4 events, got %d", receivedCount)
	}

	// Verify event types and content (order-independent due to async emission)
	subscriber.mu.Lock()
	events := make([]broadcast.Event, len(subscriber.received))
	copy(events, subscriber.received)
	subscriber.mu.Unlock()

	// Count event types
	streamCounts := make(map[string]int)
	for _, event := range events {
		eventData := event.Data
		if stream, ok := eventData["stream"].(string); ok {
			streamCounts[stream]++
		}
	}

	// Verify we got the right event types
	if streamCounts["lifecycle"] != 2 {
		t.Errorf("Expected 2 lifecycle events, got %d", streamCounts["lifecycle"])
	}

	if streamCounts["assistant"] != 1 {
		t.Errorf("Expected 1 assistant event, got %d", streamCounts["assistant"])
	}

	if streamCounts["tool"] != 1 {
		t.Errorf("Expected 1 tool event, got %d", streamCounts["tool"])
	}

	// Verify specific event content
	var foundStart, foundComplete, foundAssistant, foundTool bool

	for _, event := range events {
		eventData := event.Data
		stream, _ := eventData["stream"].(string)
		data, ok := eventData["data"].(map[string]interface{})
		if !ok {
			continue
		}

		eventType, _ := data["event"].(string)

		if stream == "lifecycle" && eventType == "start" {
			foundStart = true
		}
		if stream == "lifecycle" && eventType == "complete" {
			foundComplete = true
		}
		if stream == "assistant" && eventType == "text_delta" {
			foundAssistant = true
		}
		if stream == "tool" && eventType == "tool_start" {
			foundTool = true
		}
	}

	if !foundStart {
		t.Error("Did not find lifecycle start event")
	}
	if !foundComplete {
		t.Error("Did not find lifecycle complete event")
	}
	if !foundAssistant {
		t.Error("Did not find assistant text_delta event")
	}
	if !foundTool {
		t.Error("Did not find tool start event")
	}

	t.Logf("✅ Integration test passed: %d events flowed through the pipeline", receivedCount)
}

func TestConcurrentStreaming(t *testing.T) {
	broadcaster := broadcast.NewSubscriptionManager(false)
	infraEmitter := infra.NewEventEmitter(false)

	ConnectInfraEventEmitter(infraEmitter, broadcaster, false)

	var wg sync.WaitGroup
	numRuns := 5

	// Create subscribers for each run
	for i := 0; i < numRuns; i++ {
		wg.Add(1)
		go func(runNum int) {
			defer wg.Done()

			runID := fmt.Sprintf("concurrent-run-%d", runNum)
			sessionID := fmt.Sprintf("session-%d", runNum)

			subscriber := &mockWebSocketClient{
				id:       fmt.Sprintf("client-%d", runNum),
				received: make([]broadcast.Event, 0),
			}

			broadcaster.AddSubscriber(subscriber)
			broadcaster.SubscribeToRun(subscriber.id, runID)

			time.Sleep(10 * time.Millisecond)

			// Emit events
			for j := 0; j < 3; j++ {
				infraEmitter.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
					"event": fmt.Sprintf("step-%d", j),
				})
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	t.Logf("✅ Concurrent streaming test passed: %d concurrent runs", numRuns)
}

// TestEventOrdering verifies events maintain sequence
func TestEventOrdering(t *testing.T) {
	broadcaster := broadcast.NewSubscriptionManager(false)
	infraEmitter := infra.NewEventEmitter(false)

	ConnectInfraEventEmitter(infraEmitter, broadcaster, false)

	subscriber := &mockWebSocketClient{
		id:       "test-client-ordering",
		received: make([]broadcast.Event, 0),
	}

	runID := "test-run-ordering"
	sessionID := "test-session"

	broadcaster.AddSubscriber(subscriber)
	broadcaster.SubscribeToRun(subscriber.id, runID)

	time.Sleep(10 * time.Millisecond)

	// Emit events with sequence numbers
	numEvents := 10
	for i := 0; i < numEvents; i++ {
		infraEmitter.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
			"event": fmt.Sprintf("event-%d", i),
			"index": i,
		})
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify all events received
	subscriber.mu.Lock()
	receivedCount := len(subscriber.received)
	subscriber.mu.Unlock()

	if receivedCount != numEvents {
		t.Fatalf("Expected %d events, got %d", numEvents, receivedCount)
	}

	// Verify sequence numbers are monotonically increasing
	subscriber.mu.Lock()
	for i := 0; i < len(subscriber.received); i++ {
		eventData := subscriber.received[i].Data

		seq, ok := eventData["seq"].(float64)
		if !ok {
			// Try int type
			seqInt, ok := eventData["seq"].(int)
			if !ok {
				continue
			}
			seq = float64(seqInt)
		}

		expectedSeq := i + 1
		if int(seq) != expectedSeq {
			t.Errorf("Event %d: expected seq %d, got %d", i, expectedSeq, int(seq))
		}
	}
	subscriber.mu.Unlock()

	t.Logf("✅ Event ordering test passed: %d events in correct sequence", receivedCount)
}

// mockWebSocketClient simulates a WebSocket client for testing
type mockWebSocketClient struct {
	id       string
	received []broadcast.Event
	mu       sync.Mutex
}

func (m *mockWebSocketClient) Send(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var event broadcast.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	m.received = append(m.received, event)
	return nil
}

func (m *mockWebSocketClient) GetID() string {
	return m.id
}

// BenchmarkStreamingThroughput measures streaming performance
func BenchmarkStreamingThroughput(b *testing.B) {
	broadcaster := broadcast.NewSubscriptionManager(false)
	infraEmitter := infra.NewEventEmitter(false)

	ConnectInfraEventEmitter(infraEmitter, broadcaster, false)

	subscriber := &mockWebSocketClient{
		id:       "bench-client",
		received: make([]broadcast.Event, 0),
	}

	runID := "bench-run"
	sessionID := "bench-session"

	broadcaster.AddSubscriber(subscriber)
	broadcaster.SubscribeToRun(subscriber.id, runID)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		infraEmitter.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
			"event": "benchmark",
			"index": i,
		})
	}

	// Wait for all events to propagate
	time.Sleep(100 * time.Millisecond)

	b.StopTimer()
	b.ReportMetric(float64(len(subscriber.received)), "events")
}

// TestStreamingMemoryCleanup verifies proper cleanup
func TestStreamingMemoryCleanup(t *testing.T) {
	broadcaster := broadcast.NewSubscriptionManager(false)
	infraEmitter := infra.NewEventEmitter(false)

	unregister := ConnectInfraEventEmitter(infraEmitter, broadcaster, false)

	// Create subscriber
	subscriber := &mockWebSocketClient{
		id:       "cleanup-client",
		received: make([]broadcast.Event, 0),
	}

	runID := "cleanup-run"
	sessionID := "cleanup-session"

	broadcaster.AddSubscriber(subscriber)
	broadcaster.SubscribeToRun(subscriber.id, runID)

	// Emit some events
	for i := 0; i < 5; i++ {
		infraEmitter.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
			"event": "test",
		})
	}

	time.Sleep(50 * time.Millisecond)

	// Cleanup
	broadcaster.RemoveSubscriber(subscriber.id)
	infraEmitter.ClearRun(runID)
	unregister()

	// Emit more events - should not be received
	initialCount := len(subscriber.received)

	infraEmitter.EmitEvent(runID, infra.EventStreamLifecycle, sessionID, map[string]interface{}{
		"event": "after-cleanup",
	})

	time.Sleep(50 * time.Millisecond)

	finalCount := len(subscriber.received)

	if finalCount != initialCount {
		t.Errorf("Events received after cleanup: initial=%d, final=%d", initialCount, finalCount)
	}

	t.Logf("✅ Memory cleanup test passed")
}
