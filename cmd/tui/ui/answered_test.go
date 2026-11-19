package ui

import "testing"

// A reply tells the window's version check that this window got an answer
// (Ops.Answered); a failed turn and an empty reply do not.
func TestAReplyIsToldAndAFailedTurnIsNot(t *testing.T) {
	told := 0
	m := NewPage(PageConfig{Agent: "coder", Ops: Ops{Answered: func() { told++ }}})
	m.messages = []Message{{Role: "user", Content: "say hello"}}

	mm, _ := m.Update(executionFailedMsg{errText: "401 Unauthorized"})
	m = mm.(Model)
	mm, _ = m.Update(assistantResponseMsg{content: "  "})
	m = mm.(Model)
	if told != 0 {
		t.Fatalf("a failed turn and an empty reply are no answer: told %d", told)
	}

	mm, _ = m.Update(assistantStreamingMsg{content: "Hello from the stand-in provider."})
	m = mm.(Model)
	mm, _ = m.Update(assistantResponseMsg{content: "Hello from the stand-in provider."})
	m = mm.(Model)
	if told != 1 {
		t.Fatalf("a streamed reply is told once: told %d", told)
	}
	mm, _ = m.Update(assistantResponseMsg{content: "Hello again."})
	_ = mm.(Model)
	if told != 2 {
		t.Fatalf("each reply is told (the CLI side decides when to ask): told %d", told)
	}
}
