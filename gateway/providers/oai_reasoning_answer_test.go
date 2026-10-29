package providers

import (
	"net/http"
	"strings"
	"testing"
)

// A streamed reply whose answer is inside the reasoning, content empty, is
// answered by the reasoning, as GVS5H's harness does: a Qwen3.8-27B on
// OpenRouter did it four times in one run, the code inside 20K tokens of
// thought (2026-10-29). Both names a host may give the stream are read.
func TestAnAnswerWrittenInsideTheReasoningIsKept(t *testing.T) {
	for _, field := range []string{"reasoning", "reasoning_content"} {
		c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			sse(w,
				`{"choices":[{"index":0,"delta":{"`+field+`":"thinking... ### CODE\n"}}]}`,
				`{"choices":[{"index":0,"delta":{"`+field+`":"print(42)"}}]}`,
				`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":9}}`)
		})
		msg, err := ask(c)
		done()
		if err != nil || len(msg.Content) == 0 || !strings.Contains(msg.Content[0].Text, "print(42)") {
			t.Fatalf("%s: the answer inside the reasoning was lost: err=%v msg=%+v", field, err, msg)
		}
	}
}

// A reply with content keeps its content; the reasoning is not mixed in.
func TestContentWinsOverReasoning(t *testing.T) {
	c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`{"choices":[{"index":0,"delta":{"reasoning":"let me think"}}]}`,
			`{"choices":[{"index":0,"delta":{"content":"ANSWER"}}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":9}}`)
	})
	defer done()
	msg, err := ask(c)
	if err != nil || len(msg.Content) == 0 || msg.Content[0].Text != "ANSWER" {
		t.Fatalf("content replaced: err=%v msg=%+v", err, msg)
	}
}
