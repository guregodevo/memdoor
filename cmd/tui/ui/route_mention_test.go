package ui

import "testing"

// A leading @name runs the turn as THAT agent. The poster prepends the agent
// mention to every turn, so before this the server saw "@coder @planner …"
// and routed to the first mention, the coder, which ran a turn meant for
// another agent (live 2026-09-02).
func TestRouteByMention(t *testing.T) {
	for in, want := range map[string][2]string{
		"@planner make a plan": {"planner", "make a plan"},
		"  @researcher sync":   {"researcher", "sync"},
		"@Planner PLAN it":     {"planner", "PLAN it"},
	} {
		agent, rest := routeByMention("coder", in)
		if agent != want[0] || rest != want[1] {
			t.Errorf("routeByMention(%q) = (%q, %q), want (%q, %q)", in, agent, rest, want[0], want[1])
		}
	}
	// Everything else keeps the session's agent and the text untouched.
	for _, in := range []string{"fix the build", "ping me @ 5pm", "@", "tell @planner to plan it"} {
		if agent, rest := routeByMention("coder", in); agent != "coder" || rest != in {
			t.Errorf("routeByMention(%q) = (%q, %q), want (coder, unchanged)", in, agent, rest)
		}
	}
}
