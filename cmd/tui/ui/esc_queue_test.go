package ui

import (
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A model mid-turn the way most of a turn looks: the run is going
// (turnRunning), but nothing is "thinking" (a reply is streaming, or the
// model is between rounds) and no tool is active.
func midTurnModel(t *testing.T) (*Model, func(tea.Msg), func() []string) {
	t.Helper()
	var mu sync.Mutex
	var posted []string
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error {
		mu.Lock()
		posted = append(posted, text)
		mu.Unlock()
		return nil
	})
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	m.turnRunning = true
	m.isThinking = false
	m.activeTools = nil
	return &m, step, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), posted...) }
}

func typeAndEnter(step func(tea.Msg), text string) {
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
	step(tea.KeyMsg{Type: tea.KeyEnter})
}

// Esc stops a running turn at any point of it, not only while the spinner
// shows: live 2026-09-28, "escape dont work" — a runaway turn could only be
// ended by restarting the gateway.
func TestEscInterruptsBetweenRounds(t *testing.T) {
	m, step, _ := midTurnModel(t)
	step(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.interrupted || m.busy() {
		t.Fatalf("Esc mid-turn must interrupt: interrupted=%v busy=%v", m.interrupted, m.busy())
	}
	last := m.messages[len(m.messages)-1]
	if last.Content != interruptedNote {
		t.Fatalf("no Interrupted note: %+v", last)
	}
}

// A prompt typed mid-turn is held by the TUI — where Esc can drop it — not
// posted as a second turn the gateway runs after this one, out of reach.
func TestPromptTypedMidTurnIsQueued(t *testing.T) {
	m, step, posted := midTurnModel(t)
	typeAndEnter(step, "stop and delete the tunnel files")
	if len(posted()) != 0 {
		t.Fatalf("a mid-turn prompt was posted as a new turn: %v", posted())
	}
	if len(m.queued) != 1 {
		t.Fatalf("a mid-turn prompt must be queued, queue=%v", m.queued)
	}
	step(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.queued) != 0 {
		t.Fatal("Esc drops the queue with the turn")
	}
}

// Input queued during a turn that ended without a reply is not stranded:
// the run-complete follow-up sends it, and so does the next Enter.
func TestQueuedInputIsSentWhenTheTurnEnds(t *testing.T) {
	m, step, posted := midTurnModel(t)
	typeAndEnter(step, "first")
	step(runCompleteMsg{}) // a silent end: no assistant reply to flush the queue
	step(queueFlushMsg{})
	if got := posted(); len(got) != 1 || got[0] != "first" {
		t.Fatalf("queued input after a silent turn: %v", got)
	}

	m2, step2, posted2 := midTurnModel(t)
	typeAndEnter(step2, "first")
	m2.turnRunning = false // the turn ended; the follow-up has not fired yet
	m2.isThinking = false
	typeAndEnter(step2, "second")
	if got := posted2(); len(got) != 1 || !strings.Contains(got[0], "first") || !strings.Contains(got[0], "second") {
		t.Fatalf("idle Enter must send what was queued, then this: %v", got)
	}
	_ = m
}
