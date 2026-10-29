package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestARunReportsItselfToGitHubActions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".memdoor/runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	review := "## Findings\n- **P0** `a.go:3` — the loop skips the last layer\n"
	if err := os.WriteFile(filepath.Join(dir, ".memdoor/runs/pr-review-x.md"), []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, output := filepath.Join(dir, "summary"), filepath.Join(dir, "output")
	t.Setenv("GITHUB_STEP_SUMMARY", summary)
	t.Setenv("GITHUB_OUTPUT", output)

	st := workflowRunStatus{ID: "pr-review-1", Workflow: "pr-review", Dir: dir, State: "failed", Done: 2, Total: 3,
		Tasks:   []workflowTaskStatus{{Name: "verdict", State: "failed", Error: "task verdict: sh: exit status 1 — 1 finding"}},
		Results: []string{".memdoor/runs/pr-review-x.md", ".memdoor/runs/pr-review-x.diff"}}
	reportToGitHub(st, workflowExitFailed)

	s, _ := os.ReadFile(summary)
	for _, want := range []string{"### Memdoor workflow `pr-review`", "✗ failed", "✗ verdict", "exit status 1 — 1 finding", "**P0** `a.go:3`"} {
		if !strings.Contains(string(s), want) {
			t.Errorf("the summary has no %q:\n%s", want, s)
		}
	}
	if strings.Contains(string(s), "pr-review-x.diff\n\n---") {
		t.Errorf("only Markdown results are shown in full:\n%s", s)
	}
	o, _ := os.ReadFile(output)
	if want := "run-id=pr-review-1\nexit-code=2\nresults<<MEMDOOR_RESULTS_EOF\n.memdoor/runs/pr-review-x.md\n.memdoor/runs/pr-review-x.diff\nMEMDOOR_RESULTS_EOF\n"; string(o) != want {
		t.Errorf("outputs:\n%q\nwant\n%q", o, want)
	}
}

func TestOutsideGitHubActionsARunWritesNothing(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	t.Setenv("GITHUB_OUTPUT", "")
	reportToGitHub(workflowRunStatus{ID: "x"}, workflowExitDone) // no file named, no panic
}
