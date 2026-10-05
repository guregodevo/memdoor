package ui

import (
	"strings"
	"testing"
)

// THE FINAL REPLY IS NOT PRINTED AGAIN UNDER THE LAST FRAME.
//
// Prose streams round by round with a tool frame between; the final reply
// joins the rounds with a newline. The old byte-prefix test failed on that
// newline and the whole turn's prose came back a second time (Montebourg,
// 2026-09-19). Only what did not stream is new.
func TestAFinalReplySplitAroundToolFramesIsNotRepeated(t *testing.T) {
	m := Model{messages: []Message{
		{Role: "user", Content: "summarize the hearing"},
		{Role: "assistant", Content: "The source is a reaction video. Let me transcribe it."},
		{Role: "tool_call", ToolName: "transcribe"},
		{Role: "assistant", Content: "Good — the picture is clean hearing footage."},
		{Role: "tool_call", ToolName: "clip_inspect"},
	}}
	final := "The source is a reaction video. Let me transcribe it.\nGood — the picture is clean hearing footage.\nI'll read the transcript in chunks."
	tail, dup := m.finalBeyondStreamed(final)
	if !dup {
		t.Fatal("a final that repeats what streamed, split around frames, is a duplicate")
	}
	if tail != "I'll read the transcript in chunks." {
		t.Fatalf("only the unstreamed tail is new, got %q", tail)
	}
	if tail, dup := m.finalBeyondStreamed("The source is a reaction video. Let me transcribe it.\n\nGood — the picture is clean hearing footage."); !dup || tail != "" {
		t.Fatalf("an exact repeat with different spacing adds nothing: dup=%v tail=%q", dup, tail)
	}
	if _, dup := m.finalBeyondStreamed("Something else entirely."); dup {
		t.Fatal("a different reply is not a duplicate")
	}
}

// A ROUND'S TEXT IS NOT PRINTED AGAIN UNDER ITS OWN FRAME. The gateway
// sends each round's text once its tool frame is already on screen; that
// text is the last round, not the turn. Mutation check: compare against
// the whole turn only and the second round is "new" again.
func TestARoundsTextIsNotPrintedAgainUnderItsFrame(t *testing.T) {
	m := Model{messages: []Message{
		{Role: "user", Content: "a short on drones"},
		{Role: "assistant", Content: "Two jobs are open. Let me check where each stands."},
		{Role: "tool_call", ToolName: "bash"},
		{Role: "assistant", Content: "Two drone sources are already fetched. Let me inspect them."},
		{Role: "tool_call", ToolName: "clip_inspect"},
	}}
	if tail, dup := m.finalBeyondStreamed("Two drone sources are already fetched. Let me inspect them."); !dup || tail != "" {
		t.Fatalf("the second round's own text is a duplicate: dup=%v tail=%q", dup, tail)
	}
	if tail, dup := m.finalBeyondStreamed("Two drone sources are already fetched. Let me inspect them. Both are usable."); !dup || tail != "Both are usable." {
		t.Fatalf("only what did not stream is new: dup=%v tail=%q", dup, tail)
	}
	if _, dup := m.finalBeyondStreamed("Let me inspect them."); dup {
		t.Fatal("a fragment of a round is not the round")
	}
}

// A ROUND THAT FOLLOWS A CLOSED ROUND STARTS ITS OWN PARAGRAPH. Two brain
// rounds with no tool frame between them (a retry after a stopped reply)
// streamed into one: "…further!The last line…". Mutation check: drop the
// separator and the words touch.
func TestARoundAfterAClosedRoundStartsItsOwnParagraph(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m.messages = []Message{{Role: "user", Content: "check the seam"}}
	mm, _ := m.Update(assistantStreamingMsg{content: "If you'd like, I can explore further!"})
	m = mm.(Model)
	mm, _ = m.Update(assistantResponseMsg{content: "If you'd like, I can explore further!"})
	m = mm.(Model)
	mm, _ = m.Update(assistantStreamingMsg{content: "The last line shows the dip."})
	m = mm.(Model)
	last := m.messages[len(m.messages)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "further!\n\nThe last line") {
		t.Fatalf("the second round is its own paragraph: %q", last.Content)
	}
	// Within one round, deltas still join as they arrive.
	mm, _ = m.Update(assistantStreamingMsg{content: " Sudden, then back."})
	m = mm.(Model)
	if !strings.HasSuffix(m.messages[len(m.messages)-1].Content, "the dip. Sudden, then back.") {
		t.Fatalf("deltas of one round join: %q", m.messages[len(m.messages)-1].Content)
	}
}

// THE CALL'S TAGS ARE NOT WORDS. The stream carries "<tool_call>" around
// the hidden JSON; the round's text does not. Mutation check: compare the
// raw content and the round prints again.
func TestTheCallsTagsAreNotWords(t *testing.T) {
	m := Model{messages: []Message{
		{Role: "user", Content: "edit it"},
		{Role: "assistant", Content: "13:16 — still 3:16 over. Cuts to make:\n<tool_call>\n\n</tool_call>"},
		{Role: "tool_call", ToolName: "clip_plan"},
	}}
	if tail, dup := m.finalBeyondStreamed("13:16 — still 3:16 over. Cuts to make:"); !dup || tail != "" {
		t.Fatalf("the round with its tags stripped is the round: dup=%v tail=%q", dup, tail)
	}
}

// THE XML CALL'S MARKUP IS NOT WORDS EITHER: "</parameter>" and the stray
// "}" the stream leaves behind. Mutation check: strip only the JSON tags
// and the round prints again.
func TestTheXMLCallsMarkupIsNotWords(t *testing.T) {
	m := Model{messages: []Message{
		{Role: "user", Content: "edit it"},
		{Role: "assistant", Content: "Fits at 9:52. The two remaining overlaps are snap artifacts.\n<tool_call>\n<function=clip_stitch>\n<parameter=parts>\n\n</parameter>\n}\n</tool_call>"},
		{Role: "tool_call", ToolName: "clip_stitch"},
	}}
	if tail, dup := m.finalBeyondStreamed("Fits at 9:52. The two remaining overlaps are snap artifacts."); !dup || tail != "" {
		t.Fatalf("markup stripped, the round is the round: dup=%v tail=%q", dup, tail)
	}
}

// A slash command typed while the turn runs (/copy) is echoed as a user
// message; it must not hide what streamed before it from the dedup.
func TestASlashEchoMidTurnDoesNotResetTheDedupWindow(t *testing.T) {
	m := Model{messages: []Message{
		{Role: "user", Content: "add the history endpoint"},
		{Role: "assistant", Content: "Everything checks out. Now the implementation."},
		{Role: "user", Content: "/copy"},
		{Role: "system", Content: "Copied the last interaction to the clipboard (93 lines)."},
		{Role: "tool_call", ToolName: "apply_patch"},
		{Role: "tool_call", ToolName: "apply_patch"},
	}}
	if tail, dup := m.finalBeyondStreamed("Everything checks out. Now the implementation."); !dup || tail != "" {
		t.Fatalf("the round's text streamed before /copy: dup=%v tail=%q", dup, tail)
	}
	m.messages = append(m.messages, Message{Role: "assistant", Content: "Two fixes needed."}, Message{Role: "tool_call", ToolName: "apply_patch"})
	if tail, dup := m.finalBeyondStreamed("Everything checks out. Now the implementation.\nTwo fixes needed."); !dup || tail != "" {
		t.Fatalf("both rounds streamed: dup=%v tail=%q", dup, tail)
	}
}
