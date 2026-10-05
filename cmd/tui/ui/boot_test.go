package ui

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// TestBootRendersFirstFrame is the ink-testing-library-style smoke test: it runs
// the REAL program (Init → first View) in a simulated terminal and asserts the
// input prompt renders. This exercises the program loop end-to-end and would
// catch a regression where the program never paints a frame. An empty gateway
// URL makes Connect fail fast, so the test is hermetic (no live gateway).
func TestBootRendersFirstFrame(t *testing.T) {
	tm := teatest.NewTestModel(t, NewModel("", "", "", "", nil), teatest.WithInitialTermSize(120, 40))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Type your message")) || bytes.Contains(b, []byte("ctrl+c quit"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
