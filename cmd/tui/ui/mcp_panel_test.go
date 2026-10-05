package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMCPAddBoxTypesOneSpacePerSpaceKey(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.mcpPanel = &mcpPanelState{screen: mcpAdd}
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("npx")},
		{Type: tea.KeySpace, Runes: []rune(" ")},
		{Type: tea.KeyRunes, Runes: []rune("-y")},
	} {
		m.mcpKey(k)
	}
	if got := m.mcpPanel.input; got != "npx -y" {
		t.Fatalf("add box = %q, want %q", got, "npx -y")
	}
}
