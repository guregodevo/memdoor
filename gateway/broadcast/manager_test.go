package broadcast

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

// MockSubscriber for testing
type MockSubscriber struct {
	id       string
	messages [][]byte
	mu       sync.Mutex
	sendFail bool
}

func NewMockSubscriber(id string) *MockSubscriber {
	return &MockSubscriber{
		id:       id,
		messages: make([][]byte, 0),
	}
}

func (m *MockSubscriber) Send(data []byte) error {
	if m.sendFail {
		return errors.New("send failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, data)
	return nil
}

func (m *MockSubscriber) GetID() string {
	return m.id
}

func (m *MockSubscriber) GetMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]byte{}, m.messages...)
}

func (m *MockSubscriber) MessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func TestSubscriptionManager_AddRemoveSubscriber(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")

	// Add subscriber
	sm.AddSubscriber(sub1)

	sm.mu.RLock()
	count := len(sm.allSubscribers)
	sm.mu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 subscriber, got %d", count)
	}

	// Remove subscriber
	sm.RemoveSubscriber("client-1")

	sm.mu.RLock()
	count = len(sm.allSubscribers)
	sm.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 subscribers after removal, got %d", count)
	}
}

func TestSubscriptionManager_SubscribeToRun(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")

	sm.AddSubscriber(sub1)
	sm.SubscribeToRun("client-1", "run-123")

	sm.mu.RLock()
	subs, ok := sm.runSubscribers["run-123"]
	sm.mu.RUnlock()

	if !ok {
		t.Fatal("Expected run subscription to exist")
	}

	if len(subs) != 1 {
		t.Errorf("Expected 1 subscriber for run, got %d", len(subs))
	}
}

func TestSubscriptionManager_UnsubscribeFromRun(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")

	sm.AddSubscriber(sub1)
	sm.SubscribeToRun("client-1", "run-123")
	sm.UnsubscribeFromRun("client-1", "run-123")

	sm.mu.RLock()
	_, ok := sm.runSubscribers["run-123"]
	sm.mu.RUnlock()

	if ok {
		t.Error("Expected run subscription to be removed")
	}
}

func TestSubscriptionManager_SubscribeToSession(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")

	sm.AddSubscriber(sub1)
	sm.SubscribeToSession("client-1", "session-main")

	sm.mu.RLock()
	subs, ok := sm.sessionSubscribers["session-main"]
	sm.mu.RUnlock()

	if !ok {
		t.Fatal("Expected session subscription to exist")
	}

	if len(subs) != 1 {
		t.Errorf("Expected 1 subscriber for session, got %d", len(subs))
	}
}

func TestSubscriptionManager_BroadcastToRun(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")
	sub2 := NewMockSubscriber("client-2")

	sm.AddSubscriber(sub1)
	sm.AddSubscriber(sub2)
	sm.SubscribeToRun("client-1", "run-123")
	sm.SubscribeToRun("client-2", "run-123")

	event := Event{
		Type: "test_event",
		Data: map[string]interface{}{
			"message": "hello",
		},
	}

	sm.BroadcastToRun("run-123", event)

	// Both subscribers should receive the event
	if sub1.MessageCount() != 1 {
		t.Errorf("Expected client-1 to receive 1 message, got %d", sub1.MessageCount())
	}

	if sub2.MessageCount() != 1 {
		t.Errorf("Expected client-2 to receive 1 message, got %d", sub2.MessageCount())
	}

	// Verify message content
	messages := sub1.GetMessages()
	if len(messages) == 0 {
		t.Fatal("Expected client-1 to have messages")
	}

	var receivedEvent Event
	if err := json.Unmarshal(messages[0], &receivedEvent); err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	if receivedEvent.Type != "test_event" {
		t.Errorf("Expected event type 'test_event', got '%s'", receivedEvent.Type)
	}

	if msg, ok := receivedEvent.Data["message"].(string); !ok || msg != "hello" {
		t.Errorf("Expected message 'hello', got '%v'", receivedEvent.Data["message"])
	}
}

func TestSubscriptionManager_BroadcastToSession(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")
	sub2 := NewMockSubscriber("client-2")

	sm.AddSubscriber(sub1)
	sm.AddSubscriber(sub2)
	sm.SubscribeToSession("client-1", "session-main")

	event := Event{
		Type: "session_event",
		Data: map[string]interface{}{
			"status": "active",
		},
	}

	sm.BroadcastToSession("session-main", event)

	// Only sub1 should receive the event
	if sub1.MessageCount() != 1 {
		t.Errorf("Expected client-1 to receive 1 message, got %d", sub1.MessageCount())
	}

	if sub2.MessageCount() != 0 {
		t.Errorf("Expected client-2 to receive 0 messages, got %d", sub2.MessageCount())
	}
}

func TestSubscriptionManager_BroadcastToAll(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")
	sub2 := NewMockSubscriber("client-2")
	sub3 := NewMockSubscriber("client-3")

	sm.AddSubscriber(sub1)
	sm.AddSubscriber(sub2)
	sm.AddSubscriber(sub3)

	event := Event{
		Type: "global_event",
		Data: map[string]interface{}{
			"broadcast": true,
		},
	}

	sm.BroadcastToAll(event)

	// All subscribers should receive the event
	if sub1.MessageCount() != 1 {
		t.Errorf("Expected client-1 to receive 1 message, got %d", sub1.MessageCount())
	}

	if sub2.MessageCount() != 1 {
		t.Errorf("Expected client-2 to receive 1 message, got %d", sub2.MessageCount())
	}

	if sub3.MessageCount() != 1 {
		t.Errorf("Expected client-3 to receive 1 message, got %d", sub3.MessageCount())
	}
}

func TestSubscriptionManager_MultipleRunSubscriptions(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")

	sm.AddSubscriber(sub1)
	sm.SubscribeToRun("client-1", "run-1")
	sm.SubscribeToRun("client-1", "run-2")
	sm.SubscribeToRun("client-1", "run-3")

	// Broadcast to run-1
	event1 := Event{Type: "event-1"}
	sm.BroadcastToRun("run-1", event1)

	// Broadcast to run-2
	event2 := Event{Type: "event-2"}
	sm.BroadcastToRun("run-2", event2)

	// Subscriber should receive both events
	if sub1.MessageCount() != 2 {
		t.Errorf("Expected client-1 to receive 2 messages, got %d", sub1.MessageCount())
	}
}

func TestSubscriptionManager_RemoveSubscriberCleansUpAllSubscriptions(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")

	sm.AddSubscriber(sub1)
	sm.SubscribeToRun("client-1", "run-123")
	sm.SubscribeToSession("client-1", "session-main")

	// Remove subscriber
	sm.RemoveSubscriber("client-1")

	// Verify all subscriptions are cleaned up
	sm.mu.RLock()
	runSubsCount := len(sm.runSubscribers)
	sessionSubsCount := len(sm.sessionSubscribers)
	allSubsCount := len(sm.allSubscribers)
	sm.mu.RUnlock()

	if runSubsCount != 0 {
		t.Errorf("Expected 0 run subscriptions, got %d", runSubsCount)
	}

	if sessionSubsCount != 0 {
		t.Errorf("Expected 0 session subscriptions, got %d", sessionSubsCount)
	}

	if allSubsCount != 0 {
		t.Errorf("Expected 0 total subscribers, got %d", allSubsCount)
	}
}

func TestSubscriptionManager_BroadcastToNonexistentRun(t *testing.T) {
	sm := NewSubscriptionManager(false)

	event := Event{Type: "test"}

	// Should not panic
	sm.BroadcastToRun("nonexistent-run", event)
}

func TestSubscriptionManager_FailedSend(t *testing.T) {
	sm := NewSubscriptionManager(false)
	sub1 := NewMockSubscriber("client-1")
	sub1.sendFail = true // Make sends fail

	sm.AddSubscriber(sub1)
	sm.SubscribeToRun("client-1", "run-123")

	event := Event{Type: "test"}

	// Should not panic on failed send
	sm.BroadcastToRun("run-123", event)

	// Message count should still be 0 since send failed
	if sub1.MessageCount() != 0 {
		t.Errorf("Expected 0 messages (send failed), got %d", sub1.MessageCount())
	}
}

func TestSubscriptionManager_ConcurrentOperations(t *testing.T) {
	sm := NewSubscriptionManager(false)
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sub := NewMockSubscriber(string(rune('A' + id)))
			sm.AddSubscriber(sub)
			sm.SubscribeToRun(sub.GetID(), "run-123")
		}(i)
	}

	wg.Wait()

	// Verify all added
	sm.mu.RLock()
	count := len(sm.allSubscribers)
	sm.mu.RUnlock()

	if count != 10 {
		t.Errorf("Expected 10 subscribers, got %d", count)
	}

	// Concurrent broadcasts
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := Event{Type: "concurrent_test"}
			sm.BroadcastToRun("run-123", event)
		}()
	}

	wg.Wait()
}
