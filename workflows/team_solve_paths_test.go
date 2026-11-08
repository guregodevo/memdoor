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

// check.sh runs the path checks beside the Check: a change that fixed the
// example and left a sibling fails, where the project's own command passes.
func TestTeamSolveCheckFailsAChangeThatLeavesAPathBroken(t *testing.T) {
	wf, _ := filepath.Abs("team-solve")
	for _, c := range []struct {
		name, tree string
		code       int
		says       string
	}{
		{"a sibling left as it was", "amount: ok\ntotal: bug\n", 1, "path check total: FAILS"},
		{"every path fixed", "amount: ok\ntotal: ok\n", 0, "path check total: passes"},
	} {
		dir := t.TempDir()
		r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
		writeAll(t, map[string]string{
			filepath.Join(r, "task.md"): "# Task\n\n## Check\n\n`true`\n\n## Scope\n\n`.`\n",
			filepath.Join(dir, "a.txt"): c.tree,
		}, 0o644)
		writeAll(t, map[string]string{filepath.Join(r, "paths", "total"): "#!/bin/sh\ngrep -q 'total: ok' a.txt\n"}, 0o755)
		cmd := exec.Command("bash", filepath.Join(wf, "check.sh"), r, filepath.Join(r, "test-output.log"))
		cmd.Dir = dir
		code := exitOf(cmd.Run(), t)
		log, _ := os.ReadFile(filepath.Join(r, "test-output.log"))
		kind, _ := os.ReadFile(filepath.Join(r, "check-kind.txt"))
		if code != c.code || !strings.Contains(string(log), c.says) || strings.TrimSpace(string(kind)) != "command+paths" {
			t.Errorf("%s: exit %d (want %d), kind %q, log:\n%s", c.name, code, c.code, kind, log)
		}
	}
}
