package gateway

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"memdoor/pkg/workflow"
	"memdoor/workflows"
)

// The built-in workflows land in the library and each one loads against the
// gateway's own task types: a key the schema refuses, a cycle or a missing
// requirement fails here, not in a person's first CI job.
func TestSeedWritesBuiltinWorkflowsThatLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	SeedEmbeddedWorkflows()
	lib := filepath.Join(home, ".memdoor", "workflows")
	names, err := fs.Glob(workflows.FS, "*/README.md")
	if err != nil || len(names) == 0 {
		t.Fatalf("no built-in workflows embedded: %v", err)
	}
	s := &Server{}
	for _, readme := range names {
		name := filepath.Dir(readme)
		dir, err := workflow.Find(t.TempDir(), name)
		if err != nil {
			t.Fatalf("%s not found in the library after the seed: %v", name, err)
		}
		if dir != filepath.Join(lib, name) {
			t.Fatalf("%s resolved to %s, want the library", name, dir)
		}
		wf, err := workflow.Load(dir, workflow.Registry(s.workflowTypes(nil)...))
		if err != nil {
			t.Fatalf("%s does not load: %v", name, err)
		}
		if len(wf.Tasks) == 0 {
			t.Fatalf("%s has no tasks", name)
		}
	}
}

// A library workflow under a built-in's name that the seed never wrote is
// moved aside to <name>-mine, kept whole, and the built-in takes the name.
func TestSeedMovesASameNameCopyAsideForTheBuiltin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	lib := filepath.Join(home, ".memdoor", "workflows")
	mine := filepath.Join(lib, "pr-review")
	if err := os.MkdirAll(filepath.Join(mine, "steps"), 0o755); err != nil {
		t.Fatal(err)
	}
	own := "type: command\ncommand: echo mine\n"
	if err := os.WriteFile(filepath.Join(mine, "steps", "only.yaml"), []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	SeedEmbeddedWorkflows()
	if b, err := os.ReadFile(filepath.Join(lib, "pr-review-mine", "steps", "only.yaml")); err != nil || string(b) != own {
		t.Fatalf("the person's copy was not kept as pr-review-mine: %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(mine, "steps", "only.yaml")); !os.IsNotExist(err) {
		t.Fatalf("the person's step is still in the built-in's folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mine, "README.md")); err != nil {
		t.Fatalf("the built-in pr-review was not seeded: %v", err)
	}
	// The next boot leaves both alone: pr-review is the seed's now.
	SeedEmbeddedWorkflows()
	if _, err := os.Stat(filepath.Join(lib, "pr-review-mine-2")); !os.IsNotExist(err) {
		t.Fatalf("a second boot moved the built-in aside: %v", err)
	}
}

// A copy of a built-in made by hand, its shipped files unchanged and a file
// of its own beside them (the published example copied with its bench/), is
// adopted where it is: nothing moves.
func TestSeedAdoptsAnIdenticalCopyOfABuiltin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	lib := filepath.Join(home, ".memdoor", "workflows")
	src, err := fs.Sub(workflows.FS, "team-solve")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(lib, "team-solve"), src); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(lib, "team-solve", "NOTES.md")
	if err := os.WriteFile(extra, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	SeedEmbeddedWorkflows()
	if _, err := os.Stat(filepath.Join(lib, "team-solve-mine")); !os.IsNotExist(err) {
		t.Fatalf("an identical copy was moved aside: %v", err)
	}
	if b, err := os.ReadFile(extra); err != nil || string(b) != "mine\n" {
		t.Fatalf("the copy's own file is gone: %q %v", b, err)
	}
}

// An edit to a seeded step survives the next boot, and a step the seed wrote
// and the person deleted is written again (the workflow is still ours).
func TestSeedKeepsAnEditedStep(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	SeedEmbeddedWorkflows()
	step := filepath.Join(home, ".memdoor", "workflows", "pr-review", "steps", "verdict.yaml")
	edited := "type: command\ncommand: echo lenient\n"
	if err := os.WriteFile(step, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(home, ".memdoor", "workflows", "pr-review", "README.md")
	if err := os.Remove(readme); err != nil {
		t.Fatal(err)
	}
	SeedEmbeddedWorkflows()
	if b, _ := os.ReadFile(step); string(b) != edited {
		t.Fatalf("the edit was overwritten: %q", b)
	}
	if _, err := os.Stat(readme); err != nil {
		t.Fatalf("a deleted seeded file of our own workflow was not written again: %v", err)
	}
}

// team-solve ships with what its steps run on: the sample problem an
// empty folder gets, and the check script `test` and `verify` call.
func TestSeedWritesTeamSolveWithItsSampleAndCheck(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	SeedEmbeddedWorkflows()
	dir := filepath.Join(home, ".memdoor", "workflows", "team-solve")
	for _, rel := range []string{"check-samples.sh", "changes.sh", "check.sh", "done.sh", "task.template.md", "sample/task.md", "sample/tests/1.in", "sample/tests/5.out", "steps/problem.yaml", "steps/judge.yaml", "scope.sh", "steps/context.yaml", "steps/ideate.yaml", "steps/direct.yaml", "steps/test-0.yaml", "steps/outline.yaml", "steps/settle.yaml", "steps/recheck.yaml", "verdict.sh", "snapshot.sh", "steps/keep.yaml", "steps/baseline.yaml", "task.sh", "settle.sh", "failures.sh"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("team-solve seeded without %s: %v", rel, err)
		}
	}
}
