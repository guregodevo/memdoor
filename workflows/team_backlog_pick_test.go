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
