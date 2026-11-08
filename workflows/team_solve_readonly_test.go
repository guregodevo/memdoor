package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// readonly.sh runs the Check with the change's edits to old tests put back:
// a change that rewrote the test pinning the old behaviour is named there,
// and one that only added tests, or edited none, is not.
func TestTeamSolveReadonlyNamesTheOldTestAChangeHid(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}
	wf, _ := filepath.Abs("team-solve")
	const test = "import unittest\nfrom lib import f\nclass T(unittest.TestCase):\n    def test_f(self):\n        self.assertEqual(f(), 1)\n"
	for _, c := range []struct {
		name, lib, test, says string
		named                 bool
	}{
		{"behaviour changed, its test rewritten", "def f():\n    return 2\n", strings.Replace(test, "f(), 1", "f(), 2", 1), "1 test(s) fail", true},
		{"a test added, the old one kept", "def f():\n    return 1\n", test + "    def test_g(self):\n        self.assertTrue(True)\n", "the Check passes", false},
		{"no test edited", "def f():\n    return 1  # same\n", test, "edits no test", false},
	} {
		dir := t.TempDir()
		writeAll(t, map[string]string{
			filepath.Join(dir, "lib.py"):               "def f():\n    return 1\n",
			filepath.Join(dir, "tests", "__init__.py"): "",
			filepath.Join(dir, "tests", "test_lib.py"): test,
		}, 0o644)
		gitIn(t, dir, "init", "-q")
		gitIn(t, dir, "add", "-A")
		gitIn(t, dir, "commit", "-qm", "base")
		r := filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-1")
		writeAll(t, map[string]string{
			filepath.Join(r, "task.md"):                "# Task\n\n## Check\n\n`python3 -B -m unittest`\n",
			filepath.Join(r, "base.txt"):               gitIn(t, dir, "rev-parse", "HEAD") + "\n",
			filepath.Join(r, "untracked-before.txt"):   "",
			filepath.Join(r, "baseline-status.txt"):    "0\n",
			filepath.Join(r, "check-kind.txt"):         "command\n",
			filepath.Join(dir, "lib.py"):               c.lib,
			filepath.Join(dir, "tests", "test_lib.py"): c.test,
		}, 0o644)
		check := exec.Command("bash", filepath.Join(wf, "check.sh"), r, filepath.Join(r, "test-output.log"))
		check.Dir = dir
		if code := exitOf(check.Run(), t); code != 0 {
			log, _ := os.ReadFile(filepath.Join(r, "test-output.log"))
			t.Fatalf("%s: the Check on the change exits %d:\n%s", c.name, code, log)
		}
		cmd := exec.Command("bash", filepath.Join(wf, "readonly.sh"), r)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v: %s", c.name, err, out)
		}
		listed, _ := os.ReadFile(filepath.Join(r, "readonly.txt"))
		_, mdErr := os.Stat(filepath.Join(r, "readonly.md"))
		kind, _ := os.ReadFile(filepath.Join(r, "check-kind.txt"))
		if !strings.Contains(string(out), c.says) || strings.Contains(string(listed), "test_f") != c.named || (mdErr == nil) != c.named {
			t.Errorf("%s: said %q, readonly.txt %q, readonly.md written %v", c.name, out, listed, mdErr == nil)
		}
		if strings.TrimSpace(string(kind)) != "command" {
			t.Errorf("%s: check-kind.txt is %q: the test step's must stay", c.name, kind)
		}
		if n := strings.Count(gitIn(t, dir, "worktree", "list"), "\n"); n != 0 {
			t.Errorf("%s: the clean copy is left behind: %s", c.name, gitIn(t, dir, "worktree", "list"))
		}
	}
}
