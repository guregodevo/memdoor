package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Manual harness for the STRANDED BAR LINE: a stale copy of the transient
// activity bar left one row above the live one (seen live 2026-08-31 12:39,
// "9m 0s" above "9m 13s"). The bar must be visible WHILE settled messages
// flush to scrollback, so this alternates prose (which settles and flushes)
// with half-open JSON objects (which light the held-call bar) and animation
// ticks. Run inside tmux, then scan the FULL pane history for bar-line
// duplicates:
//
//	MEMDOOR_TUI_STRANDTEST=1 go test ./cmd/tui/ui/ -run TestStrandHarness -v
func TestStrandHarness(t *testing.T) {
	if os.Getenv("MEMDOOR_TUI_STRANDTEST") == "" {
		t.Skip("manual harness; set MEMDOOR_TUI_STRANDTEST=1 inside a terminal")
	}
	m := NewModel("ws://localhost:0/ws", "demo", "chan", "", func(string, string, string, string) error { return nil })
	p := tea.NewProgram(m, tea.WithOutput(NewSyncWriter(os.Stdout)))
	go func() {
		time.Sleep(400 * time.Millisecond)
		for round := 0; round < 30; round++ {
			p.Send(assistantThinkingMsg{})
			// Prose long enough that earlier rounds settle and flush.
			for w := 0; w < 12; w++ {
				p.Send(assistantStreamingMsg{content: fmt.Sprintf("round %02d word %02d ", round, w)})
				time.Sleep(15 * time.Millisecond)
			}
			// Half-open object: lights "Writing a … call", holds ~700ms of ticks.
			p.Send(assistantStreamingMsg{content: `{"name":"bash","arguments":{"command":"ls`})
			for i := 0; i < 7; i++ {
				p.Send(thinkingTickMsg(time.Now()))
				time.Sleep(100 * time.Millisecond)
			}
			p.Send(assistantStreamingMsg{content: ` -la"}}`})
			// Tool frames separate the rounds' messages the way real turns do —
			// without them every round's response replaces the last and nothing
			// ever settles past the budget.
			p.Send(toolCallStartMsg{toolID: fmt.Sprintf("t%d", round), toolName: "bash", toolInput: `{"command":"ls"}`})
			p.Send(toolCallCompleteMsg{toolID: fmt.Sprintf("t%d", round), toolName: "bash", toolOutput: fmt.Sprintf("out %02d", round)})
			p.Send(assistantResponseMsg{content: strings.Repeat(fmt.Sprintf("settled reply %02d. ", round), 3)})
		}
		time.Sleep(1 * time.Second)
		p.Quit()
	}()
	final, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}
	fm := final.(Model)
	t.Logf("final: messages=%d printedThrough=%d height=%d", len(fm.messages), fm.printedThrough, fm.height)
	if fm.printedThrough == 0 {
		t.Errorf("30 settled rounds and NOTHING was flushed to scrollback — printedThrough=0 (height=%d, messages=%d)", fm.height, len(fm.messages))
	}
}
