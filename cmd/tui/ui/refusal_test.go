package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A refused window says why and what to type, keeps saying it when the person
// types, and queues nothing: no reconnect will ever flush it.
func TestRefusedWindowGuidesInsteadOfQueuing(t *testing.T) {
	var posted []string
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error {
		posted = append(posted, text)
		return nil
	})
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketRefusedMsg{reason: "not your workspace (yours is mine)"})
	// The dial's own "connected" can land after the refusal closed it.
	step(websocketConnectedMsg{})
	if m.connected {
		t.Fatal("a refused window reads as connected")
	}
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	step(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.queued) != 0 || len(posted) != 0 {
		t.Fatalf("queued %v, posted %v: want neither", m.queued, posted)
	}
	notes := 0
	for _, msg := range m.messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "refused this window: not your workspace (yours is mine)") {
			if !strings.Contains(msg.Content, "Start memdoor again") {
				t.Fatalf("note does not say what to do: %q", msg.Content)
			}
			notes++
		}
		if strings.Contains(msg.Content, "reconnect") {
			t.Fatalf("a refusal talks about reconnecting: %q", msg.Content)
		}
	}
	if notes != 2 {
		t.Fatalf("refusal said %d times, want on refusal and on the typed prompt", notes)
	}
}
