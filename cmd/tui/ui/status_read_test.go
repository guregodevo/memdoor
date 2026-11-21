package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// statusRead is one status tick as the program runs it: the tick, then the
// read it started, fed back. The command returned is the next tick.
func statusRead(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	m = nm.(Model)
	if cmd == nil {
		return m, nil
	}
	r, ok := findStatusRead(cmd())
	if !ok {
		return m, cmd
	}
	nm, cmd = m.Update(r)
	return nm.(Model), cmd
}

// findStatusRead is the status read among a command's messages; the window
// title and cmux commands batched beside it answer at once.
func findStatusRead(msg tea.Msg) (statusReadMsg, bool) {
	if r, ok := msg.(statusReadMsg); ok {
		return r, true
	}
	if b, ok := msg.(tea.BatchMsg); ok {
		for _, c := range b {
			if c == nil {
				continue
			}
			if r, ok := findStatusRead(c()); ok {
				return r, true
			}
		}
	}
	return statusReadMsg{}, false
}

// A gateway slow to answer the status read must not freeze the window: the
// read runs off the event loop, and ctrl+c quits while it is out
// (2026-11-21: /api/route took 15 s on a model list that never answered, and
// ctrl+c and Esc did nothing). Mutation check: read the status inside Update
// again and this times out.
func TestASlowStatusReadLeavesTheKeysWorking(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	m := NewPage(PageConfig{Agent: "coder", Ops: Ops{Status: func() Status {
		<-release
		return Status{}
	}}})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = nm.(Model)

	done := make(chan Model, 1)
	go func() {
		nm, _ := m.Update(statusTickMsg{})
		done <- nm.(Model)
	}()
	select {
	case m = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the status tick waited on the gateway inside Update")
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c returned no command while the status read was out")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit while the status read was out")
	}
}
