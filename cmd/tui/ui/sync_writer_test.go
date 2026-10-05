package ui

import (
	"io"
	"os"
	"testing"
)

// The test last night's fix needed and did not have. Bubble Tea decides
// whether it CAN query the terminal size with one type assertion on its
// output — io.ReadWriteCloser plus Fd() (charmbracelet/x/term.File). The first
// sync writer implemented only Write, so the assertion failed, ttyOutput
// stayed nil, no WindowSizeMsg was ever sent, ready never flipped, and the
// TUI sat on "Initializing..." forever. The 244-byte capture taken as a
// receipt that night WAS this symptom: the brackets were verified, the
// working UI was not.
func TestTheSyncWriterIsStillATerminalToBubbleTea(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "tty")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := NewSyncWriter(f)
	tf, ok := w.(interface {
		io.Reader
		io.Writer
		io.Closer
		Fd() uintptr
	})
	if !ok {
		t.Fatal("the sync writer does not satisfy term.File — bubbletea cannot query the window size through it, and the TUI never leaves Initializing")
	}
	if tf.Fd() != f.Fd() {
		t.Errorf("Fd() = %d, want the underlying terminal's %d", tf.Fd(), f.Fd())
	}
}
