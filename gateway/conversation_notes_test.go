package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/providers"
	"memdoor/pkg/notes"
	"memdoor/tools"
)

// A turn's notes belong to its conversation: the screen's session, or for a
// sub-session the screen waiting on it, so what a delegated run notes its
// parent reads (skills/documentary.md hands the notes to a sub-session).
func TestNotesBelongToTheScreensConversation(t *testing.T) {
	screen := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	if got := conversationOf(context.WithValue(context.Background(), ctxSession, screen)); got != "workspace:w:channel:c" {
		t.Fatalf("the screen's own conversation, got %q", got)
	}
	sub := &Session{ID: "agent:planner:subagent:run-1", Metadata: map[string]interface{}{
		tools.RequesterSessionKey: "workspace:w:channel:c",
	}}
	if got := conversationOf(context.WithValue(context.Background(), ctxSession, sub)); got != "workspace:w:channel:c" {
		t.Fatalf("a sub-session uses the conversation waiting on it, got %q", got)
	}
	if got := conversationOf(context.Background()); got != "" {
		t.Fatalf("no session, no conversation: %q", got)
	}
}

// The model never chooses whose notes it reads or which file a rule goes
// into: the harness overwrites both.
func TestTheHarnessNamesTheNotesScope(t *testing.T) {
	in := json.RawMessage(`{"read":true,"conversation":"workspace:w:channel:someone-else","rules_file":"/etc/passwd"}`)
	var got map[string]any
	if err := json.Unmarshal(withHarness(in, map[string]any{"conversation": notes.ConversationKey("workspace:w:channel:mine").String(), "rules_file": "/p/AGENTS.md"}), &got); err != nil {
		t.Fatal(err)
	}
	if got["conversation"] != "workspace:w:channel:mine" || got["rules_file"] != "/p/AGENTS.md" || got["read"] != true {
		t.Fatalf("got %v", got)
	}
}

// A kept rule goes into the file the project's instructions are read from:
// AGENTS.md, CLAUDE.md when that is the one there, AGENTS.md when neither is.
func TestARuleGoesWhereTheInstructionsAreRead(t *testing.T) {
	dir := t.TempDir()
	if got := projectRulesFile(dir); got != filepath.Join(dir, ".memdoor", "AGENTS.md") {
		t.Fatalf("no file yet: the rules go under .memdoor, got %s", got)
	}
	// CLAUDE.md is not read (2026-10-04): it is not where the rules go either.
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := projectRulesFile(dir); got != filepath.Join(dir, ".memdoor", "AGENTS.md") {
		t.Fatalf("CLAUDE.md is not the instructions file: %s", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := projectRulesFile(dir); got != filepath.Join(dir, "AGENTS.md") {
		t.Fatalf("the project's own AGENTS.md: %s", got)
	}
	if got := projectRulesFile(""); got != "" {
		t.Fatalf("no project, no file: %s", got)
	}
}

// The runtime the server builds (NewAgentRuntimeWithFactory, server.go) wires
// the notes store under ~/.memdoor/notes. Wired only in a constructor the
// server does not use, every notes call answered "no conversation" and the
// model wrote .agents/notes.md into the project instead (live 2026-09-29).
func TestTheServersRuntimeWiresNotesOutsideTheProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := NewAgentRuntimeWithFactory(providers.NewClientFactory(), "", nil, nil, nil, false, ""); err != nil {
		t.Skipf("runtime unavailable in this environment: %v", err)
	}
	in, _ := json.Marshal(tools.NotesInput{Append: "main.go defines Add", Conversation: "workspace:w:channel:c"})
	if out, err := tools.Notes(in); err != nil || !strings.Contains(out, "noted") {
		t.Fatalf("a note must be kept: %q %v", out, err)
	}
	entries, err := os.ReadDir(filepath.Join(home, ".memdoor", "notes"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("the note lives under ~/.memdoor/notes: %v %v", entries, err)
	}
}

// The runtime the server builds compacts at 60% of the window (Greg,
// 2026-09-29: "60%, capped at 200k"); an earlier default of 45% compacted a
// remote model at 49.8% (live the same day).
func TestTheServersRuntimeCompactsAtSixtyPercent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ar, err := NewAgentRuntimeWithFactory(providers.NewClientFactory(), "", nil, nil, nil, false, "")
	if err != nil || ar.preflight == nil {
		t.Skipf("runtime unavailable in this environment: %v", err)
	}
	at := func(share float64) bool {
		limit := 32_768 - 4_096 // the placeholder window's effective limit
		chars := int(share*float64(limit)/ctxmgmt.TokenScale()) * 4
		r, _ := ar.preflight.Check("", "", "", strings.Repeat("a", chars), nil, nil)
		return r.ShouldCompact
	}
	if at(0.50) {
		t.Fatal("50% of the window must not compact")
	}
	if !at(0.62) {
		t.Fatal("62% of the window must compact")
	}
}
