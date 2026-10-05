package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The title is the only thing a multiplexer's tab, a dock icon and a window
// list can all read, so it carries the one fact they need: is this running?
// Both paths, because a title stuck on "running" is worse than none (Greg,
// 2026-09-27: "is running state for cmux terminal so that the tab show it is
// running", "test positive and negative paths").
func TestTheTitleSaysWhetherATurnIsRunning(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "myapp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	idle := Model{}
	if got := idle.windowTitle(); got != "myapp" {
		t.Errorf("idle title is the directory, got %q", got)
	}

	running := Model{turnRunning: true}
	if got := running.windowTitle(); got != "◐ myapp" {
		t.Errorf("a running turn must be visible in the tab, got %q", got)
	}
	// Every way a turn is busy counts, or the tab lies during the gaps.
	for name, m := range map[string]Model{
		"thinking":  {isThinking: true},
		"tool":      {activeTools: map[string]time.Time{"bash": time.Now()}},
		"half call": {pendingCall: "apply_patch"},
	} {
		if got := m.windowTitle(); got != "◐ myapp" {
			t.Errorf("%s must read as running, got %q", name, got)
		}
	}

	// The circle turns while the turn runs, on its own clock, and the clock
	// stops with the turn (Greg: "it should be a spinning circle").
	spinning := Model{turnRunning: true}
	if (&spinning).spin(nil) == nil {
		t.Fatal("a running turn must start the title's clock")
	}
	if (&spinning).spin(nil) != nil {
		t.Error("one clock at a time")
	}
	(&spinning).spin(titleSpinMsg{})
	if got := spinning.windowTitle(); got != "◓ myapp" {
		t.Errorf("a tick must turn the circle, got %q", got)
	}
	spinning.turnRunning = false
	if (&spinning).spin(titleSpinMsg{}) != nil {
		t.Error("the clock must stop when the turn ends")
	}

	asking := Model{turnRunning: true, pendingQuestion: &pendingQuestion{}}
	if got := asking.windowTitle(); got != "? myapp" {
		t.Errorf("a turn waiting on an answer is not the same as working, got %q", got)
	}

	// NEGATIVE: the title is written only when it changes, so a tick that
	// changes nothing does not make a terminal blink.
	m := Model{turnRunning: true}
	if cmd := (&m).titleCmd(); cmd == nil {
		t.Fatal("the first title must be set")
	}
	if cmd := (&m).titleCmd(); cmd != nil {
		t.Error("an unchanged title must not be rewritten")
	}
	m.turnRunning = false
	if cmd := (&m).titleCmd(); cmd == nil {
		t.Error("the turn ending must clear the running mark")
	}
}

// THE END OF A TURN IS A SIGNAL (Greg, 2026-09-27: "signal the end of a turn
// from tab"). A ✓ stays in the title until the person is back; the terminal is
// rung only when it told us the window is out of focus. Both directions, since
// a window that beeps at the person looking at it is worse than one that is
// silent.
func TestTheEndOfATurnIsSignalledUntilSeen(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "") // the plain-terminal path; cmux_test.go has the other
	dir := filepath.Join(t.TempDir(), "myapp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	page := func(blurred bool) Model {
		m := NewPage(PageConfig{GatewayAddr: "http://x", Workspace: "w", ChannelID: "c", Agent: "coder"})
		m.turnRunning, m.blurred = true, blurred
		return m
	}
	finish := func(m Model) (Model, bool) {
		next, _ := m.Update(runCompleteMsg{model: "z-ai/glm-5.3-flash"})
		nm := next.(Model)
		return nm, nm.doneUnseen
	}

	// Looking at it: the ✓, and no ring.
	m, done := finish(page(false))
	if !done || m.windowTitle() != "✓ myapp" {
		t.Fatalf("a finished turn must mark the tab, got %q", m.windowTitle())
	}
	if (&m).turnEnded() != nil {
		t.Error("the window in focus must not ring")
	}

	// Away from it: the ✓ AND a ring.
	away := page(true)
	if (&away).turnEnded() == nil {
		t.Error("a turn that ends while the window is out of focus must ring")
	}

	// The ✓ clears when the person is back: a key...
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if got := next.(Model).windowTitle(); got != "myapp" {
		t.Errorf("a key press means the person is back, title %q", got)
	}
	// ...or the window regaining focus.
	m2, _ := finish(page(true))
	next, _ = m2.Update(tea.FocusMsg{})
	if got := next.(Model).windowTitle(); got != "myapp" {
		t.Errorf("focus coming back means the person saw it, title %q", got)
	}

	// A turn that stops to ask a question is not finished: its own "?" says so.
	asking := page(false)
	asking.pendingQuestion = &pendingQuestion{}
	next, _ = asking.Update(runCompleteMsg{})
	if next.(Model).doneUnseen {
		t.Error("a question waiting for an answer is not a finished turn")
	}

	// Idle to idle is not the end of anything.
	idle := NewPage(PageConfig{GatewayAddr: "http://x", Workspace: "w", ChannelID: "c", Agent: "coder"})
	next, _ = idle.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if next.(Model).doneUnseen {
		t.Error("nothing ran, so nothing finished")
	}
}
