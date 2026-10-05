package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SessionsSendInput represents the input for sessions_send
type SessionsSendInput struct {
	SessionKey       string `json:"session_key" jsonschema_description:"Target session key (e.g., 'session_main', 'session_other')"`
	Message          string `json:"message" jsonschema_description:"Message to send to the target session"`
	AnnounceBack     bool   `json:"announce_back,omitempty" jsonschema_description:"If true, the result will be announced back to the requester session (default: false)"`
	RequesterSession string `json:"requester_session,omitempty" jsonschema_description:"Session key to announce results back to (required if announce_back is true)"`
	IdempotencyKey   string `json:"idempotency_key,omitempty" jsonschema_description:"Optional idempotency key to prevent duplicate execution"`
}

var SessionsSendInputSchema = GenerateSchema[SessionsSendInput]()

// SessionsSendDefinition defines the sessions_send tool
// Pattern: OpenClaw's sessions-send-tool.ts
var SessionsSendDefinition = ToolDefinition{
	Name: "sessions_send",
	Description: `Send a message to another session for agent-to-agent communication.

This tool enables inter-session communication by sending a message to a target session.
The target session's agent will process the message asynchronously and can optionally
announce the result back to the requester.

Use this for:
- Delegating tasks to other sessions
- Requesting information from specialized sessions
- Coordinating work across multiple agent instances

Pattern: OpenClaw's sessions_send - async agent execution with announce-back`,
	InputSchema: SessionsSendInputSchema,
	Function:    SessionsSend,
}

// SessionsSend sends a message to another session
// Pattern: OpenClaw's sessions_send - calls gateway.agent() with AGENT_LANE_NESTED
func SessionsSend(input json.RawMessage) (string, error) {
	// Parse input
	var params SessionsSendInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required fields
	if params.SessionKey == "" {
		return "", fmt.Errorf("session_key is required")
	}
	if params.Message == "" {
		return "", fmt.Errorf("message is required")
	}

	// Validate announce_back requirements
	if params.AnnounceBack && params.RequesterSession == "" {
		return "", fmt.Errorf("requester_session is required when announce_back is true")
	}

	// Prepare request to gateway
	requestBody := map[string]interface{}{
		"message":     params.Message,
		"session_key": params.SessionKey,
		"lane":        "nested", // OpenClaw's AGENT_LANE_NESTED
	}

	if params.IdempotencyKey != "" {
		requestBody["idempotency_key"] = params.IdempotencyKey
	}

	if params.AnnounceBack {
		requestBody["announce_back"] = true
		requestBody["requester_session"] = params.RequesterSession
	}

	// Call gateway /api/agent endpoint
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	gatewayURL := apiURL("/api/agent")

	resp, err := http.Post(gatewayURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gateway returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Format result
	output := map[string]interface{}{
		"status":      result["status"],
		"run_id":      result["run_id"],
		"session_key": result["session_key"],
	}

	if params.AnnounceBack {
		output["announce_back"] = true
		output["requester_session"] = params.RequesterSession
		output["note"] = "Result will be announced back to requester session when complete"
	} else {
		output["note"] = "Message sent asynchronously (fire-and-forget)"
	}

	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format output: %w", err)
	}

	return string(outputJSON), nil
}

// sessions_spawn has been moved to subagent_tools.go for better integration
// with SubagentRegistry and proper OpenClaw pattern implementation
