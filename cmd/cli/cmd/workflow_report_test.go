package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
