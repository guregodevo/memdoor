package gateway

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memdoor/pkg/workflow"
)

// A change request reruns the named step and every step after it that
// depends on it, in order, leaving externals and unrelated steps out.
func TestChangeChainIsTheStepAndWhatFollowsIt(t *testing.T) {
	wf := workflow.Workflow{Tasks: []workflow.Task{
		{Name: "open", Requires: []string{"pr", "review"}},
		{Name: "review", External: true},
		{Name: "pr", Requires: []string{"fix"}},
		{Name: "fix", Requires: []string{"reproduce"}},
		{Name: "reproduce", Requires: []string{"issue", "branch"}},
		{Name: "issue"},
		{Name: "branch"},
	}}
	if got := strings.Join(changeChain(wf, "fix"), " → "); got != "fix → pr → open" {
		t.Fatalf("chain: %s", got)
	}
	if got := strings.Join(changeChain(wf, "issue"), " → "); got != "issue → reproduce → fix → pr → open" {
		t.Fatalf("chain from the root: %s", got)
	}
}

// A gate shows what the run changed since it started: its commits and the
// diff stat against the base commit; nothing outside a repository.
func TestTheGateShowsWhatChanged(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "base")
	base := gitHead(dir)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc F() {}\n"), 0o644)
	git("commit", "-qam", "fix: add F")
	got := reviewOf(dir, base, time.Time{})
	if !strings.Contains(got, "fix: add F") || !strings.Contains(got, "a.go") {
		t.Fatalf("review: %q", got)
	}
	if reviewOf(t.TempDir(), gitHead(t.TempDir()), time.Time{}) != "" {
		t.Fatal("no repository, nothing to review")
	}
	// The full diff, committed and not: every line the person approves.
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc F() {}\n\nfunc G() {}\n"), 0o644)
	diff := diffOf(dir, base, time.Time{})
	if !strings.Contains(diff, "+func F() {}") || !strings.Contains(diff, "+func G() {}") {
		t.Fatalf("the gate's diff misses a change: %q", diff)
	}
	if diffOf(dir, "", time.Time{}) != "" {
		t.Fatal("no base, no diff")
	}
	// A NEW, UNTRACKED file the run made is in both (dogfood 2026-10-04: a
	// gate said "no changes" over a dozen new reports); one from before the
	// run, or an ignored one, is not.
	os.WriteFile(filepath.Join(dir, "old.txt"), []byte("before\n"), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dir, "old.txt"), old, old)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("x\n"), 0o644)
	since := time.Now().Add(-time.Minute)
	os.MkdirAll(filepath.Join(dir, "reports"), 0o755)
	os.WriteFile(filepath.Join(dir, "reports", "made.json"), []byte("{\"made\": 1}\n"), 0o644)
	review, diff := reviewOf(dir, base, since), diffOf(dir, base, since)
	if !strings.Contains(review, "new files:") || !strings.Contains(review, "reports/made.json") || strings.Contains(review, "old.txt") || strings.Contains(review, "ignored.txt") {
		t.Fatalf("review lists the run's new files only: %q", review)
	}
	if !strings.Contains(diff, `+{"made": 1}`) || strings.Contains(diff, "before") {
		t.Fatalf("diff shows the new file's lines: %q", diff)
	}
	// A big new file is named with its size, not read into the diff.
	os.WriteFile(filepath.Join(dir, "reports", "big.csv"), []byte(strings.Repeat("a,b,c\n", 300000)), 0o644)
	diff = diffOf(dir, base, since)
	if !strings.Contains(diff, "new file reports/big.csv:") || strings.Contains(diff, "+a,b,c") {
		t.Fatalf("a big new file is named, not shown: %d bytes", len(diff))
	}
}

// A cron job whose message is "/workflow run <name> [partition]" is a
// scheduled workflow; anything else is an agent turn.
func TestScheduledWorkflowMessage(t *testing.T) {
	if n, p, ok := scheduledWorkflow("/workflow run peer-comps today"); !ok || n != "peer-comps" || p != "today" {
		t.Fatalf("%q %q %v", n, p, ok)
	}
	if n, p, ok := scheduledWorkflow("  /workflow run nightly "); !ok || n != "nightly" || p != "" {
		t.Fatalf("%q %q %v", n, p, ok)
	}
	for _, m := range []string{"is CI green?", "/workflow", "/workflow stop x", "please /workflow run x"} {
		if _, _, ok := scheduledWorkflow(m); ok {
			t.Errorf("%q is not a scheduled workflow", m)
		}
	}
}

// Kept rules never become a new file in the person's tree: with no
// instruction file of its own, a project's rules go under .memdoor/ and are
// git-ignored; an existing AGENTS.md is still the one written to.
func TestKeptRulesStayOutOfTheTree(t *testing.T) {
	dir := t.TempDir()
	p := projectRulesFile(dir)
	if p != filepath.Join(dir, ".memdoor", "AGENTS.md") {
		t.Fatalf("no instruction file: rules under .memdoor, got %s", p)
	}
	ign, _ := os.ReadFile(filepath.Join(dir, ".memdoor", ".gitignore"))
	if !strings.Contains(string(ign), "AGENTS.md") {
		t.Fatalf("the kept rules are git-ignored: %s", ign)
	}
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# ours\n"), 0o644)
	if projectRulesFile(dir) != filepath.Join(dir, "AGENTS.md") {
		t.Fatal("a project with its own AGENTS.md keeps it")
	}
}
