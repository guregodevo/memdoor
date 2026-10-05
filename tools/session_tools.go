package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"memdoor/gateway/logs"
)

// SessionMetadata represents metadata for a session
type SessionMetadata struct {
	SessionKey   string `json:"session_key"`
	Kind         string `json:"kind"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

// =============================================================================
// sessions_list tool - List all active sessions via gateway API
// =============================================================================

var SessionsListDefinition = ToolDefinition{
	Name:        "sessions_list",
	Description: "List all active sessions with metadata (session_key, kind, created_at, updated_at, message_count).",
	InputSchema: SessionsListInputSchema,
	Function:    SessionsList,
}

type SessionsListInput struct {
	Kinds []string `json:"kinds,omitempty" jsonschema_description:"Filter by session kinds: main, group, cron, hook, node, other."`
}

var SessionsListInputSchema = GenerateSchema[SessionsListInput]()

func SessionsList(input json.RawMessage) (string, error) {
	var params SessionsListInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	log := logs.New("Agent")
	log.Info("Listing sessions", slog.Any("kinds", params.Kinds))

	resp, err := agentsAPICall("GET", apiURL("/sessions"), nil)
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	// If kind filter requested, filter client-side
	if len(params.Kinds) == 0 {
		return resp, nil
	}

	var result struct {
		Count    int               `json:"count"`
		Sessions []SessionMetadata `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return resp, nil
	}

	kindSet := make(map[string]bool, len(params.Kinds))
	for _, k := range params.Kinds {
		kindSet[k] = true
	}

	var filtered []SessionMetadata
	for _, s := range result.Sessions {
		if kindSet[s.Kind] {
			filtered = append(filtered, s)
		}
	}

	output, err := json.MarshalIndent(map[string]any{
		"count":    len(filtered),
		"sessions": filtered,
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// =============================================================================
// session_status tool - Get detailed status for a specific session
// =============================================================================

var SessionStatusDefinition = ToolDefinition{
	Name:        "session_status",
	Description: "Get detailed status for a specific session including metadata and message count.",
	InputSchema: SessionStatusInputSchema,
	Function:    SessionStatus,
}

type SessionStatusInput struct {
	SessionKey string `json:"session_key" jsonschema_description:"Session ID or key to get status for."`
}

var SessionStatusInputSchema = GenerateSchema[SessionStatusInput]()

func SessionStatus(input json.RawMessage) (string, error) {
	var params SessionStatusInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if params.SessionKey == "" {
		return "", fmt.Errorf("session_key cannot be empty")
	}

	log := logs.New("Agent")
	log.Info("Getting status for session", slog.String("session_key", params.SessionKey))

	url := fmt.Sprintf("%s/detail?id=%s", apiURL("/sessions"), params.SessionKey)
	return agentsAPICall("GET", url, nil)
}

// =============================================================================
// sessions_history tool - Fetch message history for a session
// =============================================================================

var SessionsHistoryDefinition = ToolDefinition{
	Name:        "sessions_history",
	Description: "Fetch message history for a session. Returns recent messages with role, content, and timestamp.",
	InputSchema: SessionsHistoryInputSchema,
	Function:    SessionsHistory,
}

type SessionsHistoryInput struct {
	SessionKey   string `json:"session_key" jsonschema_description:"Session ID or key to fetch history for."`
	Limit        int    `json:"limit,omitempty" jsonschema_description:"Maximum number of messages to return (default: all)."`
	IncludeTools bool   `json:"include_tools,omitempty" jsonschema_description:"Include tool call messages (default: false)."`
}

var SessionsHistoryInputSchema = GenerateSchema[SessionsHistoryInput]()

func SessionsHistory(input json.RawMessage) (string, error) {
	var params SessionsHistoryInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if params.SessionKey == "" {
		return "", fmt.Errorf("session_key cannot be empty")
	}

	log := logs.New("Agent")
	log.Info("Getting history for session",
		slog.String("session_key", params.SessionKey),
		slog.Int("limit", params.Limit),
		slog.Bool("include_tools", params.IncludeTools))

	url := fmt.Sprintf("%s/history?id=%s", apiURL("/sessions"), params.SessionKey)
	if params.Limit > 0 {
		url += fmt.Sprintf("&limit=%d", params.Limit)
	}
	if params.IncludeTools {
		url += "&include_tools=true"
	}

	return agentsAPICall("GET", url, nil)
}
