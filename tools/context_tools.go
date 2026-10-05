package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"memdoor/gateway/context"
	"memdoor/pkg/llm"
)

// Context command tool for context window inspection
// Pattern: OpenClaw /context command
var ContextDefinition = ToolDefinition{
	Name: "context",
	Description: `Inspect context window usage and breakdown.

Shows how much of the context window is being used and what's consuming it.

Actions:
- list: Show high-level context summary (default)
- detail: Show detailed breakdown of system prompt, tools, and messages

Examples:
- {"action":"list"} - Show context summary
- {"action":"detail"} - Show detailed breakdown`,
	InputSchema: ContextInputSchema,
	Function:    Context,
}

type ContextInput struct {
	Action string `json:"action,omitempty" jsonschema_description:"Action: 'list' (summary) or 'detail' (breakdown). Defaults to 'list'"`
}

var ContextInputSchema = GenerateSchema[ContextInput]()

// ContextToolState holds state needed for context inspection
// This is injected by the agent runtime
var ContextToolState struct {
	Model        string
	SystemPrompt string
	ToolSchemas  []llm.ToolParam
	Messages     []llm.MessageParam
}

func Context(input json.RawMessage) (string, error) {
	contextInput := ContextInput{}
	err := json.Unmarshal(input, &contextInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Default action
	action := contextInput.Action
	if action == "" {
		action = "list"
	}

	// Validate state is set
	if ContextToolState.Model == "" {
		return "", fmt.Errorf("context tool state not initialized")
	}

	// Create context inspector
	inspector, err := context.NewContextInspector(
		ContextToolState.Model,
		ContextToolState.SystemPrompt,
		ContextToolState.ToolSchemas,
		ContextToolState.Messages,
		false, // verbose
	)
	if err != nil {
		return "", fmt.Errorf("failed to create context inspector: %w", err)
	}

	// Handle actions
	switch strings.ToLower(action) {
	case "list", "summary":
		summary := inspector.GetSummary()
		return summary.FormatSummary(), nil

	case "detail", "detailed", "breakdown":
		detail := inspector.GetDetail()
		return detail.FormatDetail(), nil

	default:
		return "", fmt.Errorf("unknown action: %s (valid: list, detail)", action)
	}
}

// Status command enhancement - adds context info to session status
// Pattern: OpenClaw /status command with context information
var StatusDefinition = ToolDefinition{
	Name: "status",
	Description: `Show current session status with context information.

Returns:
- Session key and message count
- Context window usage (tokens, utilization)
- Model information
- Recent activity summary`,
	InputSchema: StatusInputSchema,
	Function:    Status,
}

type StatusInput struct {
	// No parameters needed - uses current session
}

var StatusInputSchema = GenerateSchema[StatusInput]()

func Status(input json.RawMessage) (string, error) {
	// Validate state is set
	if ContextToolState.Model == "" {
		return "", fmt.Errorf("status tool state not initialized")
	}

	// Create context inspector
	inspector, err := context.NewContextInspector(
		ContextToolState.Model,
		ContextToolState.SystemPrompt,
		ContextToolState.ToolSchemas,
		ContextToolState.Messages,
		false, // verbose
	)
	if err != nil {
		return "", fmt.Errorf("failed to create context inspector: %w", err)
	}

	// Get summary
	summary := inspector.GetSummary()

	// Format status
	var status strings.Builder
	status.WriteString("Session Status\n\n")
	status.WriteString(fmt.Sprintf("Model: %s\n", ContextToolState.Model))
	status.WriteString(fmt.Sprintf("Messages: %d\n\n", summary.MessageCount))

	// Context info
	contextStatus := "OK"
	if summary.IsOverLimit {
		contextStatus = "⚠️  OVER LIMIT"
	} else if summary.IsNearLimit {
		contextStatus = "⚠️  NEAR LIMIT"
	}

	status.WriteString(fmt.Sprintf("Context: %d / %d tokens (%.1f%%) - %s\n",
		summary.TotalTokens,
		summary.EffectiveLimit,
		summary.Utilization,
		contextStatus,
	))

	// Breakdown summary
	status.WriteString(fmt.Sprintf("├─ System Prompt: %d tokens\n", summary.SystemPromptTokens))
	status.WriteString(fmt.Sprintf("├─ Tool Schemas:  %d tokens\n", summary.ToolSchemaTokens))
	status.WriteString(fmt.Sprintf("└─ Conversation:  %d tokens\n", summary.ConversationTokens))

	return status.String(), nil
}
