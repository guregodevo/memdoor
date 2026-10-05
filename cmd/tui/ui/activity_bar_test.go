package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// A running turn always shows the activity bar, even between the states
// that used to gate it (thinking, a running tool, a call being written).
func TestRunningTurnAlwaysHasABar(t *testing.T) {
	m := NewPage(PageConfig{Agent: "coder"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)
	m.turnRunning = true
	m.thinkingStartTime = time.Now().Add(-40 * time.Second)
	m.isThinking = false // streaming cleared it; no tool is running
	if bar := m.renderThinkingBar(); !strings.Contains(bar, "Working") || !strings.Contains(bar, "esc to interrupt") {
		t.Fatalf("a running turn says it is working: %q", bar)
	}
	if view := m.View(); !strings.Contains(view, "Working") {
		t.Fatal("the bar is in the view")
	}
	// The tick keeps going while the turn runs, so the spinner moves.
	next, cmd := m.Update(thinkingTickMsg{})
	if cmd == nil {
		t.Fatal("the tick re-arms for a running turn")
	}
	m = next.(Model)
	m.turnRunning = false
	if bar := m.renderThinkingBar(); bar != "" {
		t.Fatalf("no turn, no bar: %q", bar)
	}
}
