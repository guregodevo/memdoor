package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"memdoor/gateway/compaction"
	ctxmgmt "memdoor/gateway/context"
	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
)

// A codebase agent carries the whole conversation: a follow-up is about what
// it just read (live 2026-09-29: "is this Go?" after reading a Go file got
// "No." — it loaded zero prior messages).
func TestACodebaseAgentCarriesTheWholeConversation(t *testing.T) {
	coder := context.WithValue(context.Background(), sharedctx.BuddyToolsKey, []string{"read_file", "bash"})
	if got := recentMessageLimitForAgent(coder); got != wholeTranscript {
		t.Fatalf("the coder loads %d messages, want the whole transcript", got)
	}
	if got := recentMessageLimitForAgent(context.Background()); got != 10 {
		t.Fatalf("a chat agent keeps its 10-message window, got %d", got)
	}
}

// One ladder: old tool output goes first and the dialogue stays word for
// word; it stops as soon as the conversation fits (live 2026-09-29: the
// message-count pruner kept a 20K-token file read because the conversation
// had only twelve messages).
func TestFitShedsToolOutputFirst(t *testing.T) {
	file := strings.Repeat("func x() {}\n", 4000)
	conv := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("Read inference.go")),
		llm.NewAssistantMessage(llm.NewToolUseBlock("t1", `{"path":"inference.go"}`, "read_file")),
		llm.NewUserMessage(llm.NewToolResultBlock("t1", file, false)),
		llm.NewAssistantMessage(llm.NewTextBlock("The first function is runInference.")),
		llm.NewUserMessage(llm.NewTextBlock("What did you say earlier?")),
	}
	size := func(c []llm.MessageParam) int {
		b, _ := json.Marshal(c)
		return len(b)
	}
	compactions := 0
	got, applied := fit(conv, fitLadder, func(c []llm.MessageParam, step fitStep) []llm.MessageParam {
		compactions++
		return c[len(c)-2:]
	}, func(c []llm.MessageParam) bool { return size(c) < 10_000 }, nil)
	if len(applied) != 1 || applied[0].kind != fitStub || compactions != 0 {
		t.Fatalf("one rung, stubbing old tool output, no summary: %v (compactions %d)", applied, compactions)
	}
	if len(got) != len(conv) {
		t.Fatalf("no message is dropped: %d, want %d", len(got), len(conv))
	}
	for _, i := range []int{0, 3, 4} {
		if got[i].Content[0].OfText.Text != conv[i].Content[0].OfText.Text {
			t.Fatalf("message %d changed: the dialogue must stay", i)
		}
	}
	if size(conv) < 40_000 {
		t.Fatal("the input itself must not be modified")
	}
}

// When stubbing is not enough, a summary replaces old messages: it is made
// from the conversation as stubbed, never from an earlier summary, and the
// notes flush runs once, before the first one.
func TestFitSummarizesTheStubbedConversationOnce(t *testing.T) {
	var conv []llm.MessageParam
	for i := 0; i < 20; i++ {
		conv = append(conv, llm.NewUserMessage(llm.NewTextBlock(strings.Repeat("q", 500))),
			llm.NewAssistantMessage(llm.NewTextBlock(strings.Repeat("a", 500))))
	}
	var bases []int
	flushes := 0
	keeps := map[float64]int{0.5: 10, 0.25: 4}
	got, applied := fit(conv, fitLadder, func(c []llm.MessageParam, step fitStep) []llm.MessageParam {
		bases = append(bases, len(c))
		return append([]llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("[summary]"))}, c[len(c)-keeps[step.share]:]...)
	}, func(c []llm.MessageParam) bool { return len(c) <= 5 }, func([]llm.MessageParam) { flushes++ })
	if flushes != 1 {
		t.Fatalf("one flush before the first summary, got %d", flushes)
	}
	for _, b := range bases {
		if b != len(conv) {
			t.Fatalf("every summary is made from the conversation, not a summary: %v", bases)
		}
	}
	if len(got) != 5 || len(applied) != 2 {
		t.Fatalf("summaries keep less until it fits: %d messages after %v", len(got), applied)
	}
}

// A turn whose conversation is reshaped saves the person's request first,
// then the conversation as sent; the next turn loads that. Before, the
// reshape moved the cursor past the request and it was never saved
// (2026-09-29: three requests missing from one transcript).
func TestAReshapeSavesTheRequestThenTheConversationAsSent(t *testing.T) {
	sp, key := boundaryStore(t)
	if err := sp.SaveMessages(key, []llm.MessageParam{user("q1"), asst("a1")}); err != nil {
		t.Fatal(err)
	}
	ar := &AgentRuntime{persistence: sp}
	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{
		"transcript_agent": "coder", "conversation_length": 2,
	}}
	ctx := context.WithValue(context.Background(), ctxSession, session)
	before := []llm.MessageParam{user("q1"), asst("a1"), user("q2")}
	after := []llm.MessageParam{user("[summary of q1]"), user("q2")}
	ar.saveAsSent(ctx, before, after)
	ar.updateSession(session, append(after, asst("a2")))

	history, _ := sp.LoadMessages(key)
	if got := texts(history); got != "q1|a1|q2|a2" {
		t.Fatalf("every message once, the request among them: %s", got)
	}
	sent, _ := sp.LoadRecentMessages(key, wholeTranscript)
	if got := texts(sent); got != "[summary of q1]|q2|a2" {
		t.Fatalf("the next turn sends the same prefix: %s", got)
	}
}

// A compaction aims well under the size that triggers it, so the next
// answers fit before another: stopping just under it compacted three turns
// in a row, each with a new prefix and nothing cached (live 2026-09-29).
func TestACompactionLeavesRoomToGrow(t *testing.T) {
	c, err := compaction.NewCompactor("default", false)
	if err != nil {
		t.Fatal(err)
	}
	ar := &AgentRuntime{compactor: c}
	var conv []llm.MessageParam
	for i := 0; i < 20; i++ {
		conv = append(conv, llm.NewUserMessage(llm.NewTextBlock(strings.Repeat("q", 400))),
			llm.NewAssistantMessage(llm.NewTextBlock(strings.Repeat("a", 400))))
	}
	compactAt := c.CountConversationTokens(conv) * 9 / 10 // just past the threshold
	check := func(m []llm.MessageParam) *ctxmgmt.CheckResult {
		n := c.CountConversationTokens(m)
		return &ctxmgmt.CheckResult{TotalTokens: n, CompactAt: compactAt, CanProceed: true, ShouldCompact: n >= compactAt}
	}
	out, _, err := ar.fitConversation(context.Background(), conv, fitRequest{why: compaction.CompactionThreshold, steps: fitLadder, check: check})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.CountConversationTokens(out); float64(got) > compactTarget*float64(compactAt) {
		t.Fatalf("compacted to %d tokens, want at most %.0f (%.0f%% of %d)", got, compactTarget*float64(compactAt), compactTarget*100, compactAt)
	}
}

// The first rung costs nothing and keeps the most: a file read twice gives
// up its older copy, and when that fits, nothing else is touched.
func TestFitDropsASupersededReadFirst(t *testing.T) {
	file := strings.Repeat("func x() {}\n", 3000)
	conv := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("fix main.go")),
		llm.NewAssistantMessage(llm.NewToolUseBlock("r1", `{"path":"main.go"}`, "read_file")),
		llm.NewUserMessage(llm.NewToolResultBlock("r1", file, false)),
		llm.NewAssistantMessage(llm.NewToolUseBlock("r2", `{"path":"main.go"}`, "read_file")),
		llm.NewUserMessage(llm.NewToolResultBlock("r2", file, false)),
	}
	size := func(c []llm.MessageParam) int {
		b, _ := json.Marshal(c)
		return len(b)
	}
	got, applied := fit(conv, fitLadder, func(c []llm.MessageParam, _ fitStep) []llm.MessageParam { return c },
		func(c []llm.MessageParam) bool { return size(c) < 50_000 }, nil)
	if len(applied) != 1 || applied[0].kind != fitElide {
		t.Fatalf("only the free rung: %v", applied)
	}
	if got[4].Content[0].OfToolResult.Content[0].OfText.Text != file {
		t.Fatal("the latest read stays whole")
	}
}
