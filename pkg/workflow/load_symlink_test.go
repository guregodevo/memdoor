package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

// A project's .memdoor/workflows/<name> may be a symlink to a directory kept
// elsewhere in the tree (the repo points its own pr-review at workflows/):
// it is listed like a directory, and found by name.
func TestASymlinkedWorkflowDirIsListed(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "elsewhere", "wf")
	if err := os.MkdirAll(filepath.Join(real, "steps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "steps", "a.yaml"), []byte("type: command\ncommand: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := filepath.Join(root, "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "elsewhere", "wf"), filepath.Join(lib, "wf")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	got := listWorkflows(lib)
	if len(got) != 1 || got[0] != "wf" {
		t.Fatalf("a symlinked workflow directory must be listed, got %v", got)
	}
	// Found by name through a project whose .memdoor/workflows/wf is the
	// symlink: the engine gets the real directory.
	proj := filepath.Join(root, "proj")
	if err := os.MkdirAll(filepath.Join(proj, ".memdoor", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(proj, ".memdoor", "workflows", "wf")); err != nil {
		t.Fatal(err)
	}
	dir, err := Find(proj, "wf")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(real)
	if dir != want {
		t.Fatalf("Find must return the real directory, got %q want %q", dir, want)
	}
}
