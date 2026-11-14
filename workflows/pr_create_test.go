package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// prStepCommand is a pr-create step's command, with its base arg set to main.
func prStepCommand(t *testing.T, step string) string {
	t.Helper()
	var def struct {
		Command string `yaml:"command"`
	}
	raw, err := os.ReadFile(filepath.Join("pr-create", "steps", step+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(def.Command, "{{.base}}", "main")
}

// prGit is a repository on a branch `topic`, with a bare origin whose main it was cut from.
func prGit(t *testing.T) string {
	t.Helper()
	remote, dir := t.TempDir(), t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	gitIn(t, dir, "init", "-q", "-b", "main")
	writeAll(t, map[string]string{filepath.Join(dir, "a.txt"): "one\n"}, 0o644)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "base")
	gitIn(t, dir, "remote", "add", "origin", remote)
	gitIn(t, dir, "push", "-q", "origin", "main")
	gitIn(t, dir, "checkout", "-q", "-b", "topic")
	return dir
}

func commitOn(t *testing.T, dir, branch, file, text string) {
	t.Helper()
	gitIn(t, dir, "checkout", "-q", branch)
	writeAll(t, map[string]string{filepath.Join(dir, file): text}, 0o644)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "change "+file)
}

func runStep(t *testing.T, dir, command string) (string, int) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), exitOf(err, t)
}

// ahead: a branch with nothing beyond the base has no pull request to open; one commit is enough.
func TestPRCreateAheadNeedsACommitOfItsOwn(t *testing.T) {
	dir := prGit(t)
	if out, code := runStep(t, dir, prStepCommand(t, "ahead")); code == 0 {
		t.Fatalf("a branch with no commit of its own passed:\n%s", out)
	}
	commitOn(t, dir, "topic", "b.txt", "topic\n")
	if out, code := runStep(t, dir, prStepCommand(t, "ahead")); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
}

// merges: a branch behind the base still merges; one that edits the line the base edited does not, and the
// file is named.
func TestPRCreateMergesNamesTheConflictingFile(t *testing.T) {
	dir := prGit(t)
	commitOn(t, dir, "topic", "b.txt", "topic\n")
	commitOn(t, dir, "main", "c.txt", "base moved\n")
	gitIn(t, dir, "push", "-q", "origin", "main")
	gitIn(t, dir, "checkout", "-q", "topic")
	gitIn(t, dir, "fetch", "-q", "origin", "main")
	if out, code := runStep(t, dir, prStepCommand(t, "merges")); code != 0 {
		t.Fatalf("a branch that is only behind did not merge: exit %d:\n%s", code, out)
	}
	commitOn(t, dir, "topic", "a.txt", "topic edit\n")
	commitOn(t, dir, "main", "a.txt", "base edit\n")
	gitIn(t, dir, "push", "-q", "origin", "main")
	gitIn(t, dir, "checkout", "-q", "topic")
	gitIn(t, dir, "fetch", "-q", "origin", "main")
	out, code := runStep(t, dir, prStepCommand(t, "merges"))
	if code != 1 || !strings.Contains(out, "a.txt") {
		t.Fatalf("a conflict: exit %d:\n%s", code, out)
	}
}

// pr-create ships no script: every step is YAML built from mario's features.
func TestPRCreateShipsNoScript(t *testing.T) {
	var scripts []string
	filepath.Walk("pr-create", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(p) != ".yaml" && filepath.Base(p) != "README.md" {
			scripts = append(scripts, p)
		}
		return nil
	})
	if len(scripts) != 0 {
		t.Fatalf("pr-create holds more than its README and its steps: %v", scripts)
	}
}
