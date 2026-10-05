package gateway

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"memdoor/gateway/compaction"
	"memdoor/gateway/config"
	"memdoor/pkg/llm"
)

func user(text string) llm.MessageParam { return llm.NewUserMessage(llm.NewTextBlock(text)) }
func asst(text string) llm.MessageParam { return llm.NewAssistantMessage(llm.NewTextBlock(text)) }

func texts(msgs []llm.MessageParam) string {
	var out []string
	for _, m := range msgs {
		out = append(out, messageText(m))
	}
	return strings.Join(out, "|")
}

func boundaryStore(t *testing.T) (*SessionPersistence, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return sp, transcriptKey("workspace:w:channel:c", "coder")
}

// A turn loads the conversation as the last one sent it, plus what came
// after; the history keeps every message once, without the copies.
func TestATurnLoadsFromTheLastBoundary(t *testing.T) {
	sp, key := boundaryStore(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(sp.SaveMessages(key, []llm.MessageParam{user("q1"), asst("a1"), user("q2"), asst("a2"), user("q3")}))
	must(sp.SaveConversation(key, []llm.MessageParam{user("[summary of q1..a2]"), user("q3")}))
	must(sp.SaveMessages(key, []llm.MessageParam{asst("a3")}))

	sent, err := sp.LoadRecentMessages(key, wholeTranscript)
	must(err)
	if got := texts(sent); got != "[summary of q1..a2]|q3|a3" {
		t.Fatalf("the conversation as sent: %s", got)
	}
	history, err := sp.LoadMessages(key)
	must(err)
	if got := texts(history); got != "q1|a1|q2|a2|q3|a3" {
		t.Fatalf("the history, every message once: %s", got)
	}
	// A later reshape replaces the earlier one.
	must(sp.SaveMessages(key, []llm.MessageParam{user("q4")}))
	must(sp.SaveConversation(key, []llm.MessageParam{user("[summary of q1..a3]"), user("q4")}))
	sent, _ = sp.LoadRecentMessages(key, wholeTranscript)
	if got := texts(sent); got != "[summary of q1..a3]|q4" {
		t.Fatalf("the last boundary wins: %s", got)
	}
	// A window (a chat agent's last N) is taken from the conversation as sent.
	sent, _ = sp.LoadRecentMessages(key, 1)
	if got := texts(sent); got != "q4" {
		t.Fatalf("the last message of it: %s", got)
	}
}

// Rewinding a turn in which the conversation was reshaped drops the block
// with it: the turn before loads as it was.
func TestRewindAcrossABoundary(t *testing.T) {
	sp, key := boundaryStore(t)
	_ = sp.SaveMessages(key, []llm.MessageParam{user("q1"), asst("a1"), user("q2")})
	_ = sp.SaveConversation(key, []llm.MessageParam{user("[summary]"), user("q2")})
	_ = sp.SaveMessages(key, []llm.MessageParam{asst("a2")})
	if _, err := sp.RewindSession(key, 1); err != nil {
		t.Fatal(err)
	}
	sent, _ := sp.LoadRecentMessages(key, wholeTranscript)
	if got := texts(sent); got != "q1|a1" {
		t.Fatalf("after the rewind: %s", got)
	}
}

// An oversized file is cut where it changes nothing a turn loads and never
// mid-turn: above the boundary when there is one, at a request when not.
func TestTrimPoint(t *testing.T) {
	rec := func(m llm.MessageParam) MessageRecord { return MessageRecord{Message: m} }
	turn := func(i int) []MessageRecord {
		id := "t" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		return []MessageRecord{rec(user("q")),
			rec(llm.NewAssistantMessage(llm.NewToolUseBlock(id, `{}`, "bash"))),
			rec(llm.NewUserMessage(llm.NewToolResultBlock(id, "out", false))),
			rec(asst("a"))}
	}
	var plain []MessageRecord
	for i := 0; i < 80; i++ { // 320 records; the 200-record cut lands inside a turn
		plain = append(plain, turn(i)...)
	}
	plain = append(plain, rec(user("q")), rec(asst("a"))) // 322: cut at 122, mid-turn
	cut := trimPoint(plain)
	if cut != 124 || !startsATurn(plain[cut]) {
		t.Fatalf("with no boundary the file starts at the next request: cut=%d", cut)
	}

	// A boundary early in the file: nothing at or after it is cut.
	withBoundary := append([]MessageRecord{}, plain[:40]...)
	withBoundary = append(withBoundary, MessageRecord{Message: user("[summary]"), Boundary: true, Carried: true})
	withBoundary = append(withBoundary, plain[40:]...)
	if cut := trimPoint(withBoundary); cut != 40 {
		t.Fatalf("the conversation as sent starts at the boundary (40): cut=%d", cut)
	}
	if cut := trimPoint(plain[:150]); cut != 0 {
		t.Fatalf("under the keep count nothing is cut: %d", cut)
	}
}

// A tool result is cut on a character, and the message in memory is not
// changed by the cut of its saved copy.
func TestCutToolResultsCopiesAndKeepsCharacters(t *testing.T) {
	big := strings.Repeat("é", 100) // 200 bytes, two per character
	msg := llm.NewUserMessage(llm.NewToolResultBlock("t1", big, false), llm.NewTextBlock("kept"))
	out, n := cutToolResults(msg, 51, "…")
	if n != 1 {
		t.Fatalf("one result cut, got %d", n)
	}
	got := out.Content[0].OfToolResult.Content[0].OfText.Text
	if !utf8.ValidString(got) || len(got) > 51 || !strings.HasSuffix(got, "…") {
		t.Fatalf("cut inside a character or too long: %d bytes %q", len(got), got)
	}
	if msg.Content[0].OfToolResult.Content[0].OfText.Text != big {
		t.Fatal("the message in memory was changed")
	}
	if same, n := cutToolResults(msg, 1000, "…"); n != 0 || same.Content[0].OfToolResult != msg.Content[0].OfToolResult {
		t.Fatal("under the limit nothing is copied or cut")
	}
}

// A tool result is cut as it enters the conversation, to the transcript's
// limit, so the model is sent what the transcript keeps.
func TestAToolResultIsCutAsItEnters(t *testing.T) {
	sp, _ := boundaryStore(t)
	ar := &AgentRuntime{persistence: sp}
	limit := sp.cfg.Session.MaxToolResultBytes
	big := llm.NewToolResultBlock("t1", strings.Repeat("x", limit*2), false)
	got := ar.capToolResult(big).OfToolResult.Content[0].OfText.Text
	if len(got) > limit || !strings.HasSuffix(got, sp.getTruncationTail()) {
		t.Fatalf("cut to %d bytes with the tail, got %d", limit, len(got))
	}
	small := llm.NewToolResultBlock("t2", "ok", false)
	if ar.capToolResult(small).OfToolResult != small.OfToolResult {
		t.Fatal("a small result passes untouched")
	}
}

// /compact summarizes the conversation as last sent now, the focus at the
// head of the summary, and the next turn sends that; the history keeps
// every message.
func TestCompactNow(t *testing.T) {
	sp, key := boundaryStore(t)
	c, err := compaction.NewCompactor("default", false)
	if err != nil {
		t.Fatal(err)
	}
	ar := &AgentRuntime{persistence: sp, compactor: c}
	var conv []llm.MessageParam
	for i := 0; i < 30; i++ {
		conv = append(conv, user(fmt.Sprintf("q%d %s", i, strings.Repeat("x", 3000))), asst(fmt.Sprintf("a%d", i)))
	}
	if err := sp.SaveMessages(key, conv); err != nil {
		t.Fatal(err)
	}
	res, err := ar.CompactNow(context.Background(), key, "coder", "keep the API decisions")
	if err != nil {
		t.Fatal(err)
	}
	// The recent messages it keeps fit manualKeepTokens; the summary is small.
	if !res.Summarized || res.AfterTokens > manualKeepTokens+2_000 || res.AfterMessages >= res.BeforeMessages {
		t.Fatalf("it must shrink to about %d tokens: %+v", manualKeepTokens, res)
	}
	sent, _ := sp.LoadRecentMessages(key, wholeTranscript)
	head := messageText(sent[0])
	if !strings.HasPrefix(head, "FOCUS (asked for with /compact") || !strings.Contains(head, "keep the API decisions") || !strings.Contains(head, "- asked: q0") {
		t.Fatalf("the focus leads the summary, the digest follows:\n%.300s", head)
	}
	if messageText(sent[len(sent)-1]) != "a29" {
		t.Fatal("the latest messages are kept")
	}
	if history, _ := sp.LoadMessages(key); len(history) != 60 {
		t.Fatalf("the history keeps every message: %d", len(history))
	}
	// Next to nothing left to give up: nothing saved, the prefix stays.
	records, _ := sp.readAllRecords(mustPath(t, sp, key), key)
	again, _ := ar.CompactNow(context.Background(), key, "coder", "")
	after, _ := sp.readAllRecords(mustPath(t, sp, key), key)
	if again.AfterTokens != again.BeforeTokens || len(after) != len(records) {
		t.Fatalf("a second /compact changes nothing: %+v, %d → %d records", again, len(records), len(after))
	}
}

func mustPath(t *testing.T, sp *SessionPersistence, key string) string {
	t.Helper()
	p, err := sp.resolveSessionStorePath(key)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// /compact keeps compaction.keepRecentTokens word for word when it is set.
func TestCompactNowFollowsKeepRecentTokens(t *testing.T) {
	sp, key := boundaryStore(t)
	c, err := compaction.NewCompactor("default", false)
	if err != nil {
		t.Fatal(err)
	}
	ar := &AgentRuntime{persistence: sp, compactor: c, compactionConfig: &config.CompactionConfig{KeepRecentTokens: 3_000}}
	var conv []llm.MessageParam
	for i := 0; i < 30; i++ {
		conv = append(conv, user(fmt.Sprintf("q%d %s", i, strings.Repeat("x", 3000))), asst(fmt.Sprintf("a%d", i)))
	}
	_ = sp.SaveMessages(key, conv)
	res, err := ar.CompactNow(context.Background(), key, "coder", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.AfterTokens > 3_000+2_000 {
		t.Fatalf("it keeps about 3K: %+v", res)
	}
}

// A stub names its call; recall returns the original output from the
// transcript, word for word, never running anything again.
func TestRecallReturnsAStubbedOutput(t *testing.T) {
	sp, key := boundaryStore(t)
	ar := &AgentRuntime{persistence: sp}
	orig := "commit 3f2a1b: fix the parser\n" + strings.Repeat("diff line\n", 50)
	_ = sp.SaveMessages(key, []llm.MessageParam{
		user("commit it"),
		llm.NewAssistantMessage(llm.NewToolUseBlock("c1", `{"command":"git commit -am fix"}`, "bash")),
		llm.NewUserMessage(llm.NewToolResultBlock("c1", orig, false)),
	})
	stubbed, _ := compaction.ShedToolResults(func() []llm.MessageParam { m, _ := sp.LoadRecentMessages(key, wholeTranscript); return m }(), 0)
	stub := stubbed[2].Content[0].OfToolResult.Content[0].OfText.Text
	if !strings.Contains(stub, `recall {"id": "c1"}`) {
		t.Fatalf("the stub names the call and the way back: %s", stub)
	}
	got, err := ar.recallOutput(key, "c1")
	if err != nil || got != orig {
		t.Fatalf("word for word: %v %q", err, got)
	}
	if _, err := ar.recallOutput(key, "nope"); err == nil {
		t.Fatal("an unknown id is an error")
	}
}
