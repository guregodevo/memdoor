package providers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

// OpenAI-compatible request / response types shared across every
// OAI-shape provider (OpenRouter, DeepSeek, Groq, OpenAI, any provider
// added with memdoor connect). The actual HTTP
// plumbing lives in the per-provider client (oaiClient.Messages.New);
// this file is just the wire-shape types and the Anthropic <-> OAI
// converters.

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatToolCall struct {
	// Index is the slot identifier when the tool call is delivered
	// in pieces across SSE deltas (streaming). The same Index across
	// chunks belongs to the same tool call; the streaming accumulator
	// concatenates Function.Arguments fragments by Index. Non-stream
	// responses leave it at 0 and the field is harmless.
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
	// ExtraContent is what a host attaches to a call and wants back with
	// it: Gemini 3's thinking models put a thought_signature under
	// extra_content.google and refuse the follow-up without it ("Function
	// call is missing a thought_signature in functionCall parts", live
	// 2026-10-02). Kept by call id (toolCallExtras) and replayed.
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

type chatTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	// MaxCompletionTokens is the same cap under the name OpenAI's newer
	// models require ("Unsupported parameter: 'max_tokens'", live
	// 2026-10-02 on gpt-5); one of the two is sent, never both.
	MaxCompletionTokens int                `json:"max_completion_tokens,omitempty"`
	Temperature         float64            `json:"temperature,omitempty"` // 0 = omit = provider default
	Tools               []chatTool         `json:"tools,omitempty"`
	ToolChoice          string             `json:"tool_choice,omitempty"`
	Stream              bool               `json:"stream"`
	StreamOptions       *chatStreamOptions `json:"stream_options,omitempty"`
	// Provider is OpenRouter's routing policy, set only when this gateway
	// talks to OpenRouter on the person's own key (providers/byok.go).
	Provider map[string]any `json:"provider,omitempty"`
	// Reasoning is OpenRouter's reasoning control ({"effort": "high"}), sent
	// only for a model that lists the parameter (reasoning.go).
	Reasoning map[string]any `json:"reasoning,omitempty"`
}

// chatStreamOptions is the OpenAI streaming knob bag. We only need
// include_usage today — set true so the FINAL chunk carries the
// usage block; otherwise OpenAI silently omits it and our response
// log loses token counts on streamed requests.
type chatStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatChoice struct {
	Message struct {
		Content   string         `json:"content"`
		Role      string         `json:"role"`
		ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
		// Reasoning models (Qwen3, etc.) split output: thinking goes
		// into reasoning_content, final answer into content. For some
		// queries content comes back empty and the actual answer lives
		// in reasoning_content. Fall back to it when content is empty.
		ReasoningContent string `json:"reasoning_content,omitempty"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Model   string       `json:"model"`
	// ID is the provider's own id for this call ("gen-…" on OpenRouter). It goes
	// into the meter so a line can be checked against the provider's record.
	ID string `json:"id,omitempty"`
	// Provider is the upstream host, when the vendor names it (OpenRouter).
	Provider string    `json:"provider,omitempty"`
	Usage    chatUsage `json:"usage"`
}

// chatUsage carries token counts from the OAI completions response.
// Providers expose cache hits in two different shapes:
//   - DeepSeek populates top-level prompt_cache_hit_tokens.
//   - OpenAI nests it under prompt_tokens_details.cached_tokens.
//
// We capture both and the caller normalizes.
type chatUsage struct {
	PromptTokens          int64                   `json:"prompt_tokens"`
	CompletionTokens      int64                   `json:"completion_tokens"`
	TotalTokens           int64                   `json:"total_tokens"`
	PromptCacheHitTokens  int64                   `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int64                   `json:"prompt_cache_miss_tokens,omitempty"`
	PromptTokensDetails   *oaiPromptTokensDetails `json:"prompt_tokens_details,omitempty"`
	// Cost is what the vendor charged for this call, in USD. OpenRouter puts
	// it in every response's usage (credits, 1 = $1); other hosts leave it 0.
	Cost float64 `json:"cost,omitempty"`
}

// oaiPromptTokensDetails carries the OpenAI nested cache
// hit count. Distinct from PromptCacheHitTokens (DeepSeek's flat
// version) because the OpenAI shape places it under a sub-object —
// the receiver normalizes both into the single cache-hit field
// that downstream logging consumes.
type oaiPromptTokensDetails struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

// chatStreamChunk is one SSE event from a streaming chat-completions
// response. OpenAI / DeepSeek / OpenRouter all use the same shape:
// `data: {json}\n\n` with `[DONE]` as the terminator. The final chunk
// before [DONE] carries the usage block.
//
// Per-chunk fields we care about:
//
//	choices[0].delta.content      — incremental text the model just emitted
//	choices[0].delta.tool_calls   — incremental tool-call argument fragments
//	choices[0].finish_reason      — set on the LAST content-bearing chunk
//	usage                          — only present on the FINAL chunk
//
// The streaming decoder accumulates content / tool_calls across chunks
// and snapshots usage when it appears. The final non-stream-
// shape chatResponse is reconstructed from these for downstream callers
// that haven't been adapted to streaming yet.
type chatStreamChunk struct {
	// ID is the provider's id for the call, repeated on every chunk.
	ID       string `json:"id,omitempty"`
	Model    string `json:"model,omitempty"`
	Provider string `json:"provider,omitempty"`
	// Error is how OpenRouter reports an upstream that failed mid-stream
	// (with choices[0].finish_reason "error"). It used to be dropped, and
	// the empty answer was taken as a finished turn (2026-09-26).
	Error   *chatStreamError `json:"error,omitempty"`
	Choices []struct {
		Delta struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *chatUsage `json:"usage,omitempty"`
}

// chatStreamError is the error object a stream may carry.
type chatStreamError struct {
	Message string      `json:"message"`
	Code    interface{} `json:"code,omitempty"`
}

// convertAnthropicToOAI converts Anthropic message params to OpenAI message format.
func convertAnthropicToOAI(params llm.MessageNewParams) []chatMessage {
	var messages []chatMessage

	for _, msg := range params.Messages {
		role := string(msg.Role)
		content := ""
		var toolCalls []chatToolCall

		for _, block := range msg.Content {
			blockJSON, _ := json.Marshal(block)
			var bd struct {
				Type      string                 `json:"type"`
				Text      string                 `json:"text"`
				ToolUseID string                 `json:"tool_use_id"`
				Content   json.RawMessage        `json:"content"`
				ID        string                 `json:"id"`
				Name      string                 `json:"name"`
				Input     map[string]interface{} `json:"input"`
			}
			json.Unmarshal(blockJSON, &bd)

			switch bd.Type {
			case "text":
				content += bd.Text
			case "tool_result":
				toolResultContent := toolResultText(bd.Content)
				messages = append(messages, chatMessage{
					Role:       "tool",
					Content:    toolResultContent,
					ToolCallID: bd.ToolUseID,
				})
			case "tool_use":
				inputJSON, _ := json.Marshal(bd.Input)
				argsStr := string(inputJSON)
				if bd.Input == nil || argsStr == "null" {
					argsStr = "{}"
				}
				tc := chatToolCall{
					ID:   bd.ID,
					Type: "function",
				}
				tc.Function.Name = bd.Name
				tc.Function.Arguments = argsStr
				tc.ExtraContent = toolCallExtra(bd.ID)
				toolCalls = append(toolCalls, tc)
			}
		}

		if content != "" || len(toolCalls) > 0 {
			m := chatMessage{Role: role}
			if content != "" {
				m.Content = content
			}
			if len(toolCalls) > 0 {
				m.ToolCalls = toolCalls
			}
			messages = append(messages, m)
		}
	}

	return messages
}

// convertToolsToOAI converts Anthropic tool definitions to OpenAI format.
func convertToolsToOAI(params llm.MessageNewParams) []chatTool {
	var tools []chatTool
	for _, tool := range params.Tools {
		if tool.OfTool == nil {
			continue
		}

		t := chatTool{Type: "function"}
		t.Function.Name = tool.OfTool.Name

		descBytes, _ := json.Marshal(tool.OfTool.Description)
		var desc string
		json.Unmarshal(descBytes, &desc)
		t.Function.Description = desc

		schemaMap := map[string]interface{}{"type": "object"}
		if tool.OfTool.InputSchema.Properties != nil {
			schemaMap["properties"] = tool.OfTool.InputSchema.Properties
		}
		if tool.OfTool.InputSchema.Required != nil {
			schemaMap["required"] = tool.OfTool.InputSchema.Required
		}
		t.Function.Parameters = schemaMap

		tools = append(tools, t)
	}
	return tools
}

// convertOAIToAnthropic converts an OpenAI choice + usage to an Anthropic
// Message. Callers pass the response's Usage so token counts reach
// callers — without it, every OAI-shape provider's tokens were dropped
// at the conversion boundary even though they're already on the wire.
// answeringModel is the model the vendor says answered (OpenRouter returns
// the served id in the response) or, when it says nothing, the one asked for.
func answeringModel(served, asked string) string {
	if served != "" {
		return served
	}
	return asked
}

// toolShapes is what each offered tool declares, for the adapters
// (model_adapter.go) to repair a call against.
func toolShapes(tools []chatTool) []ToolShape {
	out := make([]ToolShape, 0, len(tools))
	for _, t := range tools {
		shape := ToolShape{Name: t.Function.Name}
		if props, ok := t.Function.Parameters["properties"].(map[string]any); ok {
			// required first, so the repair aims at the field that matters.
			if req, ok := t.Function.Parameters["required"].([]any); ok {
				for _, r := range req {
					if name, ok := r.(string); ok {
						shape.Fields = append(shape.Fields, name)
					}
				}
			}
			for name := range props {
				if !contains(shape.Fields, name) {
					shape.Fields = append(shape.Fields, name)
				}
			}
		}
		out = append(out, shape)
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// splitLeakedName splits a tool name the host parsed out of the model's text
// too early. GLM writes its calls as text starting "<tool_call>name"; when its
// prose QUOTES "<tool_call>" (live 2026-09-30, the coder discussing a comment
// in apply_patch.go), the host starts the call at the quote, and the name
// becomes the rest of the prose up to the real "<tool_call>jread". The name
// is what follows the last "<tool_call>", taken only when it is a tool that
// was offered; the prose before it is returned to go back into the text.
func splitLeakedName(name string, offered []ToolShape) (prose, tool string, ok bool) {
	const tag = "<tool_call>"
	k := strings.LastIndex(name, tag)
	if k < 0 {
		return "", name, false
	}
	tool = strings.TrimSpace(name[k+len(tag):])
	if _, known := shapeOf(tool, offered); !known {
		return "", name, false
	}
	return name[:k], tool, true
}

// convertOAIToAnthropicWithTools is the conversion with the offered tools in
// hand, so the model's adapter can repair a call that does not match the schema
// it was given (model_adapter.go).
func convertOAIToAnthropicWithTools(choice chatChoice, usage chatUsage, model string, tools []chatTool) *llm.Message {
	var contentBlocks []llm.ContentBlockUnion
	adapter := AdapterFor(model)
	shapes := toolShapes(tools)

	// A name that swallowed prose goes back to being a name, and its prose
	// back to being text.
	for i, tc := range choice.Message.ToolCalls {
		if prose, name, ok := splitLeakedName(tc.Function.Name, shapes); ok {
			choice.Message.ToolCalls[i].Function.Name = name
			choice.Message.Content += prose
			logs.New("Providers").Info("tool name held prose; split", slog.String("tool", name), slog.Int("prose_chars", len(prose)))
		}
	}

	// The same debris rule as the text protocols: a marker the model read
	// back from its own reply, it writes again (toolproto.go scrubTagLitter).
	choice.Message.Content = scrubTagLitter(choice.Message.Content)
	if choice.Message.Content != "" {
		textJSON := fmt.Sprintf(`{"type":"text","text":%q}`, choice.Message.Content)
		var block llm.ContentBlockUnion
		json.Unmarshal([]byte(textJSON), &block)
		contentBlocks = append(contentBlocks, block)
	}

	for _, tc := range choice.Message.ToolCalls {
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		name := tc.Function.Name
		if fixedName, fixedArgs, changed := adapter.FixToolCall(name, []byte(args), shapes); changed {
			name, args = fixedName, string(fixedArgs)
		}
		rememberToolCallExtra(tc.ID, tc.ExtraContent)
		toolUseJSON := fmt.Sprintf(`{"type":"tool_use","id":%q,"name":%q,"input":%s}`,
			tc.ID, name, args)
		var block llm.ContentBlockUnion
		json.Unmarshal([]byte(toolUseJSON), &block)
		contentBlocks = append(contentBlocks, block)
	}

	// RULE 3, THE LIFT (model_adapter.go): no call, but the reply IS a patch
	// this harness can apply — the model meant to call the tool. Measured on
	// meta-llama/llama-4-maverick, which wrote a correct Codex patch as prose.
	if len(choice.Message.ToolCalls) == 0 && len(tools) > 0 {
		if name, args, ok := adapter.LiftFromText(choice.Message.Content, shapes); ok {
			toolUseJSON := fmt.Sprintf(`{"type":"tool_use","id":"toolu_lifted_0","name":%q,"input":%s}`, name, args)
			var block llm.ContentBlockUnion
			if json.Unmarshal([]byte(toolUseJSON), &block) == nil {
				contentBlocks = append(contentBlocks, block)
				choice.FinishReason = "tool_calls"
			}
		}
	}

	stopReason := llm.StopReasonEndTurn
	switch choice.FinishReason {
	case "tool_calls":
		stopReason = llm.StopReasonToolUse
	case "length":
		// Cut at max_tokens: the turn driver's truncation recovery acts on it.
		stopReason = llm.StopReasonMaxTokens
	case streamGuardFinish:
		// The prose breaker stopped the read on purpose; the driver retries.
		stopReason = llm.StopReasonStreamGuard
	}

	return &llm.Message{
		ID:         "oai-msg",
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
		StopReason: stopReason,
		Model:      llm.Model(model),
		Usage: llm.Usage{
			InputTokens:  usage.PromptTokens,
			OutputTokens: usage.CompletionTokens,
		},
	}
}

// toolResultText flattens a tool result's content — a plain string or a list
// of blocks — into the text a tool message carries. It is never empty: an
// empty tool message has no content field on the wire, and some OpenRouter
// upstreams reject the whole request for it ("text content parts must carry
// a string text", 2026-09-25, a grep with no matches).
func toolResultText(raw json.RawMessage) string {
	var out string
	var s string
	if json.Unmarshal(raw, &s) == nil {
		out = s
	} else {
		var blocks []map[string]interface{}
		if json.Unmarshal(raw, &blocks) == nil {
			for _, cb := range blocks {
				if ct, _ := cb["type"].(string); ct == "text" {
					if t, ok := cb["text"].(string); ok {
						out += t
					}
				}
			}
		}
	}
	if out == "" {
		out = "(no output)"
	}
	return out
}

// toolCallExtras remembers, by call id, what a host attached to a tool call
// (Gemini's thought_signature) so the rebuilt conversation carries it back.
// Ids are unique per call; the map is bounded so a long-lived gateway does
// not grow with every call it ever made.
var toolCallExtras struct {
	mu    sync.Mutex
	byID  map[string]json.RawMessage
	order []string
}

const toolCallExtrasKept = 4096

func rememberToolCallExtra(id string, extra json.RawMessage) {
	if id == "" || len(extra) == 0 || string(extra) == "null" {
		return
	}
	toolCallExtras.mu.Lock()
	defer toolCallExtras.mu.Unlock()
	if toolCallExtras.byID == nil {
		toolCallExtras.byID = map[string]json.RawMessage{}
	}
	if _, seen := toolCallExtras.byID[id]; !seen {
		toolCallExtras.order = append(toolCallExtras.order, id)
		if len(toolCallExtras.order) > toolCallExtrasKept {
			delete(toolCallExtras.byID, toolCallExtras.order[0])
			toolCallExtras.order = toolCallExtras.order[1:]
		}
	}
	toolCallExtras.byID[id] = append(json.RawMessage(nil), extra...)
}

func toolCallExtra(id string) json.RawMessage {
	toolCallExtras.mu.Lock()
	defer toolCallExtras.mu.Unlock()
	return toolCallExtras.byID[id]
}
