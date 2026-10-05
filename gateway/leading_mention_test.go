package gateway

import "testing"

// A LEADING @name routes a TUI turn to that agent; anything else is ordinary
// text. Live 2026-09-02: a turn addressed to another agent ran the CODER
// twice, with none of that agent's tools.
func TestLeadingMention(t *testing.T) {
	routes := map[string][2]string{
		"@planner make a plan":   {"planner", "make a plan"},
		"  @coder fix the build": {"coder", "fix the build"},
		"@researcher recheck":    {"researcher", "recheck"},
		"@Planner PLAN it":       {"planner", "PLAN it"},
	}
	for in, want := range routes {
		name, rest, ok := leadingMention(in)
		if !ok || name != want[0] || rest != want[1] {
			t.Errorf("leadingMention(%q) = (%q, %q, %v), want (%q, %q, true)", in, name, rest, ok, want[0], want[1])
		}
	}
	// Not a route: no mention, mid-sentence mention, bare @, email-ish.
	for _, in := range []string{
		"make a plan",
		"ping me @ 5pm about it",
		"@",
		"tell @planner to plan it", // the mention is not leading
	} {
		if name, _, ok := leadingMention(in); ok {
			t.Errorf("leadingMention(%q) routed to %q — it must stay plain text", in, name)
		}
	}
}
