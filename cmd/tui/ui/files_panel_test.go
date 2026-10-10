package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The files panel puts what the conversation touched first, newest first,
// says what happened to each, and the rest of the tree is behind the same
// filter; a patch counts for every file it names.
func TestTheFilesPanelPutsWhatTheTurnTouchedFirst(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"calc.go", "calc_test.go", "README.md", "docs/notes.md"} {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, f), []byte("package calc\n"), 0o644)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	_ = os.Chdir(dir)
	m := &Model{width: 120, height: 40}
	now := time.Now()
	m.messages = []Message{
		{Role: "tool_call", ToolName: "read_file", ToolInput: `{"path":"README.md"}`, Timestamp: now.Add(-3 * time.Second)},
		{Role: "tool_call", ToolName: "apply_patch", ToolInput: `{"input":"*** Begin Patch\n*** Update File: calc.go\n@@\n+x\n*** Add File: calc_test.go\n+y\n*** End Patch"}`, Timestamp: now.Add(-2 * time.Second)},
		{Role: "tool_call", ToolName: "read_file", ToolInput: `{"path":"calc.go"}`, Timestamp: now.Add(-1 * time.Second)},
		{Role: "tool_call", ToolName: "bash", ToolInput: `{"command":"go test ./..."}`, Timestamp: now},
	}
	m.openFilesPanel("")
	p := m.filesPanel
	if p == nil || len(p.matches) < 4 {
		t.Fatalf("panel: %+v", p)
	}
	if p.matches[0].path != "calc.go" || p.matches[0].edits != 1 || p.matches[0].reads != 1 {
		t.Fatalf("the newest touched file first, with its counts: %+v", p.matches[0])
	}
	if p.matches[1].path != "calc_test.go" || p.matches[1].edits != 1 || p.matches[2].path != "README.md" || p.matches[2].reads != 1 {
		t.Fatalf("then the others by recency: %+v %+v", p.matches[1], p.matches[2])
	}
	if p.matches[3].edits != 0 || p.matches[3].reads != 0 {
		t.Fatalf("the rest of the tree follows untouched: %+v", p.matches[3])
	}
	view := m.renderFilesPanel()
	if !strings.Contains(view, "✎ calc.go") || !strings.Contains(view, "1 edit · 1 read") || !strings.Contains(view, "ctrl+d diff") {
		t.Fatalf("the list says what happened to each file:\n%s", view)
	}
	// The filter narrows both halves; tab puts the selection in the prompt.
	p.query = "notes"
	m.filesRefresh()
	if len(p.matches) != 1 || p.matches[0].path != "docs/notes.md" {
		t.Fatalf("filtered: %+v", p.matches)
	}
}
