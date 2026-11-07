package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// keep.yaml's command, run in a git tree whose run folder says what the
// direct attempt, the Check and the judge did. The tree holds the work as the
// last step left it; direct.patch is the attempt as saved before the team.
func runKeep(t *testing.T, direct, final, verdict string, repaired bool, tree string, repro bool) (keep, file string) {
	t.Helper()
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	var step struct {
		Command string `yaml:"command"`
	}
	raw, err := os.ReadFile(filepath.Join(wf, "steps", "keep.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &step); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	r := filepath.Join(dir, ".memdoor", "runs", "team-solve-1")
	if err := os.MkdirAll(r, 0o755); err != nil {
		t.Fatal(err)
	}
	// The workflow's scripts where keep looks for them.
	if err := os.MkdirAll(filepath.Join(dir, ".memdoor", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wf, filepath.Join(dir, ".memdoor", "workflows", "team-solve")); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if !filepath.IsAbs(name) {
			name = filepath.Join(dir, name)
		}
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("a.txt", "start\n")
	git("add", "a.txt")
	git("commit", "-q", "-m", "base")
	write(filepath.Join(r, "base.txt"), git("rev-parse", "HEAD")+"\n")
	write(filepath.Join(r, "untracked-before.txt"), "")
	// The direct attempt, saved before the team; then the tree as the repair left it.
	write("a.txt", "direct\n")
	save := exec.Command("bash", filepath.Join(wf, "snapshot.sh"), "save", r, "direct")
	save.Dir = dir
	if out, err := save.CombinedOutput(); err != nil {
		t.Fatalf("snapshot save: %v\n%s", err, out)
	}
	write("a.txt", tree+"\n")
	write(filepath.Join(r, "direct-status.txt"), direct+"\n")
	write(filepath.Join(r, "final-status.txt"), final+"\n")
	write(filepath.Join(r, "verdict.md"), verdict)
	// What the DAG does between judge and keep (#160): judge-rule reads the
	// verdict when there is one, and judge-score, with no decision model on
	// this host, passes its answer through.
	if strings.TrimSpace(verdict) != "" {
		var rule struct {
			Command string `yaml:"command"`
		}
		raw, err := os.ReadFile(filepath.Join(wf, "steps", "judge-rule.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := yaml.Unmarshal(raw, &rule); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("sh", "-c", strings.ReplaceAll(rule.Command, "{{.partition}}", "1"))
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("judge-rule: %v\n%s", err, out)
		}
		score := `{"p":0,"threshold":0.7,"pass":false,"by":"judge-rule"}`
		if strings.TrimSpace(string(out)) == "PASS" {
			score = `{"p":1,"threshold":0.7,"pass":true,"by":"judge-rule"}`
		}
		write(filepath.Join(r, "judge-score.json"), score+"\n")
	}
	if repaired {
		write(filepath.Join(r, "repair.md"), "# Repair\n\nchanged a.txt\n")
	}
	if repro {
		// A reproduction that failed on the tree as found and passes now (check.sh).
		write(filepath.Join(r, "check-kind.txt"), "command+repro\n")
		write(filepath.Join(r, "check-parts.txt"), "command 0\nrepro 0\n")
		write(filepath.Join(r, "repro-baseline-status.txt"), "1\n")
		if err := os.WriteFile(filepath.Join(r, "repro"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", "-c", strings.ReplaceAll(step.Command, "{{.partition}}", "1"))
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("keep: %v\n%s", err, out)
	}
	k, err := os.ReadFile(filepath.Join(r, "keep.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(k)), strings.TrimSpace(string(f))
}

// The net, one rule for both paths: what replaces the direct attempt has to
// pass the Check with no FAIL from the judge, or, with no Check to run, get
// the judge's PASS. A repair after the review's FAIL is held to it like the
// team's work, not discarded (#152: on SWE-bench Verified 4 of the 13 repairs
// keep discarded resolved the issue, and no direct attempt they replaced did).
// The direct attempt passed the same Check, so a repair replaces it only when
// the Check has a reproduction that tells them apart: with the project's tests
// alone the review failed a correct attempt and the repair that replaced it
// was wrong (astropy-13236, 2026-11-07).
func TestTeamSolveKeepHoldsTheRepairToTheSameProof(t *testing.T) {
	for _, c := range []struct {
		name, direct, final, verdict string
		repaired                     bool
		tree                         string // a.txt as the last step left it
		repro                        bool   // the Check has a reproduction that passes now
		keep, file                   string
	}{
		{"a repair the Check and its reproduction pass, the judge does not fail", "0", "0", "verdict: PASS\n", true, "repair", true, "kept: the repair", "repair"},
		{"a repair the Check and its reproduction pass, no verdict", "0", "0", "I think it is fine.\n", true, "repair", true, "kept: the repair", "repair"},
		{"a repair the project's tests pass, no reproduction", "0", "0", "verdict: PASS\n", true, "repair", false, "put back: the direct attempt, which passed test-0", "direct"},
		{"a repair with no Check to run and the judge's PASS", "0", "none", "verdict: PASS\n", true, "repair", false, "put back: the direct attempt, which passed test-0", "direct"},
		{"a repair the judge fails", "0", "0", "verdict: FAIL\n", true, "repair", true, "put back: the direct attempt, which passed test-0", "direct"},
		{"a repair that breaks the Check", "0", "1", "verdict: PASS\n", true, "repair", true, "put back: the direct attempt, which passed test-0", "direct"},
		{"a repair with no Check and no verdict", "0", "none", "", true, "repair", false, "put back: the direct attempt, which passed test-0", "direct"},
		{"the team's work the Check passes", "1", "0", "", false, "repair", false, "kept: the team's work", "repair"},
		{"the team's work the Check fails", "1", "1", "verdict: PASS\n", false, "repair", false, "put back: the direct attempt (the team's work did not prove better", "direct"},
		{"a direct attempt nothing touched", "0", "", "", false, "direct", false, "kept: the direct attempt, untouched", "direct"},
		// A repair step that failed (its clock) wrote no repair.md and changed the tree: the tree says so.
		{"a failed repair that changed the tree", "0", "", "", false, "repair", false, "put back: the direct attempt, which passed test-0", "direct"},
		{"a failed repair that changed the tree and passed the Check and its reproduction", "0", "0", "verdict: PASS\n", false, "repair", true, "kept: the repair", "repair"},
	} {
		keep, file := runKeep(t, c.direct, c.final, c.verdict, c.repaired, c.tree, c.repro)
		if !strings.HasPrefix(keep, c.keep) || file != c.file {
			t.Errorf("%s: keep.txt %q and a.txt %q, want %q... and %q", c.name, keep, file, c.keep, c.file)
		}
	}
}
