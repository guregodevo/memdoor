package llm

import (
	"encoding/json"
)

// Drop-in replacements for the subset of anthropic-sdk-go types our
// agent runtime + compaction + context + tool definitions use. Same
// names so call sites just rename the package alias from `anthropic`
// to `llm`. We don't talk to the Anthropic API — we only call Groq
// (OAI-compatible) — so the SDK was just a Go data-shape library.
// Carrying our own copy lets us drop the dep.

// ============================================================================
// Model & basic enums
// ============================================================================

// Model is a model name string type. The SDK uses a typed string so
// callers get compile-time hints about which model constants exist;
// a plain string is enough: providers name their own models.
type Model string

// StopReason mirrors the SDK enum. Only the two values our code
// branches on are defined.
type StopReason string

const (
	StopReasonEndTurn StopReason = "end_turn"
	StopReasonToolUse StopReason = "tool_use"
	// StopReasonMaxTokens means the model was CUT OFF, not that it finished.
	// Whatever it was doing is unfinished and any tool call it was about to
	// emit does not exist — so the turn must RECOVER rather than end.
	StopReasonMaxTokens StopReason = "max_tokens"
	// StopReasonStreamGuard means the caller's StreamGuard stopped the read:
	// the reply had run past its threshold as tag-free prose with no tool
	// call started. Like max_tokens it is UNFINISHED — the turn retries
	// rather than treating the prose as an answer — but it was cut on
	// purpose, minutes earlier than the cap would have.
	StopReasonStreamGuard StopReason = "stream_guard"
	// StopReasonScrubbed means the reply had text the parser could not read
	// as prose or as a call — a tool call with no name, broken JSON — and
	// nothing is left of it. The retry says so, so the model writes the
	// call again rather than the same unreadable one.
	StopReasonScrubbed StopReason = "scrubbed"
)

// MessageParamRole is the role string for a turn in conversation history.
type MessageParamRole string

const (
	MessageParamRoleUser      MessageParamRole = "user"
	MessageParamRoleAssistant MessageParamRole = "assistant"
)

// String returns a pointer to s. Mirrors anthropic.String — used for
// optional string fields in params (the SDK uses *string so a missing
// field can be distinguished from an empty string).
func String(s string) *string { return &s }

// ============================================================================
// Content blocks (response side)
// ============================================================================

// ContentBlockUnion is one block in an assistant response. A response
// is a slice of these. Each block is either a text block or a tool-use
// block; the discriminator is Type. Other fields are populated based
// on Type.
//
// Designed as a flat struct (not a true union) so that callers can
// switch on the Type field in plain Go without needing
// type-assertion machinery the SDK builds with code generation.
type ContentBlockUnion struct {
	Type string `json:"type"`

	// text block
	Text string `json:"text,omitempty"`

	// tool_use block
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// AsToolUse returns the block as a typed ToolUseBlock view. Callers
// should check Type=="tool_use" before calling. The SDK does this
// via union accessors; we keep the same call shape.
func (c ContentBlockUnion) AsToolUse() ToolUseBlock {
	return ToolUseBlock{
		ID:    c.ID,
		Name:  c.Name,
		Input: c.Input,
		Type:  c.Type,
	}
}

// ToolUseBlock is the typed view of a tool_use ContentBlockUnion.
type ToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Type  string          `json:"type"`
}

// ============================================================================
// Content blocks (request / params side)
// ============================================================================

// ContentBlockParamUnion is the request-side counterpart to
// ContentBlockUnion. Mirrors the SDK's union shape: a wrapper struct
// with one inline pointer per concrete block type. Exactly one of
// OfText / OfToolUse / OfToolResult is set per instance; callers
// discriminate via `if block.OfText != nil` etc., same call shape
// as the SDK.
//
// MarshalJSON flattens whichever Of* pointer is set into a single
// JSON object (with a "type" discriminator) so the wire format
// stays OAI/Anthropic-compatible.
type ContentBlockParamUnion struct {
	OfText       *TextBlockParam       `json:"-"`
	OfToolUse    *ToolUseBlockParam    `json:"-"`
	OfToolResult *ToolResultBlockParam `json:"-"`
	OfImage      *ImageBlockParam      `json:"-"`
}

// ImageBlockParam is a picture in a message: a screenshot, a pasted image. Carried as a data URI so it travels
// with the conversation and needs nothing on the far side to fetch it.
type ImageBlockParam struct {
	Type string `json:"type"` // "image"
	// MediaType is the picture's MIME type ("image/png", "image/jpeg").
	MediaType string `json:"media_type"`
	// Data is the picture, base64, without the data: prefix.
	Data string `json:"data"`
}

// MarshalJSON emits the active Of* pointer's content. Empty unions
// (no Of* set) marshal as "null" — same as the SDK.
func (c ContentBlockParamUnion) MarshalJSON() ([]byte, error) {
	switch {
	case c.OfText != nil:
		return json.Marshal(c.OfText)
	case c.OfToolUse != nil:
		return json.Marshal(c.OfToolUse)
	case c.OfToolResult != nil:
		return json.Marshal(c.OfToolResult)
	case c.OfImage != nil:
		return json.Marshal(c.OfImage)
	}
	return []byte("null"), nil
}

// UnmarshalJSON dispatches on the "type" discriminator to populate
// the right Of* pointer. Skips unknown block types (image, etc.).
func (c *ContentBlockParamUnion) UnmarshalJSON(data []byte) error {
	var probe struct{ Type string }
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	switch probe.Type {
	case "text":
		var t TextBlockParam
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		c.OfText = &t
	case "tool_use":
		var t ToolUseBlockParam
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		c.OfToolUse = &t
	case "tool_result":
		var t ToolResultBlockParam
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		c.OfToolResult = &t
	}
	return nil
}

// TextBlockParam is the text-block param.
type TextBlockParam struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolUseBlockParam is the tool_use-block param.
type ToolUseBlockParam struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResultBlockParam is the tool_result-block param.
type ToolResultBlockParam struct {
	Type      string                             `json:"type"`
	ToolUseID string                             `json:"tool_use_id"`
	Content   []ToolResultBlockParamContentUnion `json:"content"`
	IsError   bool                               `json:"is_error,omitempty"`
}

// ToolResultBlockParamContentUnion mirrors the SDK's nested
// content-block union inside a tool_result block. In practice we
// only emit text content, but the union keeps the OfText pointer
// pattern callers (session_persistence's tool_result truncation)
// already use.
type ToolResultBlockParamContentUnion struct {
	OfText *TextBlockParam `json:"-"`
}

func (u ToolResultBlockParamContentUnion) MarshalJSON() ([]byte, error) {
	if u.OfText != nil {
		return json.Marshal(u.OfText)
	}
	return []byte("null"), nil
}

func (u *ToolResultBlockParamContentUnion) UnmarshalJSON(data []byte) error {
	var probe struct{ Type string }
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	if probe.Type == "text" {
		var t TextBlockParam
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}
		u.OfText = &t
	}
	return nil
}

// NewTextBlock builds a text-shaped ContentBlockParamUnion.
func NewTextBlock(text string) ContentBlockParamUnion {
	return ContentBlockParamUnion{OfText: &TextBlockParam{Type: "text", Text: text}}
}

// NewToolUseBlock builds a tool_use-shaped ContentBlockParamUnion.
// inputJSON is the already-serialized tool input.
func NewToolUseBlock(id, inputJSON, name string) ContentBlockParamUnion {
	return ContentBlockParamUnion{OfToolUse: &ToolUseBlockParam{
		Type:  "tool_use",
		ID:    id,
		Name:  name,
		Input: json.RawMessage(inputJSON),
	}}
}

// NewToolResultBlock builds a tool_result-shaped ContentBlockParamUnion.
// content is wrapped as a single text content entry.
func NewToolResultBlock(toolUseID, content string, isError bool) ContentBlockParamUnion {
	return ContentBlockParamUnion{OfToolResult: &ToolResultBlockParam{
		Type:      "tool_result",
		ToolUseID: toolUseID,
		Content:   []ToolResultBlockParamContentUnion{{OfText: &TextBlockParam{Type: "text", Text: content}}},
		IsError:   isError,
	}}
}

// ============================================================================
// Message (conversation turn)
// ============================================================================

// MessageParam is one turn of conversation history that gets sent in
// a request. Role is "user" or "assistant"; Content is one or more
// blocks (text / tool_use / tool_result).
type MessageParam struct {
	Role    MessageParamRole         `json:"role"`
	Content []ContentBlockParamUnion `json:"content"`
}

// NewUserMessage builds a user-role MessageParam.
func NewUserMessage(blocks ...ContentBlockParamUnion) MessageParam {
	return MessageParam{Role: MessageParamRoleUser, Content: blocks}
}

// NewAssistantMessage builds an assistant-role MessageParam. Used
// when persisting a previous assistant turn back into the next
// request's conversation history.
func NewAssistantMessage(blocks ...ContentBlockParamUnion) MessageParam {
	return MessageParam{Role: MessageParamRoleAssistant, Content: blocks}
}

// ============================================================================
// Tools
// ============================================================================

// ToolInputSchemaParam describes the JSON schema of a tool's input.
// Matches what the OAI function-calling layer wants under
// function.parameters. Properties is `any` (not map[string]any) so
// callers can pass an orderedmap straight through from invopop's
// jsonschema reflector — same shape the SDK accepted.
type ToolInputSchemaParam struct {
	Type       string   `json:"type"`
	Properties any      `json:"properties,omitempty"`
	Required   []string `json:"required,omitempty"`
}

// ToolParam is a tool definition.
type ToolParam struct {
	Name        string               `json:"name"`
	Description *string              `json:"description,omitempty"`
	InputSchema ToolInputSchemaParam `json:"input_schema"`
}

// ToolUnionParam wraps a ToolParam in the union shape the SDK
// exposes. We keep the OfTool field so call sites still write
// `tool.OfTool.Name` and similar.
type ToolUnionParam struct {
	OfTool *ToolParam `json:"-"`
}

// MarshalJSON emits the active OfTool variant inline (the SDK's
// MarshalUnion does the same flattening). Without this, the
// json:"-" tag on OfTool would erase the tool entirely on the wire.
// Other call sites reach into OfTool via Go
// field access and don't depend on this marshal — but the Anthropic
// path serializes params through to the SDK and needs the wire
// shape to match.
func (u ToolUnionParam) MarshalJSON() ([]byte, error) {
	if u.OfTool != nil {
		return json.Marshal(u.OfTool)
	}
	return []byte("null"), nil
}

// ============================================================================
// Request / Response
// ============================================================================

// MessageNewParams is the full request body — model, messages, tools,
// system prompt, sampling parameters.
//
// Agent is the fine-grained identity of who's making the call (chief,
// coder, verifier, …). The provider strips it out of the
// JSON body before sending and feeds it into the central LLM log so
// `memdoor tokens --by agent` can attribute cost per agent. Empty
// is allowed but every callsite that knows the agent name should
// populate it — without it, cost attribution falls back to "unknown".
type MessageNewParams struct {
	Model       Model                  `json:"model"`
	Messages    []MessageParam         `json:"messages"`
	Tools       []ToolUnionParam       `json:"tools,omitempty"`
	System      []TextBlockParam       `json:"system,omitempty"`
	MaxTokens   int64                  `json:"max_tokens"`
	Temperature float64                `json:"temperature,omitempty"` // 0 = provider default
	Agent       string                 `json:"-"`                     // log-only, never serialized
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Extras      map[string]interface{} `json:"-"`
	StopSeqs    []string               `json:"stop_sequences,omitempty"`
}

// Usage tracks token counts.
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// Message is the response from a chat completion call. Content is
// one or more blocks (text + zero or more tool_use); StopReason
// tells the agent runtime whether to loop into tool execution
// ("tool_use") or end the turn ("end_turn").
type Message struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Role       string              `json:"role"`
	Content    []ContentBlockUnion `json:"content"`
	StopReason StopReason          `json:"stop_reason"`
	Model      Model               `json:"model"`
	Usage      Usage               `json:"usage"`
	// Thinking is the model's reasoning block, kept for DISPLAY only.
	//
	// It is deliberately absent from ToParam: the model family's own chat
	// template never replays prior-turn thinking, so persisting it would bury
	// the answer and bloat every subsequent prefill. Until 2026-08-30 it was
	// simply discarded, which meant a reasoning model's work was invisible —
	// the TUI's "thinking" event was a spinner trigger carrying no content.
	Thinking string `json:"-"`
}

// ToParam converts a response Message back into an assistant-role
// MessageParam suitable for appending to the conversation history of
// the next request. Mirrors the SDK's Message.ToParam helper so the
// agent runtime's ping-pong loop keeps the same call shape.
func (m *Message) ToParam() MessageParam {
	blocks := make([]ContentBlockParamUnion, 0, len(m.Content))
	for _, b := range m.Content {
		switch b.Type {
		case "text":
			blocks = append(blocks, NewTextBlock(b.Text))
		case "tool_use":
			input := string(b.Input)
			if input == "" || input == "null" {
				input = "{}"
			}
			blocks = append(blocks, NewToolUseBlock(b.ID, input, b.Name))
		}
	}
	return MessageParam{Role: MessageParamRoleAssistant, Content: blocks}
}

// (No SDK Client placeholder — the real HTTP path lives in
// gateway/providers/groq.go via the LLMClient interface, and the
// consolidated Client interface in client.go covers the simple
// one-shot path.)
