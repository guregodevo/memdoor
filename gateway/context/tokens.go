package context

import (
	"encoding/json"
	"fmt"

	"memdoor/pkg/llm"
)

// TokenCounter provides token estimation and counting for context management
// Pattern: OpenClaw token tracking for context window management
type TokenCounter struct {
	model   string
	verbose bool
}

// NewTokenCounter creates a new token counter for the specified model
func NewTokenCounter(model string, verbose bool) *TokenCounter {
	return &TokenCounter{
		model:   model,
		verbose: verbose,
	}
}

// EstimateTokens estimates tokens for text using the standard approximation
// Pattern: OpenClaw uses ~4 characters per token
// This is an approximation - actual tokenization may vary slightly
func (tc *TokenCounter) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Approximately 4 characters per token
	return scaledEstimate(len(text))
}

// CountConversationTokens counts tokens in conversation messages
// Uses JSON serialization for simple estimation
func (tc *TokenCounter) CountConversationTokens(messages []llm.MessageParam) int {
	if len(messages) == 0 {
		return 0
	}

	// Serialize to JSON and estimate
	jsonBytes, err := json.Marshal(messages)
	if err != nil {
		// Fallback: rough estimate
		return len(messages) * 500
	}

	return tc.EstimateTokens(string(jsonBytes))
}

// CountToolOutputTokens counts the tokens of the tool results inside
// messages: the share of the conversation that tools returned, which
// stubbing gives up first and /context shows apart.
func (tc *TokenCounter) CountToolOutputTokens(messages []llm.MessageParam) int {
	chars := 0
	for _, m := range messages {
		for _, b := range m.Content {
			if b.OfToolResult == nil {
				continue
			}
			for _, c := range b.OfToolResult.Content {
				if c.OfText != nil {
					chars += len(c.OfText.Text)
				}
			}
		}
	}
	return scaledEstimate(chars)
}

// CountSystemPromptTokens counts tokens in system prompt
func (tc *TokenCounter) CountSystemPromptTokens(systemPrompt string) int {
	return tc.EstimateTokens(systemPrompt)
}

// CountToolSchemaTokens counts tokens in tool schemas
// Uses JSON serialization for simple estimation
func (tc *TokenCounter) CountToolSchemaTokens(tools []llm.ToolParam) int {
	if len(tools) == 0 {
		return 0
	}

	// Serialize to JSON and estimate
	jsonBytes, err := json.Marshal(tools)
	if err != nil {
		// Fallback: estimate 200 tokens per tool
		return len(tools) * 200
	}

	return tc.EstimateTokens(string(jsonBytes))
}

// UsageMetadata tracks actual token usage from API responses
// Pattern: OpenClaw usage tracking
type UsageMetadata struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ContextBreakdown provides detailed token breakdown
type ContextBreakdown struct {
	SystemPromptTokens int `json:"system_prompt_tokens"`
	ToolSchemaTokens   int `json:"tool_schema_tokens"`
	ConversationTokens int `json:"conversation_tokens"`
	TotalTokens        int `json:"total_tokens"`
	MessageCount       int `json:"message_count"`
}

// ComputeBreakdown computes full context breakdown
func (tc *TokenCounter) ComputeBreakdown(
	systemPrompt string,
	tools []llm.ToolParam,
	messages []llm.MessageParam,
) *ContextBreakdown {
	systemTokens := tc.CountSystemPromptTokens(systemPrompt)
	toolTokens := tc.CountToolSchemaTokens(tools)
	conversationTokens := tc.CountConversationTokens(messages)

	return &ContextBreakdown{
		SystemPromptTokens: systemTokens,
		ToolSchemaTokens:   toolTokens,
		ConversationTokens: conversationTokens,
		TotalTokens:        systemTokens + toolTokens + conversationTokens,
		MessageCount:       len(messages),
	}
}

// FormatBreakdown formats context breakdown as human-readable string
func (cb *ContextBreakdown) FormatBreakdown(contextWindow int) string {
	utilization := float64(cb.TotalTokens) / float64(contextWindow) * 100

	return fmt.Sprintf(`Context Window: %d / %d tokens (%.1f%%)

Breakdown:
- System Prompt:    %6d tokens (%.1f%%)
- Tool Schemas:     %6d tokens (%.1f%%)
- Conversation:     %6d tokens (%.1f%%)

Messages: %d`,
		cb.TotalTokens,
		contextWindow,
		utilization,
		cb.SystemPromptTokens,
		float64(cb.SystemPromptTokens)/float64(contextWindow)*100,
		cb.ToolSchemaTokens,
		float64(cb.ToolSchemaTokens)/float64(contextWindow)*100,
		cb.ConversationTokens,
		float64(cb.ConversationTokens)/float64(contextWindow)*100,
		cb.MessageCount,
	)
}
