package ui

import (
	"strings"
	"testing"
	"time"
)

// The turn's frames become a DAG: an edit requires the read of its file, a
// run requires the edits since the previous run, a failed run is red with
// its reason, and the head says whether the last run passed.
func TestTurnGraphFromAnEditingTurn(t *testing.T) {
	now := time.Now()
	msgs := []Message{
		{Role: "user", Content: "fix calc"},
		{Role: "tool_call", ToolName: "read_file", ToolInput: `{"file_path":"calc.go"}`, ToolOutput: "…", ToolDone: true, Timestamp: now},
		{Role: "tool_call", ToolName: "grep", ToolInput: `{"pattern":"Add"}`, ToolOutput: "calc.go:3:func Add", ToolDone: true, Timestamp: now},
		{Role: "tool_call", ToolName: "apply_patch", ToolInput: `{"input":"*** Update File: calc.go\n-a\n+b"}`, ToolOutput: "Patch applied.", ToolDone: true, Timestamp: now},
		{Role: "tool_call", ToolName: "bash", ToolInput: `{"command":"go test ./..."}`, ToolOutput: "Command FAILED (exit status 1): go test\n--- FAIL: TestAdd", ToolDone: true, Timestamp: now},
		{Role: "tool_call", ToolName: "apply_patch", ToolInput: `{"input":"*** Update File: calc.go\n-b\n+c"}`, ToolOutput: "Patch applied.", ToolDone: true, Timestamp: now},
		{Role: "tool_call", ToolName: "bash", ToolInput: `{"command":"go test ./..."}`, ToolOutput: "ok", ToolDone: true, Timestamp: now},
	}
	g := buildTurnGraph(msgs, 100)
	if len(g.nodes) != 6 {
		t.Fatalf("6 frames, %d nodes: %+v", len(g.nodes), g.nodes)
	}
	edit1, run1, edit2, run2 := g.nodes[2].task, g.nodes[3].task, g.nodes[4].task, g.nodes[5].task
	if len(edit1.Requires) != 1 || !strings.Contains(edit1.Requires[0], "grep") {
		t.Errorf("the edit requires the latest look at its file (the grep), got %v", edit1.Requires)
	}
	if run1.State != "failed" || !strings.Contains(run1.Error, "Command FAILED") {
		t.Errorf("the failed run is failed with its reason: %+v", run1)
	}
	if len(run1.Requires) != 1 || run1.Requires[0] != edit1.Name {
		t.Errorf("the first run requires the first edit, got %v", run1.Requires)
	}
	if len(edit2.Requires) != 1 || edit2.Requires[0] != edit1.Name {
		t.Errorf("the second edit requires the first (the last to touch calc.go), got %v", edit2.Requires)
	}
	if len(run2.Requires) != 1 || run2.Requires[0] != edit2.Name || run2.State != "done" {
		t.Errorf("the second run requires the second edit and passed: %+v", run2)
	}
	if !strings.Contains(g.head, "2 reads · 2 edits · 2 runs") || !strings.Contains(g.head, "last run passed") {
		t.Errorf("head = %q", g.head)
	}
	if len(g.layers) < 3 {
		t.Errorf("reads, edits and runs stack in layers, got %d", len(g.layers))
	}
	if g.nodes[2].path != "calc.go" {
		t.Errorf("enter on the edit opens calc.go, got %q", g.nodes[2].path)
	}

	// Drawn: the failed run's reason in the rows, the selected node on the selection pair.
	var m Model
	m.width = 100
	m.messages = msgs
	m.openTurnGraph()
	if m.turnGraph == nil {
		t.Fatal("the graph did not open")
	}
	out := plainText(m.renderTurnGraph())
	for _, want := range []string{"this turn", "✗", "Command FAILED", "← 3 Edit(calc.go)", "go test ./..."} {
		if !strings.Contains(out, want) {
			t.Errorf("graph missing %q:\n%s", want, out)
		}
	}
	// A turn with nothing in it draws nothing and says so.
	var empty Model
	empty.messages = []Message{{Role: "user", Content: "hi"}}
	empty.openTurnGraph()
	if empty.turnGraph != nil {
		t.Error("an empty turn opens no graph")
	}
}
