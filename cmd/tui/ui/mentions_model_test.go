package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// Typing @ in the box opens the file picker over the working directory,
// tab inserts the chosen path, and a space closes it.
func TestTypingAtOpensTheFilePicker(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "cmd"), 0o755)
	os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "notes.md"), []byte("x"), 0o644)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(root)

	m := NewModel("ws://localhost:0/ws", "demo", "chan", "", func(string, string, string, string) error { return nil })
	m.input.SetValue("fix @ma")
	m.refreshMentions()
	if !m.showFileMentions || len(m.fileMatches) != 1 || m.fileMatches[0] != "cmd/main.go" {
		t.Fatalf("picker: open=%v matches=%v", m.showFileMentions, m.fileMatches)
	}
	m.input.SetValue(insertMention(m.input.Value(), m.mentionStart, m.fileMatches[m.fileIndex]))
	if m.input.Value() != "fix @cmd/main.go " {
		t.Fatalf("inserted: %q", m.input.Value())
	}
	m.refreshMentions()
	if m.showFileMentions {
		t.Fatal("a completed mention closes the picker")
	}
	m.input.SetValue("plain text")
	m.refreshMentions()
	if m.showFileMentions {
		t.Fatal("no @, no picker")
	}
}
