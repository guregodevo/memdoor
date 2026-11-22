package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// "@coder /model deepseek" is a slash command, not a message: the leading
// mention names the agent, and the command after it is the intent. Before
// this, /model with a mention fell through to the gateway as plain text and
// the model answered with prose (live 2026-11-22). A mention of ANOTHER
// agent still routes to that agent with the command as its task.
func TestMentionBeforeSlashIsACommand(t *testing.T) {
	var posted string
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error {
		posted = agent + "|" + text
		return nil
	})
	m.route = func(string) (string, error) { t.Fatal("route called"); return "", nil }
	m.modelCatalog = func(q, provider string) ([]CatalogModel, error) { return nil, nil }
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})

	// "@coder /model deepseek" opens the search picker; nothing is posted.
	typeAndEnter(step, "@coder /model deepseek")
	if posted != "" {
		t.Fatalf("@coder /model deepseek posted to the gateway: %q", posted)
	}
	if m.routePicker == nil || m.routePicker.query != "deepseek" {
		t.Fatalf("@coder /model deepseek did not open the search picker on deepseek: picker=%v", m.routePicker)
	}

	// Close the picker so the next input reaches the message box.
	step(tea.KeyMsg{Type: tea.KeyEsc})

	// "@planner /model 2" routes to the planner as its task.
	typeAndEnter(step, "@planner /model 2")
	if posted != "planner|/model 2" {
		t.Fatalf("@planner /model 2 route wrong: %q", posted)
	}
}

// A leading @name runs the turn as THAT agent. The poster prepends the agent
// mention to every turn, so before this the server saw "@coder @planner …"
// and routed to the first mention, the coder, which ran a turn meant for
// another agent (live 2026-09-02).
func TestRouteByMention(t *testing.T) {
	for in, want := range map[string][2]string{
		"@planner make a plan": {"planner", "make a plan"},
		"  @researcher sync":   {"researcher", "sync"},
		"@Planner PLAN it":     {"planner", "PLAN it"},
		"@box:coder review it": {"box:coder", "review it"},
		"@box.lan:chief hi":    {"box.lan:chief", "hi"},
		"@coder: outline it":   {"coder", "outline it"},
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
