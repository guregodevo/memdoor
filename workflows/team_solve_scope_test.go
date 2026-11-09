package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scope.sh in a folder with no git base to count against names the file it
// found without "+? -?" (#216); in a repository with a base it still counts.
func TestScopeNamesAChangedFileWithoutCountsWhenThereIsNoBase(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("team-solve", "scope.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r := filepath.Join(dir, ".memdoor", "runs", "t")
	if err := os.MkdirAll(r, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "task.md"), []byte("## Scope\n\n`hello.py`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "start"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// The file is newer than start: the run made it. Start is set an hour back so
	// a filesystem with coarse mtimes (the CI runner's) cannot call them equal.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(r, "start"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.py"), []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, r)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("scope.sh: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "changed: hello.py\n") || strings.Contains(got, "?") {
		t.Fatalf("a changed file with no base is named without counts:\n%s", got)
	}
}
