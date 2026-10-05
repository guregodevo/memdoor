package remote

import (
	"memdoor/pkg/platform"
)

// Message translation layer: Memdoor format ↔ OpenAI format

// translateMessagesToOpenAI converts platform messages to OpenAI format.
// If the first message has role "system", it is folded into the first "user"
// message so that models without system-role support (e.g. Gemma) work correctly.
func (e *RemoteExecutor) translateMessagesToOpenAI(messages []platform.SessionMessage) []ChatMessage {
	openaiMessages := make([]ChatMessage, 0, len(messages))

	// Extract system prompt if present — will be merged into first user message
	var systemPrompt string
	startIdx := 0
	if len(messages) > 0 && messages[0].Role == "system" {
		systemPrompt = messages[0].Content
		startIdx = 1
	}

	for i := startIdx; i < len(messages); i++ {
		msg := messages[i]
		cm := ChatMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		// Fold system prompt into the first user message
		if systemPrompt != "" && msg.Role == "user" {
			cm.Content = systemPrompt + "\n\n" + msg.Content
			systemPrompt = ""
		}
		// Convert platform tool calls to OpenAI format
		if len(msg.ToolCalls) > 0 {
			cm.ToolCalls = make([]ToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				cm.ToolCalls = append(cm.ToolCalls, ToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
		}
		// Merge consecutive user messages (Gemma requires strict user/assistant alternation)
		if len(openaiMessages) > 0 && cm.Role == "user" && openaiMessages[len(openaiMessages)-1].Role == "user" {
			openaiMessages[len(openaiMessages)-1].Content += "\n\n" + cm.Content
			continue
		}
		openaiMessages = append(openaiMessages, cm)
	}

	return openaiMessages
}

// translateToolsToOpenAI converts platform tool definitions to OpenAI format
func (e *RemoteExecutor) translateToolsToOpenAI(tools []platform.ToolDefinition) []Tool {
	openaiTools := make([]Tool, 0, len(tools))

	for _, tool := range tools {
		openaiTools = append(openaiTools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	return openaiTools
}

// translateResponseToPlatform converts OpenAI response to platform format
func (e *RemoteExecutor) translateResponseToPlatform(resp *ChatCompletionResponse) *platform.RemoteExecutionResult {
	if len(resp.Choices) == 0 {
		return &platform.RemoteExecutionResult{
			Text:      "",
			ToolCalls: []platform.RemoteToolCall{},
			Usage: platform.TokenUsage{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			},
			Metadata: map[string]interface{}{
				"model":   resp.Model,
				"id":      resp.ID,
				"created": resp.Created,
			},
		}
	}

	choice := resp.Choices[0]

	// Extract text content
	text := choice.Message.Content

	// Fall back to reasoning_content if content is empty
	if text == "" && choice.Message.ReasoningContent != "" {
		text = choice.Message.ReasoningContent
	}

	// Extract tool calls if present
	var toolCalls []platform.RemoteToolCall
	if len(choice.Message.ToolCalls) > 0 {
		toolCalls = make([]platform.RemoteToolCall, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			toolCalls = append(toolCalls, platform.RemoteToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	return &platform.RemoteExecutionResult{
		Text:      text,
		ToolCalls: toolCalls,
		Usage: platform.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Metadata: map[string]interface{}{
			"model":         resp.Model,
			"id":            resp.ID,
			"created":       resp.Created,
			"finish_reason": choice.FinishReason,
		},
	}
}
