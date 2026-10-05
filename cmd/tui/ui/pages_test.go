package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Commands a page returns come back to that page; Bubble Tea's own
// messages pass through untagged.
func TestTagCmd(t *testing.T) {
	got := tagCmd(1, func() tea.Msg { return statusTickMsg{} })()
	pm, ok := got.(pageMsg)
	if !ok || pm.page != 1 {
		t.Fatalf("our message is tagged: %#v", got)
	}
	if _, ok := tagCmd(1, tea.Quit)().(tea.QuitMsg); !ok {
		t.Fatal("quit passes through")
	}
	b, ok := tagCmd(2, tea.Batch(func() tea.Msg { return statusTickMsg{} }, func() tea.Msg { return thinkingTickMsg{} }))().(tea.BatchMsg)
	if !ok {
		t.Fatal("a batch stays a batch")
	}
	for _, c := range b {
		if c == nil {
			continue
		}
		if pm, ok := c().(pageMsg); !ok || pm.page != 2 {
			t.Fatalf("each batched command is tagged: %#v", c())
		}
	}
	if tagCmd(0, nil) != nil {
		t.Fatal("nil stays nil")
	}
}
