package providers

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

func TestAnthropicReasoningSurvivesBothTransports(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, answer := range []string{"", "answer"} {
			t.Run(strings.Join([]string{map[bool]string{false: "json", true: "stream"}[stream], answer}, "/"), func(t *testing.T) {
				c, done := anthropicTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					if !stream {
						w.Header().Set("Content-Type", "application/json")
						io.WriteString(w, `{"id":"msg","model":"claude","stop_reason":"max_tokens","content":[{"type":"thinking","thinking":"partial plan"},{"type":"redacted_thinking","data":"hidden"},{"type":"text","text":"`+answer+`"}],"usage":{"input_tokens":4,"output_tokens":8}}`)
						return
					}
					anthropicSSE(w,
						`{"type":"message_start","message":{"id":"msg","model":"claude","usage":{"input_tokens":4}}}`,
						`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"partial "}}`,
						`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"plan"}}`,
						`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
						`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"`+answer+`"}}`,
						`{"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":8}}`)
				})
				defer done()
				assertTransportReasoning(t, c, answer, "partial plan")
			})
		}
	}
}

func TestResponsesReasoningSurvivesBothTransports(t *testing.T) {
	for _, mode := range []string{"json", "final", "deltas", "done item", "raw deltas"} {
		t.Run(mode, func(t *testing.T) {
			c, done := responsesTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				output := `[{"type":"reasoning","summary":[{"type":"summary_text","text":"partial plan"}]}]`
				final := `{"id":"resp","model":"test","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":` + output + `,"usage":{"input_tokens":4,"output_tokens":8}}`
				if mode == "json" {
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, final)
					return
				}
				events := []string{}
				switch mode {
				case "final", "deltas":
					events = append(events,
						`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"partial "}`,
						`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"plan"}`,
						`{"type":"response.reasoning_summary_text.done","output_index":0,"summary_index":0,"text":"partial plan"}`)
				case "done item":
					events = append(events, `{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","summary":[{"type":"summary_text","text":"partial plan"}]}}`)
				case "raw deltas":
					events = append(events, `{"type":"response.reasoning_text.delta","output_index":0,"delta":"partial plan"}`)
				}
				if mode != "final" {
					final = strings.Replace(final, output, "[]", 1)
				}
				events = append(events, `{"type":"response.incomplete","response":`+final+`}`)
				responsesSSE(w, events...)
			})
			defer done()
			assertTransportReasoning(t, c, "", "partial plan")
		})
	}
}

func assertTransportReasoning(t *testing.T, c LLMClient, answer, thinking string) {
	t.Helper()
	var displayed strings.Builder
	ctx := llm.WithStreamCallback(t.Context(), func(s string) { displayed.WriteString(s) })
	msg, err := c.Messages().New(ctx, llm.MessageNewParams{MaxTokens: 8, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("plan"))}})
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, b := range msg.Content {
		if b.Type == "text" {
			text.WriteString(b.Text)
		}
	}
	if msg.Thinking != thinking || text.String() != answer || msg.StopReason != llm.StopReasonMaxTokens || msg.Usage.InputTokens != 4 || msg.Usage.OutputTokens != 8 {
		t.Fatalf("reasoning, answer, stop reason or usage lost: %+v", msg)
	}
	if displayed.Len() > 0 && displayed.String() != answer {
		t.Fatalf("reasoning leaked to chat callback: %q", displayed.String())
	}
}

func TestChatJSONReasoningFallbackDoesNotChangeStreaming(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, answer := range []string{"", "answer"} {
			t.Run(strings.Join([]string{map[bool]string{false: "json", true: "stream"}[stream], answer}, "/"), func(t *testing.T) {
				c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					if stream {
						sse(w, `{"choices":[{"delta":{"content":"`+answer+`","reasoning_content":"partial plan"},"finish_reason":"length"}],"usage":{"prompt_tokens":4,"completion_tokens":8}}`)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"model":"deepseek/deepseek-r1","choices":[{"message":{"role":"assistant","content":"`+answer+`","reasoning_content":"partial plan"},"finish_reason":"length"}],"usage":{"prompt_tokens":4,"completion_tokens":8}}`)
				})
				defer done()
				want := answer
				if !stream && want == "" {
					want = "partial plan"
				}
				assertTransportReasoning(t, c, want, "partial plan")
			})
		}
	}
}
