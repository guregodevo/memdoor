package workflows

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// choose settles a tie by consensus before prefer: the leader whose change
// the other leaders share most (mario's pick, tie: consensus).
func TestTeamSolveFanoutChooseSettlesATieByConsensus(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("team-solve-fanout", "steps", "choose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var step struct {
		Type   string `yaml:"type"`
		Tie    string `yaml:"tie"`
		Prefer int    `yaml:"prefer"`
	}
	if err := yaml.Unmarshal(raw, &step); err != nil {
		t.Fatal(err)
	}
	if step.Type != "pick" || step.Tie != "consensus" || step.Prefer != 1 {
		t.Errorf("choose: type %q, tie %q, prefer %d; want pick, consensus, 1", step.Type, step.Tie, step.Prefer)
	}
}
