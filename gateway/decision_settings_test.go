package gateway

import (
	"context"
	"testing"

	"memdoor/pkg/decision"
)

// Every setting a decision consumer reads must be writable through
// PUT /api/workspace/settings, or it cannot be switched.
func TestDecisionSettingsAreWritable(t *testing.T) {
	for _, k := range []string{
		settingDecisionGate, settingDecisionThreshold,
		settingToolRouting, settingToolFamilies,
		settingTurnVerdict, settingResultAcceptance, settingResultAcceptanceThreshold,
		settingEditFormat,
	} {
		if !globalSettingKeys[k] {
			t.Errorf("%s is read by the gateway but not writable", k)
		}
	}
}

// There is no switch (2026-09-29, "remove the toggle on and off"): a
// decision_model row left in a workspace from before changes nothing — off
// does not disable, and a provider name does not pick one.
func TestDecisionModelSettingIsIgnored(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(envSystemOneKey, "")
	t.Setenv(envTypeSafeKey, "")
	// The person's own OpenRouter key is the coding decision key by default
	// (autoDecisionModel), so a shell that exports it would register one here.
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	for _, old := range []string{"off", "systemone", "local"} {
		svc, err := newDecisionService(func(key string) string {
			if key == "decision_model" {
				return old
			}
			return ""
		})
		if err != nil {
			t.Fatal(err)
		}
		q, _ := decision.Boolean("q?", "", "")
		res := svc.Evaluate(context.Background(), decision.Request{State: "s", Questions: map[string]decision.Question{"q": q}}, decision.Options{})
		if res.Reason != decision.ReasonNotConfigured {
			t.Errorf("decision_model=%s: reason %q, want not-configured (no seat, no decision key)", old, res.Reason)
		}
	}
}
