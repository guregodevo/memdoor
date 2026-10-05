package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

func oaiTestClient(t *testing.T, handler http.HandlerFunc) (LLMClient, func()) {
	t.Helper()
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	t.Setenv("HOME", t.TempDir())
	old := oaiRetryAfter
	oaiRetryAfter = []time.Duration{0, 0}
	srv := httptest.NewServer(handler)
	re := &RemoteEngine{Endpoint: srv.URL + "/api/v1/chat/completions", APIKey: "sk-or", Model: "deepseek/deepseek-v4.1-flash"}
	return newDirectOpenRouterClient(context.Background(), re, "deepseek/deepseek-v4.1-flash"), func() { srv.Close(); oaiRetryAfter = old }
}

func sse(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, ev := range events {
		_, _ = io.WriteString(w, "data: "+ev+"\n\n")
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

func ask(c LLMClient) (*llm.Message, error) {
	return c.Messages().New(context.Background(), llm.MessageNewParams{MaxTokens: 64,
		Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}})
}

// A stream the provider ends with an error before any text is tried
// again; the second answer is the turn's.
func TestProviderErrorBeforeAnyTextIsRetried(t *testing.T) {
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			sse(w, `{"error":{"message":"upstream failed","code":502},"choices":[{"index":0,"delta":{},"finish_reason":"error"}]}`)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	msg, err := ask(c)
	if err != nil || calls != 2 || len(msg.Content) == 0 || msg.Content[0].Text != "OK" {
		t.Fatalf("retried once and answered: err=%v calls=%d msg=%+v", err, calls, msg)
	}
}

// Text had already streamed to the person when the provider failed: no
// silent retry (it would repeat the text) — the turn gets an error.
func TestProviderErrorAfterTextIsReported(t *testing.T) {
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		sse(w, `{"choices":[{"index":0,"delta":{"content":"Half an ans"}}]}`, `{"error":{"message":"upstream reset"},"choices":[{"index":0,"delta":{},"finish_reason":"error"}]}`)
	})
	defer done()
	_, err := ask(c)
	if err == nil || calls != 1 || !strings.Contains(err.Error(), "upstream reset") || !strings.Contains(err.Error(), "11 characters") {
		t.Fatalf("reported, not retried: err=%v calls=%d", err, calls)
	}
}

// A 503 is tried again; a 400 is not.
func TestProviderStatusRetries(t *testing.T) {
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
	})
	defer done()
	if _, err := ask(c); err != nil || calls != 2 {
		t.Fatalf("503 then OK: err=%v calls=%d", err, calls)
	}
	var calls2 int32
	c2, done2 := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls2, 1)
		http.Error(w, "bad request", http.StatusBadRequest)
	})
	defer done2()
	if _, err := ask(c2); err == nil || calls2 != 1 {
		t.Fatalf("400 is final: err=%v calls=%d", err, calls2)
	}
}

// A connection that drops mid-stream before any text is asked again (live
// 2026-09-30: "connection reset by peer" ended a coder turn); a half-sent
// tool call is not shown, so asking again repeats nothing.
func TestADroppedConnectionBeforeAnyTextIsRetried(t *testing.T) {
	var calls int32
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"read_file","arguments":"{\"pa"}}]}}]}`+"\n\n")
			w.(http.Flusher).Flush()
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close() // the connection drops mid-stream
			}
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	msg, err := ask(c)
	if err != nil || calls != 2 || len(msg.Content) == 0 || msg.Content[0].Text != "OK" {
		t.Fatalf("asked again and answered: err=%v calls=%d msg=%+v", err, calls, msg)
	}
}

// A refusal that says how long to wait is waited for (capped), not retried
// on the client's own short schedule.
func TestRetryAfterIsHonoured(t *testing.T) {
	h := http.Header{}
	if d := retryAfter(h, 2*time.Second); d != 2*time.Second {
		t.Fatalf("no header: the fallback, got %s", d)
	}
	h.Set("Retry-After", "7")
	if d := retryAfter(h, 2*time.Second); d != 7*time.Second {
		t.Fatalf("seconds: %s", d)
	}
	h.Set("Retry-After", "900")
	if d := retryAfter(h, 2*time.Second); d != time.Minute {
		t.Fatalf("capped at a minute: %s", d)
	}
	h.Set("Retry-After", time.Now().Add(3*time.Second).UTC().Format(http.TimeFormat))
	if d := retryAfter(h, 2*time.Second); d <= 0 || d > 4*time.Second {
		t.Fatalf("an HTTP date: %s", d)
	}
	calls := 0
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"message":"Rate limit reached"}}`)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	if msg, err := ask(c); err != nil || calls != 2 || msg.Content[0].Text != "OK" {
		t.Fatalf("a 429 with Retry-After is asked again after it: err=%v calls=%d", err, calls)
	}
}

// A host that rejects max_tokens and names max_completion_tokens gets the
// request again under that name — once.
func TestMaxCompletionTokensOnRequest(t *testing.T) {
	var bodies []map[string]any
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		if _, has := body["max_tokens"]; has {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.","type":"invalid_request_error","param":"max_tokens"}}`)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	})
	defer done()
	msg, err := ask(c)
	if err != nil || msg.Content[0].Text != "OK" || len(bodies) != 2 {
		t.Fatalf("asked again under the newer name: err=%v calls=%d", err, len(bodies))
	}
	if _, has := bodies[1]["max_tokens"]; has || bodies[1]["max_completion_tokens"] != float64(64) {
		t.Fatalf("the second request carries max_completion_tokens only: %v", bodies[1])
	}
}

// What a host attaches to a tool call comes back with it: Gemini 3's
// thought_signature under extra_content, remembered by call id and replayed
// on the rebuilt assistant message.
func TestToolCallExtraContentIsReplayed(t *testing.T) {
	var bodies []map[string]any
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		sse(w,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_sig","type":"function","function":{"name":"bash","arguments":""},"extra_content":{"google":{"thought_signature":"Cq4BAXB"}}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":\"ls\"}"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":3}}`)
	})
	defer done()
	tool := llm.ToolParam{Name: "bash", InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"command": map[string]any{"type": "string"}}}}
	first, err := c.Messages().New(context.Background(), llm.MessageNewParams{MaxTokens: 64, Tools: []llm.ToolUnionParam{{OfTool: &tool}},
		Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("ls"))}})
	if err != nil || len(first.Content) != 1 || first.Content[0].ID != "call_sig" {
		t.Fatalf("the call: %+v %v", first, err)
	}
	conv := []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("ls")), first.ToParam(),
		{Role: llm.MessageParamRoleUser, Content: []llm.ContentBlockParamUnion{{OfToolResult: &llm.ToolResultBlockParam{Type: "tool_result", ToolUseID: "call_sig", Content: []llm.ToolResultBlockParamContentUnion{{OfText: &llm.TextBlockParam{Type: "text", Text: "a.go"}}}}}}}}
	if _, err := c.Messages().New(context.Background(), llm.MessageNewParams{MaxTokens: 64, Tools: []llm.ToolUnionParam{{OfTool: &tool}}, Messages: conv}); err != nil {
		t.Fatal(err)
	}
	msgs := bodies[1]["messages"].([]any)
	var replayed map[string]any
	for _, m := range msgs {
		mm := m.(map[string]any)
		if mm["role"] == "assistant" {
			replayed = mm["tool_calls"].([]any)[0].(map[string]any)
		}
	}
	if replayed == nil || replayed["id"] != "call_sig" || replayed["extra_content"].(map[string]any)["google"].(map[string]any)["thought_signature"] != "Cq4BAXB" {
		t.Fatalf("the signature travels back with the call: %v", replayed)
	}
}
