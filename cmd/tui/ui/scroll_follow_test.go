package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// A resumed history is not "new": ~110 messages loaded through
// SetContent+gotoBottom, then one refresh while scrolled up, announced
// "↓ 110 new messages" because only refreshFollow moved the baseline
// (2026-09-28).
func TestResumedHistoryIsNotAnnouncedAsNew(t *testing.T) {
	m := &Model{}
	m.viewport.Height = 3
	m.viewport.Width = 80
	for i := 0; i < 110; i++ {
		m.messages = append(m.messages, Message{Role: "assistant", Content: "history"})
	}
	m.viewport.SetContent(m.renderMessages())
	m.gotoBottom()
	m.viewport.GotoTop() // the reader scrolls up
	m.refreshFollow()
	if m.newBelow != 0 {
		t.Fatalf("no message arrived, the bar says %d", m.newBelow)
	}
}

// Letters typed into the prompt must never scroll the transcript.
func TestTypingDoesNotScrollTheTranscript(t *testing.T) {
	m := &Model{viewport: viewport.New(80, 3)} // bubbles' default keys: j/k/b/u/d/f/space scroll
	m.viewport.KeyMap = transcriptKeys()
	for i := 0; i < 40; i++ {
		m.messages = append(m.messages, Message{Role: "assistant", Content: "line"})
	}
	m.viewport.SetContent(m.renderMessages())
	m.gotoBottom()
	for _, r := range "back up" {
		m.viewport, _ = m.viewport.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !m.viewport.AtBottom() {
		t.Fatalf("typing moved the transcript to offset %d", m.viewport.YOffset)
	}
	m.viewport, _ = m.viewport.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.viewport.AtBottom() {
		t.Fatal("PgUp must still scroll")
	}
}

// An interrupted tool frame never gets its result. Left spinning it is never
// settled, and nothing after it could reach scrollback.
func TestInterruptClosesOrphanFrames(t *testing.T) {
	m := &Model{}
	m.messages = []Message{
		{Role: "tool_call", Content: "bash"},
		{Role: "assistant", Content: "after"},
	}
	if settled(m.messages[0]) {
		t.Fatal("precondition: a frame with no result is unsettled")
	}
	m.closeOrphanFrames("interrupted")
	if !settled(m.messages[0]) {
		t.Fatalf("the interrupted frame must settle: %+v", m.messages[0])
	}
}
