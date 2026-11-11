package workflows

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// selectorRun is a throwaway git tree at its base commit, a run folder with
// two tied attempts (a.txt changed two ways), a probe that prints a.txt, and
// a reproduction that passes only when a.txt says "right".
func selectorRun(t *testing.T) (dir, r string, run func(args ...string) string) {
	t.Helper()
	dir = t.TempDir()
	r = filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-1")
	run = func(args ...string) string {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	write := func(p, text string, mode os.FileMode) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "a.txt"), "start\n", 0o644)
	write(filepath.Join(dir, ".gitignore"), ".memdoor/\n", 0o644)
	run("git", "init", "-q")
	run("git", "add", "-A")
	run("git", "commit", "-qm", "base")
	write(filepath.Join(r, "base.txt"), strings.TrimSpace(run("git", "rev-parse", "HEAD"))+"\n", 0o644)
	patch := func(text string) string {
		return "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-start\n+" + text + "\n"
	}
	fanout := map[string]any{
		"attempts": []map[string]any{
			{"n": 1, "ok": true, "patch": patch("wrong")},
			{"n": 2, "ok": true, "patch": patch("right")},
			{"n": 3, "ok": true, "patch": patch("wrong")},
		},
		"points": map[string]int{"1": 2, "2": 2, "3": 2},
	}
	b, _ := json.Marshal(fanout)
	write(filepath.Join(dir, "fanout.json"), string(b), 0o644)
	write(filepath.Join(r, "probe"), "#!/bin/sh\ncat a.txt\n", 0o755)
	write(filepath.Join(r, "repro"), "#!/bin/sh\ngrep -qx right a.txt\n", 0o755)
	return dir, r, run
}

// attempts unpacks the tie; run writes, for each leader, its change and what
// the probe and the reproduction did, and decides nothing: two attempts that
// share a mistake are not outvoting the right one.
func TestTeamSolveFanoutSelectorEvidence(t *testing.T) {
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	dir, r, run := selectorRun(t)
	run("python3", filepath.Join(wf, "selector.py"), "attempts", r, filepath.Join(dir, "fanout.json"))
	if got, _ := os.ReadFile(filepath.Join(r, "tie.txt")); strings.TrimSpace(string(got)) != "differ" {
		t.Fatalf("tie.txt = %q, want differ", got)
	}
	out := run("python3", filepath.Join(wf, "selector.py"), "run", r)
	if strings.Contains(out, "attempt ") && !strings.Contains(out, "evidence for attempts") {
		t.Errorf("run named an attempt: %q", out)
	}
	ev, err := os.ReadFile(filepath.Join(r, "selector-evidence.md"))
	if err != nil {
		t.Fatal(err)
	}
	e := string(ev)
	for _, want := range []string{"## Attempt 1", "## Attempt 2", "## Attempt 3", "+right", "the probe: exit 0\nright", "the reproduction: exit 0", "the reproduction: exit 1"} {
		if !strings.Contains(e, want) {
			t.Errorf("selector-evidence.md lacks %q:\n%s", want, e)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(got) != "start\n" {
		t.Errorf("the tree changed: a.txt = %q", got)
	}
}
