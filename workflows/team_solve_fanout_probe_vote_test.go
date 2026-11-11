package workflows

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// voteRun is a throwaway git tree at its base commit and a run folder whose
// tied attempts each change a.txt to the given text; a probe prints a.txt.
func voteRun(t *testing.T, texts []string, probe bool) (dir, r string) {
	t.Helper()
	dir = t.TempDir()
	r = filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-1")
	run := func(args ...string) string {
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
	var attempts []map[string]any
	points := map[string]int{}
	for i, text := range texts {
		n := i + 1
		attempts = append(attempts, map[string]any{"n": n, "ok": true,
			"patch": "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-start\n+" + text + "\n"})
		points[strings.TrimSpace(string(rune('0'+n)))] = 2
	}
	b, _ := json.Marshal(map[string]any{"attempts": attempts, "points": points})
	write(filepath.Join(dir, "fanout.json"), string(b), 0o644)
	if probe {
		write(filepath.Join(r, "probe"), "#!/bin/sh\necho \"tree: $(pwd)\"\ncat a.txt\n", 0o755)
	}
	return dir, r
}

func selector(t *testing.T, dir string, args ...string) string {
	t.Helper()
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", append([]string{filepath.Join(wf, "selector.py")}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("selector.py %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// probe-run names the lowest attempt of the largest group that printed alike,
// none on a tie or with no probe; the copy's path, which differs per attempt,
// does not split a group; the tree is left as found.
func TestTeamSolveFanoutProbeVote(t *testing.T) {
	for _, c := range []struct {
		name  string
		texts []string
		probe bool
		want  string
	}{
		{"two alike outnumber one", []string{"wrong", "right", "right"}, true, "attempt 2"},
		{"the largest groups tie", []string{"a", "b"}, true, "none"},
		{"no probe", []string{"wrong", "right", "right"}, false, "none"},
	} {
		dir, r := voteRun(t, c.texts, c.probe)
		selector(t, dir, "attempts", r, filepath.Join(dir, "fanout.json"))
		if got := selector(t, dir, "run", r); got != c.want {
			t.Errorf("%s: run said %q, want %q", c.name, got, c.want)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(got) != "start\n" {
			t.Errorf("%s: the tree changed: a.txt = %q", c.name, got)
		}
	}
}
