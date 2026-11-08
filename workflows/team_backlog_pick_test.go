package workflows

import (
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
