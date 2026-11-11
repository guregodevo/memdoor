package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func applyIn(t *testing.T, cwd, patch string) (string, error) {
	t.Helper()
	in, _ := json.Marshal(map[string]string{"input": patch, "cwd": cwd})
	return ApplyPatch(in)
}

// A patch writes in the project folder only: a file named by an absolute path
// elsewhere, a ../ path or a move's destination outside is refused, and
// nothing in the patch is written.
func TestApplyPatchWritesOnlyInTheProject(t *testing.T) {
	project, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	away := filepath.Join(outside, "x.txt")
	rel, _ := filepath.Rel(project, filepath.Join(outside, "y.txt"))
	for name, patch := range map[string]string{
		"absolute": "*** Begin Patch\n*** Add File: " + away + "\n+hi\n*** End Patch",
		"dotdot":   "*** Begin Patch\n*** Add File: " + rel + "\n+hi\n*** End Patch",
		"move":     "*** Begin Patch\n*** Update File: a.txt\n*** Move to: " + away + "\n@@\n-one\n+two\n*** End Patch",
		"second":   "*** Begin Patch\n*** Add File: b.txt\n+inside\n*** Add File: " + away + "\n+hi\n*** End Patch",
	} {
		_, err := applyIn(t, project, patch)
		if err == nil || !strings.Contains(err.Error(), "outside the project folder") {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, p := range []string{away, filepath.Join(outside, "y.txt"), filepath.Join(project, "b.txt")} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s was written", p)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(project, "a.txt")); string(b) != "one\n" {
		t.Errorf("a.txt changed: %q", b)
	}
}

func TestApplyPatchStillWritesAnAbsolutePathInsideTheProject(t *testing.T) {
	project := t.TempDir()
	in := filepath.Join(project, "sub", "c.txt")
	if _, err := applyIn(t, project, "*** Begin Patch\n*** Add File: "+in+"\n+ok\n*** End Patch"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(in); string(b) != "ok\n" {
		t.Fatalf("%q", b)
	}
}
