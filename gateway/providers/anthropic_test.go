package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

// anthropicSSE writes the Messages API's event stream for one reply.
func anthropicSSE(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, ev := range events {
		var probe struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(ev), &probe)
		_, _ = io.WriteString(w, "event: "+probe.Type+"\ndata: "+ev+"\n\n")
	}
}

func anthropicTestClient(t *testing.T, handler http.HandlerFunc) (LLMClient, func()) {
	t.Helper()
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	old := oaiRetryAfter
	oaiRetryAfter = []time.Duration{0, 0}
	srv := httptest.NewServer(handler)
	return newAnthropicClient("sk-ant-company", srv.URL, "claude-sonnet-5"), func() { srv.Close(); oaiRetryAfter = old }
}

// The request is Anthropic's Messages API as pkg/llm already shapes it: the
// key and version headers, the system blocks, the tools, a tool_result turn,
// a picture under "source", the company's attribution headers — and the
// client's model over the caller's.
func TestAnthropicRequestIsTheMessagesAPI(t *testing.T) {
	t.Setenv("MEMDOOR_VENDOR_HEADERS", "X-Team: data-eng; X-Email: me@corp.example")
	var got map[string]any
	var hdr http.Header
	c, done := anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Clone()
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		anthropicSSE(w,
			`{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":12}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"do"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ne"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
			`{"type":"message_stop"}`)
	})
	defer done()
	desc := "Run a command"
	tool := llm.ToolParam{Name: "bash", Description: &desc, InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"command": map[string]any{"type": "string"}}}}
	params := llm.MessageNewParams{
		Model: "ignored-by-the-client", MaxTokens: 256, Temperature: 0.2, Agent: "coder",
		System: []llm.TextBlockParam{{Type: "text", Text: "You are the coder."}},
		Tools:  []llm.ToolUnionParam{{OfTool: &tool}},
		Messages: []llm.MessageParam{
			llm.NewUserMessage(llm.NewTextBlock("ls"), llm.ContentBlockParamUnion{OfImage: &llm.ImageBlockParam{Type: "image", MediaType: "image/png", Data: "AAAA"}}),
			{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{{OfToolUse: &llm.ToolUseBlockParam{Type: "tool_use", ID: "tu_1", Name: "bash", Input: json.RawMessage(`{"command":"ls"}`)}}}},
			{Role: llm.MessageParamRoleUser, Content: []llm.ContentBlockParamUnion{{OfToolResult: &llm.ToolResultBlockParam{Type: "tool_result", ToolUseID: "tu_1", Content: []llm.ToolResultBlockParamContentUnion{{OfText: &llm.TextBlockParam{Type: "text", Text: "a.go"}}}}}}},
		},
	}
	var deltas []string
	ctx := llm.WithStreamCallback(context.Background(), func(d string) { deltas = append(deltas, d) })
	msg, err := c.Messages().New(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(deltas, "|") != "do|ne" {
		t.Fatalf("text deltas reach the window as written: %v", deltas)
	}
	if got["stream"] != true {
		t.Fatal("the request asks for a stream")
	}
	if hdr.Get("x-api-key") != "sk-ant-company" || hdr.Get("anthropic-version") != anthropicVersion || hdr.Get("X-Team") != "data-eng" || hdr.Get("X-Email") != "me@corp.example" {
		t.Fatalf("headers: %v", hdr)
	}
	if hdr.Get("Authorization") != "" || hdr.Get("HTTP-Referer") != "" {
		t.Fatalf("nothing of OpenRouter's travels to a vendor: %v", hdr)
	}
	if got["model"] != "claude-sonnet-5" || got["max_tokens"] != float64(256) || got["temperature"] != 0.2 {
		t.Fatalf("model/max_tokens/temperature: %v %v %v", got["model"], got["max_tokens"], got["temperature"])
	}
	if sys := got["system"].([]any); len(sys) != 1 || sys[0].(map[string]any)["text"] != "You are the coder." {
		t.Fatalf("system: %v", got["system"])
	}
	if tools := got["tools"].([]any); len(tools) != 1 || tools[0].(map[string]any)["name"] != "bash" || tools[0].(map[string]any)["input_schema"].(map[string]any)["type"] != "object" {
		t.Fatalf("tools, every input_schema typed object: %v", got["tools"])
	}
	msgs := got["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages: %d", len(msgs))
	}
	first := msgs[0].(map[string]any)["content"].([]any)
	if pic := first[1].(map[string]any); pic["type"] != "image" || pic["source"].(map[string]any)["media_type"] != "image/png" || pic["source"].(map[string]any)["data"] != "AAAA" {
		t.Fatalf("a picture takes the source form: %v", pic)
	}
	if use := msgs[1].(map[string]any)["content"].([]any)[0].(map[string]any); use["type"] != "tool_use" || use["id"] != "tu_1" {
		t.Fatalf("tool_use: %v", use)
	}
	if res := msgs[2].(map[string]any)["content"].([]any)[0].(map[string]any); res["type"] != "tool_result" || res["tool_use_id"] != "tu_1" {
		t.Fatalf("tool_result: %v", res)
	}
	if len(msg.Content) != 1 || msg.Content[0].Text != "done" || msg.StopReason != llm.StopReasonEndTurn || msg.Usage.InputTokens != 12 || msg.Usage.OutputTokens != 3 {
		t.Fatalf("reply: %+v", msg)
	}
}

// A tool call comes back as tool_use with its input intact; a thinking block
// is kept for display and not as content; stop_reason is the API's own word.
func TestAnthropicReplyWithAToolCallAndThinking(t *testing.T) {
	c, done := anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		anthropicSSE(w,
			`{"type":"message_start","message":{"id":"msg_2","model":"claude-sonnet-5","usage":{"input_tokens":40}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"I should list the files."}}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Listing."}}`,
			`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_9","name":"bash","input":{}}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"command\":"}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"ls -la\"}"}}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`,
			`{"type":"message_stop"}`)
	})
	defer done()
	msg, err := ask(c)
	if err != nil {
		t.Fatal(err)
	}
	if msg.StopReason != llm.StopReasonToolUse || len(msg.Content) != 2 {
		t.Fatalf("reply: %+v", msg)
	}
	if use := msg.Content[1]; use.Type != "tool_use" || use.ID != "toolu_9" || use.Name != "bash" || string(use.Input) != `{"command":"ls -la"}` {
		t.Fatalf("tool_use: %+v", use)
	}
	if msg.Thinking != "I should list the files." {
		t.Fatalf("thinking kept for display: %q", msg.Thinking)
	}
}

// 529 (overloaded) and 429 are asked again; a 401 is the vendor's answer, in
// its own words, at once.
func TestAnthropicRetriesOverloadAndSurfacesAuthErrors(t *testing.T) {
	calls := 0
	c, done := anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(529)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
			return
		}
		anthropicSSE(w,
			`{"type":"message_start","message":{"id":"m","model":"claude-sonnet-5","usage":{"input_tokens":1}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
	})
	if msg, err := ask(c); err != nil || msg.Content[0].Text != "ok" || calls != 2 {
		t.Fatalf("overloaded then ok: err=%v calls=%d", err, calls)
	}
	done()

	c, done = anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	})
	defer done()
	_, err := ask(c)
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Fatalf("auth error in the vendor's words: %v", err)
	}
}

// A stream that fails mid-way says so in the vendor's words; the prose
// breaker stops a reply that runs on with no tool call.
func TestAnthropicStreamErrorsAndGuard(t *testing.T) {
	c, done := anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		anthropicSSE(w,
			`{"type":"message_start","message":{"id":"m","model":"claude-sonnet-5","usage":{"input_tokens":1}}}`,
			`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	})
	if _, err := ask(c); err == nil || !strings.Contains(err.Error(), "overloaded_error") {
		t.Fatalf("a stream error in words: %v", err)
	}
	done()
	c, done = anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		anthropicSSE(w,
			`{"type":"message_start","message":{"id":"m","model":"claude-sonnet-5","usage":{"input_tokens":1}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"a long reply that keeps going"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" and going"}}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}`)
	})
	defer done()
	ctx := llm.WithStreamGuard(context.Background(), func(n int, tool bool) bool { return n > 10 && !tool })
	msg, err := c.Messages().New(ctx, llm.MessageNewParams{MaxTokens: 64, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}})
	if err != nil || msg.StopReason != llm.StopReasonStreamGuard {
		t.Fatalf("the guard stops the read and marks it: %+v %v", msg, err)
	}
}
