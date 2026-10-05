package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// While the brain cannot answer, Enter shows why instead of sending, and
// keeps the text; a slash command still runs; with no gate the text goes.
// The mutation check: drop the gate branch and the first case sends.
func TestEnterIsGatedWhileTheBrainIsNotOn(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m.status.Gate = "no model configured — `memdoor connect`"

	m.input.SetValue("cut the best minute")
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(Model)
	if m.input.Value() != "cut the best minute" {
		t.Fatalf("the text must stay in the box, got %q", m.input.Value())
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "system" || !strings.Contains(last.Content, "no model configured") {
		t.Fatalf("the gate must be shown: %+v", last)
	}

	m.status.Gate = ""
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
