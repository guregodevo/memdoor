package a2a

import (
	"fmt"
	"strings"

	"memdoor/pkg/shared"
)

// SessionResolver resolves target sessions for A2A messaging
// Pattern: OpenClaw src/agents/tools/sessions-send-helpers.ts
type SessionResolver struct {
	// In Week 27, we'll use simple string matching
	// In production, this would integrate with SessionManager
	verbose bool
}

// NewSessionResolver creates a new session resolver
func NewSessionResolver(verbose bool) *SessionResolver {
	return &SessionResolver{
		verbose: verbose,
	}
}

// ResolveTarget resolves a target session from the given parameters
// Returns: (sessionKey, agentID, error)
func (r *SessionResolver) ResolveTarget(params shared.SessionResolutionParams) (string, string, error) {
	// Priority 1: Explicit sessionKey
	if params.SessionKey != "" {
		agentID, err := r.extractAgentFromSessionKey(params.SessionKey)
		if err != nil {
			return "", "", err
		}
		return params.SessionKey, agentID, nil
	}

	// Priority 2: Label with optional AgentID
	if params.Label != "" {
		if params.AgentID != nil && *params.AgentID != "" {
			// Cross-agent label lookup: agent:agentId:label
			sessionKey := fmt.Sprintf("agent:%s:%s", *params.AgentID, params.Label)
			return sessionKey, *params.AgentID, nil
		}

		// Same-agent label lookup would require SessionManager integration
		// For now, treat label as session ID within current agent
		return "", "", fmt.Errorf("label-only resolution requires agentId parameter")
	}

	return "", "", fmt.Errorf("must provide either sessionKey or (label + agentId)")
}

// extractAgentFromSessionKey extracts the agent ID from a session key
// Session key format: "agent:agentId:sessionId" or "cron:agentId:sessionId"
func (r *SessionResolver) extractAgentFromSessionKey(sessionKey string) (string, error) {
	parts := strings.Split(sessionKey, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid session key format (expected agent:agentId:sessionId): %s", sessionKey)
	}

	// parts[0] = "agent" or "cron"
	// parts[1] = agentId
	// parts[2] = sessionId (or "subagent" for subagent sessions)

	// Validate prefix
	prefix := parts[0]
	if prefix != "agent" && prefix != "cron" {
		return "", fmt.Errorf("invalid session key prefix (expected 'agent' or 'cron'): %s", prefix)
	}

	// Validate agent ID
	agentID := parts[1]
	if agentID == "" {
		return "", fmt.Errorf("empty agent ID in session key: %s", sessionKey)
	}

	// Validate session ID
	sessionID := parts[2]
	if sessionID == "" {
		return "", fmt.Errorf("empty session ID in session key: %s", sessionKey)
	}

	return agentID, nil
}

// ValidateSessionKey validates a session key format
func (r *SessionResolver) ValidateSessionKey(sessionKey string) error {
	if sessionKey == "" {
		return fmt.Errorf("session key cannot be empty")
	}

	parts := strings.Split(sessionKey, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid session key format (expected agent:agentId:sessionId): %s", sessionKey)
	}

	// Validate first part is "agent" or "cron"
	prefix := parts[0]
	if prefix != "agent" && prefix != "cron" {
		return fmt.Errorf("invalid session key prefix (expected 'agent' or 'cron'): %s", prefix)
	}

	// Validate agent ID is not empty
	agentID := parts[1]
	if agentID == "" {
		return fmt.Errorf("empty agent ID in session key: %s", sessionKey)
	}

	// Validate session ID is not empty
	sessionID := parts[2]
	if sessionID == "" {
		return fmt.Errorf("empty session ID in session key: %s", sessionKey)
	}

	return nil
}

// IsSubagentSession checks if a session key represents a subagent session
func (r *SessionResolver) IsSubagentSession(sessionKey string) bool {
	parts := strings.Split(sessionKey, ":")
	if len(parts) < 4 {
		return false
	}

	// Subagent sessions have format: agent:agentId:subagent:runId
	return parts[2] == "subagent"
}

// GetAgentID extracts the agent ID from a session key
func (r *SessionResolver) GetAgentID(sessionKey string) (string, error) {
	return r.extractAgentFromSessionKey(sessionKey)
}
