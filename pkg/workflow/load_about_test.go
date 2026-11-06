package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

// A step's About is the first sentence of the comment its file opens with;
// a task a sibling file defines (ideate.yaml for ideate-a) reads that file.
func TestAStepSaysWhatItDoes(t *testing.T) {
	dir := t.TempDir()
	steps := filepath.Join(dir, "steps")
	os.MkdirAll(steps, 0o755)
	os.WriteFile(filepath.Join(steps, "plan.yaml"), []byte("type: llm\n# The planner: one call, no tools, on the outline. It settles\n# what the work is checked against.\nprompt: x\n"), 0o644)
	os.WriteFile(filepath.Join(steps, "ideate.yaml"), []byte("# Ideators A and B: one call each. More.\ntype: llm\n"), 0o644)
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
