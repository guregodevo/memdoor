package gateway

import (
	"encoding/json"
	"testing"

	"memdoor/pkg/llm"
)

// A RETRY'S NUDGE ENDS THE CONVERSATION. What the model read before stays a
// prefix of what it reads with the nudge, so a prefix cache keeps it.
// Mutation check: put the nudge in a new first message and the prefix check
// fails; append to conv's own last message and the original changes.
func TestANudgeEndsTheConversationAndKeepsItsPrefix(t *testing.T) {
	results := llm.NewUserMessage(llm.NewToolResultBlock("call-1", "ok", false))
	conv := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("solve it")),
		{Role: llm.MessageParamRoleAssistant, Content: []llm.ContentBlockParamUnion{llm.NewTextBlock("reading")}},
		results,
	}
	before := jsonOf(t, conv)

	got := nudged(conv, "Call the next tool now.")
	if len(got) != len(conv) {
		t.Fatalf("after tool results the nudge joins them: %d messages, want %d", len(got), len(conv))
	}
	last := got[len(got)-1].Content
	if b := last[len(last)-1]; b.OfText == nil || b.OfText.Text != "Call the next tool now." || len(last) != 2 {
		t.Fatalf("the nudge is the last block: %s", jsonOf(t, got[len(got)-1]))
	}
	if jsonOf(t, conv) != before {
		t.Fatalf("the conversation given was changed: %s", jsonOf(t, conv))
	}
	if jsonOf(t, got[:2]) != jsonOf(t, conv[:2]) {
		t.Fatalf("the messages before the nudge changed")
	}

	afterReply := conv[:2]
	got = nudged(afterReply, "Recap.")
	if len(got) != 3 || got[2].Role != llm.MessageParamRoleUser || got[2].Content[0].OfText.Text != "Recap." {
		t.Fatalf("after an assistant message the nudge is a user message: %s", jsonOf(t, got))
	}
	if got := nudged(conv, ""); len(got) != len(conv) {
		t.Fatalf("no nudge, no change")
	}
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
