package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A note taller than the space left in a small terminal is shown to its
// last line when the reader follows the bottom.
func TestATallNoteIsShownToItsLastLineInASmallTerminal(t *testing.T) {
	m := NewPage(PageConfig{Agent: "coder"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	m.messages = append(m.messages, Message{Role: "user", Content: "/model"})
	ladder := "Ladder for **coder** — one rung per conversation, held until it ends:\n" +
		"→ 1  z-ai/glm-5.3-flash   ← this conversation · first rung · held for this session\n" +
		"  2  deepseek/deepseek-v4.1-flash\n  3  z-ai/glm-5.3\n  4  (none)\n" +
		"/model <n> pins a model for this conversation · /model auto lets it go back to the first."
	next, _ = m.Update(routeResultMsg{summary: ladder})
	m = next.(Model)
	view := m.View()
	if !strings.Contains(view, "4  (none)") || !strings.Contains(view, "/model auto") {
		t.Fatalf("the ladder's last lines are on screen:\n%s", view)
	}
}
