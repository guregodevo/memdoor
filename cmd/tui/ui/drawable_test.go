package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A running command's output reaches the terminal through the live tail. With
// its control bytes left in, llama-cli's spinner ("|\b-\b\\") and a binary
// file's bytes moved the cursor and redrew over the window.
func TestToolOutputIsDrawnWithoutControlBytes(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"plain text is untouched", "ok\n\tindented — é\n", "ok\n\tindented — é\n"},
		{"spinner backspaces", "Loading model... |\b-\b\\\b \b", "Loading model... |-\\ "},
		{"progress bar", "10%\r20%\r30%\n", "10%\n20%\n30%\n"},
		{"windows line ends", "a\r\nb\r\n", "a\nb\n"},
		{"clear screen and colour", "\x1b[2J\x1b[H\x1b[31mred\x1b[0m", "red"},
		{"alternate screen and mouse mode", "\x1b[?1049h\x1b[?1003hx", "x"},
		{"bell, NUL, DEL and C1", "a\x07b\x00c\x7fd\u009be", "abcde"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := drawable(tc.in); got != tc.want {
				t.Errorf("drawable(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLiveTailCarriesNoControlBytes(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	step(toolCallStartMsg{toolName: "bash", toolInput: `{"command":"llama-cli -m x.gguf"}`})
	step(toolOutputDeltaMsg{toolName: "bash", chunk: "Loading |\b-\b\x1b[2Jdone\x00\n"})

	// ESC is the renderer's own colour; the rest must not reach the terminal.
	screen := m.renderMessages()
	if strings.ContainsAny(screen, "\b\x00\r") || strings.Contains(screen, "\x1b[2J") {
		t.Errorf("the live tail drew control bytes: %q", screen)
	}
	if !strings.Contains(stripANSI(screen), "Loading |-done") {
		t.Errorf("the live tail lost the text:\n%s", stripANSI(screen))
	}
}
