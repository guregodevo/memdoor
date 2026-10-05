package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

// The OpenAI-compatible client on the person's own OpenRouter key: native
// tools on the wire, the routing policy sent, the tool call parsed back, and
// the tokens reported.
func TestOpenRouterClientRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the meter ledger lives under $HOME
	var got struct {
		path, auth string
		body       map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got.body)
		w.Header().Set("Content-Type", "text/event-stream")
		// OpenAI streaming: the name on the first tool delta, the
		// arguments in pieces, usage on the last chunk.
		for _, ev := range []string{
			`{"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"grep","arguments":""}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"pattern\":"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"brainFor\"}"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":120,"completion_tokens":9,"total_tokens":129}}`,
		} {
			_, _ = io.WriteString(w, "data: "+ev+"\n\n")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	re := &RemoteEngine{Endpoint: srv.URL + "/api/v1/chat/completions", APIKey: "sk-or", Model: "deepseek/deepseek-v4.1-flash"}
	c := newDirectOpenRouterClient(context.Background(), re, "deepseek/deepseek-v4.1-flash")
	ctx := context.WithValue(context.Background(), "buddy_agent_name", "coder")
	desc := "search"
	msg, err := c.Messages().New(ctx, llm.MessageNewParams{
		MaxTokens: 256,
		Messages:  []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("find brainFor"))},
		Tools: []llm.ToolUnionParam{{OfTool: &llm.ToolParam{Name: "grep", Description: &desc,
			InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"pattern": map[string]any{"type": "string"}}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/api/v1/chat/completions" || got.auth != "Bearer sk-or" {
		t.Fatalf("OpenRouter's endpoint, the person's key: %+v", got)
	}
	if got.body["model"] != "deepseek/deepseek-v4.1-flash" || got.body["tools"] == nil || got.body["provider"] == nil || got.body["stream"] != true {
		t.Fatalf("model, native tools, the routing policy, streaming on the wire: %v", got.body)
	}
	var sawCall bool
	for _, b := range msg.Content {
		if b.Type == "tool_use" && b.Name == "grep" && strings.Contains(string(b.Input), "brainFor") {
			sawCall = true
		}
	}
	if !sawCall || msg.Usage.InputTokens != 120 || msg.Usage.OutputTokens != 9 {
		t.Fatalf("the tool call and the usage come back: %+v", msg)
	}
	entries, _ := ReadMeter()
	if len(entries) != 1 || entries[0].InputTokens != 120 || entries[0].Model != "deepseek/deepseek-v4.1-flash" {
		t.Fatalf("token usage is in the meter: %+v", entries)
	}
}

// Text streams to the live-typing callback as it arrives, and the whole
// reply still comes back assembled.
func TestOpenRouterClientStreamsText(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, part := range []string{"The file ", "is ", "brains.go."} {
			b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": part}}}})
			_, _ = io.WriteString(w, "data: "+string(b)+"\n\n")
		}
		_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":50,"completion_tokens":6,"total_tokens":56}}`+"\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	re := &RemoteEngine{Endpoint: srv.URL + "/api/v1/chat/completions", APIKey: "sk-or", Model: "deepseek/deepseek-v4.1-flash"}
	var deltas []string
	ctx := llm.WithStreamCallback(context.WithValue(context.Background(), "buddy_agent_name", "coder"), func(d string) { deltas = append(deltas, d) })
	msg, err := newDirectOpenRouterClient(context.Background(), re, "deepseek/deepseek-v4.1-flash").Messages().New(ctx, llm.MessageNewParams{
		MaxTokens: 64, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("which file?"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(deltas, "") != "The file is brains.go." || len(deltas) != 3 {
		t.Fatalf("deltas as they arrive: %q", deltas)
	}
	var text string
	for _, b := range msg.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	if text != "The file is brains.go." || msg.Usage.OutputTokens != 6 {
		t.Fatalf("assembled reply and usage: %q %+v", text, msg.Usage)
	}
}

// The coder's prose breaker holds on this provider too: a reply running on as
// tool-free prose is stopped mid-stream and marked for the turn driver.
func TestOpenRouterClientHonoursTheProseBreaker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 50; i++ {
			_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"content":"rambling prose "}}]}`+"\n\n")
		}
		_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	re := &RemoteEngine{Endpoint: srv.URL + "/api/v1/chat/completions", APIKey: "sk-or", Model: "deepseek/deepseek-v4.1-flash"}
	ctx := llm.WithStreamGuard(context.Background(), func(n int, sawToolCall bool) bool { return !sawToolCall && n > 100 })
	msg, err := newDirectOpenRouterClient(context.Background(), re, "deepseek/deepseek-v4.1-flash").Messages().New(ctx, llm.MessageNewParams{
		MaxTokens: 64, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("go"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.StopReason != llm.StopReasonStreamGuard {
		t.Fatalf("stopped by the guard: %q", msg.StopReason)
	}
	var text string
	for _, b := range msg.Content {
		text += b.Text
	}
	if len(text) > 200 {
		t.Fatalf("the read stopped early, not at the end: %d bytes", len(text))
	}
}

// A tool result is never sent without content: an empty one, or one given as
// a plain string, still carries text on the wire.
func TestToolResultTextNeverEmpty(t *testing.T) {
	cases := map[string]string{
		`[{"type":"text","text":"3 matches"}]`: "3 matches",
		`"plain string output"`:                "plain string output",
		`[]`:                                   "(no output)",
		`""`:                                   "(no output)",
		`null`:                                 "(no output)",
	}
	for raw, want := range cases {
		if got := toolResultText(json.RawMessage(raw)); got != want {
			t.Errorf("%s -> %q, want %q", raw, got, want)
		}
	}
}
