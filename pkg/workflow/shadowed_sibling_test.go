package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A project's workflow of the same name wins over the built-in one. When it lacks the task another
// workflow requires, loading says which workflow was found and what it has.
func TestAWorkflowOfTheSameNameWithoutTheTaskIsNamed(t *testing.T) {
	src, _ := filepath.Abs(filepath.Join("..", "..", "workflows", "pr-review"))
	if _, err := os.Stat(src); err != nil {
		t.Skip("no built-in pr-review beside this package")
	}
	root := t.TempDir()
	if err := os.CopyFS(filepath.Join(root, "pr-review"), os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	steps := filepath.Join(root, "change", "steps")
	if err := os.MkdirAll(steps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(steps, "log.yaml"), []byte("type: command\ncommand: |\n  git log -1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(filepath.Join(root, "pr-review"), Registry(types(&fakeRunner{})...))
	want := "requires change.steps.diff: the workflow change that was found has no such task (it has: change.steps.log)"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err %v\nwant it to hold %q", err, want)
	}
}
