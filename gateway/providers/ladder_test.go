package providers

import (
	"context"
	"testing"

	sharedctx "memdoor/pkg/shared/context"
)

func TestModelForTier(t *testing.T) {
	re := &RemoteEngine{Model: "omni", AgentModels: map[string]string{"coder": "cheap", "chief": "omni"},
		AgentLadders: map[string][]string{"coder": {"cheap", "mid", "strong"}}}
	for _, c := range []struct {
		agent string
		tier  int
		want  string
	}{{"coder", 0, "cheap"}, {"coder", 1, "mid"}, {"coder", 9, "strong"}, {"chief", 2, "omni"}, {"chief", 1, "omni"}, {"CODER", 2, "strong"}} {
		if got := re.ModelForTier(c.agent, c.tier); got != c.want {
			t.Errorf("%s tier %d: %q, want %q", c.agent, c.tier, got, c.want)
		}
	}
	if TierFromContext(context.Background()) != 0 || TierFromContext(context.WithValue(context.Background(), sharedctx.TierKey, 2)) != 2 {
		t.Fatal("TierFromContext")
	}
}
