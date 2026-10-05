package remote

// OpenAI Chat Completions API type definitions
// Based on: https://platform.openai.com/docs/api-reference/chat/create

// ChatCompletionRequest represents an OpenAI chat completion request
type ChatCompletionRequest struct {
	Model            string                 `json:"model"`
	Messages         []ChatMessage          `json:"messages"`
	Temperature      *float64               `json:"temperature,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	Stop             []string               `json:"stop,omitempty"`
	Stream           bool                   `json:"stream,omitempty"`
	Tools            []Tool                 `json:"tools,omitempty"`
	ToolChoice       interface{}            `json:"tool_choice,omitempty"` // "auto", "none", or {"type": "function", "function": {"name": "my_function"}}
	User             string                 `json:"user,omitempty"`
	N                *int                   `json:"n,omitempty"`
	LogitBias        map[string]int         `json:"logit_bias,omitempty"`
	ResponseFormat   *ResponseFormat        `json:"response_format,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`
	ReasoningEffort  string                 `json:"reasoning_effort,omitempty"` // "none", "low", "medium", "high" — disables thinking for Gemma 4/DeepSeek
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ChatMessage represents a message in the chat completion request
type ChatMessage struct {
	Role             string     `json:"role"` // "system", "user", "assistant", "tool"
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"` // Thinking/reasoning (Gemma 4, DeepSeek)
	Name             string     `json:"name,omitempty"`              // Optional name for the participant
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`        // For assistant messages with tool calls
	ToolCallID       string     `json:"tool_call_id,omitempty"`      // For tool messages
}

// ToolCall represents a tool invocation from the assistant
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call with name and arguments
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded arguments
}

// Tool represents a tool definition available to the model
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction represents a function tool definition
type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"` // JSON Schema object
}

// ResponseFormat specifies the format of the response
type ResponseFormat struct {
	Type string `json:"type"` // "text" or "json_object"
}

// ChatCompletionResponse represents an OpenAI chat completion response
type ChatCompletionResponse struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"` // "chat.completion"
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	Usage             ChatCompletionUsage    `json:"usage"`
	SystemFingerprint string                 `json:"system_fingerprint,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// ChatCompletionChoice represents a single completion choice
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"` // "stop", "length", "tool_calls", "content_filter"
	LogProbs     interface{} `json:"logprobs,omitempty"`
}

// ChatCompletionUsage tracks token usage
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionStreamResponse represents a streaming chunk
type ChatCompletionStreamResponse struct {
	ID                string                       `json:"id"`
	Object            string                       `json:"object"` // "chat.completion.chunk"
	Created           int64                        `json:"created"`
	Model             string                       `json:"model"`
	Choices           []ChatCompletionStreamChoice `json:"choices"`
	SystemFingerprint string                       `json:"system_fingerprint,omitempty"`
}

// ChatCompletionStreamChoice represents a streaming choice chunk
type ChatCompletionStreamChoice struct {
	Index        int              `json:"index"`
	Delta        ChatMessageDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"` // null during streaming, set on last chunk
	LogProbs     interface{}      `json:"logprobs,omitempty"`
}

// ChatMessageDelta represents incremental message updates during streaming
type ChatMessageDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ErrorResponse represents an OpenAI API error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Param   *string     `json:"param,omitempty"`
	Code    interface{} `json:"code,omitempty"` // Can be string or null
}
