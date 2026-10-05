package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// When a turn dies, the gateway broadcasts execution.failed with the error —
// and the TUI had no handler for it. Measured live 2026-08-31 15:15: the
// stream died mid-call ("stream read: unexpected EOF"), the gateway said so
// loudly, and the screen showed "Writing a tool call… (3m 22s)" climbing
// forever with no error anywhere. The reader's session was over and nothing
// told them.
func TestAFailedExecutionShowsTheErrorAndClearsTheRun(t *testing.T) {
	var m tea.Model = NewModel("ws://localhost:0/ws", "w", "c", "", func(string, string, string, string) error { return nil })
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m, _ = m.Update(assistantThinkingMsg{})
	m, _ = m.Update(assistantStreamingMsg{content: `{"name":"apply_patch","arg`})
	if got := m.(Model).pendingCall; got == "" {
		t.Fatal("fixture broken: no call held")
	}

	m, _ = m.Update(executionFailedMsg{errText: "agent execution failed: stream read: unexpected EOF"})

	mm := m.(Model)
	if mm.pendingCall != "" {
		t.Errorf("pendingCall = %q after the run failed — the bar counts a dead run", mm.pendingCall)
	}
	if mm.isThinking {
		t.Error("still thinking after the run failed")
	}
	view := mm.renderMessages()
	if !strings.Contains(view, "unexpected EOF") {
		t.Errorf("the error is not in the transcript — the reader is never told:\n%s", view)
	}
	_ = time.Now
}
