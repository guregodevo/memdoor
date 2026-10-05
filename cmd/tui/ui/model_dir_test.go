package ui

import (
	"os"
	"strings"
	"testing"
)

// /dir re-roots the session: the process directory changes (the poster sends
// it with every turn), a bad path changes nothing, and a bare /dir reports
// where the session is.
func TestSlashDir(t *testing.T) {
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	target := t.TempDir()

	m := Model{}
	if _, handled := m.handleSlashCommand("/dir " + target); !handled {
		t.Fatal("/dir not handled")
	}
	if cwd, _ := os.Getwd(); cwd != target && !strings.HasSuffix(cwd, strings.TrimPrefix(target, "/private")) {
		t.Errorf("cwd = %q, want %q", cwd, target)
	}

	m.handleSlashCommand("/dir /definitely/not/a/dir")
	if cwd, _ := os.Getwd(); !strings.HasSuffix(cwd, strings.TrimPrefix(target, "/private")) {
		t.Errorf("a bad /dir moved the session to %q", cwd)
	}
	last := m.messages[len(m.messages)-1].Content
	if !strings.Contains(last, "not/a/dir") {
		t.Errorf("bad /dir should name the path in its error, got %q", last)
	}

	m.handleSlashCommand("/dir")
	last = m.messages[len(m.messages)-1].Content
	if !strings.Contains(last, strings.TrimPrefix(target, "/private")) {
		t.Errorf("bare /dir should report the current directory, got %q", last)
	}
}
