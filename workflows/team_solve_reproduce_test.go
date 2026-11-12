package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A run folder with a task whose Check is `check`, a project command that
// exits as `cmdExit` says, and a reproduction that exits `reproExit`
// (none when reproExit < 0), proven at baseline with `baseline`.
func reproRun(t *testing.T, check string, cmdExit, reproExit int, baseline string) (dir, r string) {
	t.Helper()
	dir = t.TempDir()
	r = filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
	if err := os.MkdirAll(r, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string, mode os.FileMode) {
		if err := os.WriteFile(name, []byte(text), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(r, "task.md"), "# Task\n\n## Prompt\n\nx is wrong\n\n## Check\n\n"+check+"\n\n## Scope\n\n`.`\n", 0o644)
	write(filepath.Join(dir, "run.sh"), "echo ran; exit "+itoa(cmdExit)+"\n", 0o644)
	if reproExit >= 0 {
		write(filepath.Join(r, "repro"), "#!/bin/sh\necho reproducing; exit "+itoa(reproExit)+"\n", 0o755)
	}
	if baseline != "" {
		write(filepath.Join(r, "repro-baseline-status.txt"), baseline+"\n", 0o644)
	}
	return dir, r
}

func itoa(n int) string { return strings.TrimSpace(strings.Repeat(" ", 0) + string(rune('0'+n))) }

// The reproduction runs beside the Check's command when the Check names
// reproduce and one was written that fails at baseline; the work is right
// only when both pass. Without one, the Check is what it was (#150).
func TestTeamSolveCheckRunsTheReproductionBesideTheCommand(t *testing.T) {
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, check      string
		cmdExit, repro   int
		baseline         string
		code             int
		kind, parts, log string
	}{
		{"command passes, reproduction fails", "`bash run.sh` reproduce", 0, 1, "1", 1, "command+repro", "command 0\nrepro 1\n", "reproduction: FAILS"},
		{"both pass", "`bash run.sh` reproduce", 0, 0, "1", 0, "command+repro", "command 0\nrepro 0\n", "reproduction: passes"},
		{"command fails", "`bash run.sh` reproduce", 1, 0, "1", 1, "command+repro", "command 1\nrepro 0\n", ""},
		{"reproduce alone, fails", "reproduce", 0, 1, "1", 1, "repro", "repro 1\n", "reproduction: FAILS"},
		{"reproduce alone, passes", "reproduce", 0, 0, "1", 0, "repro", "repro 0\n", ""},
		{"the Check does not name it: ignored", "`bash run.sh`", 0, 1, "1", 0, "command", "command 0\n", ""},
		{"named, none written: the command alone", "`bash run.sh` reproduce", 0, -1, "", 0, "command", "command 0\n", "no reproduction"},
		{"named, one that passed at baseline: ignored", "reproduce `bash run.sh`", 0, 1, "0", 0, "command", "command 0\n", "no reproduction"},
		{"named, nothing else to run, none written: the judge", "reproduce", 0, -1, "", 3, "judge", "", "no reproduction"},
	} {
		dir, r := reproRun(t, c.check, c.cmdExit, c.repro, c.baseline)
		cmd := exec.Command("bash", filepath.Join(wf, "check.sh"), r, filepath.Join(r, "out.log"))
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		code := 0
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else if err != nil {
			t.Fatalf("%s: %v\n%s", c.name, err, out)
		}
		kind, _ := os.ReadFile(filepath.Join(r, "check-kind.txt"))
		parts, _ := os.ReadFile(filepath.Join(r, "check-parts.txt"))
		log, _ := os.ReadFile(filepath.Join(r, "out.log"))
		if code != c.code || strings.TrimSpace(string(kind)) != c.kind || string(parts) != c.parts || !strings.Contains(string(log), c.log) {
			t.Errorf("%s: exit %d kind %q parts %q, want %d %q %q; log:\n%s", c.name, code, strings.TrimSpace(string(kind)), parts, c.code, c.kind, c.parts, log)
		}
	}
}

// The reproduction's target: it exists, names no absolute path of this tree,
// and fails on the tree as the run found it (the clean copy); one that
// passes there reproduces nothing.
func TestTeamSolveDoneReproduceProvesItOnTheTreeAsFound(t *testing.T) {
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, repro string
		done        bool
		says        string
	}{
		{"fails on the clean copy", "#!/bin/sh\ntest -f fixed.txt\n", true, "reproduce: done"},
		{"passes on the clean copy", "#!/bin/sh\nexit 0\n", false, "reproduces nothing"},
		{"names this tree's absolute path", "#!/bin/sh\ncd ABS && exit 1\n", false, "absolute path"},
		{"missing", "", false, "missing"},
	} {
		dir := t.TempDir()
		r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
		if err := os.MkdirAll(filepath.Join(r, "baseline-tree"), 0o755); err != nil {
			t.Fatal(err)
		}
		// The main tree has the fix; the clean copy does not.
		if err := os.WriteFile(filepath.Join(dir, "fixed.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if c.repro != "" {
			pwd := exec.Command("bash", "-c", "pwd -P") // what the script compares against: a POSIX path, also under Git Bash
			pwd.Dir = dir
			p, err := pwd.Output()
			if err != nil {
				t.Fatal(err)
			}
			abs := strings.TrimSpace(string(p))
			if err := os.WriteFile(filepath.Join(r, "repro"), []byte(strings.ReplaceAll(c.repro, "ABS", abs)), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command("bash", filepath.Join(wf, "done.sh"), "reproduce", ".memdoor/runs/team-solve-1")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME="+dir)
		out, err := cmd.CombinedOutput()
		if (err == nil) != c.done || !strings.Contains(string(out), c.says) {
			t.Errorf("%s: done %v (%v), says %q; want done %v and %q", c.name, err == nil, err, out, c.done, c.says)
		}
		if c.done {
			if s, _ := os.ReadFile(filepath.Join(r, "repro-baseline-status.txt")); strings.TrimSpace(string(s)) != "1" {
				t.Errorf("%s: repro-baseline-status.txt = %q, want 1", c.name, s)
			}
		}
	}
}

// With no clean copy of the tree (not in git, or a repository with no commit yet)
// the reproduction is not run on the live tree, where the first attempt is already
// at work: the step ends done, no baseline status is written, and the Check does
// not use the reproduction. A reproduction that passes there used to fail the step
// and loop the agent for the whole clock (the sample run, 2026-11-09).
func TestTeamSolveDoneReproduceWithNoCleanCopyDoesNotLoop(t *testing.T) {
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
	if err := os.MkdirAll(r, 0o755); err != nil {
		t.Fatal(err)
	}
	// The live tree already has the attempt's fix: the reproduction passes on it.
	if err := os.WriteFile(filepath.Join(dir, "fixed.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "repro"), []byte("#!/bin/sh\ntest -f fixed.txt\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(wf, "done.sh"), "reproduce", ".memdoor/runs/team-solve-1")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "not proven") {
		t.Fatalf("no clean copy: err %v, output %q", err, out)
	}
	if _, err := os.Stat(filepath.Join(r, "repro-baseline-status.txt")); err == nil {
		t.Error("no clean copy: a baseline status was written for a reproduction that was never proven")
	}
}
