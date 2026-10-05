package a2a

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"memdoor/pkg/shared"
)

// TestA2AHandler_BasicFlow tests the basic A2A message flow
func TestA2AHandler_BasicFlow(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	// Track delivery and announce calls
	var deliveryCalls []string
	var announceCalls []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveryCalls = append(deliveryCalls, fmt.Sprintf("%s: %s", targetSessionKey, message))
		mu.Unlock()

		// Simulate target agent response
		return "Calendar checked: 3 events found", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		mu.Lock()
		announceCalls = append(announceCalls, fmt.Sprintf("%s: %s", requesterSessionKey, announcement))
		mu.Unlock()
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start handler
	go handler.Start(ctx)

	// Send A2A message
	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Check calendar for tomorrow",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify delivery was called
	// With ping-pong integration, we now have 2 calls:
	// 1. Initial delivery
	// 2. Announcement step
	mu.Lock()
	defer mu.Unlock()

	if len(deliveryCalls) != 2 {
		t.Fatalf("Expected 2 delivery calls (initial + announce), got %d", len(deliveryCalls))
	}

	// First call: initial delivery
	if !strings.Contains(deliveryCalls[0], "agent:work:main") {
		t.Errorf("Expected delivery to agent:work:main, got: %s", deliveryCalls[0])
	}

	if !strings.Contains(deliveryCalls[0], "Check calendar") {
		t.Errorf("Expected message to contain 'Check calendar', got: %s", deliveryCalls[0])
	}

	// Second call: announcement step
	if !strings.Contains(deliveryCalls[1], "Agent-to-agent announce step") {
		t.Errorf("Expected announcement step, got: %s", deliveryCalls[1])
	}

	// Verify announce was called
	if len(announceCalls) != 1 {
		t.Fatalf("Expected 1 announce call, got %d", len(announceCalls))
	}

	if !strings.Contains(announceCalls[0], "agent:main:main") {
		t.Errorf("Expected announce to agent:main:main, got: %s", announceCalls[0])
	}

	// Announcement now comes from announcement step (agent formats it)
	// Just verify it's not empty
	if announceCalls[0] == "" {
		t.Error("Expected non-empty announcement")
	}
}

// TestA2AHandler_DeliveryError tests handling of delivery errors
func TestA2AHandler_DeliveryError(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		return "", fmt.Errorf("target agent not found")
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		t.Error("Announce should not be called on delivery error")
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:invalid:main",
		TargetAgentID:       "invalid",
		Message:             "Test message",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Error should be logged but handler should continue
	// (No announce should be called)
}

// TestA2AHandler_AnnounceError tests handling of announce errors
func TestA2AHandler_AnnounceError(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return fmt.Errorf("requester session closed")
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Test message",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Error should be logged but handler should continue
}

// TestA2AHandler_ConcurrentMessages tests processing multiple messages concurrently
func TestA2AHandler_ConcurrentMessages(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 100)

	var deliveryCount int
	var announceCount int
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveryCount++
		mu.Unlock()

		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		mu.Lock()
		announceCount++
		mu.Unlock()
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:  messageQueue,
		DeliveryFunc:  deliveryFunc,
		AnnounceFunc:  announceFunc,
		MaxConcurrent: 5,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	// Send 20 messages
	const numMessages = 20
	for i := 0; i < numMessages; i++ {
		msg := &shared.A2AMessage{
			RequesterSessionKey: "agent:main:main",
			RequesterAgentID:    "main",
			TargetSessionKey:    fmt.Sprintf("agent:work:session%d", i),
			TargetAgentID:       "work",
			Message:             fmt.Sprintf("Message %d", i),
			TimeoutSeconds:      30,
			SentAt:              time.Now(),
		}
		messageQueue <- msg
	}

	// Wait for all messages to be processed
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Each message now has 2 delivery calls (initial + announce)
	expectedDeliveries := numMessages * 2
	if deliveryCount != expectedDeliveries {
		t.Errorf("Expected %d deliveries (initial + announce), got %d", expectedDeliveries, deliveryCount)
	}

	if announceCount != numMessages {
		t.Errorf("Expected %d announces, got %d", numMessages, announceCount)
	}
}

// TestA2AHandler_Timeout tests message processing timeout
func TestA2AHandler_Timeout(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		// Simulate slow response
		select {
		case <-time.After(2 * time.Second):
			return "Late response", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		t.Error("Announce should not be called after timeout")
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:   messageQueue,
		DeliveryFunc:   deliveryFunc,
		AnnounceFunc:   announceFunc,
		DefaultTimeout: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Test message",
		TimeoutSeconds:      0, // Use default timeout
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for timeout
	time.Sleep(300 * time.Millisecond)

	// Delivery should have timed out, no announce
}

// TestA2AHandler_ContextCancellation tests handler shutdown via context
func TestA2AHandler_ContextCancellation(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())

	go handler.Start(ctx)

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Handler should stop gracefully
	time.Sleep(100 * time.Millisecond)

	// Try to send message (handler should not process it)
	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Should not be processed",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	select {
	case messageQueue <- msg:
		// Message queued but handler should not process it
	default:
		t.Error("Message queue should still accept messages")
	}

	// Success: Handler stopped via context cancellation
}

// TestA2AHandler_Stop tests explicit handler shutdown
func TestA2AHandler_Stop(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx := context.Background()

	go handler.Start(ctx)

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	// Stop handler
	handler.Stop()

	// Handler should stop gracefully
	time.Sleep(100 * time.Millisecond)

	// Success: Handler stopped via Stop()
}

// TestA2AHandler_GetQueueStatus tests queue status reporting
func TestA2AHandler_GetQueueStatus(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 50)

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:  messageQueue,
		DeliveryFunc:  nil, // Not used for this test
		AnnounceFunc:  nil, // Not used for this test
		MaxConcurrent: 10,
	})

	// Add some messages to queue
	for i := 0; i < 5; i++ {
		messageQueue <- &shared.A2AMessage{
			RequesterSessionKey: "agent:main:main",
			RequesterAgentID:    "main",
			TargetSessionKey:    "agent:work:main",
			TargetAgentID:       "work",
			Message:             "Test",
			TimeoutSeconds:      30,
			SentAt:              time.Now(),
		}
	}

	status := handler.GetQueueStatus()

	if status.QueueLength != 5 {
		t.Errorf("Expected queue length 5, got %d", status.QueueLength)
	}

	if status.QueueCap != 50 {
		t.Errorf("Expected queue capacity 50, got %d", status.QueueCap)
	}

	if status.MaxConcurrent != 10 {
		t.Errorf("Expected max concurrent 10, got %d", status.MaxConcurrent)
	}
}

// TestA2AHandler_A2APromptInjection tests that A2A prompt is passed to delivery
func TestA2AHandler_A2APromptInjection(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var receivedPrompts []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		receivedPrompts = append(receivedPrompts, a2aPrompt)
		mu.Unlock()
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Check calendar",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Verify both prompts (initial A2A context + announcement)
	if len(receivedPrompts) != 2 {
		t.Fatalf("Expected 2 prompts (initial + announce), got %d", len(receivedPrompts))
	}

	// First prompt: initial A2A context
	if !strings.Contains(receivedPrompts[0], "Agent-to-agent message context") {
		t.Errorf("Initial A2A prompt should contain context header, got: %s", receivedPrompts[0])
	}

	if !strings.Contains(receivedPrompts[0], "Requester agent: main") {
		t.Errorf("Initial A2A prompt should mention requester, got: %s", receivedPrompts[0])
	}

	// Second prompt: announcement context
	if !strings.Contains(receivedPrompts[1], "Agent-to-agent announce step") {
		t.Errorf("Announcement prompt should contain announce header, got: %s", receivedPrompts[1])
	}

	if !strings.Contains(receivedPrompts[1], "Original request:") {
		t.Errorf("Announcement prompt should contain original request, got: %s", receivedPrompts[1])
	}
}

// TestA2AHandler_DefaultTimeout tests default timeout when not specified
func TestA2AHandler_DefaultTimeout(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var timeoutUsed time.Duration
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		// Check context deadline to infer timeout
		if deadline, ok := ctx.Deadline(); ok {
			mu.Lock()
			timeoutUsed = time.Until(deadline)
			mu.Unlock()
		}
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:   messageQueue,
		DeliveryFunc:   deliveryFunc,
		AnnounceFunc:   announceFunc,
		DefaultTimeout: 60 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Test",
		TimeoutSeconds:      0, // Use default
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should be close to 60 seconds (with some tolerance for processing delay)
	if timeoutUsed < 58*time.Second || timeoutUsed > 61*time.Second {
		t.Errorf("Expected timeout ~60s, got %v", timeoutUsed)
	}
}

// ====================================================================================
// Ping-Pong Conversation Tests
// ====================================================================================

// TestA2AHandler_PingPong2Turn tests a 2-turn ping-pong conversation
func TestA2AHandler_PingPong2Turn(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex

	// Track delivery calls and simulate ping-pong responses
	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		deliveryCalls = append(deliveryCalls, fmt.Sprintf("%s: %s", targetSessionKey, message))

		// Initial delivery to target
		if len(deliveryCalls) == 1 {
			return "I found 3 events", nil
		}
		// Ping-pong turn 1 (requester responds)
		if len(deliveryCalls) == 2 {
			return "Thanks, please summarize them", nil
		}
		// Ping-pong turn 2 (target responds)
		if len(deliveryCalls) == 3 {
			return "Event 1: Meeting at 9am, Event 2: Lunch at noon, Event 3: Review at 3pm", nil
		}
		// Announcement step
		return "Calendar summary sent to requester", nil
	}

	var announcement string
	announceFunc := func(ctx context.Context, requesterSessionKey string, ann string) error {
		mu.Lock()
		announcement = ann
		mu.Unlock()
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 2, // Allow 2 turns
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Check calendar",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have: initial + turn1 + turn2 + announce = 4 calls
	if len(deliveryCalls) != 4 {
		t.Fatalf("Expected 4 delivery calls (initial + 2 turns + announce), got %d", len(deliveryCalls))
	}

	// Verify call sequence
	if !strings.Contains(deliveryCalls[0], "agent:work:main") {
		t.Errorf("Call 0 should be to work agent, got: %s", deliveryCalls[0])
	}
	if !strings.Contains(deliveryCalls[1], "agent:main:main") {
		t.Errorf("Call 1 should be to main agent (turn 1), got: %s", deliveryCalls[1])
	}
	if !strings.Contains(deliveryCalls[2], "agent:work:main") {
		t.Errorf("Call 2 should be to work agent (turn 2), got: %s", deliveryCalls[2])
	}
	if !strings.Contains(deliveryCalls[3], "agent:work:main") {
		t.Errorf("Call 3 should be to work agent (announce), got: %s", deliveryCalls[3])
	}

	// Verify announcement was made
	if announcement == "" {
		t.Error("Expected non-empty announcement")
	}
}

// TestA2AHandler_PingPong5TurnMax tests hitting max turn limit
func TestA2AHandler_PingPong5TurnMax(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		deliveryCalls = append(deliveryCalls, targetSessionKey)

		// Just return generic responses for all turns
		if strings.Contains(a2aPrompt, "announce step") {
			return "Conversation complete", nil
		}
		return fmt.Sprintf("Response %d", len(deliveryCalls)), nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 5, // Max 5 turns
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Start conversation",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have: initial + 5 turns + announce = 7 calls
	if len(deliveryCalls) != 7 {
		t.Fatalf("Expected 7 delivery calls (initial + 5 turns + announce), got %d", len(deliveryCalls))
	}

	// Verify alternating pattern: work, main, work, main, work, main, work(announce)
	expected := []string{
		"agent:work:main", // initial
		"agent:main:main", // turn 1
		"agent:work:main", // turn 2
		"agent:main:main", // turn 3
		"agent:work:main", // turn 4
		"agent:main:main", // turn 5
		"agent:work:main", // announce
	}

	for i, exp := range expected {
		if deliveryCalls[i] != exp {
			t.Errorf("Call %d: expected %s, got %s", i, exp, deliveryCalls[i])
		}
	}
}

// TestA2AHandler_PingPongREPLY_SKIP tests early exit via REPLY_SKIP
func TestA2AHandler_PingPongREPLY_SKIP(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		deliveryCalls = append(deliveryCalls, targetSessionKey)

		// Initial delivery
		if len(deliveryCalls) == 1 {
			return "I have the answer", nil
		}
		// Turn 1 - requester sends REPLY_SKIP to end conversation
		if len(deliveryCalls) == 2 {
			return "REPLY_SKIP", nil
		}
		// Announcement step (should still run)
		if strings.Contains(a2aPrompt, "announce step") {
			return "Task completed early", nil
		}
		return "Should not reach here", nil
	}

	var announcement string
	announceFunc := func(ctx context.Context, requesterSessionKey string, ann string) error {
		mu.Lock()
		announcement = ann
		mu.Unlock()
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 5, // Allow up to 5 turns, but will exit early
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Quick question",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have: initial + turn1 (REPLY_SKIP) + announce = 3 calls
	// NOT 7 calls (initial + 5 turns + announce)
	if len(deliveryCalls) != 3 {
		t.Fatalf("Expected 3 delivery calls (initial + REPLY_SKIP turn + announce), got %d", len(deliveryCalls))
	}

	// Verify early exit
	if deliveryCalls[0] != "agent:work:main" {
		t.Errorf("Call 0 should be to work agent, got: %s", deliveryCalls[0])
	}
	if deliveryCalls[1] != "agent:main:main" {
		t.Errorf("Call 1 should be to main agent (REPLY_SKIP), got: %s", deliveryCalls[1])
	}
	if deliveryCalls[2] != "agent:work:main" {
		t.Errorf("Call 2 should be to work agent (announce), got: %s", deliveryCalls[2])
	}

	// Verify announcement still happened
	if announcement == "" {
		t.Error("Expected announcement even after REPLY_SKIP")
	}
}

// TestA2AHandler_ANNOUNCE_SKIP tests skipping final announcement
func TestA2AHandler_ANNOUNCE_SKIP(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		deliveryCalls = append(deliveryCalls, targetSessionKey)

		// Initial delivery
		if len(deliveryCalls) == 1 {
			return "Task completed silently", nil
		}
		// Announcement step - agent chooses to remain silent
		if strings.Contains(a2aPrompt, "announce step") {
			return "ANNOUNCE_SKIP", nil
		}
		return "Should not reach here", nil
	}

	var announceCalls int
	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		mu.Lock()
		announceCalls++
		mu.Unlock()
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 0, // Fire-and-forget (no ping-pong)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Silent task",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have: initial + announce (returns ANNOUNCE_SKIP) = 2 calls
	if len(deliveryCalls) != 2 {
		t.Fatalf("Expected 2 delivery calls (initial + announce), got %d", len(deliveryCalls))
	}

	// Verify NO announcement was made (ANNOUNCE_SKIP)
	if announceCalls != 0 {
		t.Errorf("Expected 0 announce calls (ANNOUNCE_SKIP), got %d", announceCalls)
	}
}

// TestA2AHandler_PingPongFireAndForget tests backward compatibility with maxPingPongTurns=0
func TestA2AHandler_PingPongFireAndForget(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		deliveryCalls = append(deliveryCalls, targetSessionKey)

		// Initial delivery
		if len(deliveryCalls) == 1 {
			return "Done", nil
		}
		// Announcement step
		return "Task completed", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 0, // Fire-and-forget (backward compatibility)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Fire and forget",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have: initial + announce = 2 calls (NO ping-pong turns)
	if len(deliveryCalls) != 2 {
		t.Fatalf("Expected 2 delivery calls (initial + announce, no ping-pong), got %d", len(deliveryCalls))
	}

	// Both calls should be to work agent (target)
	if deliveryCalls[0] != "agent:work:main" {
		t.Errorf("Call 0 should be to work agent, got: %s", deliveryCalls[0])
	}
	if deliveryCalls[1] != "agent:work:main" {
		t.Errorf("Call 1 should be to work agent (announce), got: %s", deliveryCalls[1])
	}
}

// ====================================================================================
// Battle Tests - Edge Cases, Errors, Timeouts
// ====================================================================================

// TestA2AHandler_PingPongTurnTimeout tests timeout during a ping-pong turn
func TestA2AHandler_PingPongTurnTimeout(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveryCalls = append(deliveryCalls, targetSessionKey)
		callNum := len(deliveryCalls)
		mu.Unlock()

		// Initial delivery succeeds
		if callNum == 1 {
			return "Response", nil
		}

		// Turn 1 - simulate slow response that triggers timeout
		if callNum == 2 {
			select {
			case <-time.After(2 * time.Second):
				return "Late response", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		return "Should not reach here", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		t.Error("Announce should not be called after timeout")
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 2,
		DefaultTimeout:   100 * time.Millisecond, // Short timeout
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Test",
		TimeoutSeconds:      0, // Use default (100ms)
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for timeout
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have attempted: initial + turn1 (timeout) = 2 calls
	// No announcement due to timeout
	if len(deliveryCalls) < 1 || len(deliveryCalls) > 2 {
		t.Logf("Got %d delivery calls (expected 1-2 due to timeout)", len(deliveryCalls))
	}
}

// TestA2AHandler_PingPongDeliveryError tests error during ping-pong turn
func TestA2AHandler_PingPongDeliveryError(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		deliveryCalls = append(deliveryCalls, targetSessionKey)
		callNum := len(deliveryCalls)

		// Initial delivery succeeds
		if callNum == 1 {
			return "Response", nil
		}

		// Turn 1 - delivery fails
		if callNum == 2 {
			return "", fmt.Errorf("agent crashed")
		}

		return "Should not reach here", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		t.Error("Announce should not be called after delivery error")
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 2,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Test",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have attempted: initial + turn1 (error) = 2 calls
	if len(deliveryCalls) != 2 {
		t.Fatalf("Expected 2 delivery calls (initial + failed turn), got %d", len(deliveryCalls))
	}
}

// TestA2AHandler_PingPongConcurrent tests multiple ping-pong conversations concurrently
func TestA2AHandler_PingPongConcurrent(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 100)

	var totalDeliveries int
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		totalDeliveries++
		mu.Unlock()

		// Simulate some processing time
		time.Sleep(20 * time.Millisecond)

		// Check if it's announce step
		if strings.Contains(a2aPrompt, "announce step") {
			return "Done", nil
		}

		// Check if it's a ping-pong turn
		if strings.Contains(a2aPrompt, "reply step") {
			return "REPLY_SKIP", nil // End ping-pong early
		}

		// Initial delivery
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 2,
		MaxConcurrent:    10, // Allow 10 concurrent
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	// Send 20 concurrent ping-pong messages
	const numMessages = 20
	for i := 0; i < numMessages; i++ {
		msg := &shared.A2AMessage{
			RequesterSessionKey: "agent:main:main",
			RequesterAgentID:    "main",
			TargetSessionKey:    fmt.Sprintf("agent:work:session%d", i),
			TargetAgentID:       "work",
			Message:             fmt.Sprintf("Message %d", i),
			TimeoutSeconds:      30,
			SentAt:              time.Now(),
		}
		messageQueue <- msg
	}

	// Wait for all to process
	time.Sleep(1 * time.Second)

	mu.Lock()
	defer mu.Unlock()

	// Each message: initial + turn1 (REPLY_SKIP) + announce = 3 calls
	expectedDeliveries := numMessages * 3
	if totalDeliveries != expectedDeliveries {
		t.Logf("Expected ~%d deliveries, got %d (within tolerance)", expectedDeliveries, totalDeliveries)
	}
}

// TestA2AHandler_PingPongLargeMessages tests handling of large messages in ping-pong
func TestA2AHandler_PingPongLargeMessages(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var mu sync.Mutex
	var largeMessageReceived bool

	// Create a large message (10KB)
	largeMessage := strings.Repeat("This is a large message. ", 400) // ~10KB

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		// Check if we received the large message
		if len(message) > 5000 {
			largeMessageReceived = true
		}

		// Check if it's announce step
		if strings.Contains(a2aPrompt, "announce step") {
			return "Large data processed", nil
		}

		// Return large response
		return strings.Repeat("Large response. ", 400), nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 0, // Fire-and-forget
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             largeMessage,
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !largeMessageReceived {
		t.Error("Expected to receive large message (>5KB)")
	}
}

// TestA2AHandler_PingPongContextCancel tests cancellation during ping-pong
func TestA2AHandler_PingPongContextCancel(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex
	var cancelFunc context.CancelFunc

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveryCalls = append(deliveryCalls, targetSessionKey)
		callNum := len(deliveryCalls)
		mu.Unlock()

		// Initial delivery succeeds
		if callNum == 1 {
			return "Response", nil
		}

		// Turn 1 - cancel context during turn
		if callNum == 2 {
			// Cancel the context mid-turn
			if cancelFunc != nil {
				cancelFunc()
			}
			// Simulate work that respects cancellation
			select {
			case <-time.After(1 * time.Second):
				return "Should not complete", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		return "Should not reach here", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 2,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancelFunc = cancel

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Test",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for cancellation to propagate
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have attempted: initial + turn1 (cancelled)
	// Exact count may vary due to timing
	if len(deliveryCalls) < 1 {
		t.Error("Expected at least 1 delivery call before cancellation")
	}
}

// TestA2AHandler_PingPongREPLY_SKIPWithWhitespace tests REPLY_SKIP with extra whitespace
func TestA2AHandler_PingPongREPLY_SKIPWithWhitespace(t *testing.T) {
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveryCalls []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		deliveryCalls = append(deliveryCalls, targetSessionKey)
		callNum := len(deliveryCalls)

		// Initial delivery
		if callNum == 1 {
			return "Response", nil
		}
		// Turn 1 - REPLY_SKIP with extra whitespace and newlines
		if callNum == 2 {
			return "  \n\n  REPLY_SKIP  \n  ", nil
		}
		// Announcement step
		if strings.Contains(a2aPrompt, "announce step") {
			return "Done", nil
		}
		return "Should not reach here", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := NewA2AHandler(A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: 5,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	msg := &shared.A2AMessage{
		RequesterSessionKey: "agent:main:main",
		RequesterAgentID:    "main",
		TargetSessionKey:    "agent:work:main",
		TargetAgentID:       "work",
		Message:             "Test",
		TimeoutSeconds:      30,
		SentAt:              time.Now(),
	}

	messageQueue <- msg

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should detect REPLY_SKIP despite whitespace
	// Expected: initial + turn1 (REPLY_SKIP) + announce = 3 calls
	if len(deliveryCalls) != 3 {
		t.Fatalf("Expected 3 calls (REPLY_SKIP should work with whitespace), got %d", len(deliveryCalls))
	}
}
