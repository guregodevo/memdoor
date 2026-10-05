package a2a

import (
	"testing"

	"memdoor/pkg/shared"
)

// TestResolveTarget_ExplicitSessionKey tests resolution with explicit session key
func TestResolveTarget_ExplicitSessionKey(t *testing.T) {
	resolver := NewSessionResolver(false)

	params := shared.SessionResolutionParams{
		SessionKey: "agent:work:main",
	}

	sessionKey, agentID, err := resolver.ResolveTarget(params)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if sessionKey != "agent:work:main" {
		t.Errorf("Expected sessionKey 'agent:work:main', got '%s'", sessionKey)
	}

	if agentID != "work" {
		t.Errorf("Expected agentID 'work', got '%s'", agentID)
	}
}

// TestResolveTarget_LabelWithAgentID tests resolution with label + agentID
func TestResolveTarget_LabelWithAgentID(t *testing.T) {
	resolver := NewSessionResolver(false)

	agentID := "work"
	params := shared.SessionResolutionParams{
		Label:   "main",
		AgentID: &agentID,
	}

	sessionKey, resolvedAgentID, err := resolver.ResolveTarget(params)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedKey := "agent:work:main"
	if sessionKey != expectedKey {
		t.Errorf("Expected sessionKey '%s', got '%s'", expectedKey, sessionKey)
	}

	if resolvedAgentID != "work" {
		t.Errorf("Expected agentID 'work', got '%s'", resolvedAgentID)
	}
}

// TestResolveTarget_LabelOnly tests that label-only fails without agentID
func TestResolveTarget_LabelOnly(t *testing.T) {
	resolver := NewSessionResolver(false)

	params := shared.SessionResolutionParams{
		Label: "main",
		// No AgentID provided
	}

	_, _, err := resolver.ResolveTarget(params)
	if err == nil {
		t.Error("Expected error for label-only resolution, got nil")
	}

	// Error message should mention agentId
	if err != nil && err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

// TestResolveTarget_Empty tests that empty parameters return error
func TestResolveTarget_Empty(t *testing.T) {
	resolver := NewSessionResolver(false)

	params := shared.SessionResolutionParams{
		// All empty
	}

	_, _, err := resolver.ResolveTarget(params)
	if err == nil {
		t.Error("Expected error for empty parameters, got nil")
	}
}

// TestResolveTarget_Priority tests that sessionKey takes priority over label
func TestResolveTarget_Priority(t *testing.T) {
	resolver := NewSessionResolver(false)

	agentID := "data"
	params := shared.SessionResolutionParams{
		SessionKey: "agent:work:main", // This should be used
		Label:      "other",           // This should be ignored
		AgentID:    &agentID,          // This should be ignored
	}

	sessionKey, resolvedAgentID, err := resolver.ResolveTarget(params)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should use sessionKey, not label
	if sessionKey != "agent:work:main" {
		t.Errorf("Expected sessionKey to be used: 'agent:work:main', got '%s'", sessionKey)
	}

	if resolvedAgentID != "work" {
		t.Errorf("Expected agentID 'work' (from sessionKey), got '%s'", resolvedAgentID)
	}
}

// TestExtractAgentFromSessionKey tests extracting agent ID from session keys
func TestExtractAgentFromSessionKey(t *testing.T) {
	resolver := NewSessionResolver(false)

	testCases := []struct {
		sessionKey      string
		expectedAgentID string
		expectError     bool
	}{
		{"agent:main:session1", "main", false},
		{"agent:work:session2", "work", false},
		{"agent:data:main", "data", false},
		{"cron:scheduler:daily", "scheduler", false},
		{"agent:work:subagent:run-123", "work", false}, // Subagent session
		{"invalid", "", true},                          // Too few parts
		{"agent:", "", true},                           // Empty agent ID
		{"agent::session", "", true},                   // Empty agent ID
	}

	for _, tc := range testCases {
		t.Run(tc.sessionKey, func(t *testing.T) {
			agentID, err := resolver.extractAgentFromSessionKey(tc.sessionKey)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for session key '%s', got nil", tc.sessionKey)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for session key '%s': %v", tc.sessionKey, err)
				}

				if agentID != tc.expectedAgentID {
					t.Errorf("Expected agentID '%s', got '%s'", tc.expectedAgentID, agentID)
				}
			}
		})
	}
}

// TestValidateSessionKey tests session key validation
func TestValidateSessionKey(t *testing.T) {
	resolver := NewSessionResolver(false)

	testCases := []struct {
		name        string
		sessionKey  string
		expectError bool
	}{
		{"Valid agent session", "agent:main:session1", false},
		{"Valid cron session", "cron:scheduler:daily", false},
		{"Valid subagent session", "agent:main:subagent:run-123", false},
		{"Empty session key", "", true},
		{"Too few parts", "agent:main", true},
		{"Invalid prefix", "invalid:main:session", true},
		{"Empty agent ID", "agent::session", true},
		{"Empty session ID", "agent:main:", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := resolver.ValidateSessionKey(tc.sessionKey)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected validation error for '%s', got nil", tc.sessionKey)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error for '%s': %v", tc.sessionKey, err)
				}
			}
		})
	}
}

// TestIsSubagentSession tests detecting subagent sessions
func TestIsSubagentSession(t *testing.T) {
	resolver := NewSessionResolver(false)

	testCases := []struct {
		sessionKey string
		isSubagent bool
	}{
		{"agent:main:main", false},
		{"agent:work:session1", false},
		{"cron:scheduler:daily", false},
		{"agent:main:subagent:run-123", true},
		{"agent:work:subagent:run-456", true},
		{"invalid", false},
	}

	for _, tc := range testCases {
		t.Run(tc.sessionKey, func(t *testing.T) {
			result := resolver.IsSubagentSession(tc.sessionKey)
			if result != tc.isSubagent {
				t.Errorf("Expected IsSubagentSession(%s) = %v, got %v",
					tc.sessionKey, tc.isSubagent, result)
			}
		})
	}
}

// TestGetAgentID tests getting agent ID from session key
func TestGetAgentID(t *testing.T) {
	resolver := NewSessionResolver(false)

	testCases := []struct {
		sessionKey      string
		expectedAgentID string
		expectError     bool
	}{
		{"agent:main:session1", "main", false},
		{"agent:work:main", "work", false},
		{"cron:scheduler:daily", "scheduler", false},
		{"agent:main:subagent:run-123", "main", false},
		{"invalid", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.sessionKey, func(t *testing.T) {
			agentID, err := resolver.GetAgentID(tc.sessionKey)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for '%s', got nil", tc.sessionKey)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for '%s': %v", tc.sessionKey, err)
				}

				if agentID != tc.expectedAgentID {
					t.Errorf("Expected agentID '%s', got '%s'", tc.expectedAgentID, agentID)
				}
			}
		})
	}
}
