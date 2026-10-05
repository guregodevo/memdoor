package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// runCmd executes a tea.Cmd (and any batched children) to trigger side effects
// like the poster — bubbletea would do this on the event loop.
func runCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			runCmd(c)
		}
	}
}

// TestAlwaysAuto pins the no-modes contract: the coder always runs in auto
// (permission_mode "acceptEdits"), Shift+Tab no longer cycles anything, and a
// submitted turn dispatches straight to the coder. The ONLY interruption point
// is the ask_user_question picker (covered below).
func TestAlwaysAuto(t *testing.T) {
	var modes []string
	var agents []string
	poster := func(agent, workspace, text, mode string) error {
		agents = append(agents, agent)
		modes = append(modes, mode)
		return nil
	}
	m := NewModel("http://x", "ws/demo", "chan", "", poster)
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }

	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})

	if got := m.permissionModeString(); got != "acceptEdits" {
		t.Fatalf("always-auto: permission mode must be acceptEdits, got %q", got)
	}
	if !strings.Contains(m.modeLabel(), "auto") {
		t.Fatalf("header badge should say auto, got %q", m.modeLabel())
	}

	// Shift+Tab is a no-op now — it must not change the dispatch mode.
	step(tea.KeyMsg{Type: tea.KeyShiftTab})
	m.input.SetValue("write a fizzbuzz")
	step(tea.KeyMsg{Type: tea.KeyEnter})
	if len(agents) != 1 || agents[0] != "coder" {
		t.Fatalf("turn should go straight to the coder, got %v", agents)
	}
	if modes[0] != "acceptEdits" {
		t.Fatalf("dispatch must carry acceptEdits, got %q", modes[0])
	}
	_ = time.Now // keep the time import used by the remaining tests
}

// TestInteractiveQuestion drives the ask_user_question picker: a questionMsg sets
// the pending picker, ↑/↓ move the highlight, ⏎ and number keys select (clearing
// it). The answer send goes over the WS (nil in tests), so we assert on state.
func TestInteractiveQuestion(t *testing.T) {
	newM := func() (Model, func(tea.Msg) Model) {
		m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
		step := func(msg tea.Msg) Model { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd); return m }
		step(tea.WindowSizeMsg{Width: 100, Height: 40})
		step(websocketConnectedMsg{})
		return m, step
	}
	q := questionMsg{id: "q1", question: "Which?", options: []string{"A", "B", "C"}}

	t.Run("question sets picker; arrows move; enter selects", func(t *testing.T) {
		m, step := newM()
		m = step(q)
		if m.pendingQuestion == nil || m.pendingQuestion.index != 0 {
			t.Fatalf("questionMsg should set the picker at index 0, got %+v", m.pendingQuestion)
		}
		m = step(tea.KeyMsg{Type: tea.KeyDown})
		m = step(tea.KeyMsg{Type: tea.KeyDown})
		if m.pendingQuestion.index != 2 {
			t.Fatalf("two ↓ should move to index 2, got %d", m.pendingQuestion.index)
		}
		m = step(tea.KeyMsg{Type: tea.KeyUp})
		if m.pendingQuestion.index != 1 {
			t.Fatalf("↑ should move to index 1, got %d", m.pendingQuestion.index)
		}
		m = step(tea.KeyMsg{Type: tea.KeyEnter})
		if m.pendingQuestion != nil {
			t.Fatal("⏎ should select and clear the picker")
		}
	})

	t.Run("number key selects directly", func(t *testing.T) {
		m, step := newM()
		m = step(q)
		m = step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
		if m.pendingQuestion != nil {
			t.Fatal("number key should select and clear the picker")
		}
	})

	t.Run("picker keys do not leak into the input field", func(t *testing.T) {
		m, step := newM()
		m = step(q)
		m = step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		if v := m.input.Value(); v != "" {
			t.Fatalf("selecting should not type into the input, got %q", v)
		}
	})
}

// TestSpinnerWatchdog: a thinking spinner with no activity past the threshold is
// cleared by the tick handler, so a lost completion event can't hang the UI.
func TestSpinnerWatchdog(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})

	// Simulate a stuck run: thinking, last activity well past the stall timeout.
	m.isThinking = true
	m.thinkingStartTime = time.Now().Add(-watchdogStallTimeout - time.Minute)
	m.lastEventAt = m.thinkingStartTime
	step(thinkingTickMsg(time.Now()))
	if m.isThinking {
		t.Fatal("watchdog should clear the spinner after prolonged silence")
	}

	// A recent event keeps the spinner alive.
	m2 := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
	step2 := func(msg tea.Msg) { nm, cmd := m2.Update(msg); m2 = nm.(Model); runCmd(cmd) }
	step2(tea.WindowSizeMsg{Width: 100, Height: 40})
	m2.isThinking = true
	m2.thinkingStartTime = time.Now().Add(-200 * time.Second)
	m2.lastEventAt = time.Now() // fresh activity
	step2(thinkingTickMsg(time.Now()))
	if !m2.isThinking {
		t.Fatal("recent activity should keep the spinner alive")
	}
}

// TestRunCompleteClearsSpinner: a SILENT completion (no final assistant text —
// the agent's last act was a tool call) must clear the spinner. runCompleteMsg is
// the only signal that does it, and the lifecycle-complete event is what sends it.
func TestRunCompleteClearsSpinner(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})

	m.isThinking = true
	step(runCompleteMsg{})
	if m.isThinking {
		t.Fatal("runCompleteMsg should clear the spinner on a silent completion")
	}
}
