package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// A manual harness, not a unit test: drives the REAL program in a REAL
// terminal with synthetic one-token deltas, so the renderer can be watched
// (and tmux-captured) under exactly the load a 44 tok/s stream produces —
// with no model and no gateway. Run inside tmux:
//
//	MEMDOOR_TUI_TEARTEST=1 go test ./cmd/tui/ui/ -run TestTearHarness -v
//
// Skipped everywhere else.
func TestTearHarness(t *testing.T) {
	if os.Getenv("MEMDOOR_TUI_TEARTEST") == "" {
		t.Skip("manual harness; set MEMDOOR_TUI_TEARTEST=1 inside a terminal")
	}

	text := strings.Repeat("The existing google_news.py is incomplete and truncated, so the next step is to write the full program: fetch the Google News RSS feed, parse the items, and summarize each story with a dependency-free extractive algorithm. ", 12)
	// One-token-ish deltas: split keeping leading spaces, the way BPE does.
	var tokens []string
	for _, w := range strings.SplitAfter(text, " ") {
		if len(w) > 4 { // split long words into sub-tokens
			tokens = append(tokens, w[:3], w[3:])
		} else {
			tokens = append(tokens, w)
		}
	}

	m := NewModel("ws://localhost:0/ws", "demo", "chan", "", func(string, string, string, string) error { return nil })
	p := tea.NewProgram(m, tea.WithOutput(NewSyncWriter(os.Stdout))) // what ships: cmd/cli wraps the same way
	go func() {
		time.Sleep(500 * time.Millisecond)
		p.Send(assistantThinkingMsg{})
		for _, tok := range tokens {
			p.Send(assistantStreamingMsg{content: tok})
			time.Sleep(22 * time.Millisecond) // ~45 deltas/s
		}
		p.Send(assistantResponseMsg{content: text})
		time.Sleep(2 * time.Second)
		p.Quit()
	}()
	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}
}
