package ui

import (
	"strings"
	"testing"

	"memdoor/tools"
)

// The renderer parses what the tool actually emits — FormatTodos is the source
// of that string, so drive the test from it rather than from a copy that can
// drift out of sync.
func todoOutput(t *testing.T) string {
	t.Helper()
	return tools.FormatTodos([]tools.TodoItem{
		{Content: "Read sq.py", Status: "completed"},
		{Content: "Fix the bug", Status: "in_progress", ActiveForm: "Fixing the bug"},
		{Content: "Run it", Status: "pending"},
	})
}

func TestChecklistRendersStates(t *testing.T) {
	body := plainText(renderChecklist(todoOutput(t), false))
	if body == "" {
		t.Fatal("todo output rendered nothing")
	}
	for _, want := range []string{
		"Checklist", "1/3",
		glyphDone + " Read sq.py",
		glyphActive + " Fix the bug",
		glyphPending + " Run it",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("checklist missing %q:\n%s", want, body)
		}
	}
	// The follow-up names the next action and the agent is told not to stop
	// while one exists — it must survive rendering.
	if !strings.Contains(body, "→") || !strings.Contains(body, "Fix the bug") {
		t.Errorf("checklist dropped the follow-up:\n%s", body)
	}
	// The raw "1. [x]" scaffolding is replaced, not repeated.
	if strings.Contains(body, "[x]") || strings.Contains(body, "[~]") {
		t.Errorf("checklist still shows raw markers:\n%s", body)
	}
}

func TestChecklistProgressBar(t *testing.T) {
	all := tools.FormatTodos([]tools.TodoItem{
		{Content: "a", Status: "completed"},
		{Content: "b", Status: "completed"},
	})
	body := plainText(renderChecklist(all, false))
	if !strings.Contains(body, "2/2") {
		t.Errorf("finished list should read 2/2:\n%s", body)
	}
	if strings.Contains(body, "▱") {
		t.Errorf("a finished list should have no empty bar segments:\n%s", body)
	}
}

// Anything that isn't a checklist falls through to the generic body.
func TestChecklistIgnoresOtherOutput(t *testing.T) {
	for _, out := range []string{"", "No task list yet — create one with todo_write.", "some tool output"} {
		if got := renderChecklist(out, false); got != "" {
			t.Errorf("renderChecklist(%q) = %q, want empty", out, got)
		}
	}
}

// The tool frame reaches the checklist through the view, for both todo tools.
func TestPlanViewUsesChecklist(t *testing.T) {
	for _, tool := range []string{"todo_write", "todo_read"} {
		body := plainText(viewFor(tool).Body(ToolRender{Output: todoOutput(t), Width: 100}))
		if !strings.Contains(body, "Checklist") {
			t.Errorf("%s did not render a checklist:\n%s", tool, body)
		}
	}
}

// A long list is capped and says so.
func TestChecklistTruncates(t *testing.T) {
	var items []tools.TodoItem
	for i := 0; i < 30; i++ {
		items = append(items, tools.TodoItem{Content: "step", Status: "pending"})
	}
	body := plainText(renderChecklist(tools.FormatTodos(items), false))
	if !strings.Contains(body, "more (ctrl+o)") {
		t.Errorf("capped checklist does not announce the cap:\n%s", body)
	}
	if strings.Contains(plainText(renderChecklist(tools.FormatTodos(items), true)), "more (ctrl+o)") {
		t.Error("expanded checklist still truncates")
	}
}
