package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Real logs (pytest 8.x, python -m unittest as Django's runner prints it,
// go test), trimmed to the lines that matter.
const (
	pytestLog = `============================= test session starts ==============================
collected 4 items

test_a.py FF.                                                            [ 75%]
test_b.py E                                                              [100%]

=========================== short test summary info ============================
FAILED test_a.py::T::test_bad - AssertionError: 1 != 2
FAILED test_a.py::T::test_err - RuntimeError: x
ERROR test_b.py::test_uses - ValueError: setup
===================== 2 failed, 1 passed, 1 error in 0.01s =====================
`
	unittestLog = `FE.
======================================================================
ERROR: test_err (test_a.T.test_err)
----------------------------------------------------------------------
RuntimeError: x

======================================================================
FAIL: test_bad (test_a.T.test_bad)
----------------------------------------------------------------------
AssertionError: 1 != 2

----------------------------------------------------------------------
Ran 3 tests in 0.000s

FAILED (failures=1, errors=1)
`
	goLog = `# m/b [m/b.test]
b/b_test.go:3:27: undefined: undefined
--- FAIL: TestBad (0.00s)
    --- FAIL: TestBad/sub (0.00s)
        a_test.go:4: no
FAIL
FAIL	m/a	0.327s
FAIL	m/b [build failed]
FAIL
`
)

func runFailures(t *testing.T, log string) (string, int) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "check.log")
	if err := os.WriteFile(p, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", "team-solve/failures.sh", p).Output()
	code := 0
	if e, ok := err.(*exec.ExitError); ok {
		code = e.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

// failures.sh lists the failing tests a runner printed, and says it cannot
// tell (exit 2) when the log is not a finished run, its count does not match
// its list, or it names no failing test.
func TestTeamSolveFailuresReadsTheRunnersLists(t *testing.T) {
	for _, c := range []struct {
		name, log, want string
		code            int
	}{
		{"pytest", pytestLog, "test_a.py::T::test_bad\ntest_a.py::T::test_err\ntest_b.py::test_uses\n", 0},
		{"unittest", unittestLog, "test_bad (test_a.T.test_bad)\ntest_err (test_a.T.test_err)\n", 0},
		{"go test", goLog, "TestBad\nTestBad/sub\npackage m/a\npackage m/b [build failed]\n", 0},
		// pytest -rN: the count says 2 failed and 1 error, the list says nothing.
		{"pytest without its summary", strings.Join(strings.Split(pytestLog, "\n")[:6], "\n") + "\n===================== 2 failed, 1 passed, 1 error in 0.01s =====================\n", "", 2},
		{"no runner", "bash: line 1: python: command not found\n", "", 2},
		{"no tests ran", "ERROR: file or directory not found: tests/test_new.py\n\n============================ no tests ran in 0.01s =============================\n", "", 2},
	} {
		got, code := runFailures(t, c.log)
		if got != c.want || code != c.code {
			t.Errorf("%s: got %q exit %d, want %q exit %d", c.name, got, code, c.want, c.code)
		}
	}
}

// When the Check already fails on the tree as the run found it, check.sh
// counts what the change adds: no new failing test passes, a new one fails
// and is named, and a log that lists no tests leaves it to the judge (#109:
// one unrelated failing test in astropy turned the whole Check off).
func TestTeamSolveCheckCountsTheFailuresAChangeAdds(t *testing.T) {
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(pytestLog, "2 failed, 1 passed, 1 error", "3 failed, 1 error", 1)
	broken = strings.Replace(broken, "ERROR test_b.py", "FAILED test_a.py::T::test_ok - assert 0\nERROR test_b.py", 1)
	for _, c := range []struct {
		name, after string
		code        int
		says        string
	}{
		{"the same failures", pytestLog, 0, "adds no failing test"},
		{"one more failure", broken, 1, "test_a.py::T::test_ok"},
		{"a log with no tests", "bash: line 1: python: command not found\n", 3, "the judge decides"},
	} {
		dir := t.TempDir()
		r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
		if err := os.MkdirAll(r, 0o755); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{
			filepath.Join(r, "task.md"):             "# Task\n\n## Check\n\n`bash run.sh`\n\n## Scope\n\n`.`\n",
			filepath.Join(r, "baseline-status.txt"): "1\n",
			filepath.Join(r, "baseline-check.log"):  "check: bash run.sh\n" + pytestLog,
			filepath.Join(dir, "run.sh"):            "cat after.log; exit 1\n",
			filepath.Join(dir, "after.log"):         c.after,
		}
		for p, text := range files {
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
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
