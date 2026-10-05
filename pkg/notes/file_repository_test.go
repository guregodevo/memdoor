package notes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConversationKeyFailsFast(t *testing.T) {
	if _, err := ParseConversationKey("  "); err != ErrNoConversation {
		t.Fatalf("an empty key must be refused, got %v", err)
	}
	if k, err := ParseConversationKey(" workspace:w:channel:c "); err != nil || k != "workspace:w:channel:c" {
		t.Fatalf("got %q %v", k, err)
	}
}

// Each conversation has its own notes, kept under the root and never in a
// project folder; Delete forgets them, and deleting none is not an error.
func TestFileRepositoryKeepsEachConversationApart(t *testing.T) {
	if _, err := NewFileRepository(""); err == nil {
		t.Fatal("a repository needs a folder")
	}
	root := filepath.Join(t.TempDir(), "notes")
	repo, err := NewFileRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	a, b := ConversationKey("workspace:w:channel:a"), ConversationKey("agent:planner:subagent:x/../../y")
	if s, err := repo.Load(a); err != nil || s != "" {
		t.Fatalf("no notes yet: %q %v", s, err)
	}
	if err := repo.Append(a, "one\n"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Append(a, "two\n"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Append(b, "other\n"); err != nil {
		t.Fatal(err)
	}
	if s, _ := repo.Load(a); s != "one\ntwo\n" {
		t.Fatalf("appends in order: %q", s)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 2 {
		t.Fatalf("one file per conversation, all under the root: %v", entries)
	}
	if err := repo.Delete(a); err != nil {
		t.Fatal(err)
	}
	if s, _ := repo.Load(a); s != "" {
		t.Fatalf("deleted notes read back: %q", s)
	}
	if s, _ := repo.Load(b); s != "other\n" {
		t.Fatalf("deleting one conversation touched another: %q", s)
	}
	if err := repo.Delete(a); err != nil {
		t.Fatalf("deleting none is not an error: %v", err)
	}
}
