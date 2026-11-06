package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A step's tool calls are one line each in the run view (#94): what a call
// read, changed or ran, and how it ended; its beats and its end replace its
// start, by the call's id.
func TestAStepsCallsAreOneLineEach(t *testing.T) {
	dir := "/work/proj"
	var c stepCalls
	type ev = map[string]interface{}
	steps := []struct {
		data      ev
		key, line string
	}{
		{ev{"event": "start", "id": "c1", "tool": "apply_patch", "input": `{"input":"*** Begin Patch\n*** Update File: /work/proj/scripts/scan.sh\n@@\n-a\n+b\n*** End Patch"}`}, "c1", "✎ scripts/scan.sh"},
		{ev{"event": "complete", "id": "c1", "tool": "apply_patch", "output": "Success. Updated: scripts/scan.sh"}, "c1", "✎ scripts/scan.sh"},
		{ev{"event": "start", "id": "c2", "tool": "bash", "input": `{"command":"bash scripts/scan.sh\necho done"}`}, "c2", "$ bash scripts/scan.sh …"},
		{ev{"event": "update", "id": "c2", "tool": "bash", "output": "partial"}, "", ""},
		{ev{"event": "progress", "id": "c2", "tool": "bash", "seconds": 4}, "c2", "$ bash scripts/scan.sh … · 4s"},
		{ev{"event": "complete", "id": "c2", "tool": "bash", "output": "Command FAILED (exit status 2): `bash scripts/scan.sh`\nOutput: no"}, "c2", "$ bash scripts/scan.sh … → exit 2"},
		{ev{"event": "start", "id": "c3", "tool": "bash", "input": `{"command":"go test ./..."}`}, "c3", "$ go test ./..."},
		{ev{"event": "complete", "id": "c3", "tool": "bash", "output": "ok"}, "c3", "$ go test ./... → exit 0"},
		{ev{"event": "start", "id": "c4", "tool": "read_file", "input": `{"path":"task.md"}`}, "c4", "read task.md"},
		{ev{"event": "complete", "id": "c4", "tool": "read_file", "error": "no such file"}, "c4", "read task.md ✗ no such file"},
		{ev{"event": "start", "id": "c5", "tool": "write_file", "input": `{"path":"/work/proj/review.md"}`}, "c5", "✎ review.md"},
		{ev{"event": "start", "id": "c6", "tool": "todo_write", "input": `{}`}, "c6", "⏺ todo_write"},
		{ev{"type": "todo"}, "", ""},
		{ev{"event": "progress", "id": "unknown", "tool": "bash", "seconds": 9}, "", ""},
	}
	for _, s := range steps {
		key, line := c.line(dir, s.data)
		if key != s.key || line != s.line {
			t.Errorf("%v: got %q %q, want %q %q", s.data, key, line, s.key, s.line)
		}
	}
}

// A finished step leaves the file it owed, with its verdict when the file
// opens with one, as team-solve's verdict.sh reads it.
func TestAFinishedStepsOutcomeIsItsFileAndVerdict(t *testing.T) {
	dir := t.TempDir()
	r := filepath.Join(dir, ".memdoor", "runs", "team-solve-p1")
	if err := os.MkdirAll(r, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(r, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("review.md", "# Review\n\n**Verdict: pass**\nfine\n")
	write("plan.md", "the plan\n")
	write("proposal-b.md", "idea\n")
	for _, c := range []struct{ name, target, want string }{
		{"review", "file .memdoor/runs/team-solve-{{.partition}}/review.md", "review.md: verdict: PASS"},
		{"plan", "file .memdoor/runs/team-solve-{{.partition}}/plan.md", "wrote plan.md"},
		{"ideate-b", "file .memdoor/runs/team-solve-{{.partition}}/proposal-{{.item}}.md", "wrote proposal-b.md"},
		{"judge", "file .memdoor/runs/team-solve-{{.partition}}/verdict.md", ""},
		{"test", "command: bash check.sh", ""},
	} {
		if got := stepOutcome(dir, "p1", c.name, c.target); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// A team run's end tells the conversation the verdicts, what keep kept, the
// Scope report and the diff (#94); any other workflow's end is as before.
func TestATeamRunsEndHasTheVerdictTheScopeAndTheDiff(t *testing.T) {
	dir := t.TempDir()
	r := filepath.Join(dir, ".memdoor", "runs", "team-solve-p1")
	if err := os.MkdirAll(r, 0o755); err != nil {
		t.Fatal(err)
	}
	var diff strings.Builder
	diff.WriteString("diff --git a/scripts/scan.sh b/scripts/scan.sh\n")
	for i := 0; i < 70; i++ {
		diff.WriteString("+line\n")
	}
	for name, body := range map[string]string{
		"verdict.md":   "verdict: PASS\nit does what the task asks\n",
		"review.md":    "verdict: FAIL\n",
		"keep.txt":     "kept: the team's work (check 0, judge PASS)\n",
		"scope.txt":    "scope: scripts/scan.sh\nchanged: scripts/scan.sh +1 -1\n",
		"changes.diff": diff.String(),
	} {
		if err := os.WriteFile(filepath.Join(r, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := teamReport(&workflowRun{Workflow: teamWorkflow, Dir: dir, Partition: "p1"})
	for _, want := range []string{
		"verdict: judge PASS · review FAIL",
		"kept: the team's work (check 0, judge PASS)",
		"Scope:\n    scope: scripts/scan.sh\n    changed: scripts/scan.sh +1 -1",
		"The diff:\n    diff --git a/scripts/scan.sh b/scripts/scan.sh",
		"… 11 more lines in .memdoor/runs/team-solve-p1/changes.diff",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if teamReport(&workflowRun{Workflow: "pr-review", Dir: dir, Partition: "p1"}) != "" {
		t.Error("another workflow got a team report")
	}
}
