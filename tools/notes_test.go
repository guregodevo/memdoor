package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/pkg/notes"
)

// withNotes wires a notes store in a temporary folder and returns a
// conversation to write under.
func withNotes(t *testing.T) notes.ConversationKey {
	t.Helper()
	repo, err := notes.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	prev := notesRepo
	SetNotesRepository(repo)
	t.Cleanup(func() { SetNotesRepository(prev) })
	return notes.ConversationKey("workspace:w:channel:" + t.Name())
}

func TestNotes_AppendAndRead(t *testing.T) {
	key := withNotes(t)
	call := func(in NotesInput) (string, error) {
		in.Conversation = key.String()
		b, _ := json.Marshal(in)
		return Notes(b)
	}
	if out, err := call(NotesInput{Read: true}); err != nil || !strings.Contains(out, "no notes yet") {
		t.Fatalf("empty read: %q %v", out, err)
	}
	if _, err := call(NotesInput{}); err == nil {
		t.Fatal("append-or-read must be required")
	}
	if out, err := call(NotesInput{Append: "intro #20-26, sleep #27-47"}); err != nil || !strings.Contains(out, "noted") {
		t.Fatalf("append: %q %v", out, err)
	}
	if err := AppendNotes(key, "analysis saved by the editor", "the second intro is the good one"); err != nil {
		t.Fatal(err)
	}
	out, err := call(NotesInput{Read: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## note —", "intro #20-26", "## analysis saved by the editor", "second intro"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

// Notes belong to one conversation: another starts with none, a call with no
// conversation is refused, and DeleteNotes (/fresh, /clear) forgets them
// (2026-09-29: per-folder notes steered every later task in the folder).
func TestNotesBelongToTheConversation(t *testing.T) {
	key := withNotes(t)
	if err := AppendNotes(key, "note", "RULE: work happens on the remote-control branch"); err != nil {
		t.Fatal(err)
	}
	other, _ := json.Marshal(NotesInput{Read: true, Conversation: "workspace:w:channel:another"})
	if out, err := Notes(other); err != nil || !strings.Contains(out, "no notes yet") {
		t.Fatalf("another conversation must start with none: %q %v", out, err)
	}
	none, _ := json.Marshal(NotesInput{Read: true})
	if _, err := Notes(none); err == nil {
		t.Fatal("a call with no conversation must be refused")
	}
	if err := DeleteNotes(key); err != nil {
		t.Fatal(err)
	}
	if s, n := ReadNotes(key); n != 0 {
		t.Fatalf("deleted notes still read back: %q", s)
	}
}

// A rule kept for later is written into the project's instructions file —
// not into notes, which end with the conversation (live 2026-09-29: the
// rule went into notes, and the next conversation ran tests without -race).
func TestARuleIsKeptInTheInstructionsFile(t *testing.T) {
	key := withNotes(t)
	file := filepath.Join(t.TempDir(), "AGENTS.md")
	in, _ := json.Marshal(NotesInput{Rule: "Run tests with `go test -race ./...`.", Conversation: key.String(), RulesFile: file})
	out, err := Notes(in)
	if err != nil || !strings.Contains(out, "kept in "+file) {
		t.Fatalf("%q %v", out, err)
	}
	got, _ := os.ReadFile(file)
	if !strings.Contains(string(got), "- Run tests with `go test -race ./...`.") {
		t.Fatalf("the rule is not in the file:\n%s", got)
	}
	if s, n := ReadNotes(key); n != 0 {
		t.Fatalf("a rule does not go into notes: %q", s)
	}
	noFile, _ := json.Marshal(NotesInput{Rule: "x", Conversation: key.String()})
	if _, err := Notes(noFile); err == nil {
		t.Fatal("with no instructions file the rule is refused, not lost")
	}
}

// The pre-compaction flush runs once per conversation; its entry in the
// conversation's notes is the record that it ran.
func TestNotesHaveEntry(t *testing.T) {
	key := withNotes(t)
	if NotesHaveEntry(key, "Before compaction") {
		t.Fatal("no entry yet")
	}
	if err := AppendNotes(key, "Before compaction", "Nothing new worth keeping at this point."); err != nil {
		t.Fatal(err)
	}
	if !NotesHaveEntry(key, "Before compaction") || NotesHaveEntry(key, "Before") {
		t.Fatal("the entry is found by its whole heading")
	}
}

// A grep flag no longer offered in the schema still works when sent.
func TestGrepStillReadsItsHiddenFlags(t *testing.T) {
	var in GrepInput
	if err := json.Unmarshal([]byte(`{"pattern":"x","-A":2,"type":"go"}`), &in); err != nil || in.A != 2 || in.Type != "go" {
		t.Fatalf("hidden flags parse: %+v %v", in, err)
	}
}
