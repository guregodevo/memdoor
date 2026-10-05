package compaction

import (
	"fmt"
	"strings"
	"testing"

	"memdoor/gateway/context"
	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

func TestFindSafeSplitPoint(t *testing.T) {
	tests := []struct {
		name          string
		messages      []llm.MessageParam
		desiredSplit  int
		expectedSplit int
	}{
		{
			name:          "empty messages",
			messages:      []llm.MessageParam{},
			desiredSplit:  0,
			expectedSplit: 0,
		},
		{
			name: "split at boundary (no adjustment needed)",
			messages: []llm.MessageParam{
				llm.NewUserMessage(llm.NewTextBlock("user1")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant1")),
				llm.NewUserMessage(llm.NewTextBlock("user2")),
			},
			desiredSplit:  1,
			expectedSplit: 1,
		},
		{
			name: "split breaks assistant-user pair (should adjust)",
			messages: []llm.MessageParam{
				llm.NewUserMessage(llm.NewTextBlock("user1")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant1")),
				llm.NewUserMessage(llm.NewTextBlock("user2 with tool_result")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant2")),
			},
			desiredSplit:  2, // Would split between assistant and user
			expectedSplit: 3, // Adjusted to include user message
		},
		{
			name: "split at start",
			messages: []llm.MessageParam{
				llm.NewUserMessage(llm.NewTextBlock("user1")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant1")),
			},
			desiredSplit:  0,
			expectedSplit: 0,
		},
		{
			name: "split at end",
			messages: []llm.MessageParam{
				llm.NewUserMessage(llm.NewTextBlock("user1")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant1")),
			},
			desiredSplit:  2,
			expectedSplit: 2,
		},
		{
			name: "split with tool use pattern",
			messages: []llm.MessageParam{
				llm.NewUserMessage(llm.NewTextBlock("user1")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant1 - tool use")),
				llm.NewUserMessage(llm.NewTextBlock("tool result")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant2")),
				llm.NewUserMessage(llm.NewTextBlock("user2")),
			},
			desiredSplit:  2, // Would split between assistant and tool result
			expectedSplit: 3, // Adjusted to include tool result
		},
		{
			name: "multiple assistant-user pairs",
			messages: []llm.MessageParam{
				llm.NewUserMessage(llm.NewTextBlock("user1")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant1")),
				llm.NewUserMessage(llm.NewTextBlock("tool result 1")),
				llm.NewAssistantMessage(llm.NewTextBlock("assistant2")),
				llm.NewUserMessage(llm.NewTextBlock("tool result 2")),
			},
			desiredSplit:  4, // Would split between assistant2 and tool result 2
			expectedSplit: 5, // Adjusted to include tool result 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSafeSplitPoint(tt.messages, tt.desiredSplit)
			if result != tt.expectedSplit {
				t.Errorf("findSafeSplitPoint() = %d, expected %d", result, tt.expectedSplit)
			}
		})
	}
}

func TestFindSafeSplitPoint_PreservesToolPairs(t *testing.T) {
	// Realistic scenario: conversation with tool uses
	messages := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock("Hello")),
		llm.NewAssistantMessage(llm.NewTextBlock("Hi, let me help")),
		llm.NewUserMessage(llm.NewTextBlock("Read file X")),
		llm.NewAssistantMessage(llm.NewTextBlock("Reading...")), // Has tool_use in real scenario
		llm.NewUserMessage(llm.NewTextBlock("File contents")),   // Has tool_result in real scenario
		llm.NewAssistantMessage(llm.NewTextBlock("Got it")),
		llm.NewUserMessage(llm.NewTextBlock("Thanks")),
	}

	// Try to split at message 4 (would orphan tool result at message 5)
	desiredSplit := 4
	result := findSafeSplitPoint(messages, desiredSplit)

	// Should adjust to 5 to include the tool result
	if result != 5 {
		t.Errorf("Expected split to adjust from %d to 5, got %d", desiredSplit, result)
	}

	// Verify the split keeps assistant+user together
	if messages[result-2].Role != "assistant" {
		t.Error("Expected assistant message before split point")
	}
	if messages[result-1].Role != "user" {
		t.Error("Expected user message at split point")
	}
}

// TestCompactLocallyKeepRecent verifies the count-based prune actually drops the
// oldest messages down to keepRecentN (+1 placeholder). This is the guarantee the
// doer path relies on: its token trigger fires with few-but-large messages, so a
// small keepRecentN must produce a REAL prune, not a no-op.
func TestCompactKeepRecent(t *testing.T) {
	c := &Compactor{
		tokenCounter: context.NewTokenCounter("default", false),
		log:          logs.New("Agent"),
	}

	// 12 alternating messages (well above the doer keep of 6).
	var msgs []llm.MessageParam
	for i := 0; i < 6; i++ {
		msgs = append(msgs, llm.NewUserMessage(llm.NewTextBlock("u")))
		msgs = append(msgs, llm.NewAssistantMessage(llm.NewTextBlock("a")))
	}

	// keepRecentN below the count → must prune to placeholder + ~keepRecentN.
	res, out := c.Compact(msgs, 6)
	if len(out) > 8 { // placeholder + 6, allow +1 for tool-pair safe split
		t.Fatalf("keepRecentN=6 should prune 12 msgs to ~7, got %d", len(out))
	}
	if len(out) >= len(msgs) {
		t.Fatalf("prune was a no-op: before=%d after=%d", len(msgs), len(out))
	}
	if res.MessagesRemoved == 0 {
		t.Fatalf("expected messages removed, got 0")
	}

	// keepRecentN >= count → no-op (the exact trap the doer path avoids by using 6).
	_, out2 := c.Compact(msgs, 20)
	if len(out2) != len(msgs) {
		t.Fatalf("keepRecentN=20 on 12 msgs must be a no-op, got %d", len(out2))
	}
}

// TestElideOldToolResults: short-but-heavy conversations (few messages, huge
// tool outputs) must shrink via output elision — message pruning can't touch
// them (live: should_compact stayed true at 21% util while keep-20 no-opped
// forever). The last keepIntactToolResults messages stay verbatim.
func TestElideOldToolResults(t *testing.T) {
	big := strings.Repeat("x", 2000)
	mk := func(id string) llm.MessageParam {
		return llm.NewUserMessage(llm.NewToolResultBlock(id, big, false))
	}
	msgs := []llm.MessageParam{
		mk("t1"), mk("t2"),
		llm.NewUserMessage(llm.NewTextBlock("hello")),
		mk("t3"), mk("t4"), mk("t5"), mk("t6"), mk("t7"), mk("t8"),
	}
	out, changed := elideOldToolResults(msgs, keepIntactToolResults)
	if !changed {
		t.Fatal("elision should fire on old big outputs")
	}
	// first 3 messages are beyond keepLast=6 → t1,t2 elided, text untouched
	txt := func(m llm.MessageParam) string {
		for _, b := range m.Content {
			if b.OfToolResult != nil {
				s := ""
				for _, cu := range b.OfToolResult.Content {
					if cu.OfText != nil {
						s += cu.OfText.Text
					}
				}
				return s
			}
			if b.OfText != nil {
				return b.OfText.Text
			}
		}
		return ""
	}
	if !strings.Contains(txt(out[0]), "elided") || !strings.Contains(txt(out[1]), "elided") {
		t.Fatal("old big outputs should be stubbed")
	}
	if txt(out[2]) != "hello" {
		t.Fatal("plain text must be untouched")
	}
	for i := 3; i < len(out); i++ {
		if !strings.Contains(txt(out[i]), "xxx") {
			t.Fatalf("recent output %d must stay verbatim", i)
		}
	}
	// pairing preserved: tool_use_id intact on the stub
	if out[0].Content[0].OfToolResult.ToolUseID != "t1" {
		t.Fatal("tool_use_id must survive elision")
	}
}

// The user's request is the oldest message of a turn, so a keep-recent
// pruner drops it first; the agent then guesses what it was asked (live
// 2026-09-02: asked to tighten a video, it made three shorts). The task
// must lead the compacted conversation.
func TestCompactKeepsTheTask(t *testing.T) {
	c := &Compactor{
		tokenCounter: context.NewTokenCounter("default", false),
		log:          logs.New("Agent"),
	}
	task := "Tighten rubio_interview.mp4 into rubio_tight.mp4 and tell me what you removed."
	msgs := []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock(task))}
	for i := 0; i < 8; i++ {
		msgs = append(msgs,
			llm.NewAssistantMessage(llm.NewToolUseBlock("t"+string(rune('a'+i)), `{}`, "bash")),
			llm.NewUserMessage(llm.NewToolResultBlock("t"+string(rune('a'+i)), "output", false)))
	}
	_, out := c.Compact(msgs, 6)
	if len(out) >= len(msgs) {
		t.Fatalf("no pruning happened: %d → %d", len(msgs), len(out))
	}
	first := out[0]
	if first.Role != llm.MessageParamRoleUser || len(first.Content) == 0 || first.Content[0].OfText == nil ||
		!strings.Contains(first.Content[0].OfText.Text, task) {
		t.Fatalf("the task must lead the compacted conversation, got %+v", first)
	}
	// A second compaction must not stack task headers.
	_, out2 := c.Compact(append(out, msgs[len(msgs)-6:]...), 6)
	if n := strings.Count(out2[0].Content[0].OfText.Text, "YOUR TASK"); n != 1 {
		t.Errorf("task header repeated %d times after a second compaction", n)
	}
}

// A compaction carries the receipts of the tool calls it drops — and a
// second compaction carries them again without stacking headers.
func TestCompactionKeepsToolReceipts(t *testing.T) {
	c := &Compactor{
		tokenCounter: context.NewTokenCounter("default", false),
		log:          logs.New("Agent"),
	}
	msgs := []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("cut the best 30s into thanks30.mp4"))}
	msgs = append(msgs,
		llm.NewAssistantMessage(llm.NewToolUseBlock("t1", `{"source":"thanksgiving.mp4","start":78,"end":112,"out":"thanks30.mp4"}`, "clip_cut")),
		llm.NewUserMessage(llm.NewToolResultBlock("t1", "thanks30.mp4: 34.0s, 3182347 bytes\ncovered lines: [78-80s] Tomorrow's Thanksgiving.", false)),
		llm.NewAssistantMessage(llm.NewToolUseBlock("t2", `{"source":"thanks30.mp4","out":"thanks30_tight.mp4"}`, "clip_tighten")),
		llm.NewUserMessage(llm.NewToolResultBlock("t2", "thanks30_tight.mp4: 31.5s (removed 1 pause, 0 fillers)", false)))
	for i := 0; i < 6; i++ {
		msgs = append(msgs,
			llm.NewAssistantMessage(llm.NewToolUseBlock("b"+string(rune('a'+i)), `{"command":"ls"}`, "bash")),
			llm.NewUserMessage(llm.NewToolResultBlock("b"+string(rune('a'+i)), "listing", false)))
	}
	_, out := c.Compact(msgs, 6)
	head := out[0].Content[0].OfText.Text
	for _, want := range []string{"clip_cut(source:thanksgiving.mp4 start:78 end:112 out:thanks30.mp4) → thanks30.mp4: 34.0s", "clip_tighten(", "31.5s (removed 1 pause"} {
		if !strings.Contains(head, want) {
			t.Errorf("receipt %q missing from:\n%s", want, head)
		}
	}
	if strings.Count(head, receiptsHeader) != 1 {
		t.Errorf("one receipts header expected:\n%s", head)
	}
	// Second compaction: the carried receipts survive, still one header.
	_, out2 := c.Compact(append(out, msgs[len(msgs)-6:]...), 6)
	head2 := out2[0].Content[0].OfText.Text
	if !strings.Contains(head2, "clip_cut(") || strings.Count(head2, receiptsHeader) != 1 {
		t.Errorf("receipts lost or header stacked after a second compaction:\n%s", head2)
	}
}

// A conversation carried across turns keeps what was asked and answered when
// its old turns are compacted, and a second compaction carries the digest
// again. Its OLDEST request is not labelled "your task" while the current
// one is kept: that would send the model back to finished work.
func TestCompactKeepsADigestOfEarlierTurns(t *testing.T) {
	c := &Compactor{tokenCounter: context.NewTokenCounter("default", false), log: logs.New("Agent")}
	var msgs []llm.MessageParam
	for i := 0; i < 12; i++ {
		msgs = append(msgs,
			llm.NewUserMessage(llm.NewTextBlock(fmt.Sprintf("request %d: add func F%d", i, i))),
			llm.NewAssistantMessage(llm.NewTextBlock(fmt.Sprintf("Added F%d and its test.", i))))
	}
	_, out := c.Compact(msgs, 4)
	head := out[0].Content[0].OfText.Text
	for _, want := range []string{digestHeader, "- asked: request 0: add func F0", "- answered: Added F0 and its test.", "- asked: request 9: add func F9"} {
		if !strings.Contains(head, want) {
			t.Fatalf("missing %q in:\n%s", want, head)
		}
	}
	for _, never := range []string{taskHeader, "sliding window", "RAG"} {
		if strings.Contains(head, never) {
			t.Fatalf("%q must not be in:\n%s", never, head)
		}
	}
	if last := out[len(out)-2].Content[0].OfText.Text; last != "request 11: add func F11" {
		t.Fatalf("the current request is kept as it was: %q", last)
	}
	// A second compaction carries the digest: F0 is still there, once.
	more := append(out, llm.NewUserMessage(llm.NewTextBlock("request 12")), llm.NewAssistantMessage(llm.NewTextBlock("done 12")))
	_, out2 := c.Compact(more, 2)
	head2 := out2[0].Content[0].OfText.Text
	if strings.Count(head2, "request 0: add func F0") != 1 || strings.Count(head2, digestHeader) != 1 {
		t.Fatalf("the digest must carry forward once:\n%s", head2)
	}
}

// A summary the model wrote replaces the digest, keeps the receipts, and is
// carried word for word through a later, mechanical compaction.
func TestAWrittenSummaryIsCarriedForward(t *testing.T) {
	c := &Compactor{tokenCounter: context.NewTokenCounter("default", false), log: logs.New("Agent")}
	var msgs []llm.MessageParam
	for i := 0; i < 10; i++ {
		msgs = append(msgs, llm.NewUserMessage(llm.NewTextBlock(fmt.Sprintf("request %d", i))),
			llm.NewAssistantMessage(llm.NewTextBlock(fmt.Sprintf("answer %d", i))))
	}
	written := "Goal: a CLI for X. Decided: cobra, because the repo uses it. Next: tests."
	_, out := c.CompactWritten(msgs, 4, written)
	head := out[0].Content[0].OfText.Text
	if !strings.Contains(head, writtenHeader+"\n"+written) || strings.Contains(head, digestHeader) {
		t.Fatalf("the written summary stands in for the digest:\n%s", head)
	}
	more := append(out, llm.NewUserMessage(llm.NewTextBlock("request 10")), llm.NewAssistantMessage(llm.NewTextBlock("answer 10")),
		llm.NewUserMessage(llm.NewTextBlock("request 11")), llm.NewAssistantMessage(llm.NewTextBlock("answer 11")))
	_, out2 := c.Compact(more, 2)
	head2 := out2[0].Content[0].OfText.Text
	if strings.Count(head2, written) != 1 || strings.Count(head2, writtenHeader) != 1 {
		t.Fatalf("a mechanical compaction carries the written summary once:\n%s", head2)
	}
	if !strings.Contains(head2, "- asked: request 10") {
		t.Fatalf("and adds the digest of what came after:\n%s", head2)
	}
}
