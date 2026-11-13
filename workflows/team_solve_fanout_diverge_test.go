package workflows

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type divergeStep struct {
	Type     string `yaml:"type"`
	When     string `yaml:"when"`
	System   string `yaml:"system"`
	Requires []struct {
		TablePattern string `yaml:"table_pattern"`
	} `yaml:"requires"`
	Prompt string `yaml:"prompt"`
	Target struct {
		File string `yaml:"file"`
	} `yaml:"target"`
}

func readDivergeStep(t *testing.T, name string) divergeStep {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("team-solve-fanout", "steps", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var s divergeStep
	if err := yaml.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// diverge reads the fan-out's saved record and writes the deciding cases;
// rework runs only on its output; test judges whichever turn (solve or
// rework) worked the tree.
func TestTeamSolveFanoutDivergeReadsTheFanOutAndReworkCarriesIt(t *testing.T) {
	dv := readDivergeStep(t, "diverge")
	if dv.Type != "llm" || !dv.requires("direct") || !dv.requires("test-0") {
		t.Errorf("diverge: type %q, requires direct %v test-0 %v; want llm, true, true", dv.Type, dv.requires("direct"), dv.requires("test-0"))
	}
	if !strings.Contains(dv.Prompt, `{{ output "direct" }}`) {
		t.Error("diverge: the prompt does not carry the fan-out's saved record")
	}
	if !strings.Contains(dv.Target.File, "/divergence.md") {
		t.Errorf("diverge: target %q, want the run's divergence.md", dv.Target.File)
	}
	if dv.When == "" || !strings.Contains(dv.When, "direct-status.txt") {
		t.Error("diverge: it does not run only when the picked attempt did not pass")
	}
	rw := readFanoutStep(t, "rework")
	if rw.Type != "agent" || !rw.Optional {
		t.Errorf("rework: type %q optional %v; want agent, true", rw.Type, rw.Optional)
	}
	raw, err := os.ReadFile(filepath.Join("team-solve-fanout", "steps", "rework.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "table_pattern: diverge") {
		t.Error("rework does not wait for diverge")
	}
	if !strings.Contains(rw.Prompt, "divergence.md") {
		t.Error("rework: the turn is not told to read divergence.md")
	}
	test := readFanoutStep(t, "test")
	if !test.requires("rework") {
		t.Error("test does not accept the rework turn as its work")
	}
}

// The rework turn's target is done.sh's solve receipt: the same gate the
// solver passes, so the Check judges its tree as it judges the solver's.
func TestTeamSolveFanoutReworkTargetsTheSolveReceipt(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("team-solve-fanout", "steps", "rework.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "done.sh solve") {
		t.Error("rework: target is not done.sh solve")
	}
}

// diverge and rework stay out of the run when the pick passed: the when of
// diverge fails on direct-status.txt = 0, so divergence.md is never written
// and rework's when finds no file.
func TestTeamSolveFanoutDivergeStaysOutOfAPassingRun(t *testing.T) {
	wf, err := filepath.Abs("team-solve")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r := filepath.Join(dir, ".memdoor", "runs", "team-solve-fanout-1")
	for _, d := range []string{r, filepath.Join(dir, ".memdoor", "workflows")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(wf, filepath.Join(dir, ".memdoor", "workflows", "team-solve")); err != nil {
		t.Fatal(err)
	}
	sh := func(script string) (string, error) {
		cmd := exec.Command("bash", "-c", strings.ReplaceAll(script, "{{.partition}}", "1"))
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := sh("printf '0\\n' > .memdoor/runs/team-solve-fanout-1/direct-status.txt"); err != nil {
		t.Fatal(err, out)
	}
	dv := readDivergeStep(t, "diverge")
	if _, err := sh(dv.When); err == nil {
		t.Error("diverge ran on a passing pick (direct-status.txt = 0)")
	}
	if _, err := sh(`test -s .memdoor/runs/team-solve-fanout-1/divergence.md`); err == nil {
		t.Error("divergence.md exists with no diverge step run")
	}
	rw := readFanoutStep(t, "rework")
	if rw.Type != "agent" || !rw.Optional {
		t.Errorf("rework: type %q optional %v; want agent, true", rw.Type, rw.Optional)
	}
	raw, err := os.ReadFile(filepath.Join("team-solve-fanout", "steps", "rework.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		When string `yaml:"when"`
	}
	if err := yaml.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if _, err := sh(s.When); err == nil {
		t.Error("rework ran with no divergence.md")
	}
}

func (s divergeStep) requires(name string) bool {
	for _, r := range s.Requires {
		if r.TablePattern == name {
			return true
		}
	}
	return false
}
