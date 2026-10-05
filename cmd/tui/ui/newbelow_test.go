package ui

import (
	"strings"
	"testing"
)

// The jump bar must count MESSAGES, not refreshes.
//
// refreshFollow incremented once per call, and streaming calls it once per
// delta — so a single reply arriving in hundreds of chunks reported "↓ 87 new
// messages" when three had actually appeared (2026-08-30). A count that grows
// with the model's typing speed tells the reader nothing about what they
// missed.
func TestJumpBarCountsMessagesNotChunks(t *testing.T) {
	m := &Model{}
	m.viewport.Height = 10
	m.viewport.Width = 80

	// Reader has scrolled up.
	m.messages = append(m.messages, Message{Role: "assistant", Content: "one"})
	m.refreshFollow()
	m.viewport.SetContent("x\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\ny")
	m.viewport.GotoTop()
	m.newBelow, m.lastCounted = 0, len(m.messages)

	// ONE new message, delivered as fifty streamed chunks.
	m.messages = append(m.messages, Message{Role: "assistant", Content: ""})
	for i := 0; i < 50; i++ {
		m.messages[len(m.messages)-1].Content += "tok"
		m.refreshFollow()
	}

	if m.newBelow > 1 {
		t.Errorf("one message arriving in 50 chunks must count as 1, got %d — "+
			"this is the '↓ 87 new messages' bug", m.newBelow)
	}
}

// Several real messages must still be counted, or the reader loses the jump bar
// entirely and never learns anything arrived.
func TestJumpBarCountsSeveralMessages(t *testing.T) {
	m := &Model{}
	m.viewport.Height = 3
	m.viewport.Width = 80

	// Enough content that the viewport genuinely cannot show it all.
	for i := 0; i < 20; i++ {
		m.messages = append(m.messages, Message{Role: "assistant", Content: "filler"})
	}
	m.refreshFollow()
	m.viewport.GotoTop() // reader scrolls up
	m.newBelow, m.lastCounted = 0, len(m.messages)

	for i := 0; i < 3; i++ {
		m.messages = append(m.messages, Message{Role: "assistant", Content: "new"})
		m.refreshFollow()
	}
	if m.newBelow != 3 {
		t.Errorf("three new messages below the fold must be announced, got %d", m.newBelow)
	}
}

// BOTH PATHS OF THE SCROLL PROMISE (battle test, 2026-09-27). The window says
// scrolling up is never yanked back down; that is two behaviours, and only one
// of them is interesting when it breaks.
func TestScrollHoldsWhenScrolledAndFollowsWhenAtBottom(t *testing.T) {
	long := strings.Repeat("filler\n", 60)

	// NEGATIVE PATH: the reader is scrolled up. New output must not move them,
	// and it must be counted for the jump bar.
	up := &Model{}
	up.viewport.Height, up.viewport.Width = 10, 80
	up.messages = []Message{{Role: "assistant", Content: long}}
	up.refreshFollow()
	up.viewport.GotoTop()
	up.newBelow, up.lastCounted = 0, len(up.messages)
	at := up.viewport.YOffset
	for i := 0; i < 20; i++ {
		up.messages[0].Content += "more streamed output\n"
		up.refreshFollow()
	}
	if up.viewport.YOffset != at {
		t.Errorf("a scrolled-up reader was moved: offset %d → %d", at, up.viewport.YOffset)
	}
	if up.viewport.AtBottom() {
		t.Error("a scrolled-up reader ended up at the bottom")
	}

	// POSITIVE PATH: the reader is at the bottom. New output must follow, or
	// the window looks frozen while a turn works.
	down := &Model{}
	down.viewport.Height, down.viewport.Width = 10, 80
	down.messages = []Message{{Role: "assistant", Content: long}}
	down.refreshFollow()
	down.gotoBottom()
	for i := 0; i < 20; i++ {
		down.messages[0].Content += "more streamed output\n"
		down.refreshFollow()
	}
	if !down.viewport.AtBottom() {
		t.Error("a reader at the bottom must keep following the output")
	}
	if down.newBelow != 0 {
		t.Errorf("nothing is 'below' for a reader who is already there: %d", down.newBelow)
	}
}
