package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// backlogStep is a team-backlog step as written: its when and its command.
func backlogStep(t *testing.T, name string) (when, command string) {
	t.Helper()
	var def struct {
		When    string `yaml:"when"`
		Command string `yaml:"command"`
	}
	raw, err := os.ReadFile(filepath.Join("team-backlog", "steps", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	return def.When, def.Command
}

// runWithGH runs a shell line with a fake gh first on the PATH; the fake logs its arguments and answers
// with script.
func runWithGH(t *testing.T, line, script string) (out, calls string, code int) {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.txt")
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho \"$*\" >> '"+log+"'\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", line)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	b, err := cmd.CombinedOutput()
	c, _ := os.ReadFile(log)
	return string(b), string(c), exitOf(err, t)
}

// next asks GitHub for the oldest open issue nobody is on: unassigned, no pull request linked, not
// commented on by this account, with the label when one is set. Its output is the number.
func TestBacklogNextAsksGitHubForTheOldestFreeIssue(t *testing.T) {
	_, command := backlogStep(t, "next")
	plain := strings.NewReplacer(`{{ if .label }}label:"{{.label}}" {{ end }}`, "").Replace(command)
	out, calls, code := runWithGH(t, plain, "echo 27\n")
	if code != 0 || strings.TrimSpace(out) != "27" {
		t.Fatalf("exit %d, output %q", code, out)
	}
	for _, want := range []string{"issue list", "no:assignee", "-linked:pr", "-commenter:@me", "sort:created-asc", "--limit 1"} {
		if !strings.Contains(calls, want) {
			t.Errorf("gh was not asked for %q:\n%s", want, calls)
		}
	}
	labelled := strings.NewReplacer(`{{ if .label }}label:"{{.label}}" {{ end }}`, `label:"bug" `).Replace(command)
	if _, calls, _ := runWithGH(t, labelled, "echo 27\n"); !strings.Contains(calls, `label:"bug"`) {
		t.Errorf("the label is not in the search:\n%s", calls)
	}
}

// next's when is the cap: at max_open_prs pull requests of this account open, nothing starts.
func TestBacklogNextStopsAtTheCap(t *testing.T) {
	when, _ := backlogStep(t, "next")
	line := strings.ReplaceAll(when, "{{.max_open_prs}}", "5")
	if _, _, code := runWithGH(t, line, "echo 4\n"); code != 0 {
		t.Fatal("4 open of 5: it should start")
	}
	if _, calls, code := runWithGH(t, line, "echo 5\n"); code == 0 || !strings.Contains(calls, "--author @me") {
		t.Fatalf("5 open of 5: it should not start (exit %d)\n%s", code, calls)
	}
}

// team-backlog ships no script: a README and its YAML steps.
func TestBacklogShipsNoScript(t *testing.T) {
	var other []string
	filepath.Walk("team-backlog", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(p) != ".yaml" && filepath.Base(p) != "README.md" {
			other = append(other, p)
		}
		return nil
	})
	if len(other) != 0 {
		t.Fatalf("team-backlog holds more than its README and its steps: %v", other)
	}
}

// Every command of team-backlog and pr-create is written as a block. In a one-line YAML value a space then
// # starts a comment, so a command holding one (a jq string, `(issue #...)`) is cut off mid-quote: the
// first real run of team-backlog failed at its task step with an unmatched quote. A block cannot be cut.
func TestCommandsAreWrittenAsBlocks(t *testing.T) {
	for _, wf := range []string{"team-backlog", "pr-create"} {
		steps, _ := filepath.Glob(filepath.Join(wf, "steps", "*.yaml"))
		if len(steps) == 0 {
			t.Fatalf("no steps in %s", wf)
		}
		for _, f := range steps {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.HasPrefix(line, "command: ") && line != "command: |" {
					t.Errorf("%s: write the command as a block (command: | and the line under it): %s", f, line)
				}
			}
		}
	}
	_, command := backlogStep(t, "task")
	if !strings.Contains(command, "(issue #") {
		t.Fatalf("the task command lost its end: %q", command)
	}
}

// One pull request at a time by default: with more, two issues about the same lines are worked from the
// same base and their pull requests conflict.
func TestBacklogTakesOneIssueAtATimeByDefault(t *testing.T) {
	var def struct {
		Args map[string]string `yaml:"args"`
	}
	raw, err := os.ReadFile(filepath.Join("team-backlog", "steps", "next.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	if def.Args["max_open_prs"] != "1" {
		t.Fatalf("max_open_prs defaults to %q, want 1", def.Args["max_open_prs"])
	}
	when, _ := backlogStep(t, "next")
	line := strings.ReplaceAll(when, "{{.max_open_prs}}", def.Args["max_open_prs"])
	if _, _, code := runWithGH(t, line, "echo 0\n"); code != 0 {
		t.Fatal("no pull request open: it should start")
	}
	if _, _, code := runWithGH(t, line, "echo 1\n"); code == 0 {
		t.Fatal("one pull request open: it should not start")
	}
}
