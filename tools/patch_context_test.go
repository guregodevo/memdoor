package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A hunk needs only the lines it changes when they are unique; when they are
// not, it is refused with where they are, never applied to the first match
// (2026-09-30: `-	return 1` meant for B() used to change A()).
func TestContextFreeHunks(t *testing.T) {
	patchVerify = false
	t.Cleanup(func() { patchVerify = true })
	src := "package x\n\nfunc A() int {\n\treturn 1\n}\n\nfunc B() int {\n\treturn 1\n}\n\nfunc C() int {\n\treturn 3\n}\n"
	run := func(patch string) (string, error) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o644)
		return runPatch(t, dir, "*** Begin Patch\n*** Update File: x.go\n"+patch+"\n*** End Patch", "x.go")
	}
	if got, err := run("@@\n-\treturn 3\n+\treturn 4"); err != nil || !strings.Contains(got, "func C() int {\n\treturn 4") {
		t.Fatalf("unique, no context: %v\n%s", err, got)
	}
	if _, err := run("@@\n-\treturn 1\n+\treturn 2"); err == nil || !strings.Contains(err.Error(), "appear more than once (lines 4 and 8)") {
		t.Fatalf("ambiguous, no context: want a refusal naming lines 4 and 8, got %v", err)
	}
	if got, err := run("@@ func B() int {\n-\treturn 1\n+\treturn 2"); err != nil || !strings.Contains(got, "func B() int {\n\treturn 2") || !strings.Contains(got, "func A() int {\n\treturn 1") {
		t.Fatalf("anchored: %v\n%s", err, got)
	}
	if got, err := run(" func B() int {\n-\treturn 1\n+\treturn 2"); err != nil || !strings.Contains(got, "func B() int {\n\treturn 2") {
		t.Fatalf("one line of context: %v\n%s", err, got)
	}
}
