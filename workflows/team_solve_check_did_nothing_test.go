package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A Check that fails as it did on the tree as the run found it, with no file
// of the Scope changed, fails the change (exit 1): the change did nothing the
// Check can see. With a file of the Scope changed, or another failure, the
// judge decides as before (exit 3).
func TestTeamSolveCheckFailsAChangeThatDidNothing(t *testing.T) {
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	same := "Traceback (most recent call last):\n  File \"grade.py\", line 18\nFileNotFoundError: [Errno 2] No such file or directory: 'solution.py'\n"
	for _, c := range []struct {
		name, file, after string
		code              int
		says              string
	}{
		{"only a scratch file", "scratch/poly.py", same, 1, "did nothing it can see"},
		{"a file of the Scope", "solution.py", same, 3, "the judge decides"},
		{"another failure", "scratch/poly.py", "ValueError: bad input\n", 3, "the judge decides"},
	} {
		dir := t.TempDir()
		r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
		run := func(args ...string) {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v: %v\n%s", args, err, out)
			}
		}
		write := func(p, text string) {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write(filepath.Join(dir, "task.md"), "the problem\n")
		write(filepath.Join(dir, "run.sh"), "cat after.log; exit 1\n")
		write(filepath.Join(dir, ".gitignore"), "after.log\n.memdoor/\n")
		run("git", "init", "-q")
		run("git", "add", "-A")
		run("git", "commit", "-qm", "base")
		head, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		write(filepath.Join(r, "base.txt"), string(head))
		write(filepath.Join(r, "untracked-before.txt"), "")
		write(filepath.Join(r, "task.md"), "# Task\n\n## Check\n\n`bash run.sh`\n\n## Scope\n\n`solution.py`\n")
		write(filepath.Join(r, "baseline-status.txt"), "1\n")
		write(filepath.Join(r, "baseline-check.log"), "check: bash run.sh\n"+same)
		write(filepath.Join(dir, "after.log"), c.after)
		write(filepath.Join(dir, c.file), "print(1)\n")
		cmd := exec.Command("bash", filepath.Join(wf, "check.sh"), r, filepath.Join(r, "test-output.log"))
		cmd.Dir = dir
		err := cmd.Run()
		code := 0
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		log, _ := os.ReadFile(filepath.Join(r, "test-output.log"))
		if code != c.code || !strings.Contains(string(log), c.says) {
			t.Errorf("%s: exit %d, want %d, and the log should say %q:\n%s", c.name, code, c.code, c.says, log)
		}
	}
}
