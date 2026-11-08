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

// done.sh paths keeps the path checks that fail on the tree as found and sets
// aside one that passes there: it proves nothing about its path.
func TestTeamSolvePathChecksThatPassOnTheTreeAsFoundAreSetAside(t *testing.T) {
	wf, _ := filepath.Abs("team-solve")
	dir := t.TempDir()
	r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
	writeAll(t, map[string]string{filepath.Join(dir, "a.txt"): "bug\n", filepath.Join(r, "paths.md"): "amount: a.txt\n"}, 0o644)
	writeAll(t, map[string]string{
		filepath.Join(r, "paths", "amount"):  "#!/bin/sh\ngrep -q fixed a.txt\n",
		filepath.Join(r, "paths", "vacuous"): "#!/bin/sh\ntest -f a.txt\n",
	}, 0o755)
	cmd := exec.Command("bash", filepath.Join(wf, "done.sh"), "paths", r)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("done.sh paths: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(r, "paths", "amount")); err != nil {
		t.Fatal("a check that fails on the tree as found is kept")
	}
	if _, err := os.Stat(filepath.Join(r, "paths-rejected", "vacuous")); err != nil {
		t.Fatal("a check that passes on the tree as found is set aside")
	}
	if entries, _ := os.ReadDir(filepath.Join(r, "paths")); len(entries) != 1 {
		t.Fatalf("paths/ holds only checks: %v", entries)
	}
	// Without paths.md the step is not done.
	os.Remove(filepath.Join(r, "paths.md"))
	cmd = exec.Command("bash", filepath.Join(wf, "done.sh"), "paths", r)
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		t.Fatal("paths.md is the step's record: without it, not done")
	}
}

// A path check is independent evidence: every attempt scores on it, so an
// attempt that fixed the sibling leads where the attempts' own checks tie.
func TestCrossScoresEveryAttemptOnThePathChecks(t *testing.T) {
	wf, _ := filepath.Abs("team-solve")
	dir := t.TempDir()
	sh := func(line string) error {
		cmd := exec.Command("sh", "-c", line)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		return cmd.Run()
	}
	r := filepath.Join(dir, "r")
	os.MkdirAll(filepath.Join(r, "attempts"), 0o755)
	sh("git init -q && printf 'amount: bug\\ntotal: bug\\n' > a.txt && git add a.txt && git commit -q -m base && git rev-parse HEAD > r/base.txt && : > r/untracked-before.txt")
	// Attempt 1 fixes the amount only; attempt 2 fixes both.
	sh("printf 'amount: ok\\ntotal: bug\\n' > a.txt && git diff > r/attempts/1.patch && git checkout -q a.txt")
	sh("printf 'amount: ok\\ntotal: ok\\n' > a.txt && git diff > r/attempts/2.patch && git checkout -q a.txt")
	writeAll(t, map[string]string{
		filepath.Join(r, "attempt-1"):       "#!/bin/sh\ngrep -q 'amount: ok' a.txt\n",
		filepath.Join(r, "attempt-2"):       "#!/bin/sh\ngrep -q 'amount: ok' a.txt\n",
		filepath.Join(r, "paths", "total"):  "#!/bin/sh\ngrep -q 'total: ok' a.txt\n",
		filepath.Join(r, "paths", "amount"): "#!/bin/sh\ngrep -q 'amount: ok' a.txt\n",
	}, 0o755)
	b, _ := os.ReadFile(filepath.Join(wf, "cross.sh"))
	writeAll(t, map[string]string{filepath.Join(r, "cross.sh"): string(b)}, 0o755)
	if err := sh("bash r/cross.sh r > r/out.txt"); err != nil {
		out, _ := os.ReadFile(filepath.Join(r, "out.txt"))
		t.Fatalf("attempt 2 leads on the path checks, settled; got %v:\n%s", err, out)
	}
	points, _ := os.ReadFile(filepath.Join(r, "pick-points.txt"))
	if !strings.Contains(string(points), "1 2 ") || !strings.Contains(string(points), "2 3 ") {
		t.Fatalf("points (attempt points size):\n%s", points)
	}
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
