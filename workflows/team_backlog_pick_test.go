package workflows

import (
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pick-issue.sh on a listing as next.yaml writes it: number, assignees, labels, title.
func pickIssue(t *testing.T, tsv, tried, label string) string {
	t.Helper()
	dir := t.TempDir()
	tsvPath, triedPath := filepath.Join(dir, "issues.tsv"), filepath.Join(dir, "tried.txt")
	for p, s := range map[string]string{tsvPath: tsv, triedPath: tried} {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script, err := filepath.Abs(filepath.Join("team-backlog", "pick-issue.sh"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", script, tsvPath, triedPath, label).CombinedOutput()
	if err != nil {
		t.Fatalf("pick-issue.sh: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

const issues = "12\t0\tbug,good first issue\tThe flag is ignored\n" +
	"7\t1\t\tAssigned to someone\n" +
	"9\t0\tbug\tA panic on empty input\n" +
	"30\t0\t\tNo labels, the newest\n"

// The lowest open issue nobody is on: 9, not 7 (assigned) nor 12.
func TestPickIssueTakesTheOldestUnassigned(t *testing.T) {
	if got := pickIssue(t, issues, "", ""); got != "9" {
		t.Fatalf("picked %q, want 9", got)
	}
}

// An issue tried before is never picked again; the next one is.
func TestPickIssueSkipsTheTried(t *testing.T) {
	if got := pickIssue(t, issues, "9\n12\n", ""); got != "30" {
		t.Fatalf("picked %q, want 30", got)
	}
}

// With a label, only issues carrying it, the whole label, not a prefix.
func TestPickIssueHonoursTheLabel(t *testing.T) {
	if got := pickIssue(t, issues, "9\n", "bug"); got != "12" {
		t.Fatalf("picked %q, want 12", got)
	}
	if got := pickIssue(t, issues, "", "good first issue"); got != "12" {
		t.Fatalf("picked %q, want 12", got)
	}
	if got := pickIssue(t, issues, "", "good"); got != "" {
		t.Fatalf("picked %q on a label prefix, want nothing", got)
	}
}

// Nothing left: no output, exit 0, so next.yaml ends the run quietly.
func TestPickIssueSaysNothingWhenNoneIsLeft(t *testing.T) {
	if got := pickIssue(t, issues, "9\n12\n30\n", ""); got != "" {
		t.Fatalf("picked %q, want nothing", got)
	}
}

// next.yaml's command on a throwaway git tree with a fake gh on the PATH.
// ghScript is the body of the fake gh: it receives the gh arguments.
func runNext(t *testing.T, ghScript string) (out string, code int) {
	t.Helper()
	var step struct {
		Command string `yaml:"command"`
	}
	raw, err := os.ReadFile(filepath.Join("team-backlog", "steps", "next.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &step); err != nil {
		t.Fatal(err)
	}
	cmdText := strings.NewReplacer("{{.partition}}", "t", "{{.max_open_prs}}", "5", "{{.label}}", "").Replace(step.Command)
	wf, err := filepath.Abs("team-backlog")
	if err != nil {
		t.Fatal(err)
	}
	dir, bin := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\n"+ghScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".memdoor", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wf, filepath.Join(dir, ".memdoor", "workflows", "team-backlog")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", cmdText)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	b, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(b), code
}

// gh not signed in is a stop with a reason, never "no open issue left": the
// run could not read the issues (found on a clean machine, 2026-11-09).
func TestNextStopsWhenGhIsNotSignedIn(t *testing.T) {
	out, code := runNext(t, `[ "$1 $2" = "auth status" ] && exit 1; exit 0`)
	if code == 0 || !strings.Contains(out, "gh is not signed in") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
}

// A listing that fails while signed in is a stop too, not an empty backlog.
func TestNextStopsWhenTheIssueListFails(t *testing.T) {
	out, code := runNext(t, `[ "$1 $2" = "issue list" ] && { echo boom >&2; exit 1; }; exit 0`)
	if code == 0 || !strings.Contains(out, "gh issue list failed") || strings.Contains(out, "no open issue left") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
}

// Signed in with an empty list is the quiet end the run is meant to have.
func TestNextEndsQuietlyOnAnEmptyBacklog(t *testing.T) {
	out, code := runNext(t, `exit 0`)
	if code != 0 || !strings.Contains(out, "no open issue left to try") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
}

// prStep runs team-backlog's pr step on a worktree with a change in it, a fake gh and a fake memdoor.
// pw is the pr_workflow arg. It returns the output, the exit code, the project dir and the fake tools' call log.
func prStep(t *testing.T, pw, memdoorExit string) (string, int, string, string) {
	t.Helper()
	var step struct {
		Command string `yaml:"command"`
	}
	raw, err := os.ReadFile(filepath.Join("team-backlog", "steps", "pr.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &step); err != nil {
		t.Fatal(err)
	}
	cmdText := strings.NewReplacer("{{.partition}}", "t", "{{.pr_workflow}}", pw).Replace(step.Command)
	dir, bin, remote := t.TempDir(), t.TempDir(), t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare")
	wt := filepath.Join(dir, "wt")
	writeAll(t, map[string]string{filepath.Join(wt, "a.txt"): "a\n"}, 0o644)
	gitIn(t, wt, "init", "-q", "-b", "main")
	gitIn(t, wt, "add", "a.txt")
	gitIn(t, wt, "commit", "-qm", "base")
	gitIn(t, wt, "remote", "add", "origin", remote)
	gitIn(t, wt, "checkout", "-q", "-b", "memdoor/issue-5")
	writeAll(t, map[string]string{filepath.Join(wt, "fix.txt"): "fixed\n"}, 0o644)
	run := filepath.Join(dir, "solve")
	r := filepath.Join(dir, ".memdoor", "runs", "team-backlog-t")
	writeAll(t, map[string]string{
		filepath.Join(r, "number.txt"):         "5\n",
		filepath.Join(r, "worktree.txt"):       wt + "\n",
		filepath.Join(r, "solve-run.txt"):      run + "\n",
		filepath.Join(run, "keep.txt"):         "keep: the change passed\n",
		filepath.Join(run, "check-kind.txt"):   "tests\n",
		filepath.Join(run, "final-status.txt"): "0\n",
	}, 0o644)
	calls := filepath.Join(bin, "calls.txt")
	writeAll(t, map[string]string{
		filepath.Join(bin, "gh"):      "#!/bin/sh\necho \"gh $*\" >> '" + calls + "'\n[ \"$1 $2\" = \"issue view\" ] && echo 'Fix the thing'\n[ \"$1 $2\" = \"pr create\" ] && echo https://github.com/o/r/pull/7\nexit 0\n",
		filepath.Join(bin, "memdoor"): "#!/bin/sh\necho \"memdoor $*\" >> '" + calls + "'\nwhile [ $# -gt 0 ]; do [ \"$1\" = --dir ] && d=\"$2\"; shift; done\nif [ " + memdoorExit + " = 0 ]; then echo https://github.com/o/r/pull/9 > \"$d/.memdoor/runs/pr-create-backlog-5/url.txt\"; fi\nexit " + memdoorExit + "\n",
	}, 0o755)
	cmd := exec.Command("sh", "-c", cmdText)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(calls)
	return string(out), code, dir, string(b)
}

// With pr_workflow set the named workflow opens the pull request in the worktree, with the team's evidence
// as extra.md, and the step itself pushes nothing and calls no gh pr create.
func TestPRStepHandsTheChangeToPRCreate(t *testing.T) {
	out, code, dir, calls := prStep(t, "pr-create", "0")
	wt := filepath.Join(dir, "wt")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(calls, "memdoor workflow run pr-create --dir "+wt+" --partition backlog-5 --wait") || strings.Contains(calls, "pr create") {
		t.Fatalf("calls:\n%s", calls)
	}
	extra, _ := os.ReadFile(filepath.Join(wt, ".memdoor", "runs", "pr-create-backlog-5", "extra.md"))
	if !strings.Contains(string(extra), "keep: the change passed") || strings.Contains(string(extra), "Closes #5") {
		t.Fatalf("extra.md:\n%s", extra)
	}
	if got := gitIn(t, wt, "log", "--format=%s", "-1"); got != "Fix the thing (#5)" {
		t.Fatalf("commit %q", got)
	}
	if s, _ := os.ReadFile(filepath.Join(dir, ".memdoor", "runs", "team-backlog-t", "summary.txt")); !strings.Contains(string(s), "/pull/9") {
		t.Fatalf("summary %q", s)
	}
}

// A nested workflow that fails leaves the change committed and fails the step, naming the exit status.
func TestPRStepFailsWhenPRCreateFails(t *testing.T) {
	out, code, _, _ := prStep(t, "pr-create", "2")
	if code == 0 || !strings.Contains(out, "pr-create ended with exit 2") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
}

// With no pr_workflow the step opens the pull request itself, as before.
func TestPRStepDefaultStillOpensItsOwn(t *testing.T) {
	out, code, _, calls := prStep(t, "", "0")
	if code != 0 || !strings.Contains(calls, "gh pr create --head memdoor/issue-5") || strings.Contains(calls, "memdoor workflow") {
		t.Fatalf("exit %d:\n%s\ncalls:\n%s", code, out, calls)
	}
}
