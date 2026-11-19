package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A first run with no key asks nothing in the terminal: the window opens on
// its /connect list (PageConfig.OfferConnect). Once only, never with a model
// connected, and never in a window that was not asked to.
func TestTheWindowOpensOnConnectWhenAskedAndNoModel(t *testing.T) {
	connect := ConnectOps{Connect: func(ConnectRequest) (ConnectResult, error) { return ConnectResult{}, nil }}
	open := func(offer bool, gate string) Model {
		m := NewPage(PageConfig{Agent: "coder", OfferConnect: offer,
			Ops: Ops{Connect: connect, Status: func() Status { return Status{Gate: gate} }}})
		for _, msg := range []tea.Msg{tea.WindowSizeMsg{Width: 100, Height: 40}, statusTickMsg{}} {
			nm, _ := m.Update(msg)
			m = nm.(Model)
		}
		return m
	}

	m := open(true, "no model yet")
	if m.connect == nil {
		t.Fatal("the /connect list did not open")
	}
	if !strings.Contains(notesOf(m), "No model connected yet: pick one below") {
		t.Fatalf("no line says what the list is for:\n%s", notesOf(m))
	}
	// Esc closes it; the next status read does not open it again.
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	nm, _ = m.Update(statusTickMsg{})
	m = nm.(Model)
	if m.connect != nil {
		t.Fatal("the list came back after Esc")
	}

	if m := open(true, ""); m.connect != nil || strings.Contains(notesOf(m), "pick one below") {
		t.Fatal("a window with a model opened the list")
	}
	if m := open(false, "no model yet"); m.connect != nil {
		t.Fatal("a window that was not asked to opened the list")
	}
}
