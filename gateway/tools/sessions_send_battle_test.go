package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/a2a"
	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// TestSessionsSendTool_ConcurrentMessages tests multiple goroutines sending messages
func TestSessionsSendTool_ConcurrentMessages(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 300) // Large enough for all messages

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	const numGoroutines = 20
	const messagesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	successCount := 0
	var mu sync.Mutex

	// Launch concurrent senders
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				sessionKey := fmt.Sprintf("agent:worker%d:main", goroutineID%5)
				params := SessionsSendParams{
					SessionKey: &sessionKey,
					Message:    fmt.Sprintf("Message %d from goroutine %d", j, goroutineID),
				}

				paramsJSON, _ := json.Marshal(params)
				resultJSON, err := tool.Execute(string(paramsJSON))
				if err != nil {
					t.Errorf("Goroutine %d: Unexpected error: %v", goroutineID, err)
					continue
				}

				var result SessionsSendResult
				json.Unmarshal([]byte(resultJSON), &result)

				if result.Success {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	expectedMessages := numGoroutines * messagesPerGoroutine
	if successCount != expectedMessages {
		t.Errorf("Expected %d successful messages, got %d", expectedMessages, successCount)
	}

	// Verify all messages were queued
	queuedCount := len(messageQueue)
	if queuedCount != expectedMessages {
		t.Errorf("Expected %d messages in queue, got %d", expectedMessages, queuedCount)
	}

	t.Logf("Successfully sent %d concurrent messages", successCount)
}

// TestSessionsSendTool_LargeMessage tests handling of very large messages
func TestSessionsSendTool_LargeMessage(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	// Create a 100KB message
	largeMessage := strings.Repeat("This is a large message with lots of data. ", 2000) // ~88KB

	sessionKey := "agent:work:main"
	params := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    largeMessage,
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if !result.Success {
		t.Errorf("Expected success for large message, got failure: %s", result.Message)
	}

	// Verify message was queued with full content
	msg := <-messageQueue
	if len(msg.Message) != len(largeMessage) {
		t.Errorf("Expected message length %d, got %d", len(largeMessage), len(msg.Message))
	}

	if msg.Message != largeMessage {
		t.Error("Large message content was corrupted")
	}

	t.Logf("Successfully sent large message: %d bytes", len(largeMessage))
}

// TestSessionsSendTool_UnicodeAndSpecialChars tests unicode and special characters
func TestSessionsSendTool_UnicodeAndSpecialChars(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	testMessages := []string{
		"Hello 世界 🌍",                                               // Unicode + emoji
		"SELECT * FROM users WHERE name = 'O\"Reilly'",             // SQL with quotes
		"{\n  \"nested\": {\n    \"json\": true\n  }\n}",           // Nested JSON
		"Path: C:\\Users\\test\\file.txt",                          // Windows path
		"Regex: ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$", // Regex
		"Math: ∑ ∫ ∂ ∇ ∞ ≈ ≠ ≤ ≥",                                  // Math symbols
		"Line1\nLine2\rLine3\r\nLine4\tTabbed",                     // Newlines and tabs
	}

	sessionKey := "agent:work:main"

	for i, testMsg := range testMessages {
		params := SessionsSendParams{
			SessionKey: &sessionKey,
			Message:    testMsg,
		}

		paramsJSON, _ := json.Marshal(params)
		resultJSON, err := tool.Execute(string(paramsJSON))
		if err != nil {
			t.Errorf("Test %d: Unexpected error: %v", i, err)
			continue
		}

		var result SessionsSendResult
		json.Unmarshal([]byte(resultJSON), &result)

		if !result.Success {
			t.Errorf("Test %d: Expected success, got failure: %s", i, result.Message)
		}

		// Verify message was queued correctly
		msg := <-messageQueue
		if msg.Message != testMsg {
			t.Errorf("Test %d: Message corrupted.\nExpected: %s\nGot: %s", i, testMsg, msg.Message)
		}
	}

	t.Logf("Successfully tested %d messages with special characters", len(testMessages))
}

// TestSessionsSendTool_RapidFire tests rapid message sending
func TestSessionsSendTool_RapidFire(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 1000)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	const numMessages = 500
	start := time.Now()

	for i := 0; i < numMessages; i++ {
		sessionKey := "agent:work:main"
		params := SessionsSendParams{
			SessionKey: &sessionKey,
			Message:    fmt.Sprintf("Rapid message %d", i),
		}

		paramsJSON, _ := json.Marshal(params)
		resultJSON, err := tool.Execute(string(paramsJSON))
		if err != nil {
			t.Fatalf("Message %d: Unexpected error: %v", i, err)
		}

		var result SessionsSendResult
		json.Unmarshal([]byte(resultJSON), &result)

		if !result.Success {
			t.Errorf("Message %d: Expected success, got failure: %s", i, result.Message)
		}
	}

	elapsed := time.Since(start)
	messagesPerSecond := float64(numMessages) / elapsed.Seconds()

	if len(messageQueue) != numMessages {
		t.Errorf("Expected %d messages in queue, got %d", numMessages, len(messageQueue))
	}

	t.Logf("Sent %d messages in %v (%.0f msg/sec)", numMessages, elapsed, messagesPerSecond)
}

// TestSessionsSendTool_PolicyConcurrentChecks tests concurrent policy validation
func TestSessionsSendTool_PolicyConcurrentChecks(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"},
			{From: "main", To: "data"},
			{From: "work", To: "data"},
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 100)

	// Create tools for different requester agents
	mainTool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)
	workTool := NewSessionsSendTool(policy, resolver, messageQueue, "work", false)
	dataTool := NewSessionsSendTool(policy, resolver, messageQueue, "data", false)

	var wg sync.WaitGroup
	testCases := []struct {
		tool          *SessionsSendTool
		target        string
		shouldSucceed bool
	}{
		{mainTool, "agent:work:main", true},  // main→work: allowed
		{mainTool, "agent:data:main", true},  // main→data: allowed
		{workTool, "agent:data:main", true},  // work→data: allowed
		{workTool, "agent:main:main", false}, // work→main: denied
		{dataTool, "agent:work:main", false}, // data→work: denied
		{dataTool, "agent:main:main", false}, // data→main: denied
	}

	results := make([]bool, len(testCases))
	var mu sync.Mutex

	wg.Add(len(testCases))

	// Run all policy checks concurrently
	for i, tc := range testCases {
		go func(idx int, testCase struct {
			tool          *SessionsSendTool
			target        string
			shouldSucceed bool
		}) {
			defer wg.Done()

			params := SessionsSendParams{
				SessionKey: &testCase.target,
				Message:    "Policy check",
			}

			paramsJSON, _ := json.Marshal(params)
			resultJSON, _ := testCase.tool.Execute(string(paramsJSON))

			var result SessionsSendResult
			json.Unmarshal([]byte(resultJSON), &result)

			mu.Lock()
			results[idx] = result.Success
			mu.Unlock()
		}(i, tc)
	}

	wg.Wait()

	// Verify results
	for i, tc := range testCases {
		if results[i] != tc.shouldSucceed {
			t.Errorf("Test case %d: Expected success=%v, got %v", i, tc.shouldSucceed, results[i])
		}
	}

	t.Logf("Successfully validated %d concurrent policy checks", len(testCases))
}

// TestSessionsSendTool_QueueRecovery tests recovery from queue full condition
func TestSessionsSendTool_QueueRecovery(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 5) // Small queue

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	sessionKey := "agent:work:main"

	// Fill the queue
	for i := 0; i < 5; i++ {
		params := SessionsSendParams{
			SessionKey: &sessionKey,
			Message:    fmt.Sprintf("Message %d", i),
		}
		paramsJSON, _ := json.Marshal(params)
		tool.Execute(string(paramsJSON))
	}

	// Try to send when full (should fail)
	params := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "This should fail",
	}
	paramsJSON, _ := json.Marshal(params)
	resultJSON, _ := tool.Execute(string(paramsJSON))

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if result.Success {
		t.Error("Expected failure when queue is full")
	}

	// Drain one message
	<-messageQueue

	// Now it should succeed again
	params2 := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "This should succeed",
	}
	paramsJSON2, _ := json.Marshal(params2)
	resultJSON2, _ := tool.Execute(string(paramsJSON2))

	var result2 SessionsSendResult
	json.Unmarshal([]byte(resultJSON2), &result2)

	if !result2.Success {
		t.Error("Expected success after draining queue")
	}

	t.Log("Successfully recovered from queue full condition")
}

// TestSessionsSendTool_SubagentSession tests messaging to subagent sessions
func TestSessionsSendTool_SubagentSession(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	// Subagent session format: agent:agentId:subagent:runId
	subagentSessionKey := "agent:work:subagent:run-12345"
	params := SessionsSendParams{
		SessionKey: &subagentSessionKey,
		Message:    "Message to subagent",
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if !result.Success {
		t.Errorf("Expected success for subagent session, got failure: %s", result.Message)
	}

	if result.TargetAgentID != "work" {
		t.Errorf("Expected targetAgentId 'work', got '%s'", result.TargetAgentID)
	}

	// Verify message was queued
	msg := <-messageQueue
	if msg.TargetSessionKey != subagentSessionKey {
		t.Errorf("Expected target session '%s', got '%s'", subagentSessionKey, msg.TargetSessionKey)
	}

	t.Log("Successfully sent message to subagent session")
}

// TestSessionsSendTool_MultipleAgentPairs tests various agent-to-agent combinations
func TestSessionsSendTool_MultipleAgentPairs(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "*"},       // main can message anyone
			{From: "*", To: "monitoring"}, // anyone can message monitoring
			{From: "work", To: "data"},    // specific pair
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 100)

	testCases := []struct {
		from          string
		to            string
		shouldSucceed bool
		reason        string
	}{
		{"main", "work", true, "main→* wildcard"},
		{"main", "data", true, "main→* wildcard"},
		{"main", "monitoring", true, "main→* wildcard"},
		{"work", "monitoring", true, "*→monitoring wildcard"},
		{"data", "monitoring", true, "*→monitoring wildcard"},
		{"work", "data", true, "explicit work→data rule"},
		{"data", "work", false, "no rule for data→work"},
		{"work", "main", false, "no rule for work→main"},
		{"scheduler", "monitoring", true, "*→monitoring wildcard"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_to_%s", tc.from, tc.to), func(t *testing.T) {
			tool := NewSessionsSendTool(policy, resolver, messageQueue, tc.from, false)

			targetSession := fmt.Sprintf("agent:%s:main", tc.to)
			params := SessionsSendParams{
				SessionKey: &targetSession,
				Message:    fmt.Sprintf("Test message from %s to %s", tc.from, tc.to),
			}

			paramsJSON, _ := json.Marshal(params)
			resultJSON, _ := tool.Execute(string(paramsJSON))

			var result SessionsSendResult
			json.Unmarshal([]byte(resultJSON), &result)

			if result.Success != tc.shouldSucceed {
				t.Errorf("Expected success=%v (%s), got %v", tc.shouldSucceed, tc.reason, result.Success)
			}

			// Drain queue if message was sent
			if result.Success {
				<-messageQueue
			}
		})
	}

	t.Logf("Successfully tested %d agent pair combinations", len(testCases))
}

// TestSessionsSendTool_StressTest combines multiple stress factors
func TestSessionsSendTool_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 2000)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	const (
		numGoroutines        = 50
		messagesPerGoroutine = 20
		messageSizeVariance  = true
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				// Vary message size
				messageSize := 100
				if messageSizeVariance {
					messageSize = 100 + (goroutineID*j)%5000
				}
				message := strings.Repeat("x", messageSize)

				// Vary timeout
				timeout := 30 + (j % 60)

				sessionKey := fmt.Sprintf("agent:worker%d:session%d", goroutineID%10, j%5)
				params := SessionsSendParams{
					SessionKey:     &sessionKey,
					Message:        message,
					TimeoutSeconds: &timeout,
				}

				paramsJSON, _ := json.Marshal(params)
				resultJSON, err := tool.Execute(string(paramsJSON))

				mu.Lock()
				if err != nil {
					errorCount++
				} else {
					var result SessionsSendResult
					json.Unmarshal([]byte(resultJSON), &result)
					if result.Success {
						successCount++
					} else {
						errorCount++
					}
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalMessages := numGoroutines * messagesPerGoroutine
	messagesPerSecond := float64(successCount) / elapsed.Seconds()

	if errorCount > 0 {
		t.Errorf("Expected no errors, got %d errors out of %d messages", errorCount, totalMessages)
	}

	if successCount != totalMessages {
		t.Errorf("Expected %d successful messages, got %d", totalMessages, successCount)
	}

	t.Logf("Stress test results:")
	t.Logf("  Total messages: %d", totalMessages)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Throughput: %.0f msg/sec", messagesPerSecond)
	t.Logf("  Goroutines: %d", numGoroutines)
}

// UNHAPPY PATH BATTLE TESTS

// TestSessionsSendTool_MalformedSessionKeys tests various invalid session key formats
func TestSessionsSendTool_MalformedSessionKeys(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	malformedKeys := []struct {
		sessionKey string
		reason     string
	}{
		{"", "empty string"},
		{"agent", "too few parts"},
		{"agent:work", "missing session ID"},
		{"invalid:work:main", "invalid prefix"},
		{"agent::main", "empty agent ID"},
		{"agent:work:", "empty session ID"},
		{"agent:work:main:extra:parts:here", "too many parts (valid, should work)"},
		{"cron:", "incomplete cron key"},
		{"::::", "all colons"},
		{"agent work main", "spaces instead of colons"},
		{"AGENT:WORK:MAIN", "uppercase (should work)"},
	}

	for _, tc := range malformedKeys {
		t.Run(tc.reason, func(t *testing.T) {
			params := SessionsSendParams{
				SessionKey: &tc.sessionKey,
				Message:    "Test message",
			}

			paramsJSON, _ := json.Marshal(params)
			resultJSON, err := tool.Execute(string(paramsJSON))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var result SessionsSendResult
			json.Unmarshal([]byte(resultJSON), &result)

			// Most should fail (except the valid edge cases)
			validEdgeCases := []string{
				"agent:work:main:extra:parts:here", // Subagent format
				"AGENT:WORK:MAIN",                  // Uppercase (if supported)
			}

			isValidEdgeCase := false
			for _, valid := range validEdgeCases {
				if tc.sessionKey == valid {
					isValidEdgeCase = true
					break
				}
			}

			if !isValidEdgeCase && result.Success {
				t.Errorf("Malformed key '%s' should have failed (%s)", tc.sessionKey, tc.reason)
			}

			if !isValidEdgeCase && result.ErrorCode != "RESOLUTION_FAILED" {
				t.Logf("Expected RESOLUTION_FAILED for malformed key, got: %s", result.ErrorCode)
			}
		})
	}

	// Verify no messages were queued for invalid keys
	queuedCount := len(messageQueue)
	if queuedCount > 2 { // At most 2 valid edge cases
		t.Errorf("Too many messages queued for malformed keys: %d", queuedCount)
	}
}

// TestSessionsSendTool_PolicyDisabled tests that disabled policy rejects everything
func TestSessionsSendTool_PolicyDisabled(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: false, // Disabled!
		Allow: []authorization.A2ARule{
			{From: "*", To: "*"}, // Even allow-all rule should be ignored
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	// Try to send multiple messages
	testTargets := []string{"agent:work:main", "agent:data:main", "agent:monitoring:main"}

	for _, target := range testTargets {
		sessionKey := target
		params := SessionsSendParams{
			SessionKey: &sessionKey,
			Message:    "This should be denied",
		}

		paramsJSON, _ := json.Marshal(params)
		resultJSON, _ := tool.Execute(string(paramsJSON))

		var result SessionsSendResult
		json.Unmarshal([]byte(resultJSON), &result)

		if result.Success {
			t.Errorf("Disabled policy should reject all messages to %s", target)
		}

		if result.Status != "forbidden" {
			t.Errorf("Expected status 'forbidden', got '%s'", result.Status)
		}
	}

	// Verify no messages were queued
	if len(messageQueue) > 0 {
		t.Errorf("Disabled policy should not queue any messages, got %d", len(messageQueue))
	}
}

// TestSessionsSendTool_ConcurrentFailures tests concurrent failure scenarios
func TestSessionsSendTool_ConcurrentFailures(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"}, // Only this pair allowed
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 100)

	const numGoroutines = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	forbiddenCount := 0
	var mu sync.Mutex

	// Launch goroutines trying forbidden agent pairs
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			// Try various forbidden combinations
			forbiddenPairs := []struct {
				from string
				to   string
			}{
				{"work", "main"}, // Reverse of allowed
				{"data", "work"}, // Not in policy
				{"main", "data"}, // Not in policy
				{"foo", "bar"},   // Not in policy
			}

			for _, pair := range forbiddenPairs {
				tool := NewSessionsSendTool(policy, resolver, messageQueue, pair.from, false)

				targetSession := fmt.Sprintf("agent:%s:main", pair.to)
				params := SessionsSendParams{
					SessionKey: &targetSession,
					Message:    "Should be forbidden",
				}

				paramsJSON, _ := json.Marshal(params)
				resultJSON, _ := tool.Execute(string(paramsJSON))

				var result SessionsSendResult
				json.Unmarshal([]byte(resultJSON), &result)

				if !result.Success && result.Status == "forbidden" {
					mu.Lock()
					forbiddenCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	expectedForbidden := numGoroutines * 4 // 4 forbidden pairs per goroutine
	if forbiddenCount != expectedForbidden {
		t.Errorf("Expected %d forbidden results, got %d", expectedForbidden, forbiddenCount)
	}

	// Verify no forbidden messages were queued
	if len(messageQueue) > 0 {
		t.Errorf("Forbidden messages should not be queued, got %d in queue", len(messageQueue))
	}

	t.Logf("Successfully rejected %d concurrent forbidden requests", forbiddenCount)
}

// TestSessionsSendTool_InvalidParameterCombinations tests various invalid param combos
func TestSessionsSendTool_InvalidParameterCombinations(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	testCases := []struct {
		name          string
		paramsJSON    string
		expectedError bool
	}{
		{
			name:          "Missing message field",
			paramsJSON:    `{"sessionKey": "agent:work:main"}`,
			expectedError: true,
		},
		{
			name:          "Empty JSON object",
			paramsJSON:    `{}`,
			expectedError: true,
		},
		{
			name:          "Label without agentId",
			paramsJSON:    `{"label": "main", "message": "test"}`,
			expectedError: true,
		},
		{
			name:          "Only timeout, no target",
			paramsJSON:    `{"message": "test", "timeoutSeconds": 30}`,
			expectedError: true,
		},
		{
			name:          "Null sessionKey",
			paramsJSON:    `{"sessionKey": null, "message": "test"}`,
			expectedError: true,
		},
		{
			name:          "Array instead of object",
			paramsJSON:    `["sessionKey", "message"]`,
			expectedError: true,
		},
		{
			name:          "String instead of object",
			paramsJSON:    `"not an object"`,
			expectedError: true,
		},
		{
			name:          "Number instead of object",
			paramsJSON:    `12345`,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resultJSON, err := tool.Execute(tc.paramsJSON)
			if err != nil && !tc.expectedError {
				t.Errorf("Unexpected error: %v", err)
			}

			var result SessionsSendResult
			if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
				// Some cases might not even produce valid JSON result
				return
			}

			if tc.expectedError && result.Success {
				t.Errorf("Expected failure for invalid params: %s", tc.name)
			}
		})
	}
}

// TestSessionsSendTool_EdgeCaseAgentIDs tests unusual but valid agent IDs
func TestSessionsSendTool_EdgeCaseAgentIDs(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 20)

	edgeCaseAgents := []struct {
		agentID     string
		shouldWork  bool
		description string
	}{
		{"a", true, "single character"},
		{"agent-with-dashes", true, "dashes"},
		{"agent_with_underscores", true, "underscores"},
		{"agent123", true, "numbers"},
		{"Agent-V2.1", true, "dots and version"},
		{"very-long-agent-name-with-many-words-and-dashes", true, "very long name"},
		{"🤖", true, "emoji (if supported)"},
		{"agent.prod", true, "dot notation"},
	}

	for _, tc := range edgeCaseAgents {
		t.Run(tc.description, func(t *testing.T) {
			tool := NewSessionsSendTool(policy, resolver, messageQueue, tc.agentID, false)

			targetSession := "agent:work:main"
			params := SessionsSendParams{
				SessionKey: &targetSession,
				Message:    "Test edge case agent ID",
			}

			paramsJSON, _ := json.Marshal(params)
			resultJSON, err := tool.Execute(string(paramsJSON))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var result SessionsSendResult
			json.Unmarshal([]byte(resultJSON), &result)

			if tc.shouldWork && !result.Success {
				t.Errorf("Agent ID '%s' should work, got failure: %s", tc.agentID, result.Message)
			}

			if result.Success {
				// Verify message was queued with correct requester
				msg := <-messageQueue
				if msg.RequesterAgentID != tc.agentID {
					t.Errorf("Expected requester '%s', got '%s'", tc.agentID, msg.RequesterAgentID)
				}
			}
		})
	}
}

// TestSessionsSendTool_ConcurrentQueueExhaustion tests queue exhaustion under load
func TestSessionsSendTool_ConcurrentQueueExhaustion(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10) // Very small queue

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	const numGoroutines = 50
	const messagesPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	queueFullCount := 0
	successCount := 0
	var mu sync.Mutex

	// Many goroutines trying to overwhelm small queue
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				sessionKey := "agent:work:main"
				params := SessionsSendParams{
					SessionKey: &sessionKey,
					Message:    fmt.Sprintf("Msg %d-%d", goroutineID, j),
				}

				paramsJSON, _ := json.Marshal(params)
				resultJSON, _ := tool.Execute(string(paramsJSON))

				var result SessionsSendResult
				json.Unmarshal([]byte(resultJSON), &result)

				mu.Lock()
				if result.Success {
					successCount++
				} else if result.ErrorCode == "QUEUE_FULL" {
					queueFullCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	totalAttempts := numGoroutines * messagesPerGoroutine

	t.Logf("Queue exhaustion results:")
	t.Logf("  Total attempts: %d", totalAttempts)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Queue full errors: %d", queueFullCount)
	t.Logf("  Queue capacity: %d", cap(messageQueue))

	// Should have many queue full errors due to small queue
	if queueFullCount == 0 {
		t.Error("Expected some QUEUE_FULL errors with small queue under load")
	}

	// Success + queue full should equal total attempts
	if successCount+queueFullCount != totalAttempts {
		t.Errorf("Expected %d total results, got %d", totalAttempts, successCount+queueFullCount)
	}
}

// TestSessionsSendTool_MixedValidInvalid tests concurrent valid and invalid requests
func TestSessionsSendTool_MixedValidInvalid(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"},
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 100)

	const numGoroutines = 30

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := struct {
		success   int
		forbidden int
		errors    int
		mu        sync.Mutex
	}{}

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			// Mix of valid and invalid requests
			requests := []struct {
				from       string
				to         string
				message    string
				expectType string // "success", "forbidden", "error"
			}{
				{"main", "work", "Valid message", "success"},
				{"work", "main", "Forbidden", "forbidden"},
				{"main", "work", "", "error"}, // Empty message
				{"data", "work", "Forbidden", "forbidden"},
				{"main", "work", "Valid again", "success"},
			}

			for _, req := range requests {
				tool := NewSessionsSendTool(policy, resolver, messageQueue, req.from, false)

				targetSession := fmt.Sprintf("agent:%s:main", req.to)
				params := SessionsSendParams{
					SessionKey: &targetSession,
					Message:    req.message,
				}

				paramsJSON, _ := json.Marshal(params)
				resultJSON, _ := tool.Execute(string(paramsJSON))

				var result SessionsSendResult
				json.Unmarshal([]byte(resultJSON), &result)

				results.mu.Lock()
				if result.Success {
					results.success++
				} else if result.Status == "forbidden" {
					results.forbidden++
				} else {
					results.errors++
				}
				results.mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Expected: 30 goroutines * (2 success + 2 forbidden + 1 error) = 60/60/30
	expectedSuccess := numGoroutines * 2
	expectedForbidden := numGoroutines * 2
	expectedErrors := numGoroutines * 1

	if results.success != expectedSuccess {
		t.Errorf("Expected %d successful, got %d", expectedSuccess, results.success)
	}

	if results.forbidden != expectedForbidden {
		t.Errorf("Expected %d forbidden, got %d", expectedForbidden, results.forbidden)
	}

	if results.errors != expectedErrors {
		t.Errorf("Expected %d errors, got %d", expectedErrors, results.errors)
	}

	t.Logf("Mixed valid/invalid results: success=%d, forbidden=%d, errors=%d",
		results.success, results.forbidden, results.errors)
}
