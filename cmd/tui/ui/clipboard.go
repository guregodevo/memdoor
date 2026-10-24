package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
)

// copyText puts text where the person can paste it, and says where. A
// desktop's clipboard first; a box with none (ssh, Cloud Shell, 2026-10-24:
// "No clipboard utilities available. Please install xsel, xclip…") gets the
// text sent to the terminal's own clipboard with the OSC 52 escape, which
// most terminals honour across ssh, and written to a file named in the
// note, so nothing is lost either way.
func copyText(text string, lines int) string {
	if err := clipboard.WriteAll(text); err == nil {
		return fmt.Sprintf("Copied to the clipboard (%d lines).", lines)
	}
	_, _ = os.Stdout.WriteString(osc52(text))
	path := copyFile()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return fmt.Sprintf("No clipboard on this machine: sent to your terminal's clipboard, if it takes OSC 52 (%d lines).", lines)
	}
	return fmt.Sprintf("No clipboard on this machine: sent to your terminal's clipboard, if it takes OSC 52, and saved to %s (%d lines).", path, lines)
}

// osc52 is the escape that asks the terminal to set its clipboard: the
// text base64-encoded between OSC 52 ; c ; and BEL. Under tmux it is
// wrapped so that tmux passes it through to the outer terminal.
func osc52(text string) string {
	seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07"
	if os.Getenv("TMUX") != "" {
		return "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
	}
	return seq
}

// copyFile is where /copy writes when there is no clipboard.
func copyFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "memdoor-copy.txt")
	}
	return filepath.Join(home, ".memdoor", "copy.txt")
}
