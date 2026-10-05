package a2a

import "testing"

// The A2A reply/announce prompts carry the requester's and target's channels,
// which were hardcoded "". They are derived from the session keys the message
// already holds — a "workspace:<ws>:channel:<id>" key yields its channel, an
// agent/subagent key yields "".
func TestChannelFromSessionKey(t *testing.T) {
	cases := map[string]string{
		"workspace:acme:channel:general": "general",
		"workspace:w:channel:c":          "c",
		"agent:coder:step-1":             "",
		"":                               "",
		"workspace:only":                 "",
	}
	for key, want := range cases {
		if got := channelFromSessionKey(key); got != want {
			t.Errorf("channelFromSessionKey(%q) = %q, want %q", key, got, want)
		}
	}
}
