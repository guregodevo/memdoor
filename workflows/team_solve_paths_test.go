package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeAll(t *testing.T, files map[string]string, mode os.FileMode) {
	t.Helper()
	for p, text := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), mode); err != nil {
			t.Fatal(err)
		}
	}
}

func exitOf(err error, t *testing.T) int {
	t.Helper()
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0
}

// The path checks are the fan-out's evidence, not the Check's: a change the
// project's own command passes passes the Check, whatever a path check says.
// A wrong path check vetoed a right fix (matplotlib-23476, 2026-11-09).
func TestTeamSolveCheckLeavesThePathChecksToTheFanout(t *testing.T) {
	wf, _ := filepath.Abs("team-solve")
	for _, tree := range []string{"amount: ok\ntotal: bug\n", "amount: ok\ntotal: ok\n"} {
		dir := t.TempDir()
		r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
		writeAll(t, map[string]string{
			filepath.Join(r, "task.md"): "# Task\n\n## Check\n\n`true`\n\n## Scope\n\n`.`\n",
			filepath.Join(dir, "a.txt"): tree,
		}, 0o644)
		writeAll(t, map[string]string{filepath.Join(r, "paths", "total"): "#!/bin/sh\ngrep -q 'total: ok' a.txt\n"}, 0o755)
		cmd := exec.Command("bash", filepath.Join(wf, "check.sh"), r, filepath.Join(r, "test-output.log"))
		cmd.Dir = dir
		code := exitOf(cmd.Run(), t)
		log, _ := os.ReadFile(filepath.Join(r, "test-output.log"))
		kind, _ := os.ReadFile(filepath.Join(r, "check-kind.txt"))
		if code != 0 || strings.Contains(string(log), "path check") || strings.TrimSpace(string(kind)) != "command" {
			t.Errorf("tree %q: exit %d, kind %q, log:\n%s", tree, code, kind, log)
		}
	}
}
