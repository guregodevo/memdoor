package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

// anthropicClient speaks Anthropic's Messages API on a company's key
// (vendor.go). pkg/llm's request types are that API's own shapes, so the
// body is the params as they are; only a picture changes form (Anthropic
// wants it under "source"). One call, no stream: the reply arrives whole.
type anthropicClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

const anthropicVersion = "2023-06-01"

func newAnthropicClient(apiKey, baseURL, model string) LLMClient {
	return &anthropicClient{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), model: model, httpClient: oaiHTTPClient()}
}

func (c *anthropicClient) Messages() MessageService { return &anthropicMessages{client: c} }

type anthropicMessages struct{ client *anthropicClient }

// anthropicRequest is the Messages API body. Messages are built by hand so a
// picture takes Anthropic's "source" form; everything else marshals as
// pkg/llm already does.
type anthropicRequest struct {
	Model       string               `json:"model"`
	MaxTokens   int64                `json:"max_tokens"`
	System      []llm.TextBlockParam `json:"system,omitempty"`
	Messages    []anthropicMessage   `json:"messages"`
	Tools       []llm.ToolUnionParam `json:"tools,omitempty"`
	Temperature float64              `json:"temperature,omitempty"`
	StopSeqs    []string             `json:"stop_sequences,omitempty"`
	Stream      bool                 `json:"stream"`
}

type anthropicMessage struct {
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
}

func anthropicMessagesOf(in []llm.MessageParam) ([]anthropicMessage, error) {
	out := make([]anthropicMessage, 0, len(in))
	for _, m := range in {
		am := anthropicMessage{Role: string(m.Role)}
		for _, b := range m.Content {
			var raw []byte
			var err error
			switch {
			case b.OfImage != nil:
				raw, err = json.Marshal(map[string]any{"type": "image", "source": map[string]string{
					"type": "base64", "media_type": b.OfImage.MediaType, "data": b.OfImage.Data}})
			default:
				raw, err = json.Marshal(b)
			}
			if err != nil {
				return nil, err
			}
			am.Content = append(am.Content, raw)
		}
		if len(am.Content) == 0 {
			continue
		}
		out = append(out, am)
	}
	return out, nil
}

// anthropicResponse is the reply; content blocks decode straight into
// pkg/llm's union (text, tool_use), with a thinking block kept for display.
type anthropicResponse struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Role       string            `json:"role"`
	Model      string            `json:"model"`
	StopReason string            `json:"stop_reason"`
	Content    []json.RawMessage `json:"content"`
	Usage      struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (m *anthropicMessages) New(ctx context.Context, params llm.MessageNewParams) (*llm.Message, error) {
	model := string(params.Model)
	if m.client.model != "" {
		model = m.client.model
	}
	msgs, err := anthropicMessagesOf(params.Messages)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	maxTokens := params.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	body, err := json.Marshal(anthropicRequest{
		Model: model, MaxTokens: maxTokens, System: params.System, Messages: msgs,
		Tools: anthropicTools(params.Tools), Temperature: params.Temperature, StopSeqs: params.StopSeqs, Stream: true,
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	toolNames := make([]string, 0, len(params.Tools))
	for _, t := range params.Tools {
		if t.OfTool != nil {
			toolNames = append(toolNames, t.OfTool.Name)
		}
	}
	logLLMReqShape(llmReqShape{Caller: "agent", Model: model, BodyBytes: len(body), MessageCount: len(msgs), ToolCount: len(toolNames), SystemPromptLen: systemLen(params.System)})
	logLLMRequest(llmReqInfo{Caller: "agent", Agent: params.Agent, Model: model, MessageCount: len(msgs), ToolCount: len(toolNames), ToolNames: toolNames,
		SystemHead: head(systemText(params.System), 1200), UserHead: head(lastUserText(params.Messages), 600), SystemPromptLn: systemLen(params.System)})

	start := time.Now()
	var resp *http.Response
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.client.baseURL+"/v1/messages", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", m.client.apiKey)
		SetAppHeaders(req)
		req.Header.Set("anthropic-version", anthropicVersion)
		setVendorHeaders(req.Header)
		setAttributionHeaders(ctx, req.Header)
		resp, err = m.client.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("anthropic request failed: %w", err)
		}
		// 429 and 529 (overloaded) are asked again after a pause, as the
		// OpenAI-shaped client does; anything else is the vendor's answer.
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < len(oaiRetryAfter) && ctx.Err() == nil {
			resp.Body.Close()
			logs.New("Remote").Warn("the vendor refused the request; retrying", slog.Int("status", resp.StatusCode), slog.Int("attempt", attempt+1), slog.String("model", model))
			time.Sleep(retryAfter(resp.Header, oaiRetryAfter[attempt]))
			continue
		}
		break
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		var ar anthropicResponse
		msg := truncate(string(raw), 200)
		if json.Unmarshal(raw, &ar) == nil && ar.Error != nil {
			msg = ar.Error.Type + ": " + ar.Error.Message
		}
		return nil, fmt.Errorf("HTTP %d from the vendor: %s", resp.StatusCode, msg)
	}
	// Streamed (stream: true): text deltas reach the window as they are
	// written; the blocks are assembled in order for the turn.
	var acc *anthropicStream
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		// A proxy that answers one object although a stream was asked.
		raw, _ := io.ReadAll(resp.Body)
		var ar anthropicResponse
		if err := json.Unmarshal(raw, &ar); err != nil {
			return nil, fmt.Errorf("anthropic reply was not JSON: %s", truncate(string(raw), 200))
		}
		acc = &anthropicStream{id: ar.ID, model: ar.Model, stopReason: ar.StopReason, inputTokens: ar.Usage.InputTokens, outputTokens: ar.Usage.OutputTokens, cacheRead: ar.Usage.CacheReadInputTokens, cacheWrite: ar.Usage.CacheCreationInputTokens}
		for _, rb := range ar.Content {
			var b llm.ContentBlockUnion
			if json.Unmarshal(rb, &b) != nil {
				continue
			}
			blk := &anthropicBlock{kind: b.Type, id: b.ID, name: b.Name}
			if b.Type == "text" {
				blk.text.WriteString(b.Text)
			} else if b.Type == "tool_use" {
				blk.text.Write(b.Input)
			}
			acc.blocks = append(acc.blocks, blk)
		}
	} else {
		acc, err = consumeAnthropicStream(newIdleTimeoutReader(resp.Body, oaiStallAfter), llm.StreamCallbackFromContext(ctx), llm.StreamGuardFromContext(ctx))
		if err != nil {
			return nil, err
		}
	}
	out := &llm.Message{ID: acc.id, Type: "message", Role: "assistant", Model: llm.Model(acc.model), StopReason: llm.StopReason(acc.stopReason), Thinking: acc.thinking.String()}
	switch acc.stopReason {
	case "stop_sequence", "":
		out.StopReason = llm.StopReasonEndTurn
	}
	if acc.guarded {
		out.StopReason = llm.StopReasonStreamGuard
	}
	out.Usage.InputTokens, out.Usage.OutputTokens = acc.inputTokens, acc.outputTokens
	var text strings.Builder
	toolCalls := 0
	for _, b := range acc.blocks {
		switch b.kind {
		case "text":
			if b.text.Len() == 0 {
				continue
			}
			text.WriteString(b.text.String())
			out.Content = append(out.Content, llm.ContentBlockUnion{Type: "text", Text: b.text.String()})
		case "tool_use":
			toolCalls++
			args := strings.TrimSpace(b.text.String())
			if args == "" {
				args = "{}"
			}
			out.Content = append(out.Content, llm.ContentBlockUnion{Type: "tool_use", ID: b.id, Name: b.name, Input: json.RawMessage(args)})
		}
	}
	logLLMResponse(llmRespInfo{Caller: "agent", Agent: params.Agent, Model: model, FinishReason: acc.stopReason,
		InputTokens: acc.inputTokens, OutputTokens: acc.outputTokens, TotalTokens: acc.inputTokens + acc.outputTokens,
		CacheHitTokens: acc.cacheRead, CacheMissTokens: acc.cacheWrite,
		ContentLen: text.Len(), ContentHead: head(text.String(), 200), ToolCalls: toolCalls, FirstTokenMs: acc.firstTokenMs(start)})
	return out, nil
}

// anthropicStream is the Messages API's event stream assembled: blocks by
// index (text, tool_use with its JSON arriving in fragments, thinking kept
// for display), the stop reason and usage from message_delta.
type anthropicStream struct {
	id, model, stopReason string
	inputTokens           int64
	outputTokens          int64
	cacheRead, cacheWrite int64
	blocks                []*anthropicBlock
	thinking              strings.Builder
	firstToken            time.Time
	guarded               bool
}

type anthropicBlock struct {
	kind, id, name string
	text           strings.Builder
}

func (a *anthropicStream) firstTokenMs(start time.Time) float64 {
	if a.firstToken.IsZero() {
		return float64(time.Since(start).Milliseconds())
	}
	return float64(a.firstToken.Sub(start).Milliseconds())
}

func (a *anthropicStream) block(index int) *anthropicBlock {
	for len(a.blocks) <= index {
		a.blocks = append(a.blocks, &anthropicBlock{})
	}
	return a.blocks[index]
}

func consumeAnthropicStream(r io.Reader, cb llm.StreamCallback, guard llm.StreamGuard) (*anthropicStream, error) {
	acc := &anthropicStream{}
	var streamErr error
	textBytes, sawTool := 0, false
	err := readEvents(r, func(event string, data []byte) error {
		var ev struct {
			Type    string `json:"type"`
			Index   int    `json:"index"`
			Message struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage struct {
					InputTokens              int64 `json:"input_tokens"`
					CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Text string `json:"text"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				Thinking    string `json:"thinking"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int64 `json:"output_tokens"`
			} `json:"usage"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil
		}
		switch ev.Type {
		case "message_start":
			acc.id, acc.model = ev.Message.ID, ev.Message.Model
			acc.inputTokens, acc.cacheRead, acc.cacheWrite = ev.Message.Usage.InputTokens, ev.Message.Usage.CacheReadInputTokens, ev.Message.Usage.CacheCreationInputTokens
		case "content_block_start":
			b := acc.block(ev.Index)
			b.kind, b.id, b.name = ev.ContentBlock.Type, ev.ContentBlock.ID, ev.ContentBlock.Name
			if ev.ContentBlock.Type == "tool_use" {
				sawTool = true
			}
			if ev.ContentBlock.Text != "" {
				b.text.WriteString(ev.ContentBlock.Text)
			}
		case "content_block_delta":
			b := acc.block(ev.Index)
			switch ev.Delta.Type {
			case "text_delta":
				if acc.firstToken.IsZero() {
					acc.firstToken = time.Now()
				}
				b.text.WriteString(ev.Delta.Text)
				textBytes += len(ev.Delta.Text)
				if cb != nil {
					cb(ev.Delta.Text)
				}
				if guard != nil && guard(textBytes, sawTool) {
					acc.guarded = true
					return io.EOF
				}
			case "input_json_delta":
				b.text.WriteString(ev.Delta.PartialJSON)
			case "thinking_delta":
				acc.thinking.WriteString(ev.Delta.Thinking)
			}
		case "message_delta":
			if ev.Delta.StopReason != "" {
				acc.stopReason = ev.Delta.StopReason
			}
			if ev.Usage.OutputTokens > 0 {
				acc.outputTokens = ev.Usage.OutputTokens
			}
		case "error":
			if ev.Error != nil {
				streamErr = fmt.Errorf("the vendor's stream failed: %s: %s", ev.Error.Type, ev.Error.Message)
			}
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("anthropic stream: %w", err)
	}
	if streamErr != nil {
		return nil, streamErr
	}
	return acc, nil
}

// anthropicTools is the tool list with every input_schema typed: the
// Messages API refuses a schema whose "type" is not "object", and a tool
// declared with properties alone (the conformance probe's) has none.
func anthropicTools(in []llm.ToolUnionParam) []llm.ToolUnionParam {
	out := make([]llm.ToolUnionParam, 0, len(in))
	for _, t := range in {
		if t.OfTool == nil {
			continue
		}
		if t.OfTool.InputSchema.Type == "" {
			tool := *t.OfTool
			tool.InputSchema.Type = "object"
			t = llm.ToolUnionParam{OfTool: &tool}
		}
		out = append(out, t)
	}
	return out
}

func systemText(blocks []llm.TextBlockParam) string {
	var b strings.Builder
	for _, t := range blocks {
		b.WriteString(t.Text)
	}
	return b.String()
}

func systemLen(blocks []llm.TextBlockParam) int { return len(systemText(blocks)) }

func lastUserText(msgs []llm.MessageParam) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != llm.MessageParamRoleUser {
			continue
		}
		for _, b := range msgs[i].Content {
			if b.OfText != nil {
				return b.OfText.Text
			}
		}
	}
	return ""
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
