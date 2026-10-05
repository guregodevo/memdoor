package ui

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// TestTypedInputRenders is the stdin.write + lastFrame equivalent (cf. Claude
// Code's ink-testing-library tests): it drives real keystrokes into the running
// program and asserts they render in the input box. Hermetic — an empty gateway
// URL makes Connect fail fast, so no live gateway is needed.
func TestTypedInputRenders(t *testing.T) {
	tm := teatest.NewTestModel(t, NewModel("", "", "", "", nil), teatest.WithInitialTermSize(120, 40))

	// Boot: wait for the first painted frame.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("ctrl+c quit"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	// Type a message and assert it shows up in the input line.
	tm.Type("hello harness")
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("hello harness"))
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(50*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
