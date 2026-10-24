package ui

import (
	"encoding/base64"
	"strings"
	"testing"
)

// A box with no clipboard still hands the text to the terminal: the OSC 52
// escape carries it base64-encoded, wrapped for tmux when inside one.
func TestCopyWithoutAClipboardSpeaksOSC52(t *testing.T) {
	t.Setenv("TMUX", "")
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("hello\nworld")) + "\x07"
	if got := osc52("hello\nworld"); got != want {
		t.Fatalf("osc52: %q", got)
	}
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")
	got := osc52("x")
	if !strings.HasPrefix(got, "\x1bPtmux;\x1b\x1b]52;c;") || !strings.HasSuffix(got, "\x1b\\") {
		t.Fatalf("wrapped for tmux: %q", got)
	}
	t.Setenv("HOME", t.TempDir())
	if p := copyFile(); !strings.HasSuffix(p, "/.memdoor/copy.txt") {
		t.Fatalf("the file is under ~/.memdoor: %q", p)
	}
}
