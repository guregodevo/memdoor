package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"memdoor/pkg/llm"
)

// A tool that succeeds with nothing to say used to be sent as an empty text
// block, and a Claude request refuses the whole message for it ("text type
// must have non-empty text field", live 2026-11-18, a grep with no matches —
// message[18], a record already in the transcript). The request carries
// "(no output)": for a result built empty from now on, and for one persisted
// empty before the guard.
func TestAnEmptyToolResultIsSentAsWords(t *testing.T) {
	var anthropic map[string]any
	ac, adone := anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &anthropic)
		anthropicSSE(w,
			`{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-5","usage":{"input_tokens":5}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"OK"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
			`{"type":"message_stop"}`)
	})
	defer adone()
	var responses map[string]any
	rc, rdone := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &responses)
		responsesSSE(w,
			`{"type":"response.created","response":{"id":"resp_1"}}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1"}}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"OK"}`,
			`{"type":"response.completed","response":{"id":"resp_1","model":"mytestprovider-slug","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":5,"output_tokens":1}}}`)
	})
	defer rdone()

	msgs := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("find the gate")),
		{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{{OfToolUse: &llm.ToolUseBlockParam{Type: "tool_use", ID: "tu_1", Name: "grep", Input: json.RawMessage(`{"pattern":"x"}`)}}}},
		// built by hand, as a record persisted before the guard was:
		{Role: llm.MessageParamRoleUser, Content: []llm.ContentBlockParamUnion{{OfToolResult: &llm.ToolResultBlockParam{Type: "tool_result", ToolUseID: "tu_1", Content: []llm.ToolResultBlockParamContentUnion{{OfText: &llm.TextBlockParam{Type: "text", Text: ""}}}}}}},
		{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{{OfToolUse: &llm.ToolUseBlockParam{Type: "tool_use", ID: "tu_2", Name: "grep", Input: json.RawMessage(`{"pattern":"y"}`)}}}},
		llm.NewUserMessage(llm.NewToolResultBlock("tu_2", "", false)),
	}
	params := llm.MessageNewParams{Model: "m", MaxTokens: 16, Messages: msgs}
	if _, err := ac.Messages().New(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	am := anthropic["messages"].([]any)
	if got := am[2].(map[string]any)["content"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"]; got != "(no output)" {
		t.Fatalf("a persisted empty result is sent with words: %v", got)
	}
	if got := am[4].(map[string]any)["content"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"]; got != "(no output)" {
		t.Fatalf("a result built empty is sent with words: %v", got)
	}

	if _, err := rc.Messages().New(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	in := responses["input"].([]any)
	if got := in[2].(map[string]any)["output"]; got != "(no output)" {
		t.Fatalf("responses, a persisted empty result: %v", got)
	}
	if got := in[4].(map[string]any)["output"]; got != "(no output)" {
		t.Fatalf("responses, a result built empty: %v", got)
	}
}
