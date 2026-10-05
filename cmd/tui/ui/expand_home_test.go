package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// A path typed into the TUI never passes through a shell, so "~/clips/a.mp4"
// reaches os.Stat with a literal tilde and fails as "no such file" for a file
// that plainly exists (live 2026-09-02).
func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := expandHome("~/memdoor-clip/body.mp4"); got != filepath.Join(home, "memdoor-clip/body.mp4") {
		t.Errorf("expandHome = %q", got)
	}
	if got := expandHome("~"); got != home {
		t.Errorf("bare ~ = %q, want %q", got, home)
	}
	for _, p := range []string{"body.mp4", "/tmp/x.mp4", "./a/b.mp4", "~notauser/x"} {
		if got := expandHome(p); got != p {
			t.Errorf("expandHome(%q) = %q — must be unchanged", p, got)
		}
	}
}
