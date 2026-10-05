package gateway

import "testing"

func TestAgentIDFromSessionKey(t *testing.T) {
	cases := map[string]string{
		"agent:coder:task-1":              "coder",
		"agent:planner:abc":               "planner",
		"workspace:ws-uuid:channel:ch-id": "", // channel session — no agent encoded
		"agent:coder":                     "", // too short (no label)
		"":                                "",
	}
	for key, want := range cases {
		if got := agentIDFromSessionKey(key); got != want {
			t.Errorf("agentIDFromSessionKey(%q) = %q, want %q", key, got, want)
		}
	}
}
