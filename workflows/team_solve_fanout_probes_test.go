package workflows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The time probes are checks of the fan-out only: direct scores its
// attempts on probes/*, and the Check (check.sh) never runs them, as with
// the path checks (#212).
func TestTeamSolveFanoutProbesAreFanoutChecksNotTheCheck(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("team-solve-fanout", "steps", "direct.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var step struct {
		Requires []struct {
			TablePattern string `yaml:"table_pattern"`
		} `yaml:"requires"`
		Fanout struct {
			Checks []string `yaml:"checks"`
		} `yaml:"fanout"`
	}
	if err := yaml.Unmarshal(raw, &step); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(step.Fanout.Checks, " "), "/probes/*") {
		t.Errorf("direct's checks %v do not include probes/*", step.Fanout.Checks)
	}
	found := false
	for _, r := range step.Requires {
		found = found || r.TablePattern == "probes"
	}
	if !found {
		t.Errorf("direct does not require probes: its checks would not exist yet")
	}
	check, err := os.ReadFile(filepath.Join("team-solve", "check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(check), "probes/") {
		t.Errorf("check.sh runs the probes: they would veto the Check")
	}
}
