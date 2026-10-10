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

// responsesSSE writes the Responses API's event stream.
func responsesSSE(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, ev := range events {
		var probe struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(ev), &probe)
		_, _ = io.WriteString(w, "event: "+probe.Type+"\ndata: "+ev+"\n\n")
	}
}

func responsesTestClient(t *testing.T, handler http.HandlerFunc) (LLMClient, func()) {
	t.Helper()
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	old := oaiRetryAfter
	oaiRetryAfter = []time.Duration{0, 0}
	srv := httptest.NewServer(handler)
	return newResponsesClient("pat-123", srv.URL+"/ai", "mytestprovider-slug"), func() { srv.Close(); oaiRetryAfter = old }
}

// The request is the company gateway's: POST <base>/v1/responses, Bearer
// token, model = the provider slug, instructions, input items (a user
// message, the assistant's function_call, its function_call_output), tools
// as functions — and the reply's message and function_call come back as
// text and tool_use the agent loop already reads.
func TestResponsesRequestAndReply(t *testing.T) {
	t.Setenv("MEMDOOR_VENDOR_HEADERS", "X-Team: data-eng")
	var got map[string]any
	var hdr http.Header
	var path string
	c, done := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hdr, path = r.Header.Clone(), r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		responsesSSE(w,
			`{"type":"response.created","response":{"id":"resp_1"}}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1"}}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"List"}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"ing."}`,
			`{"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"fc_1","call_id":"call_9","name":"bash"}}`,
			`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"command\":"}`,
			`{"type":"response.function_call_arguments.done","output_index":1,"arguments":"{\"command\":\"ls\"}"}`,
			`{"type":"response.completed","response":{"id":"resp_1","model":"mytestprovider-slug","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Listing."}]},{"type":"function_call","call_id":"call_9","name":"bash","arguments":"{\"command\":\"ls\"}"}],"usage":{"input_tokens":30,"output_tokens":9}}}`)
	})
	defer done()
	desc := "Run a command"
	tool := llm.ToolParam{Name: "bash", Description: &desc, InputSchema: llm.ToolInputSchemaParam{Type: "object", Properties: map[string]any{"command": map[string]any{"type": "string"}}}}
	var deltas []string
	ctx := llm.WithStreamCallback(context.Background(), func(d string) { deltas = append(deltas, d) })
	msg, err := c.Messages().New(ctx, llm.MessageNewParams{
		Model: "ignored", MaxTokens: 300, Agent: "coder",
		System: []llm.TextBlockParam{{Type: "text", Text: "You are the coder."}},
		Tools:  []llm.ToolUnionParam{{OfTool: &tool}},
		Messages: []llm.MessageParam{
			llm.NewUserMessage(llm.NewTextBlock("ls please")),
			{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{{OfToolUse: &llm.ToolUseBlockParam{Type: "tool_use", ID: "call_1", Name: "bash", Input: json.RawMessage(`{"command":"ls"}`)}}}},
			{Role: llm.MessageParamRoleUser, Content: []llm.ContentBlockParamUnion{{OfToolResult: &llm.ToolResultBlockParam{Type: "tool_result", ToolUseID: "call_1", Content: []llm.ToolResultBlockParamContentUnion{{OfText: &llm.TextBlockParam{Type: "text", Text: "a.go"}}}}}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(deltas, "|") != "List|ing." || got["stream"] != true {
		t.Fatalf("text deltas reach the window; the request asks for a stream: %v %v", deltas, got["stream"])
	}
	if path != "/ai/v1/responses" || hdr.Get("Authorization") != "Bearer pat-123" || hdr.Get("X-Team") != "data-eng" {
		t.Fatalf("path %s headers %v", path, hdr)
	}
	if got["model"] != "mytestprovider-slug" || got["instructions"] != "You are the coder." || got["max_output_tokens"] != float64(300) {
		t.Fatalf("model/instructions/max: %v %v %v", got["model"], got["instructions"], got["max_output_tokens"])
	}
	in := got["input"].([]any)
	if len(in) != 3 {
		t.Fatalf("input items: %d", len(in))
	}
	if u := in[0].(map[string]any); u["role"] != "user" || u["content"].([]any)[0].(map[string]any)["type"] != "input_text" {
		t.Fatalf("user item: %v", u)
	}
	if fc := in[1].(map[string]any); fc["type"] != "function_call" || fc["call_id"] != "call_1" || fc["arguments"] != `{"command":"ls"}` {
		t.Fatalf("function_call item: %v", fc)
	}
	if fo := in[2].(map[string]any); fo["type"] != "function_call_output" || fo["call_id"] != "call_1" || fo["output"] != "a.go" {
		t.Fatalf("function_call_output item: %v", fo)
	}
	if tl := got["tools"].([]any)[0].(map[string]any); tl["type"] != "function" || tl["name"] != "bash" || tl["parameters"] == nil {
		t.Fatalf("tools: %v", got["tools"])
	}
	if msg.StopReason != llm.StopReasonToolUse || len(msg.Content) != 2 || msg.Content[0].Text != "Listing." || msg.Content[1].ID != "call_9" || string(msg.Content[1].Input) != `{"command":"ls"}` || msg.Usage.OutputTokens != 9 {
		t.Fatalf("reply: %+v", msg)
	}
}

// A cut-off reply is max_tokens, and the gateway's error comes back in its words.
func TestResponsesIncompleteAndErrors(t *testing.T) {
	c, done := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		responsesSSE(w,
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"message"}}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"half"}`,
			`{"type":"response.incomplete","response":{"id":"r","model":"m","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"half"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}`)
	})
	if msg, err := ask(c); err != nil || msg.StopReason != llm.StopReasonMaxTokens || msg.Content[0].Text != "half" {
		t.Fatalf("incomplete: %+v %v", msg, err)
	}
	done()
	c, done = responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"code":"forbidden","message":"token has no access to this provider"}}`)
	})
	defer done()
	if _, err := ask(c); err == nil || !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "no access") {
		t.Fatalf("gateway error in its words: %v", err)
	}
}

// A gateway that streams deltas but sends no final object still yields the
// reply, assembled from what arrived; a failed stream says why.
func TestResponsesStreamWithoutAFinalObjectAndFailure(t *testing.T) {
	c, done := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		responsesSSE(w,
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"message"}}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"only "}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"deltas"}`,
			`{"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","call_id":"c1","name":"bash"}}`,
			`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"command\":\"ls\"}"}`)
	})
	msg, err := ask(c)
	if err != nil || len(msg.Content) != 2 || msg.Content[0].Text != "only deltas" || msg.Content[1].ID != "c1" || string(msg.Content[1].Input) != `{"command":"ls"}` || msg.StopReason != llm.StopReasonToolUse {
		t.Fatalf("assembled from deltas: %+v %v", msg, err)
	}
	done()
	c, done = responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		responsesSSE(w, `{"type":"response.failed","response":{"id":"r","error":{"code":"server_error","message":"upstream down"}}}`)
	})
	defer done()
	if _, err := ask(c); err == nil || !strings.Contains(err.Error(), "upstream down") {
		t.Fatalf("a failed stream in words: %v", err)
	}
}

// A stream whose completed object carries no output (a ChatGPT plan with
// store:false, live 2026-10-10) is read from its events: the text from the
// deltas, the call from the item's done event, the usage from the final.
func TestAStreamWhoseFinalObjectIsEmptyIsReadFromItsEvents(t *testing.T) {
	c, done := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		responsesSSE(w,
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"m1","content":[]}}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"On "}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"it."}`,
			`{"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"fc1","call_id":"call_9","name":"bash","arguments":""}}`,
			`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"command\":"}`,
			`{"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","id":"fc1","call_id":"call_9","name":"bash","arguments":"{\"command\":\"go test ./...\"}"}}`,
			`{"type":"response.completed","response":{"id":"r9","model":"gpt-6.1-sol","status":"completed","output":[],"usage":{"input_tokens":13,"output_tokens":5}}}`)
	})
	defer done()
	msg, err := c.Messages().New(context.Background(), llm.MessageNewParams{Model: "x", Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}})
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID != "r9" || msg.Usage.InputTokens != 13 || msg.Usage.OutputTokens != 5 || string(msg.Model) != "gpt-6.1-sol" {
		t.Fatalf("the final object's id, model and usage are kept: %+v", msg)
	}
	if len(msg.Content) != 2 || msg.Content[0].Text != "On it." || msg.Content[1].Type != "tool_use" || msg.Content[1].Name != "bash" || msg.Content[1].ID != "call_9" || string(msg.Content[1].Input) != `{"command":"go test ./..."}` {
		t.Fatalf("the text and the whole call come from the events: %+v", msg.Content)
	}
	if msg.StopReason != llm.StopReasonToolUse {
		t.Fatalf("a call ends the turn as tool_use: %v", msg.StopReason)
	}
}

// A retry that must open on a call says tool_choice required, as the chat
// client does; an ordinary request leaves the choice to the model.
func TestAForcedCallIsRequiredOnTheResponsesRequest(t *testing.T) {
	var bodies []map[string]any
	c, done := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		bodies = append(bodies, b)
		responsesSSE(w, `{"type":"response.completed","response":{"id":"r1","model":"m","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}`)
	})
	defer done()
	tool := llm.ToolParam{Name: "bash", InputSchema: llm.ToolInputSchemaParam{Properties: map[string]any{"command": map[string]any{"type": "string"}}}}
	params := llm.MessageNewParams{Model: "m", Tools: []llm.ToolUnionParam{{OfTool: &tool}}, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}
	if _, err := c.Messages().New(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Messages().New(llm.WithForcedToolCall(context.Background()), params); err != nil {
		t.Fatal(err)
	}
	if _, there := bodies[0]["tool_choice"]; there {
		t.Fatalf("an ordinary request leaves the choice to the model: %v", bodies[0]["tool_choice"])
	}
	if bodies[1]["tool_choice"] != "required" {
		t.Fatalf("a forced call says required: %v", bodies[1]["tool_choice"])
	}
	for _, tc := range []struct {
		name  string
		tools []llm.ToolUnionParam
	}{
		{name: "no tools"},
		{name: "no usable tools", tools: []llm.ToolUnionParam{{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params.Tools = tc.tools
			if _, err := c.Messages().New(llm.WithForcedToolCall(context.Background()), params); err != nil {
				t.Fatal(err)
			}
			body := bodies[len(bodies)-1]
			if _, there := body["tool_choice"]; there {
				t.Fatalf("a forced request without usable tools omits tool_choice: %v", body["tool_choice"])
			}
			if _, there := body["tools"]; there {
				t.Fatalf("a request without usable tools omits tools: %v", body["tools"])
			}
		})
	}
}
