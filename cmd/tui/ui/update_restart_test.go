package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func notesOf(m Model) string {
	var b strings.Builder
	for _, msg := range m.messages {
		b.WriteString(msg.Content)
		b.WriteString("\n")
	}
	return b.String()
}

// /update stops the gateway to replace its binary: the window says the
// gateway restarts on the new build, not that the connection was lost.
func TestUpdateSaysTheGatewayRestartsNotLost(t *testing.T) {
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error { return nil })
	m.updateOp = func() (string, error) { return "Installed v9.9.9.", nil }
	step := func(msg tea.Msg) { nm, _ := m.Update(msg); m = nm.(Model) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	typeAndEnter(step, "/update")
	if !m.updating {
		t.Fatal("/update must mark the window as updating")
	}
	step(websocketDisconnectedMsg{})
	step(websocketConnectedMsg{})
	notes := notesOf(m)
	if strings.Contains(notes, "connection lost") || strings.Contains(notes, "events sent while offline were missed") {
		t.Fatalf("an update's restart read as a lost gateway:\n%s", notes)
	}
	if !strings.Contains(notes, "the gateway restarts on the new build") || !strings.Contains(notes, "the gateway is up on the new build") {
		t.Fatalf("the restart is not said:\n%s", notes)
	}
	step(updateResultMsg{summary: "Installed v9.9.9."})
	if m.updating || !m.restartAfterUpdate {
		t.Fatalf("after the installer: updating=%v restart=%v", m.updating, m.restartAfterUpdate)
	}
}

// Outside an update a drop is still said loudly.
func TestADropOutsideAnUpdateIsStillLoud(t *testing.T) {
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error { return nil })
	step := func(msg tea.Msg) { nm, _ := m.Update(msg); m = nm.(Model) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	step(websocketDisconnectedMsg{})
	if !strings.Contains(notesOf(m), "gateway connection lost") {
		t.Fatalf("a drop must be said:\n%s", notesOf(m))
	}
}

// The model that answered is named in the footer, not in a line under the
// answer.
func TestTheAnsweringModelIsInTheFooterNotUnderTheAnswer(t *testing.T) {
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error { return nil })
	step := func(msg tea.Msg) { nm, _ := m.Update(msg); m = nm.(Model) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	step(runCompleteMsg{model: "z-ai/glm-5.3-flash"})
	if strings.Contains(notesOf(m), "answered by") {
		t.Fatalf("a line under the answer:\n%s", notesOf(m))
	}
	if m.status.Model != "z-ai/glm-5.3-flash" {
		t.Fatalf("footer model = %q", m.status.Model)
	}
}
