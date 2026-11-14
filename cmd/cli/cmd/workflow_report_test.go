package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/internal/testenv"
)

// A run folder as team-solve leaves it, read into the report and the list line.
func TestARunsReportIsReadOffItsFolder(t *testing.T) {
	project := t.TempDir()
	run := filepath.Join(project, ".memdoor", "runs", "team-solve-2026-11-07T131111")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"task.md":           "# Task\n\n## Prompt\n\nCount the balanced substrings.\n\n## Deliverable\n\nsolution.py\n",
		"pick.txt":          "picked: attempt c (3 checks that fail on the tree as found; points: a 1, b 1, c 2; a tie goes to the smaller change)\n",
		"direct-status.txt": "0\n",
		"check-kind.txt":    "command+repro\n",
		"review.md":         "verdict: PASS\nThe count uses prefix differences.\n",
		"verdict.md":        "verdict: PASS\nDoes what the prompt asks.\n",
		"keep.txt":          "kept: the direct attempt, untouched\n",
		"final-check.log":   "check: python3 check.py\n5 of 5 pass\n",
		"changes.diff":      "diff --git a/solution.py b/solution.py\nnew file mode 100644\n--- /dev/null\n+++ b/solution.py\n@@ -0,0 +1 @@\n+print(1)\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(run, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(project, ".memdoor", "runs", "team-solve-2026-11-06T090000"), 0o755); err != nil {
		t.Fatal(err)
	}
	dirs, err := runFolders(project)
	if err != nil || len(dirs) != 2 || filepath.Base(dirs[0]) != "team-solve-2026-11-07T131111" {
		t.Fatalf("newest first: %v, %v", dirs, err)
	}
	r := readRunReport(dirs[0])
	line := r.line()
	for _, want := range []string{"team-solve-2026-11-07T131111", "2026-11-07 13:11:11", "verdict PASS", "kept: the direct attempt, untouched", "1 file(s) changed"} {
		if !strings.Contains(line, want) {
			t.Errorf("the list line lacks %q: %s", want, line)
		}
	}
	var out bytes.Buffer
	r.write(&out)
	for _, want := range []string{"Count the balanced substrings.", "picked: attempt c", "check-kind: command+repro", "5 of 5 pass", "## The review", "## The judge", "kept: the direct attempt", "+print(1)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, out.String())
		}
	}
	empty := readRunReport(dirs[1])
	if empty.verdictLine() != "none" || !strings.Contains(empty.line(), "no keep") {
		t.Errorf("a folder with nothing in it: %s", empty.line())
	}
}

// The engine's own tree of step outputs, .memdoor/runs/<workflow>/<group>/<task>/<partition>, is not a
// run: it holds directories only. Written to last, it sorted first, and `memdoor workflow report` with no
// run named printed a header and nothing else (a pull request body on 2026-11-14).
func TestTheStepOutputsTreeIsNotARun(t *testing.T) {
	project := t.TempDir()
	run := filepath.Join(project, ".memdoor", "runs", "team-solve-2026-11-14T124825")
	outputs := filepath.Join(project, ".memdoor", "runs", "team-solve", "steps", "keep")
	for _, d := range []string{run, outputs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(run, "keep.txt"), []byte("kept: the direct attempt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputs, "2026-11-14T124825"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs, err := runFolders(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 || filepath.Base(dirs[0]) != "team-solve-2026-11-14T124825" {
		t.Fatalf("runs = %v, want only the run's folder", dirs)
	}
}

// Each step's model calls are summed from the ledger by the step's session:
// this run's only (not another run's, not a window's turn), a resumed
// trigger's included, in the order the steps first called a model. Cost is
// what the provider reported, "-" when none. Mutation check: drop the run
// match in runSteps and the other run's 999 tokens are counted.
func TestARunsReportSumsEachStepsCallsFromTheLedger(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	ledger := strings.Join([]string{
		`{"session":"workflow:team-solve-fanout:2026-11-14T204149@20261114-204149:direct-a","input_tokens":1000,"cached_tokens":800,"output_tokens":100,"duration_ms":2000,"cost_usd":0.01}`,
		`{"session":"workflow:team-solve-fanout:2026-11-14T204149@20261114-204149:direct-b","input_tokens":2000,"output_tokens":200,"duration_ms":3000}`,
		`{"session":"workflow:team-solve-fanout:2026-11-14T204149@20261114-210000-2:direct-a","input_tokens":500,"output_tokens":50,"duration_ms":1000,"cost_usd":0.005}`,
		`{"session":"workflow:team-solve-fanout:2026-11-13T090000@20261113-090000:direct-a","input_tokens":999,"output_tokens":999}`,
		`{"session":"ws:chan","input_tokens":999,"output_tokens":999}`,
	}, "\n") + "\n"
	if err := os.MkdirAll(filepath.Join(home, ".memdoor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".memdoor", "meter.jsonl"), []byte(ledger), 0o600); err != nil {
		t.Fatal(err)
	}
	run := filepath.Join(t.TempDir(), ".memdoor", "runs", "team-solve-fanout-2026-11-14T204149")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	r := readRunReport(run)
	if len(r.Steps) != 0 {
		t.Fatalf("the run list reads no ledger: %+v", r.Steps)
	}
	r.readSteps()
	want := []stepCost{
		{Task: "direct-a", Calls: 2, Priced: 2, In: 1500, Cached: 800, Out: 150, MS: 3000, Cost: 0.015},
		{Task: "direct-b", Calls: 1, In: 2000, Out: 200, MS: 3000},
	}
	if len(r.Steps) != len(want) {
		t.Fatalf("steps = %+v", r.Steps)
	}
	for i := range want {
		got := r.Steps[i]
		got.Cost = float64(int(got.Cost*1e6+0.5)) / 1e6
		if got != want[i] {
			t.Errorf("step %d = %+v, want %+v", i, got, want[i])
		}
	}
	var out bytes.Buffer
	r.write(&out)
	for _, want := range []string{"## The steps", "direct-a", "0.0150", "total", "3500", "0.0150*", "- where it reported none"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, out.String())
		}
	}
}
