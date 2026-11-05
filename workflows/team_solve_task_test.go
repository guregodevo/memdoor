package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// team-solve's task.md is filled and settled by bash and awk alone, whatever
// the locale: a non-ASCII prompt under LC_ALL=C, with no Python on the PATH,
// comes through byte for byte (#110: a Python 3.6 with an ASCII locale failed
// the problem step at once, and the run did nothing).
func TestTeamSolveTaskNeedsNoPythonAndKeepsAnyText(t *testing.T) {
	bin := t.TempDir()
	for _, tool := range []string{"bash", "awk", "grep", "sed", "sort", "tr", "mkdir", "mv", "rm", "cat"} {
		p, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s not found: %v", tool, err)
		}
		if err := os.Symlink(p, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
	for _, d := range []string{r, filepath.Join(dir, "pkg", "x")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "pkg", "x", "a.go"), "package x\n")
	prompt := "\nFix pkg/x/a.go — the café test says “hi”.\n## Context\nNot in nope.txt.\nCheck: go test ./pkg/x/ passes.\n\n"
	write(filepath.Join(r, "prompt.md"), prompt)
	// The planner left out the Deliverable, the Done when, and a runnable Check and Scope.
	write(filepath.Join(r, "plan.md"), "## Check\nrun the tests\n\n## Scope\nthe package\n")

	run := func(script string, args ...string) string {
		t.Helper()
		cmd := exec.Command(filepath.Join(bin, "bash"), append([]string{filepath.Join(wf, script)}, args...)...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + bin, "LC_ALL=C", "LANG=C", "HOME=" + dir}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", script, err, out)
		}
		return string(out)
	}
	run("task.sh", filepath.Join(wf, "task.template.md"), filepath.Join(r, "prompt.md"), filepath.Join(r, "task.md"))
	out := run("settle.sh", r)

	b, err := os.ReadFile(filepath.Join(r, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	task := string(b)
	for _, want := range []string{
		"Fix pkg/x/a.go — the café test says “hi”.", // every byte as written
		"\n### Context\n", // the prompt's heading one level down
		"## Deliverable\n\nchanges to this tree\n",
		"## Check\n\n`go test ./pkg/x/`\n", // the Check the prompt names
		"`pkg/x/a.go`",                     // a path the prompt names that exists
		"## Done when\n\nThe Prompt is done as written.\n",
	} {
		if !strings.Contains(task, want) {
			t.Fatalf("task.md lacks %q:\n%s", want, task)
		}
	}
	if strings.Contains(task, "nope.txt`") || strings.Contains(task, "@@PROMPT@@") {
		t.Fatalf("task.md names a path that does not exist, or kept the placeholder:\n%s", task)
	}
	if !strings.Contains(out, "Check (filled safely)") || !strings.Contains(out, "Scope (filled safely)") {
		t.Fatalf("settle did not say what it filled: %s", out)
	}
}
