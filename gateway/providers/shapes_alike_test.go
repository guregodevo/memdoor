package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memdoor/pkg/llm"
)

// The three wire shapes (chat, Anthropic, Responses) treat a provider that
// goes quiet or drops, and an empty piece of the transcript, the same way.
// Probed on main 2026-11-20, after #368: a stream that went silent or was
// cut after its headers was asked again on the chat shape and failed on the
// first try on the other two; an empty text block was dropped on chat and
// sent as "text": "" on the other two.

var (
	anthropicOK = []string{
		`{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":5}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"OK"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		`{"type":"message_stop"}`}
	responsesOK = []string{
		`{"type":"response.created","response":{"id":"resp_1"}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"OK"}`,
		`{"type":"response.completed","response":{"id":"resp_1","model":"mytestprovider-slug","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":5,"output_tokens":1}}}`}
	chatOK = []string{`{"choices":[{"index":0,"delta":{"content":"OK"}}]}`, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`}
)

// wireShape is one shape under test: its client, a whole answer, and an
// answer that shows text and then never ends.
type wireShape struct {
	name    string
	client  func(*testing.T, http.HandlerFunc) (LLMClient, func())
	ok      func(http.ResponseWriter)
	started func(http.ResponseWriter)
}

func wireShapes() []wireShape {
	begin := func(w http.ResponseWriter, events ...string) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range events {
			_, _ = io.WriteString(w, "data: "+ev+"\n\n")
		}
		w.(http.Flusher).Flush()
	}
	return []wireShape{
		{"chat", oaiTestClient, func(w http.ResponseWriter) { sse(w, chatOK...) }, func(w http.ResponseWriter) { begin(w, chatOK[0]) }},
		{"anthropic", anthropicTestClient, func(w http.ResponseWriter) { anthropicSSE(w, anthropicOK...) }, func(w http.ResponseWriter) { begin(w, anthropicOK[:3]...) }},
		{"responses", responsesTestClient, func(w http.ResponseWriter) { responsesSSE(w, responsesOK...) }, func(w http.ResponseWriter) { begin(w, responsesOK[:3]...) }},
	}
}

func headersOnly(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()
}

func drop(w http.ResponseWriter) {
	if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
		conn.Close()
	}
}

func TestEveryShapeAsksAgainWhenTheProviderGoesQuietOrDrops(t *testing.T) {
	breaks := map[string]func(http.ResponseWriter, *http.Request){
		"no headers":                         func(w http.ResponseWriter, r *http.Request) { silent(r) },
		"headers, then silence":              func(w http.ResponseWriter, r *http.Request) { headersOnly(w); silent(r) },
		"headers, then the connection drops": func(w http.ResponseWriter, r *http.Request) { headersOnly(w); drop(w) },
	}
	for what, first := range breaks {
		for _, s := range wireShapes() {
			t.Run(what+"/"+s.name, func(t *testing.T) {
				withStallAfter(t, 150*time.Millisecond)
				var calls int32
				c, done := s.client(t, func(w http.ResponseWriter, r *http.Request) {
					if atomic.AddInt32(&calls, 1) == 1 {
						first(w, r)
						return
					}
					s.ok(w)
				})
				defer done()
				msg, err := ask(c)
				if err != nil || atomic.LoadInt32(&calls) != 2 || len(msg.Content) == 0 || msg.Content[0].Text != "OK" {
					t.Fatalf("calls=%d err=%v reply=%+v", atomic.LoadInt32(&calls), err, msg)
				}
			})
		}
	}
}

// Text already on the screen is not asked for again: the person would read
// the start twice. The turn ends on the break.
func TestNoShapeAsksAgainOnceTextWasShown(t *testing.T) {
	for _, s := range wireShapes() {
		t.Run(s.name, func(t *testing.T) {
			withStallAfter(t, 150*time.Millisecond)
			var calls int32
			c, done := s.client(t, func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				s.started(w)
				silent(r)
			})
			defer done()
			_, err := ask(c)
			if err == nil || atomic.LoadInt32(&calls) != 1 || !strings.Contains(err.Error(), "sent nothing") {
				t.Fatalf("calls=%d err=%v", atomic.LoadInt32(&calls), err)
			}
		})
	}
}

// Silent after the headers on every attempt: each attempt is spent, then the
// turn fails in the same words on every shape.
func TestEveryShapeFailsTheSameWayWhenTheStreamIsSilentEveryTime(t *testing.T) {
	for _, s := range wireShapes() {
		t.Run(s.name, func(t *testing.T) {
			withStallAfter(t, 100*time.Millisecond)
			var calls int32
			c, done := s.client(t, func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				headersOnly(w)
				silent(r)
			})
			defer done()
			_, err := ask(c)
			if err == nil || !strings.Contains(err.Error(), "the model's provider sent nothing for") {
				t.Fatalf("want the silence named, got %v", err)
			}
			if n := atomic.LoadInt32(&calls); int(n) != len(oaiRetryAfter)+1 {
				t.Fatalf("every attempt is spent before failing: %d", n)
			}
		})
	}
}

// What each shape puts on the wire for the empty pieces of a transcript: a
// tool result with empty text or no content says "(no output)", and an
// empty text block is not sent.
func TestEveryShapeSendsNoEmptyText(t *testing.T) {
	result := func(id string, content ...llm.ToolResultBlockParamContentUnion) llm.MessageParam {
		return llm.MessageParam{Role: llm.MessageParamRoleUser, Content: []llm.ContentBlockParamUnion{{OfToolResult: &llm.ToolResultBlockParam{Type: "tool_result", ToolUseID: id, Content: content}}}}
	}
	call := func(id string) llm.MessageParam {
		return llm.MessageParam{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{{OfToolUse: &llm.ToolUseBlockParam{Type: "tool_use", ID: id, Name: "grep", Input: json.RawMessage(`{"pattern":"x"}`)}}}}
	}
	msgs := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("find")),
		call("tu_1"), result("tu_1", llm.ToolResultBlockParamContentUnion{OfText: &llm.TextBlockParam{Type: "text", Text: ""}}),
		call("tu_2"), result("tu_2"),
		{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{llm.NewTextBlock("")}},
		llm.NewUserMessage(llm.NewTextBlock(""), llm.NewTextBlock("go on")),
	}
	for _, s := range wireShapes() {
		t.Run(s.name, func(t *testing.T) {
			var body []byte
			c, done := s.client(t, func(w http.ResponseWriter, r *http.Request) { body, _ = io.ReadAll(r.Body); s.ok(w) })
			defer done()
			if _, err := c.Messages().New(context.Background(), llm.MessageNewParams{Model: "m", MaxTokens: 16, Messages: msgs}); err != nil {
				t.Fatal(err)
			}
			wire := string(body)
			if strings.Contains(wire, `"text":""`) || strings.Contains(wire, `"content":""`) || strings.Contains(wire, `"content":null`) || strings.Contains(wire, `"output":""`) {
				t.Fatalf("an empty piece went on the wire: %s", wire)
			}
			if n := strings.Count(wire, "(no output)"); n != 2 {
				t.Fatalf("both empty tool results say (no output), got %d: %s", n, wire)
			}
			if !strings.Contains(wire, "go on") || !strings.Contains(wire, "find") {
				t.Fatalf("the words that were there are kept: %s", wire)
			}
		})
	}
}
