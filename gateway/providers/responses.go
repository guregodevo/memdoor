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

// responsesClient speaks the OpenAI Responses API — `input` items, not
// `messages` — which is what a company's own AI gateway exposes (Greg,
// 2026-10-02: `POST …/ai/v1/responses` with `{"model": "<provider slug>",
// "input": …}` and a personal or service-account token as Bearer). One
// call, no stream; the model is the provider slug the company set up.
type responsesClient struct {
	token      string
	url        string // the full /v1/responses URL
	model      string
	httpClient *http.Client
	// tokenSource, when set, is the bearer for each request instead of
	// token: a ChatGPT plan's access token, refreshed before it expires
	// (chatgpt.go).
	tokenSource func(context.Context) (string, error)
	// plan is ChatGPT plan usage: store:false on every request, and none
	// of the fields the preview refuses (temperature, max_output_tokens …).
	plan bool
}

// responsesURL is the endpoint under a gateway base: ".../ai" → ".../ai/v1/
// responses"; a base that already names /v1 gets "/responses".
func responsesURL(base string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/responses") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/responses"
	}
	return base + "/v1/responses"
}

func newResponsesClient(token, base, model string) LLMClient {
	return &responsesClient{token: token, url: responsesURL(base), model: model, httpClient: oaiHTTPClient()}
}

func (c *responsesClient) Messages() MessageService { return &responsesMessages{client: c} }

type responsesMessages struct{ client *responsesClient }

type responsesRequest struct {
	Model           string          `json:"model"`
	Instructions    string          `json:"instructions,omitempty"`
	Input           []any           `json:"input"`
	Tools           []responsesTool `json:"tools,omitempty"`
	MaxOutputTokens int64           `json:"max_output_tokens,omitempty"`
	Temperature     float64         `json:"temperature,omitempty"`
	Stream          bool            `json:"stream"`
	Store           *bool           `json:"store,omitempty"`
}

type responsesTool struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

// responsesInput turns the conversation into Responses items: a user or
// assistant message with text and picture parts, a function_call for a
// tool_use, a function_call_output for a tool_result.
func responsesInput(msgs []llm.MessageParam) []any {
	var items []any
	for _, m := range msgs {
		var parts []map[string]any
		flush := func() {
			if len(parts) > 0 {
				items = append(items, map[string]any{"role": string(m.Role), "content": parts})
				parts = nil
			}
		}
		textType := "input_text"
		if m.Role == llm.MessageParamRoleAssistant {
			textType = "output_text"
		}
		for _, b := range m.Content {
			switch {
			case b.OfText != nil:
				parts = append(parts, map[string]any{"type": textType, "text": b.OfText.Text})
			case b.OfImage != nil:
				parts = append(parts, map[string]any{"type": "input_image", "image_url": "data:" + b.OfImage.MediaType + ";base64," + b.OfImage.Data})
			case b.OfToolUse != nil:
				flush()
				items = append(items, map[string]any{"type": "function_call", "call_id": b.OfToolUse.ID, "name": b.OfToolUse.Name, "arguments": string(b.OfToolUse.Input)})
			case b.OfToolResult != nil:
				flush()
				var out strings.Builder
				for _, c := range b.OfToolResult.Content {
					if c.OfText != nil {
						out.WriteString(c.OfText.Text)
					}
				}
				items = append(items, map[string]any{"type": "function_call_output", "call_id": b.OfToolResult.ToolUseID, "output": out.String()})
			}
		}
		flush()
	}
	return items
}

type responsesResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Status string `json:"status"`
	Output []struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		CallID  string `json:"call_id"`
		Name    string `json:"name"`
		Args    string `json:"arguments"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Usage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (m *responsesMessages) New(ctx context.Context, params llm.MessageNewParams) (*llm.Message, error) {
	model := string(params.Model)
	if m.client.model != "" {
		model = m.client.model
	}
	req := responsesRequest{Model: model, Instructions: systemText(params.System), Input: responsesInput(params.Messages),
		MaxOutputTokens: params.MaxTokens, Temperature: params.Temperature, Stream: true}
	if m.client.plan {
		// developers.openai.com/siwc/token-sharing-open-source/preview-limitations:
		// store false, stream true, and no temperature, max_output_tokens,
		// top_p, truncation, metadata, user … on a plan's request.
		noStore := false
		req.Store, req.MaxOutputTokens, req.Temperature = &noStore, 0, 0
	}
	token := m.client.token
	if m.client.tokenSource != nil {
		t, err := m.client.tokenSource(ctx)
		if err != nil {
			return nil, err
		}
		token = t
	}
	toolNames := make([]string, 0, len(params.Tools))
	for _, t := range params.Tools {
		if t.OfTool == nil {
			continue
		}
		desc := ""
		if t.OfTool.Description != nil {
			desc = *t.OfTool.Description
		}
		req.Tools = append(req.Tools, responsesTool{Type: "function", Name: t.OfTool.Name, Description: desc, Parameters: t.OfTool.InputSchema})
		toolNames = append(toolNames, t.OfTool.Name)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("responses request: %w", err)
	}
	logLLMReqShape(llmReqShape{Caller: "agent", Model: model, BodyBytes: len(body), MessageCount: len(req.Input), ToolCount: len(toolNames), SystemPromptLen: len(req.Instructions)})
	logLLMRequest(llmReqInfo{Caller: "agent", Agent: params.Agent, Model: model, MessageCount: len(req.Input), ToolCount: len(toolNames), ToolNames: toolNames,
		SystemHead: head(req.Instructions, 1200), UserHead: head(lastUserText(params.Messages), 600), SystemPromptLn: len(req.Instructions)})

	start := time.Now()
	var resp *http.Response
	for attempt := 0; ; attempt++ {
		hr, err := http.NewRequestWithContext(ctx, http.MethodPost, m.client.url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		hr.Header.Set("Content-Type", "application/json")
		hr.Header.Set("Authorization", "Bearer "+token)
		SetAppHeaders(hr)
		setVendorHeaders(hr.Header)
		setAttributionHeaders(ctx, hr.Header)
		resp, err = m.client.httpClient.Do(hr)
		if err != nil {
			return nil, fmt.Errorf("gateway request failed: %w", err)
		}
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < len(oaiRetryAfter) && ctx.Err() == nil {
			resp.Body.Close()
			logs.New("Remote").Warn("the gateway refused the request; retrying", slog.Int("status", resp.StatusCode), slog.Int("attempt", attempt+1), slog.String("model", model))
			time.Sleep(retryAfter(resp.Header, oaiRetryAfter[attempt]))
			continue
		}
		break
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		var rr responsesResponse
		msg := truncate(string(raw), 200)
		if json.Unmarshal(raw, &rr) == nil && rr.Error != nil {
			if m.client.plan {
				return nil, chatgptRefusal(resp.StatusCode, rr.Error.Code, rr.Error.Message)
			}
			msg = rr.Error.Code + ": " + rr.Error.Message
		}
		if m.client.plan {
			return nil, chatgptRefusal(resp.StatusCode, "", msg)
		}
		return nil, fmt.Errorf("HTTP %d from the gateway: %s", resp.StatusCode, msg)
	}
	// Streamed: output_text deltas reach the window as they are written;
	// the final response object (response.completed / .incomplete) is the
	// reply, assembled from the deltas when a gateway sends no final object.
	// A gateway that answers one JSON object although a stream was asked
	// (its content type says so) is read as that object.
	var rr *responsesResponse
	var guarded bool
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		raw, _ := io.ReadAll(resp.Body)
		var one responsesResponse
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil, fmt.Errorf("gateway reply was not JSON: %s", truncate(string(raw), 200))
		}
		if one.Error != nil {
			return nil, fmt.Errorf("the gateway answered: %s: %s", one.Error.Code, one.Error.Message)
		}
		rr = &one
	} else {
		rr, guarded, err = consumeResponsesStream(newIdleTimeoutReader(resp.Body, oaiStallAfter), llm.StreamCallbackFromContext(ctx), llm.StreamGuardFromContext(ctx))
		if err != nil {
			return nil, err
		}
	}
	out := &llm.Message{ID: rr.ID, Type: "message", Role: "assistant", Model: llm.Model(rr.Model), StopReason: llm.StopReasonEndTurn}
	out.Usage.InputTokens, out.Usage.OutputTokens = rr.Usage.InputTokens, rr.Usage.OutputTokens
	var text strings.Builder
	toolCalls := 0
	for _, item := range rr.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" && c.Text != "" {
					text.WriteString(c.Text)
					out.Content = append(out.Content, llm.ContentBlockUnion{Type: "text", Text: c.Text})
				}
			}
		case "function_call":
			toolCalls++
			args := item.Args
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			out.Content = append(out.Content, llm.ContentBlockUnion{Type: "tool_use", ID: item.CallID, Name: item.Name, Input: json.RawMessage(args)})
		}
	}
	finish := "stop"
	switch {
	case guarded:
		out.StopReason, finish = llm.StopReasonStreamGuard, "stream_guard"
	case toolCalls > 0:
		out.StopReason, finish = llm.StopReasonToolUse, "tool_calls"
	case rr.Status == "incomplete" && rr.IncompleteDetails != nil && rr.IncompleteDetails.Reason == "max_output_tokens":
		out.StopReason, finish = llm.StopReasonMaxTokens, "length"
	}
	logLLMResponse(llmRespInfo{Caller: "agent", Agent: params.Agent, Model: model, FinishReason: finish,
		InputTokens: rr.Usage.InputTokens, OutputTokens: rr.Usage.OutputTokens, TotalTokens: rr.Usage.InputTokens + rr.Usage.OutputTokens,
		ContentLen: text.Len(), ContentHead: head(text.String(), 200), ToolCalls: toolCalls, FirstTokenMs: float64(time.Since(start).Milliseconds())})
	return out, nil
}

// consumeResponsesStream reads the Responses API's events: text deltas go
// to the window; items are assembled by output index; the final response
// object wins when one arrives.
func consumeResponsesStream(r io.Reader, cb llm.StreamCallback, guard llm.StreamGuard) (*responsesResponse, bool, error) {
	var final *responsesResponse
	built := &responsesResponse{}
	type item struct {
		typ, id, callID, name string
		text, args            strings.Builder
	}
	items := map[int]*item{}
	order := []int{}
	at := func(i int) *item {
		it, ok := items[i]
		if !ok {
			it = &item{}
			items[i] = it
			order = append(order, i)
		}
		return it
	}
	var streamErr error
	guarded, textBytes, sawTool := false, 0, false
	err := readEvents(r, func(event string, data []byte) error {
		var ev struct {
			Type        string          `json:"type"`
			OutputIndex int             `json:"output_index"`
			Delta       string          `json:"delta"`
			Arguments   string          `json:"arguments"`
			Item        json.RawMessage `json:"item"`
			Response    json.RawMessage `json:"response"`
			Error       *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil
		}
		switch ev.Type {
		case "response.output_item.added":
			var it struct {
				Type   string `json:"type"`
				ID     string `json:"id"`
				CallID string `json:"call_id"`
				Name   string `json:"name"`
			}
			_ = json.Unmarshal(ev.Item, &it)
			x := at(ev.OutputIndex)
			x.typ, x.id, x.callID, x.name = it.Type, it.ID, it.CallID, it.Name
			if it.Type == "function_call" {
				sawTool = true
			}
		case "response.output_text.delta":
			x := at(ev.OutputIndex)
			x.typ = "message"
			x.text.WriteString(ev.Delta)
			textBytes += len(ev.Delta)
			if cb != nil {
				cb(ev.Delta)
			}
			if guard != nil && guard(textBytes, sawTool) {
				guarded = true
				return io.EOF
			}
		case "response.function_call_arguments.delta":
			at(ev.OutputIndex).args.WriteString(ev.Delta)
		case "response.function_call_arguments.done":
			x := at(ev.OutputIndex)
			x.args.Reset()
			x.args.WriteString(ev.Arguments)
		case "response.completed", "response.incomplete":
			var rr responsesResponse
			if json.Unmarshal(ev.Response, &rr) == nil {
				final = &rr
			}
		case "response.failed", "error":
			if ev.Error != nil {
				streamErr = fmt.Errorf("the gateway's stream failed: %s: %s", ev.Error.Code, ev.Error.Message)
			} else {
				var rr responsesResponse
				if json.Unmarshal(ev.Response, &rr) == nil && rr.Error != nil {
					streamErr = fmt.Errorf("the gateway's stream failed: %s: %s", rr.Error.Code, rr.Error.Message)
				} else {
					streamErr = fmt.Errorf("the gateway's stream failed")
				}
			}
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, false, fmt.Errorf("responses stream: %w", err)
	}
	if streamErr != nil {
		return nil, false, streamErr
	}
	if final != nil && !guarded {
		return final, false, nil
	}
	for _, i := range order {
		x := items[i]
		switch x.typ {
		case "message":
			built.Output = append(built.Output, struct {
				Type    string `json:"type"`
				Role    string `json:"role"`
				CallID  string `json:"call_id"`
				Name    string `json:"name"`
				Args    string `json:"arguments"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}{Type: "message", Role: "assistant", Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{Type: "output_text", Text: x.text.String()}}})
		case "function_call":
			built.Output = append(built.Output, struct {
				Type    string `json:"type"`
				Role    string `json:"role"`
				CallID  string `json:"call_id"`
				Name    string `json:"name"`
				Args    string `json:"arguments"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}{Type: "function_call", CallID: x.callID, Name: x.name, Args: x.args.String()})
		}
	}
	if final != nil {
		built.ID, built.Model, built.Usage, built.Status = final.ID, final.Model, final.Usage, final.Status
	}
	return built, guarded, nil
}
