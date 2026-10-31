package providers

import (
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

func TestStreamReasoningStaysSeparateFromChatAnswer(t *testing.T) {
	for _, field := range []string{"reasoning", "reasoning_content"} {
		t.Run(field, func(t *testing.T) {
			stream := `data: {"choices":[{"delta":{"` + field + `":"partial "}}]}` + "\n\n" +
				`data: {"choices":[{"delta":{"` + field + `":"plan"},"finish_reason":"length"}]}` + "\n\ndata: [DONE]\n\n"
			var acc streamAccumulator
			var displayed strings.Builder
			if err := acc.consume(strings.NewReader(stream), func(s string) { displayed.WriteString(s) }, nil); err != nil {
				t.Fatal(err)
			}
			resp := acc.build()
			msg := convertOAIToAnthropicWithTools(resp.Choices[0], resp.Usage, "test", nil)
			if msg.Thinking != "partial plan" || len(msg.Content) != 0 || displayed.Len() != 0 || msg.StopReason != llm.StopReasonMaxTokens {
				t.Fatalf("reasoning leaked or lost: message %+v, displayed %q", msg, displayed.String())
			}
		})
	}
}

func TestStreamReasoningAliasesAreNotDuplicated(t *testing.T) {
	var acc streamAccumulator
	err := acc.consume(strings.NewReader("data: "+`{"choices":[{"delta":{"reasoning":"plan","reasoning_content":"plan","content":"answer"},"finish_reason":"stop"}]}`+"\n\ndata: [DONE]\n\n"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp := acc.build()
	msg := convertOAIToAnthropicWithTools(resp.Choices[0], resp.Usage, "test", nil)
	if msg.Thinking != "plan" || len(msg.Content) != 1 || msg.Content[0].Text != "answer" {
		t.Fatalf("got %+v", msg)
	}
}
