package context

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memdoor/pkg/llm"
)

// ContextInspector provides context inspection and analysis
// Pattern: OpenClaw context commands (/context list, /context detail)
type ContextInspector struct {
	model        string
	systemPrompt string
	toolSchemas  []llm.ToolParam
	messages     []llm.MessageParam
	tokenCounter *TokenCounter
	limits       *ModelLimits
	verbose      bool
}

// NewContextInspector creates a new context inspector
func NewContextInspector(
	model string,
	systemPrompt string,
	toolSchemas []llm.ToolParam,
	messages []llm.MessageParam,
	verbose bool,
) (*ContextInspector, error) {
	limits, err := GetModelLimits(model)
	if err != nil {
		return nil, err
	}

	return &ContextInspector{
		model:        model,
		systemPrompt: systemPrompt,
		toolSchemas:  toolSchemas,
		messages:     messages,
		tokenCounter: NewTokenCounter(model, verbose),
		limits:       limits,
		verbose:      verbose,
	}, nil
}

// ContextSummary provides high-level context summary
type ContextSummary struct {
	TotalTokens        int     `json:"total_tokens"`
	SystemPromptTokens int     `json:"system_prompt_tokens"`
	ToolSchemaTokens   int     `json:"tool_schema_tokens"`
	ConversationTokens int     `json:"conversation_tokens"`
	ToolOutputTokens   int     `json:"tool_output_tokens"` // the tool results inside the conversation
	MessageCount       int     `json:"message_count"`
	ContextWindow      int     `json:"context_window"`
	EffectiveLimit     int     `json:"effective_limit"`
	Utilization        float64 `json:"utilization"` // Percentage (0-100)
	IsNearLimit        bool    `json:"is_near_limit"`
	IsOverLimit        bool    `json:"is_over_limit"`
}

// GetSummary returns high-level context summary
// Pattern: OpenClaw /context list output
func (ci *ContextInspector) GetSummary() *ContextSummary {
	systemTokens := ci.tokenCounter.CountSystemPromptTokens(ci.systemPrompt)
	toolTokens := ci.tokenCounter.CountToolSchemaTokens(ci.toolSchemas)
	conversationTokens := ci.tokenCounter.CountConversationTokens(ci.messages)
	totalTokens := systemTokens + toolTokens + conversationTokens

	return &ContextSummary{
		TotalTokens:        totalTokens,
		SystemPromptTokens: systemTokens,
		ToolSchemaTokens:   toolTokens,
		ConversationTokens: conversationTokens,
		ToolOutputTokens:   ci.tokenCounter.CountToolOutputTokens(ci.messages),
		MessageCount:       len(ci.messages),
		ContextWindow:      ci.limits.ContextWindow,
		EffectiveLimit:     ci.limits.EffectiveLimit(),
		Utilization:        ci.limits.GetUtilization(totalTokens),
		IsNearLimit:        ci.limits.IsNearLimit(totalTokens, 80.0),
		IsOverLimit:        ci.limits.IsOverLimit(totalTokens),
	}
}

// FormatSummary formats summary as human-readable text
// Pattern: OpenClaw /context list output format
func (cs *ContextSummary) FormatSummary() string {
	var status string
	if cs.IsOverLimit {
		status = " ⚠️  OVER LIMIT"
	} else if cs.IsNearLimit {
		status = " ⚠️  NEAR LIMIT"
	}

	return fmt.Sprintf(`Context Window: %d / %d tokens (%.1f%%)%s

Breakdown:
- System Prompt:    %6d tokens (%.1f%%)
- Tool Schemas:     %6d tokens (%.1f%%)
- Conversation:     %6d tokens (%.1f%%)

Messages: %d
Effective Limit: %d tokens (%.1f%% reserve)`,
		cs.TotalTokens,
		cs.ContextWindow,
		cs.Utilization,
		status,
		cs.SystemPromptTokens,
		float64(cs.SystemPromptTokens)/float64(cs.ContextWindow)*100,
		cs.ToolSchemaTokens,
		float64(cs.ToolSchemaTokens)/float64(cs.ContextWindow)*100,
		cs.ConversationTokens,
		float64(cs.ConversationTokens)/float64(cs.ContextWindow)*100,
		cs.MessageCount,
		cs.EffectiveLimit,
		float64(cs.ContextWindow-cs.EffectiveLimit)/float64(cs.ContextWindow)*100,
	)
}

// ToolBreakdown provides per-tool token counts
type ToolBreakdown struct {
	ToolName string `json:"tool_name"`
	Tokens   int    `json:"tokens"`
}

// MessageBreakdown provides per-message token counts
type MessageBreakdown struct {
	Index     int    `json:"index"`
	Role      string `json:"role"`
	Tokens    int    `json:"tokens"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ContextDetail provides detailed context breakdown
type ContextDetail struct {
	Summary          *ContextSummary    `json:"summary"`
	ToolBreakdown    []ToolBreakdown    `json:"tool_breakdown,omitempty"`
	MessageBreakdown []MessageBreakdown `json:"message_breakdown,omitempty"`
}

// GetDetail returns detailed context breakdown
// Pattern: OpenClaw /context detail output
func (ci *ContextInspector) GetDetail() *ContextDetail {
	summary := ci.GetSummary()

	// Compute tool breakdown
	toolBreakdown := ci.getToolBreakdown()

	// Compute message breakdown
	messageBreakdown := ci.getMessageBreakdown()

	return &ContextDetail{
		Summary:          summary,
		ToolBreakdown:    toolBreakdown,
		MessageBreakdown: messageBreakdown,
	}
}

// getToolBreakdown computes per-tool token breakdown
func (ci *ContextInspector) getToolBreakdown() []ToolBreakdown {
	breakdown := make([]ToolBreakdown, 0, len(ci.toolSchemas))

	for _, tool := range ci.toolSchemas {
		// Estimate tokens for this tool (serialize to JSON)
		toolJSON, err := json.Marshal(tool)
		tokens := 200 // Fallback estimate
		if err == nil {
			tokens = ci.tokenCounter.EstimateTokens(string(toolJSON))
		}

		// Try to extract tool name - it's a simple string field
		toolName := fmt.Sprintf("tool-%d", len(breakdown)+1) // Fallback
		if jsonMap := make(map[string]interface{}); json.Unmarshal(toolJSON, &jsonMap) == nil {
			if name, ok := jsonMap["name"].(string); ok {
				toolName = name
			}
		}

		breakdown = append(breakdown, ToolBreakdown{
			ToolName: toolName,
			Tokens:   tokens,
		})
	}

	return breakdown
}

// getMessageBreakdown computes per-message token breakdown
func (ci *ContextInspector) getMessageBreakdown() []MessageBreakdown {
	breakdown := make([]MessageBreakdown, 0, len(ci.messages))

	for i, msg := range ci.messages {
		// Estimate tokens for this message (serialize to JSON)
		msgJSON, err := json.Marshal(msg)
		tokens := 100 // Fallback estimate
		if err == nil {
			tokens = ci.tokenCounter.EstimateTokens(string(msgJSON))
		}

		// Extract role from JSON
		role := "user" // Default
		if jsonMap := make(map[string]interface{}); json.Unmarshal(msgJSON, &jsonMap) == nil {
			if r, ok := jsonMap["role"].(string); ok {
				role = r
			}
		}

		breakdown = append(breakdown, MessageBreakdown{
			Index:  i + 1,
			Role:   role,
			Tokens: tokens,
		})
	}

	return breakdown
}

// FormatDetail formats detailed breakdown as human-readable text
// Pattern: OpenClaw /context detail output format
func (cd *ContextDetail) FormatDetail() string {
	var sb strings.Builder

	// Summary section
	sb.WriteString(cd.Summary.FormatSummary())
	sb.WriteString("\n\n")

	// Tool breakdown (top 10)
	if len(cd.ToolBreakdown) > 0 {
		sb.WriteString(fmt.Sprintf("Tool Schemas: %d tokens (%d tools)\n",
			cd.Summary.ToolSchemaTokens, len(cd.ToolBreakdown)))

		displayCount := len(cd.ToolBreakdown)
		if displayCount > 10 {
			displayCount = 10
		}

		for i := 0; i < displayCount; i++ {
			tool := cd.ToolBreakdown[i]
			sb.WriteString(fmt.Sprintf("├─ %-20s %4d tokens\n", tool.ToolName+":", tool.Tokens))
		}

		if len(cd.ToolBreakdown) > 10 {
			remaining := len(cd.ToolBreakdown) - 10
			sb.WriteString(fmt.Sprintf("└─ ... (%d more tools)\n", remaining))
		}

		sb.WriteString("\n")
	}

	// Message breakdown (last 15 messages)
	if len(cd.MessageBreakdown) > 0 {
		sb.WriteString(fmt.Sprintf("Conversation: %d tokens (%d messages)\n",
			cd.Summary.ConversationTokens, len(cd.MessageBreakdown)))

		startIndex := 0
		if len(cd.MessageBreakdown) > 15 {
			startIndex = len(cd.MessageBreakdown) - 15
			sb.WriteString(fmt.Sprintf("... (%d earlier messages)\n", startIndex))
		}

		for i := startIndex; i < len(cd.MessageBreakdown); i++ {
			msg := cd.MessageBreakdown[i]
			prefix := "├─"
			if i == len(cd.MessageBreakdown)-1 {
				prefix = "└─"
			}
			sb.WriteString(fmt.Sprintf("%s Message %d (%s): %d tokens\n",
				prefix, msg.Index, msg.Role, msg.Tokens))
		}
	}

	return sb.String()
}

// SessionContextInfo provides context info for session status
type SessionContextInfo struct {
	Tokens        int       `json:"tokens"`
	ContextWindow int       `json:"context_window"`
	Utilization   float64   `json:"utilization"`
	Status        string    `json:"status"`
	MessageCount  int       `json:"message_count"`
	LastActivity  time.Time `json:"last_activity,omitempty"`
}
