package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"memdoor/gateway/a2a"
	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// SessionsSendParams defines parameters for the sessions_send tool
// Pattern: OpenClaw src/agents/tools/sessions-send-tool.ts
type SessionsSendParams struct {
	// Target resolution (one of these required)
	SessionKey *string `json:"sessionKey,omitempty"` // Direct session key
	Label      *string `json:"label,omitempty"`      // Session label
	AgentID    *string `json:"agentId,omitempty"`    // Target agent ID (required with label)

	// Message
	Message string `json:"message"` // Required: message to send

	// Optional timeout
	TimeoutSeconds *int `json:"timeoutSeconds,omitempty"` // Default: 30 seconds
}

// SessionsSendResult is the result of a sessions_send call
type SessionsSendResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	TargetSession  string `json:"targetSession,omitempty"`
	TargetAgentID  string `json:"targetAgentId,omitempty"`
	Status         string `json:"status"` // "accepted", "forbidden", "error"
	ErrorCode      string `json:"errorCode,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

// SessionsSendTool implements the sessions_send tool for A2A messaging
type SessionsSendTool struct {
	policy           *authorization.A2APolicy
	resolver         *a2a.SessionResolver
	messageQueue     chan *shared.A2AMessage
	verbose          bool
	defaultTimeout   int
	maxTimeout       int
	requesterAgentID string // The agent ID making the request
}

// NewSessionsSendTool creates a new sessions_send tool
func NewSessionsSendTool(
	policy *authorization.A2APolicy,
	resolver *a2a.SessionResolver,
	messageQueue chan *shared.A2AMessage,
	requesterAgentID string,
	verbose bool,
) *SessionsSendTool {
	return &SessionsSendTool{
		policy:           policy,
		resolver:         resolver,
		messageQueue:     messageQueue,
		requesterAgentID: requesterAgentID,
		verbose:          verbose,
		defaultTimeout:   30,
		maxTimeout:       300, // 5 minutes max
	}
}

// Name returns the tool name
func (t *SessionsSendTool) Name() string {
	return "sessions_send"
}

// Description returns the tool description
func (t *SessionsSendTool) Description() string {
	return "Send a message to another agent session. Use sessionKey for direct addressing, or label+agentId for cross-agent messaging."
}

// InputSchema returns the JSON schema for tool parameters
func (t *SessionsSendTool) InputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sessionKey": map[string]interface{}{
				"type":        "string",
				"description": "Direct session key (e.g., 'agent:work:main')",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Session label (requires agentId)",
			},
			"agentId": map[string]interface{}{
				"type":        "string",
				"description": "Target agent ID (required when using label)",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Message to send to the target session",
			},
			"timeoutSeconds": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30, max: 300)",
			},
		},
		"required": []string{"message"},
	}
}

// Execute runs the sessions_send tool
func (t *SessionsSendTool) Execute(paramsJSON string) (string, error) {
	// Parse parameters
	var params SessionsSendParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return t.formatError("INVALID_PARAMS", "Failed to parse parameters: "+err.Error())
	}

	// Validate message
	if params.Message == "" {
		return t.formatError("INVALID_PARAMS", "Message cannot be empty")
	}

	// Resolve target session
	resolutionParams := shared.SessionResolutionParams{}
	if params.SessionKey != nil {
		resolutionParams.SessionKey = *params.SessionKey
	}
	if params.Label != nil {
		resolutionParams.Label = *params.Label
	}
	if params.AgentID != nil {
		resolutionParams.AgentID = params.AgentID
	}

	targetSessionKey, targetAgentID, err := t.resolver.ResolveTarget(resolutionParams)
	if err != nil {
		return t.formatError("RESOLUTION_FAILED", "Failed to resolve target: "+err.Error())
	}

	// Check A2A policy
	if !t.policy.IsAllowed(t.requesterAgentID, targetAgentID) {
		return t.formatForbidden(targetSessionKey, targetAgentID)
	}

	// Determine timeout
	timeout := t.defaultTimeout
	if params.TimeoutSeconds != nil {
		timeout = *params.TimeoutSeconds
		if timeout <= 0 {
			timeout = t.defaultTimeout
		}
		if timeout > t.maxTimeout {
			timeout = t.maxTimeout
		}
	}

	// Create A2A message
	msg := &shared.A2AMessage{
		RequesterSessionKey: fmt.Sprintf("agent:%s:main", t.requesterAgentID), // Simplified for Week 27
		RequesterAgentID:    t.requesterAgentID,
		TargetSessionKey:    targetSessionKey,
		TargetAgentID:       targetAgentID,
		Message:             params.Message,
		TimeoutSeconds:      timeout,
		SentAt:              time.Now(),
	}

	// Queue the message
	select {
	case t.messageQueue <- msg:
		return t.formatSuccess(targetSessionKey, targetAgentID, timeout)
	default:
		return t.formatError("QUEUE_FULL", "Message queue is full, try again later")
	}
}

// formatSuccess formats a successful result
func (t *SessionsSendTool) formatSuccess(targetSession, targetAgentID string, timeout int) (string, error) {
	result := SessionsSendResult{
		Success:        true,
		Message:        fmt.Sprintf("Message queued for delivery to %s", targetAgentID),
		TargetSession:  targetSession,
		TargetAgentID:  targetAgentID,
		Status:         "accepted",
		TimeoutSeconds: timeout,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to format result: %w", err)
	}

	return string(data), nil
}

// formatForbidden formats a forbidden result
func (t *SessionsSendTool) formatForbidden(targetSession, targetAgentID string) (string, error) {
	result := SessionsSendResult{
		Success:       false,
		Message:       fmt.Sprintf("A2A messaging from %s to %s is not allowed by policy", t.requesterAgentID, targetAgentID),
		TargetSession: targetSession,
		TargetAgentID: targetAgentID,
		Status:        "forbidden",
		ErrorCode:     "POLICY_DENIED",
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to format result: %w", err)
	}

	return string(data), nil
}

// formatError formats an error result
func (t *SessionsSendTool) formatError(errorCode, errorMsg string) (string, error) {
	result := SessionsSendResult{
		Success:   false,
		Message:   errorMsg,
		Status:    "error",
		ErrorCode: errorCode,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to format result: %w", err)
	}

	return string(data), nil
}
