package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type fallbackStep struct {
	Command string `yaml:"command"`
	When    string `yaml:"when"`
}

func loadFallbackStep(t *testing.T, name string) fallbackStep {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("team-solve-fanout", "steps", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var s fallbackStep
	if err := yaml.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// fallbackRun is a throwaway git tree at the point keep has decided: a.txt
// holds what keep kept ("kept"), review-rule.txt holds the review's rule.
func fallbackRun(t *testing.T, rule string) (dir, r string, sh func(script string) (string, error)) {
	t.Helper()
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	r = filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-1")
	for _, d := range []string{r, filepath.Join(dir, ".memdoor", "workflows")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(wf, filepath.Join(dir, ".memdoor", "workflows", "team-solve")); err != nil {
		t.Fatal(err)
	}
	sh = func(script string) (string, error) {
		cmd := exec.Command("bash", "-c", strings.ReplaceAll(script, "{{.partition}}", "1"))
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	for _, s := range []string{
		"git init -q && git config core.autocrlf false && printf 'start\\n' > a.txt && git add a.txt && git commit -qm base",
		"git rev-parse HEAD > .memdoor/runs/team-solve-fanout-1/base.txt && : > .memdoor/runs/team-solve-fanout-1/untracked-before.txt",
		"printf 'kept\\n' > a.txt && printf 'kept: the direct attempt, untouched\\n' > .memdoor/runs/team-solve-fanout-1/keep.txt",
		"printf '" + rule + "\\n' > .memdoor/runs/team-solve-fanout-1/review-rule.txt",
	} {
		if out, err := sh(s); err != nil {
			t.Fatalf("%s: %v\n%s", s, err, out)
		}
	}
	return dir, r, sh
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The three fallback steps share one when: they run only on the review's FAIL.
func TestTeamSolveFanoutFallbackRunsOnlyWhenTheReviewFailed(t *testing.T) {
	for _, step := range []string{"fallback-reset", "fallback", "fallback-done"} {
		when := loadFallbackStep(t, step).When
		for rule, want := range map[string]bool{"FAIL": true, "PASS": false} {
			_, _, sh := fallbackRun(t, rule)
			_, err := sh(when)
			if (err == nil) != want {
				t.Errorf("%s with the review's %s: when %q ran = %v", step, rule, when, err == nil)
			}
		}
	}
}

// The review failed and the pick applied the next attempt: the tree is that
// attempt, and keep.txt says so.
func TestTeamSolveFanoutFallbackAppliesTheNextAttempt(t *testing.T) {
	dir, r, sh := fallbackRun(t, "FAIL")
	if out, err := sh(loadFallbackStep(t, "fallback-reset").Command); err != nil {
		t.Fatalf("fallback-reset: %v\n%s", err, out)
	}
	if got := read(t, filepath.Join(dir, "a.txt")); got != "start\n" {
		t.Fatalf("after fallback-reset a.txt = %q, want the tree as the run found it", got)
	}
	// What the fallback pick does: apply attempt 2, write its output.
	if out, err := sh("printf 'attempt 2\\n' > a.txt && printf '{\"winner\":2,\"by\":\"prefer\",\"leaders\":[2,3]}\\n' > .memdoor/runs/team-solve-fanout-1/fallback.json"); err != nil {
		t.Fatal(out)
	}
	if out, err := sh(loadFallbackStep(t, "fallback-done").Command); err != nil {
		t.Fatalf("fallback-done: %v\n%s", err, out)
	}
	if got := read(t, filepath.Join(dir, "a.txt")); got != "attempt 2\n" {
		t.Errorf("a.txt = %q, want attempt 2", got)
	}
	if got := read(t, filepath.Join(r, "keep.txt")); !strings.Contains(got, "fallback: the review failed; attempt 2 of the fan-out") {
		t.Errorf("keep.txt = %q", got)
	}
}

// The review failed and the pick applied nothing (no other attempt): what
// keep kept is put back, so the fallback never leaves less than keep did.
func TestTeamSolveFanoutFallbackWithNothingAppliedPutsBackWhatKeepKept(t *testing.T) {
	dir, r, sh := fallbackRun(t, "FAIL")
	if out, err := sh(loadFallbackStep(t, "fallback-reset").Command); err != nil {
		t.Fatalf("fallback-reset: %v\n%s", err, out)
	}
	if out, err := sh(loadFallbackStep(t, "fallback-done").Command); err != nil {
		t.Fatalf("fallback-done: %v\n%s", err, out)
	}
	if got := read(t, filepath.Join(dir, "a.txt")); got != "kept\n" {
		t.Errorf("a.txt = %q, want what keep kept", got)
	}
	if got := read(t, filepath.Join(r, "keep.txt")); !strings.Contains(got, "no other attempt applies; what keep kept stays") {
		t.Errorf("keep.txt = %q", got)
	}
}
