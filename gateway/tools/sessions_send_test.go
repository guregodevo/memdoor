package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"memdoor/gateway/a2a"
	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// TestSessionsSendTool_Success_WithSessionKey tests successful message send with sessionKey
func TestSessionsSendTool_Success_WithSessionKey(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"},
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	sessionKey := "agent:work:main"
	params := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "Check calendar for tomorrow",
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse result
	var result SessionsSendResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify success
	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	if result.Status != "accepted" {
		t.Errorf("Expected status 'accepted', got '%s'", result.Status)
	}

	if result.TargetAgentID != "work" {
		t.Errorf("Expected targetAgentId 'work', got '%s'", result.TargetAgentID)
	}

	if result.TargetSession != "agent:work:main" {
		t.Errorf("Expected targetSession 'agent:work:main', got '%s'", result.TargetSession)
	}

	// Verify message was queued
	select {
	case msg := <-messageQueue:
		if msg.RequesterAgentID != "main" {
			t.Errorf("Expected requester 'main', got '%s'", msg.RequesterAgentID)
		}
		if msg.TargetAgentID != "work" {
			t.Errorf("Expected target 'work', got '%s'", msg.TargetAgentID)
		}
		if msg.Message != "Check calendar for tomorrow" {
			t.Errorf("Expected message 'Check calendar for tomorrow', got '%s'", msg.Message)
		}
		if msg.TimeoutSeconds != 30 { // Default timeout
			t.Errorf("Expected default timeout 30, got %d", msg.TimeoutSeconds)
		}
	default:
		t.Error("No message was queued")
	}
}

// TestSessionsSendTool_Success_WithLabelAndAgentID tests message send with label + agentId
func TestSessionsSendTool_Success_WithLabelAndAgentID(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"},
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	label := "main"
	agentID := "work"
	params := SessionsSendParams{
		Label:   &label,
		AgentID: &agentID,
		Message: "Status update request",
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	if result.TargetSession != "agent:work:main" {
		t.Errorf("Expected constructed session key 'agent:work:main', got '%s'", result.TargetSession)
	}
}

// TestSessionsSendTool_PolicyDenied tests that policy denial is enforced
func TestSessionsSendTool_PolicyDenied(t *testing.T) {
	policy := &authorization.A2APolicy{
		Enabled: true,
		Allow: []authorization.A2ARule{
			{From: "main", To: "work"}, // Only main→work allowed
		},
	}

	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	// Try to send from main→data (not allowed)
	sessionKey := "agent:data:main"
	params := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "This should be denied",
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	// Should fail with forbidden status
	if result.Success {
		t.Error("Expected failure due to policy denial")
	}

	if result.Status != "forbidden" {
		t.Errorf("Expected status 'forbidden', got '%s'", result.Status)
	}

	if result.ErrorCode != "POLICY_DENIED" {
		t.Errorf("Expected errorCode 'POLICY_DENIED', got '%s'", result.ErrorCode)
	}

	// Verify no message was queued
	select {
	case <-messageQueue:
		t.Error("Message should not have been queued due to policy denial")
	default:
		// Expected: queue is empty
	}
}

// TestSessionsSendTool_EmptyMessage tests that empty messages are rejected
func TestSessionsSendTool_EmptyMessage(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	sessionKey := "agent:work:main"
	params := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "", // Empty message
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if result.Success {
		t.Error("Expected failure for empty message")
	}

	if result.ErrorCode != "INVALID_PARAMS" {
		t.Errorf("Expected errorCode 'INVALID_PARAMS', got '%s'", result.ErrorCode)
	}

	if !strings.Contains(result.Message, "cannot be empty") {
		t.Errorf("Expected error message about empty message, got: %s", result.Message)
	}
}

// TestSessionsSendTool_ResolutionFailure tests handling of resolution failures
func TestSessionsSendTool_ResolutionFailure(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	// Label without agentID (will fail resolution)
	label := "main"
	params := SessionsSendParams{
		Label:   &label,
		Message: "This will fail",
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if result.Success {
		t.Error("Expected failure due to resolution error")
	}

	if result.ErrorCode != "RESOLUTION_FAILED" {
		t.Errorf("Expected errorCode 'RESOLUTION_FAILED', got '%s'", result.ErrorCode)
	}
}

// TestSessionsSendTool_CustomTimeout tests custom timeout handling
func TestSessionsSendTool_CustomTimeout(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	sessionKey := "agent:work:main"
	customTimeout := 60
	params := SessionsSendParams{
		SessionKey:     &sessionKey,
		Message:        "Long running task",
		TimeoutSeconds: &customTimeout,
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, err := tool.Execute(string(paramsJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	if result.TimeoutSeconds != 60 {
		t.Errorf("Expected timeout 60, got %d", result.TimeoutSeconds)
	}

	// Verify message has correct timeout
	msg := <-messageQueue
	if msg.TimeoutSeconds != 60 {
		t.Errorf("Expected message timeout 60, got %d", msg.TimeoutSeconds)
	}
}

// TestSessionsSendTool_TimeoutLimits tests timeout boundary conditions
func TestSessionsSendTool_TimeoutLimits(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	testCases := []struct {
		name            string
		inputTimeout    int
		expectedTimeout int
	}{
		{"Negative timeout uses default", -1, 30},
		{"Zero timeout uses default", 0, 30},
		{"Over max capped at max", 500, 300},
		{"Valid timeout used as-is", 120, 120},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sessionKey := "agent:work:main"
			params := SessionsSendParams{
				SessionKey:     &sessionKey,
				Message:        "Test message",
				TimeoutSeconds: &tc.inputTimeout,
			}

			paramsJSON, _ := json.Marshal(params)
			resultJSON, _ := tool.Execute(string(paramsJSON))

			var result SessionsSendResult
			json.Unmarshal([]byte(resultJSON), &result)

			if result.TimeoutSeconds != tc.expectedTimeout {
				t.Errorf("Expected timeout %d, got %d", tc.expectedTimeout, result.TimeoutSeconds)
			}

			// Drain queue
			<-messageQueue
		})
	}
}

// TestSessionsSendTool_QueueFull tests handling of full message queue
func TestSessionsSendTool_QueueFull(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 1) // Tiny queue

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	sessionKey := "agent:work:main"

	// Fill the queue
	params1 := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "Message 1",
	}
	paramsJSON1, _ := json.Marshal(params1)
	tool.Execute(string(paramsJSON1))

	// Try to send another (queue should be full)
	params2 := SessionsSendParams{
		SessionKey: &sessionKey,
		Message:    "Message 2",
	}
	paramsJSON2, _ := json.Marshal(params2)
	resultJSON, err := tool.Execute(string(paramsJSON2))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if result.Success {
		t.Error("Expected failure due to full queue")
	}

	if result.ErrorCode != "QUEUE_FULL" {
		t.Errorf("Expected errorCode 'QUEUE_FULL', got '%s'", result.ErrorCode)
	}
}

// TestSessionsSendTool_InvalidJSON tests handling of malformed JSON
func TestSessionsSendTool_InvalidJSON(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	invalidJSON := `{"message": "test", invalid json}`
	resultJSON, err := tool.Execute(invalidJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var result SessionsSendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if result.Success {
		t.Error("Expected failure for invalid JSON")
	}

	if result.ErrorCode != "INVALID_PARAMS" {
		t.Errorf("Expected errorCode 'INVALID_PARAMS', got '%s'", result.ErrorCode)
	}
}

// TestSessionsSendTool_ToolMetadata tests tool name, description, and schema
func TestSessionsSendTool_ToolMetadata(t *testing.T) {
	policy := &authorization.A2APolicy{Enabled: true, Allow: []authorization.A2ARule{{From: "*", To: "*"}}}
	resolver := a2a.NewSessionResolver(false)
	messageQueue := make(chan *shared.A2AMessage, 10)

	tool := NewSessionsSendTool(policy, resolver, messageQueue, "main", false)

	// Test name
	if tool.Name() != "sessions_send" {
		t.Errorf("Expected name 'sessions_send', got '%s'", tool.Name())
	}

	// Test description
	desc := tool.Description()
	if !strings.Contains(desc, "agent session") {
		t.Errorf("Description should mention agent session: %s", desc)
	}

	// Test schema
	schema := tool.InputSchema()
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		t.Fatal("Schema should be a map")
	}

	properties, ok := schemaMap["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have properties")
	}

	// Verify message is in schema
	if _, exists := properties["message"]; !exists {
		t.Error("Schema should include 'message' property")
	}

	// Verify required fields
	required, ok := schemaMap["required"].([]string)
	if !ok {
		t.Fatal("Schema should have required array")
	}

	foundMessage := false
	for _, field := range required {
		if field == "message" {
			foundMessage = true
		}
	}

	if !foundMessage {
		t.Error("Schema should require 'message' field")
	}
}
