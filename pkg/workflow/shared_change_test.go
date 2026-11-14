package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

// The workflows that work on a change require the change workflow's diff step, and none carries a copy of
// it: the step that finds the change is written once.
func TestTheChangeIsFoundByOneSharedStep(t *testing.T) {
	root := filepath.Join("..", "..", "workflows")
	if _, err := os.Stat(filepath.Join(root, "change", "steps", "diff.yaml")); err != nil {
		t.Skip("no built-in change workflow beside this package")
	}
	for _, name := range []string{"pr-description", "pr-review", "pr-team-review", "security-review", "write-tests"} {
		if _, err := os.Stat(filepath.Join(root, name, "steps", "diff.yaml")); err == nil {
			t.Errorf("%s has its own diff step again", name)
		}
		dir, _ := filepath.Abs(filepath.Join(root, name))
		wf := load(t, dir, &fakeRunner{})
		if _, ok := wf.Defs["change.steps.diff"]; !ok {
			t.Errorf("%s does not pull in change's diff", name)
		}
		for full, d := range wf.Defs {
			for _, r := range d.Requires {
				if r.Name() == name+".steps.diff" {
					t.Errorf("%s still requires a diff step of its own", full)
				}
			}
		}
	}
	dir, _ := filepath.Abs(filepath.Join(root, "change"))
	wf := load(t, dir, &fakeRunner{})
	if about := aboutOf(dir, "change", "change.steps.diff"); len([]rune(about)) == 0 || len([]rune(about)) > 90 {
		t.Errorf("change's diff: about line of %d runes: %q", len([]rune(about)), about)
	}
	if len(wf.Defs) != 1 {
		t.Errorf("change has %d steps, want 1", len(wf.Defs))
	}
}
