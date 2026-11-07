package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A step's About is the first sentence of the comment its file opens with;
// a task a sibling file defines (ideate.yaml for ideate-a) reads that file.
func TestAStepSaysWhatItDoes(t *testing.T) {
	dir := t.TempDir()
	steps := filepath.Join(dir, "steps")
	os.MkdirAll(steps, 0o755)
	os.WriteFile(filepath.Join(steps, "plan.yaml"), []byte("type: llm\n# The planner: one call, no tools, on the outline. It settles\n# what the work is checked against.\nprompt: x\n"), 0o644)
	os.WriteFile(filepath.Join(steps, "ideate.yaml"), []byte("type: llm\neach: [a, b]\n# Ideators A and B: one call each. More.\nprompt: x\n"), 0o644)
	os.WriteFile(filepath.Join(steps, "bare.yaml"), []byte("type: command\ncommand: true\n"), 0o644)
	name := filepath.Base(dir)
	for full, want := range map[string]string{
		name + ".steps.plan":     "The planner: one call, no tools, on the outline.",
		name + ".steps.ideate-a": "Ideators A and B: one call each.",
		name + ".steps.bare":     "",
	} {
		if got := aboutOf(dir, name, full); got != want {
			t.Errorf("%s: %q, want %q", full, got, want)
		}
	}
}

// The built-in team-solve: every step's first sentence is for the person
// watching the run, and fits the one line the window gives it (a message is
// at most 100 columns, the line 90). The run view showed "The single call,
// first, while plan writes the task beside it: one turn on the person's pr…"
// under a running step (2026-11-06): the maintainer's note, cut mid-word.
func TestEveryTeamSolveStepSaysWhatItDoesInOneLine(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "workflows", "team-solve"))
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "steps", "*.yaml"))
	if len(files) == 0 {
		t.Skip("no built-in team-solve beside this package")
	}
	for _, f := range files {
		step := strings.TrimSuffix(filepath.Base(f), ".yaml")
		about := aboutOf(dir, "team-solve", "team-solve.steps."+step)
		switch n := len([]rune(about)); {
		case n == 0:
			t.Errorf("%s: no opening comment to say what it does", step)
		case n > 90:
			t.Errorf("%s: %d runes, more than the line shows: %q", step, n, about)
		}
	}
}

// The built-in team-solve loads against the types' schemas: every key its
// steps use (optional, target, when, each) is one the schemas allow, and
// the steps the harness cannot do without are not optional.
func TestTheBuiltInTeamSolveLoadsAndMarksItsOptionalSteps(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "workflows", "team-solve"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no built-in team-solve beside this package")
	}
	wf := load(t, dir, &fakeRunner{})
	optional := map[string]bool{"direct": true, "outline": true, "plan": true, "ideate-a": true, "ideate-b": true, "solve": true, "review": true, "repair": true, "judge": true}
	for _, name := range []string{"problem", "settle", "baseline", "test-0", "context", "test", "recheck", "keep", "verify", "direct", "plan", "solve", "repair", "judge"} {
		task, ok := wf.Task(name)
		if !ok {
			t.Fatalf("no step %s", name)
		}
		if task.Optional != optional[name] {
			t.Errorf("%s: optional %v, want %v", name, task.Optional, optional[name])
		}
	}
	if task, _ := wf.Task("direct"); !strings.Contains(task.Target, "done.sh direct") {
		t.Errorf("direct's target is done.sh direct, got %q", task.Target)
	}
}
