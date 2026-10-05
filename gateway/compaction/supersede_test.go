package compaction

import (
	"strings"
	"testing"

	"memdoor/gateway/context"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

func call(id, name, input string) llm.MessageParam {
	return llm.NewAssistantMessage(llm.NewToolUseBlock(id, input, name))
}
func result(id, text string, isErr bool) llm.MessageParam {
	return llm.NewUserMessage(llm.NewToolResultBlock(id, text, isErr))
}
func resultText(m llm.MessageParam) string { return m.Content[0].OfToolResult.Content[0].OfText.Text }

// A result read again later is stubbed, its call kept; before this turn a
// failed call and a search that found nothing are dropped with their
// results; the current turn stays as it was. No model call.
func TestElideSpent(t *testing.T) {
	msgs := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("fix main.go")),
		call("r1", "read_file", `{"path":"main.go"}`), result("r1", "old main.go", false),
		call("b1", "apply_patch", `{"input":"a whole patch"}`), result("b1", "patch failed", true),
		call("g1", "grep", `{"pattern":"TODO"}`), result("g1", "", false),
		call("r2", "read_file", `{ "path": "main.go" }`), result("r2", "new main.go", false),
		llm.NewAssistantMessage(llm.NewTextBlock("fixed")),
		llm.NewUserMessage(llm.NewTextBlock("now run it")),
		call("b2", "bash", `{"command":"go run ."}`), result("b2", "failed again", true),
		call("g2", "grep", `{"pattern":"main"}`), result("g2", "", false),
	}
	out, changed := ElideSpent(msgs)
	if !changed {
		t.Fatal("something to elide")
	}
	var ids []string
	for _, m := range out {
		for _, b := range m.Content {
			if b.OfToolUse != nil {
				ids = append(ids, b.OfToolUse.ID)
			}
		}
	}
	if got := strings.Join(ids, ","); got != "r1,r2,b2,g2" {
		t.Fatalf("the failed patch and the empty search are dropped, calls and all: %s", got)
	}
	if got := resultText(out[2]); got != supersededStub {
		t.Fatalf("the first read of main.go is superseded (same call, respaced input): %q", got)
	}
	if resultText(out[4]) != "new main.go" {
		t.Fatal("the latest read stays")
	}
	if len(out) != len(msgs)-4 {
		t.Fatalf("four messages go (two calls, two results): %d", len(out))
	}
	if resultText(msgs[2]) != "old main.go" {
		t.Fatal("the input is not changed")
	}
	if _, again := ElideSpent(out); again {
		t.Fatal("nothing is given up twice")
	}
}

// Recent messages are kept by tokens, never fewer than the minimum.
func TestKeepForTokens(t *testing.T) {
	c := &Compactor{tokenCounter: context.NewTokenCounter("default", false), log: logs.New("Agent")}
	var msgs []llm.MessageParam
	for i := 0; i < 10; i++ {
		msgs = append(msgs, llm.NewUserMessage(llm.NewTextBlock(strings.Repeat("x", 4000))))
	}
	one := c.CountConversationTokens(msgs[:1])
	if got := c.KeepForTokens(msgs, one*3+one/2, 2); got != 3 {
		t.Fatalf("three messages fit in three and a half: %d", got)
	}
	if got := c.KeepForTokens(msgs, 10, 2); got != 2 {
		t.Fatalf("never fewer than the minimum: %d", got)
	}
}
