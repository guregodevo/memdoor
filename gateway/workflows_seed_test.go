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

// A library workflow the person wrote under a built-in's name stays theirs:
// the seed adds no file to it and changes none.
func TestSeedLeavesAPersonsWorkflowOfTheSameName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mine := filepath.Join(home, ".memdoor", "workflows", "pr-review")
	if err := os.MkdirAll(filepath.Join(mine, "steps"), 0o755); err != nil {
		t.Fatal(err)
	}
	own := "type: command\ncommand: echo mine\n"
	if err := os.WriteFile(filepath.Join(mine, "steps", "only.yaml"), []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	SeedEmbeddedWorkflows()
	var files []string
	_ = filepath.WalkDir(mine, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if len(files) != 1 {
		t.Fatalf("the seed wrote into the person's pr-review: %v", files)
	}
	if b, _ := os.ReadFile(filepath.Join(mine, "steps", "only.yaml")); string(b) != own {
		t.Fatalf("the person's file changed: %q", b)
	}
	if _, err := os.Stat(filepath.Join(home, ".memdoor", "workflows", "fix-tests", "README.md")); err != nil {
		t.Fatalf("the other built-ins were not seeded: %v", err)
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
