package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// While the brain cannot answer, Enter shows why instead of sending and
// keeps the text out of the box (left there, the /connect typed next was
// appended to it, 2026-10-12); the footer's next status with no gate puts it
// back; then Enter sends it. The mutation check: drop the gate branch and the
// first case sends.
func TestEnterIsGatedWhileTheBrainIsNotOn(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	gate := "no model configured — `memdoor connect`"
	m.status.Gate = gate
	m.statusOp = func() Status { return Status{Gate: gate} }

	m.input.SetValue("cut the best minute")
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(Model)
	if m.input.Value() != "" {
		t.Fatalf("the refused text must leave the box, got %q", m.input.Value())
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "system" || !strings.Contains(last.Content, "no model configured") || !strings.Contains(last.Content, "kept") {
		t.Fatalf("the gate and that the text is kept must be shown: %+v", last)
	}

	// Still gated: the text stays held.
	mm, _ = m.Update(statusTickMsg{})
	m = mm.(Model)
	if m.input.Value() != "" {
		t.Fatalf("still gated, the box must stay empty, got %q", m.input.Value())
	}

	// The gate lifts: the text is back in the box.
	gate = ""
	mm, _ = m.Update(statusTickMsg{})
	m = mm.(Model)
	if m.input.Value() != "cut the best minute" {
		t.Fatalf("the kept text must come back once the gate lifts, got %q", m.input.Value())
	}

	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(Model)
	// Offline in a test, so it queues — the point is that it LEFT the box.
	if m.input.Value() != "" || len(m.queued) != 1 {
		t.Fatalf("with no gate the turn goes: box=%q queued=%d", m.input.Value(), len(m.queued))
	}
}

// The footer refreshes every few seconds while a gate is up, every half
// minute otherwise — a starting brain is watched, not waited for.
func TestTheFooterWatchesAGatedBrain(t *testing.T) {
	m := NewPage(PageConfig{Agent: "coder", Ops: Ops{Status: func() Status { return Status{Gate: "no brain here"} }}})
	mm, cmd := m.Update(statusTickMsg{})
	if cmd == nil {
		t.Fatal("no follow-up tick")
	}
	if mm.(Model).status.Gate == "" {
		t.Fatal("the status was not refreshed")
	}
}
