package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// inColour draws with the 256-colour profile a terminal has, so a test can
// read which hue a word wears; tests run with no terminal and get none.
func inColour(t *testing.T) {
	t.Helper()
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(was) })
}

// fg is the SGR a word in this palette colour starts with.
func fg(c string) string { return "\x1b[38;5;" + c + "m" }

// #132: each dropdown entry says what it does, in grey on its right: a
// built-in the line /help prints, a workflow the description `memdoor
// workflow` prints, a skill its own heading. /help prints the same table.
func TestTheDropdownSaysWhatEachCommandDoes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	SetSessionDir(dir)
	t.Cleanup(func() { SetSessionDir("") })
	wf := filepath.Join(dir, ".memdoor", "workflows", "digest")
	os.MkdirAll(filepath.Join(wf, "steps"), 0o755)
	os.WriteFile(filepath.Join(wf, "steps", "a.yaml"), []byte("type: agent\nprompt: x\n"), 0o644)
	os.WriteFile(filepath.Join(wf, "README.md"), []byte("# digest\n\nSummarizes the last ten commits into DIGEST.md.\n"), 0o644)

	cmds := loadSlashCommands("coder")
	for _, c := range []struct{ entry, says string }{
		{"/w", "The workflows and their runs"},
		{"/usage", "What this workspace has used this month"},
		{"/w digest", "Summarizes the last ten commits into DIGEST.md."},
		{"/skill:workflow", "build a DAG of agent tasks"},
	} {
		if !strings.Contains(slashDescriptions[c.entry], c.says) {
			t.Errorf("%s says %q, want %q", c.entry, slashDescriptions[c.entry], c.says)
		}
	}
	if len(cmds) == 0 {
		t.Fatal("no commands")
	}

	inColour(t)
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.width = 120
	m.filteredCommands = []string{"/usage", "/w digest"}
	box := m.renderAutocomplete()
	if !strings.Contains(plainText(box), "/w digest") || !strings.Contains(plainText(box), "Summarizes the last ten commits") {
		t.Fatalf("the box:\n%s", plainText(box))
	}
	if !strings.Contains(box, fg(colDim)+"Summarizes") && !strings.Contains(box, fg(colDim)+"What this") {
		t.Fatalf("the description is not grey:\n%q", box)
	}

	help := slashHelpText()
	for _, want := range []string{"- /w - The workflows and their runs", "- /usage - What this workspace has used this month", "Shift+Tab - The mode: auto (one turn) or team"} {
		if !strings.Contains(help, want) {
			t.Errorf("/help misses %q", want)
		}
	}
	if strings.Contains(help, "Cycle the reasoning effort") {
		t.Error("/help still says Shift+Tab cycles the effort")
	}
}

// #133: the state wears its glyph's hue, stopped is no longer grey, and a
// step that ran or runs is brighter than one waiting (ADR 0018).
func TestTheWorkflowGraphWearsThePalette(t *testing.T) {
	inColour(t)
	if _, c := workflowGlyph("stopped"); c != colWarn {
		t.Fatalf("stopped wears %s, want the warning hue %s", c, colWarn)
	}
	now := time.Now()
	r := WorkflowRun{ID: "r1", Workflow: "release", State: "failed", Done: 1, Total: 4, Tasks: []WorkflowTask{
		{Name: "tests", State: "done", Took: "2s"},
		{Name: "notes", State: "failed", Requires: []string{"tests"}, Error: "exit status 1"},
		{Name: "tag", State: "stopped", Requires: []string{"tests"}},
		{Name: "announce", State: "waiting", Requires: []string{"notes"}, Started: &now},
	}}
	out := strings.Join(workflowGraphRows(r, 100, false), "\n")
	for _, c := range []struct{ word, hue string }{{"done", colOK}, {"failed", colErr}, {"stopped", colWarn}} {
		if !strings.Contains(out, fg(c.hue)+c.word) {
			t.Errorf("%q is not in its hue %s:\n%q", c.word, c.hue, out)
		}
	}
	if !strings.Contains(out, fg(colText)+"tests") || !strings.Contains(out, fg(colDim)+"announce") {
		t.Errorf("a step that ran is bright and one waiting dim:\n%q", out)
	}
	head := workflowGraphMessage(r, 100, false).Content
	if !strings.Contains(head, fg(colErr)+"failed") {
		t.Errorf("the header's state is not in its hue:\n%q", head)
	}
}

// #134: a graph of more than 8 steps is grouped by state in the
// conversation, a few lines whatever its size; ctrl+o draws every step.
func TestALongGraphFitsTheWindow(t *testing.T) {
	started := time.Now().Add(-12 * time.Second)
	var tasks []WorkflowTask
	for i, n := range []string{"problem", "outline", "plan", "settle", "baseline", "direct", "test-0", "context", "ideate-a", "ideate-b", "solve", "test", "review", "repair", "recheck", "judge", "keep", "verify"} {
		t := WorkflowTask{Name: n, State: "waiting"}
		if i > 0 {
			t.Requires = []string{"problem"}
		}
		switch {
		case i < 6:
			t.State = "done"
		case i == 6:
			t.State, t.Started = "running", &started
			t.Lines = []stepLine{{text: "⏺ bash · 4s"}}
		case i == 7:
			t.State, t.Error = "failed", "exit status 1 — no prompt"
		}
		tasks = append(tasks, t)
	}
	r := WorkflowRun{ID: "r1", Workflow: "team-solve", State: "running", Done: 6, Total: 18, Tasks: tasks}

	full := workflowGraphMessage(r, 120, true).Content
	compact := workflowGraphMessage(r, 120, false).Content
	nFull, nCompact := strings.Count(full, "\n")+1, strings.Count(compact, "\n")+1
	if nCompact > 10 || nFull < 20 {
		t.Fatalf("compact %d lines, full %d:\n%s", nCompact, nFull, plainText(compact))
	}
	plain := plainText(compact)
	for _, want := range []string{"✓✓✓✓✓✓▶✗○", "6 done", "problem, outline", "running", "test-0 12s", "test-0: ⏺ bash · 4s", "failed", "context — exit status 1 — no prompt", "10 waiting", "ctrl+o shows every step (18)"} {
		if !strings.Contains(plain, want) {
			t.Errorf("compact misses %q:\n%s", want, plain)
		}
	}
	small := WorkflowRun{ID: "r2", Workflow: "release", State: "running", Total: 3, Tasks: tasks[:3]}
	if strings.Contains(plainText(workflowGraphMessage(small, 120, false).Content), "ctrl+o shows every step") {
		t.Error("a short graph is drawn in full")
	}
}
