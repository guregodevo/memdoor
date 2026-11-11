package workflows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type fanoutStep struct {
	Type     string `yaml:"type"`
	Optional bool   `yaml:"optional"`
	Requires []struct {
		TablePattern string `yaml:"table_pattern"`
	} `yaml:"requires"`
	Prompt string `yaml:"prompt"`
	Target struct {
		File string `yaml:"file"`
	} `yaml:"target"`
}

func readFanoutStep(t *testing.T, name string) fanoutStep {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("team-solve-fanout", "steps", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var s fanoutStep
	if err := yaml.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (s fanoutStep) requires(name string) bool {
	for _, r := range s.Requires {
		if r.TablePattern == name {
			return true
		}
	}
	return false
}

// cases is an optional read-only turn after the survey that writes cases.md;
// the attempts wait for it, and every attempt but the first reads it, as they
// read the survey.
func TestTeamSolveFanoutAttemptsReadTheCases(t *testing.T) {
	cases := readFanoutStep(t, "cases")
	if cases.Type != "agent" || !cases.Optional || !cases.requires("survey") {
		t.Errorf("cases: type %q, optional %v, requires survey %v; want agent, true, true", cases.Type, cases.Optional, cases.requires("survey"))
	}
	if !strings.HasSuffix(cases.Target.File, "/cases.md") {
		t.Errorf("cases: target %q, want the run's cases.md", cases.Target.File)
	}
	if !strings.Contains(cases.Prompt, "change no file of the project") {
		t.Error("cases: the prompt does not keep the turn read-only")
	}
	direct := readFanoutStep(t, "direct")
	if !direct.requires("cases") {
		t.Error("direct does not wait for cases")
	}
	open := strings.Index(direct.Prompt, `{{ if ne (printf "%v" .n) "1" }}`)
	end := strings.Index(direct.Prompt, "{{ end }}")
	at := strings.Index(direct.Prompt, "cases.md")
	if open < 0 || at < open || at > end {
		t.Error("direct: cases.md is not read inside the block for the attempts after the first")
	}
}
