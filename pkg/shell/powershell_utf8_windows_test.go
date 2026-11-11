//go:build windows

package shell

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Windows PowerShell 5.1 itself: a file written with > is UTF-8, not UTF-16.
func TestWindowsPowerShellWritesUTF8WithRedirection(t *testing.T) {
	dir := t.TempDir()
	ps := choice{name: PowerShell, prog: "powershell", args: []string{"-NoProfile", "-NonInteractive", "-Command"}}
	cmd := ps.command(context.Background(), "echo hello > notes.txt; 'héllo' | Out-File other.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, name := range []string{"notes.txt", "other.txt"} {
		b, _ := os.ReadFile(filepath.Join(dir, name))
		t.Logf("%s: % x", name, b)
		if bytes.HasPrefix(b, []byte{0xff, 0xfe}) || bytes.Contains(b, []byte{'h', 0}) {
			t.Errorf("%s is UTF-16: % x", name, b)
		}
		body := bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
		if !bytes.HasPrefix(body, []byte("h")) {
			t.Errorf("%s does not start with its text: % x", name, b)
		}
	}
}
