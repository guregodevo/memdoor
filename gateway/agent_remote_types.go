package gateway

// OpenAI Chat Completions API types
// Reference: https://platform.openai.com/docs/api-reference/chat
// These types define the OpenAI-compatible protocol for remote agent communication

// OpenAIRequest represents a request to an OpenAI-compatible chat completions endpoint
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	ToolChoice  interface{}     `json:"tool_choice,omitempty"` // "auto", "none", or specific tool
}

// OpenAIMessage represents a message in the conversation
type OpenAIMessage struct {
	Role       string           `json:"role"` // "system", "user", "assistant", "tool"
	Content    string           `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// OpenAIToolCall represents a tool call made by the assistant
type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // "function"
	Function OpenAIFunctionCall `json:"function"`
}

// OpenAIFunctionCall represents a function call with name and arguments
type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// OpenAITool represents a tool definition
type OpenAITool struct {
	Type     string            `json:"type"` // "function"
	Function OpenAIFunctionDef `json:"function"`
}

// OpenAIFunctionDef defines a function that can be called
type OpenAIFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// OpenAIResponse represents the response from an OpenAI-compatible endpoint
type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"` // "chat.completion"
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage,omitempty"`
}

// OpenAIChoice represents one completion choice
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"` // "stop", "length", "tool_calls"
}

// OpenAIUsage represents token usage statistics
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
