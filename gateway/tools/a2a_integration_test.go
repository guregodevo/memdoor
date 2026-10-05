package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/a2a"
	"memdoor/gateway/logs"
	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// TestIntegration_EndToEndFlow tests complete A2A message flow from tool to announce
func TestIntegration_EndToEndFlow(t *testing.T) {
	if err := logs.InitGlobalLoggerDefault(false); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	// Setup policy
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"},
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	// Track agent interactions
	var deliveredMessages []string
	var announcements []string
	var mu sync.Mutex

	// Mock delivery function (simulates work agent processing)
	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveredMessages = append(deliveredMessages, fmt.Sprintf("%s: %s", targetSessionKey, message))
		mu.Unlock()

		// Simulate work agent checking calendar
		return "Calendar checked: 3 events on 2026-02-21, conflict at 2pm", nil
	}

	// Mock announce function (simulates announcing back to main agent)
	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		mu.Lock()
		announcements = append(announcements, fmt.Sprintf("%s: %s", requesterSessionKey, announcement))
		mu.Unlock()
		return nil
	}

	// Create handler
	handler := a2a.NewA2AHandler(a2a.A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start handler
	go handler.Start(ctx)

	// Create sessions_send tool
	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	// Step 1: Main agent sends message to work agent
	sessionKey := "agent:work:main"
	params := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "Check calendar for conflicts on 2026-02-21",
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("sessions_send failed: %v", err)
	}

	var result SessionsSendResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify tool result
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Message)
	}

	if result.Status != "accepted" {
		t.Errorf("Expected status 'accepted', got '%s'", result.Status)
	}

	// Step 2: Wait for handler to process
	time.Sleep(200 * time.Millisecond)

	// Step 3: Verify message was delivered to work agent
	mu.Lock()
	defer mu.Unlock()

	// Week 28: Expect 2 deliveries (initial message + announcement formatting step)
	if len(deliveredMessages) != 2 {
		t.Fatalf("Expected 2 delivered messages (initial + announce), got %d", len(deliveredMessages))
	}

	// First delivery: the actual message
	if deliveredMessages[0] != "agent:work:main: Check calendar for conflicts on 2026-02-21" {
		t.Errorf("Unexpected first delivery: %s", deliveredMessages[0])
	}

	// Second delivery: announcement formatting step
	if deliveredMessages[1] != "agent:work:main: Agent-to-agent announce step." {
		t.Errorf("Unexpected second delivery (announce step): %s", deliveredMessages[1])
	}

	// Step 4: Verify response was announced back to main agent
	if len(announcements) != 1 {
		t.Fatalf("Expected 1 announcement, got %d", len(announcements))
	}

	announcement := announcements[0]
	if !contains(announcement, "agent:main:main") {
		t.Errorf("Announcement should go to main agent: %s", announcement)
	}

	// Week 28: Target agent formats its own announcement, no longer wrapped with "Response from work agent"
	// Just verify the actual content is present
	if !contains(announcement, "3 events") {
		t.Errorf("Announcement should include work agent's response: %s", announcement)
	}

	if !contains(announcement, "conflict at 2pm") {
		t.Errorf("Announcement should include full response: %s", announcement)
	}
}

// TestIntegration_PolicyDenial tests that policy denial prevents full flow
func TestIntegration_PolicyDenial(t *testing.T) {
	// Policy: Only main→work allowed
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"},
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveredMessages []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveredMessages = append(deliveredMessages, targetSessionKey)
		mu.Unlock()
		return "Should not happen", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := a2a.NewA2AHandler(a2a.A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	// Try to send from work→main (forbidden)
	tool := NewSessionsSendTool(policy, resolver, messageQueue, "work", false)

	sessionKey := "agent:main:main"
	params := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "This should be denied",
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, _ := tool.Execute(string(paramsJSON))

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	// Verify tool denied the request
	if result.Success {
		t.Error("Expected policy denial")
	}

	if result.Status != "forbidden" {
		t.Errorf("Expected status 'forbidden', got '%s'", result.Status)
	}

	// Wait to ensure no message was delivered
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(deliveredMessages) != 0 {
		t.Errorf("No messages should be delivered on policy denial, got: %v", deliveredMessages)
	}
}

// TestIntegration_MultipleConcurrentA2A tests multiple agents messaging concurrently
func TestIntegration_MultipleConcurrentA2A(t *testing.T) {
	// Policy: Allow all
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "*", To: "*"},
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 100)

	var deliveryCount int
	var announceCount int
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveryCount++
		mu.Unlock()

		time.Sleep(10 * time.Millisecond) // Simulate processing
		return fmt.Sprintf("Response from %s", targetSessionKey), nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		mu.Lock()
		announceCount++
		mu.Unlock()
		return nil
	}

	handler := a2a.NewA2AHandler(a2a.A2AHandlerConfig{
		MessageQueue:  messageQueue,
		DeliveryFunc:  deliveryFunc,
		AnnounceFunc:  announceFunc,
		MaxConcurrent: 10,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	// Test scenario: 3 agents sending messages concurrently
	testCases := []struct {
		from  string
		to    string
		count int
	}{
		{"main", "work", 5},
		{"main", "data", 5},
		{"work", "data", 5},
	}

	var wg sync.WaitGroup

	for _, tc := range testCases {
		wg.Add(1)
		go func(from, to string, count int) {
			defer wg.Done()

			tool := NewSessionsSendTool(policy, resolver, messageQueue, from, false)

			for i := 0; i < count; i++ {
				sessionKey := fmt.Sprintf("agent:%s:main", to)
				params := SessionsSendParams{
					SessionKey: &sessionKey,
					Message:    fmt.Sprintf("Message %d from %s to %s", i, from, to),
				}

				paramsJSON, _ := json.Marshal(params)
				resultJSON, err := tool.Execute(string(paramsJSON))
				if err != nil {
					t.Errorf("Tool execution failed: %v", err)
					return
				}

				var result SessionsSendResult
				json.Unmarshal([]byte(resultJSON), &result)

				if !result.Success {
					t.Errorf("Expected success, got: %s", result.Message)
				}
			}
		}(tc.from, tc.to, tc.count)
	}

	wg.Wait()

	// Wait for all messages to be processed
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	expectedMessages := 15 // 5+5+5
	// Week 28: Each message results in 2 deliveries (initial + announce formatting)
	expectedDeliveries := expectedMessages * 2 // 30 total
	if deliveryCount != expectedDeliveries {
		t.Errorf("Expected %d deliveries (15 messages × 2 calls), got %d", expectedDeliveries, deliveryCount)
	}

	if announceCount != expectedMessages {
		t.Errorf("Expected %d announces, got %d", expectedMessages, announceCount)
	}
}

// TestIntegration_DeliveryTimeout tests timeout handling in full flow
func TestIntegration_DeliveryTimeout(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	var announceCount int
	var mu sync.Mutex

	// Slow delivery function (will timeout)
	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		select {
		case <-time.After(2 * time.Second):
			return "Late response", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		mu.Lock()
		announceCount++
		mu.Unlock()
		return nil
	}

	handler := a2a.NewA2AHandler(a2a.A2AHandlerConfig{
		MessageQueue:   messageQueue,
		DeliveryFunc:   deliveryFunc,
		AnnounceFunc:   announceFunc,
		DefaultTimeout: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	sessionKey := "agent:work:main"
	timeout := 1 // 1 second timeout (but delivery takes 2 seconds)
	params := SessionsSendParams{
		SessionKey:     &sessionKey,
		Message:        "This will timeout",
		TimeoutSeconds: &timeout,
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, _ := tool.Execute(string(paramsJSON))

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	// Tool should accept the message
	if !result.Success {
		t.Errorf("Tool should accept message, got: %s", result.Message)
	}

	// Wait for timeout
	time.Sleep(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()

	// Announce should NOT be called due to timeout
	if announceCount != 0 {
		t.Errorf("Announce should not be called after timeout, got %d calls", announceCount)
	}
}

// TestIntegration_LabelResolution tests label+agentId resolution in full flow
func TestIntegration_LabelResolution(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	var deliveredTo string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveredTo = targetSessionKey
		mu.Unlock()
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := a2a.NewA2AHandler(a2a.A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	// Use label + agentId instead of sessionKey
	label := "main"
	agentId := "work"
	params := SessionsSendParams{
		Label:   &label,
		AgentID: &agentId,
		Message: "Test label resolution",
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, _ := tool.Execute(string(paramsJSON))

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Message)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Verify message was delivered to constructed session key
	if deliveredTo != "agent:work:main" {
		t.Errorf("Expected delivery to 'agent:work:main', got '%s'", deliveredTo)
	}
}

// TestIntegration_PolicyMatrix tests various policy combinations
func TestIntegration_PolicyMatrix(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"},
			{From: "main", To: "data"},
			{From: "*", To: "monitoring"},
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 100)

	var deliveredPairs []string
	var mu sync.Mutex

	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		mu.Lock()
		deliveredPairs = append(deliveredPairs, targetSessionKey)
		mu.Unlock()
		return "Response", nil
	}

	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		return nil
	}

	handler := a2a.NewA2AHandler(a2a.A2AHandlerConfig{
		MessageQueue: messageQueue,
		DeliveryFunc: deliveryFunc,
		AnnounceFunc: announceFunc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handler.Start(ctx)

	testCases := []struct {
		from          string
		to            string
		shouldSucceed bool
	}{
		{"main", "work", true},       // Explicit rule
		{"main", "data", true},       // Explicit rule
		{"work", "monitoring", true}, // Wildcard to monitoring
		{"data", "monitoring", true}, // Wildcard to monitoring
		{"work", "main", false},      // No rule
		{"work", "data", false},      // No rule
		{"data", "work", false},      // No rule
	}

	for _, tc := range testCases {
		tool := NewSessionsSendTool(policy, resolver, messageQueue, tc.from, false)

		sessionKey := fmt.Sprintf("agent:%s:main", tc.to)
		params := SessionsSendParams{
			SessionKey: &sessionKey,
			Message:    fmt.Sprintf("Message from %s to %s", tc.from, tc.to),
		}

		paramsJSON, _ := json.Marshal(params)
		resultJSON, _ := tool.Execute(string(paramsJSON))

		var result SessionsSendResult
		json.Unmarshal([]byte(resultJSON), &result)

		if tc.shouldSucceed && !result.Success {
			t.Errorf("%s→%s should succeed, got: %s", tc.from, tc.to, result.Message)
		}

		if !tc.shouldSucceed && result.Success {
			t.Errorf("%s→%s should fail, but succeeded", tc.from, tc.to)
		}
	}

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Week 28: 4 successful messages, each with 2 deliveries (initial + announce)
	expectedDeliveries := 4 * 2 // 8 total
	if len(deliveredPairs) != expectedDeliveries {
		t.Errorf("Expected %d successful deliveries (4 messages × 2 calls), got %d: %v", expectedDeliveries, len(deliveredPairs), deliveredPairs)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != "" && substr != "" &&
		len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
