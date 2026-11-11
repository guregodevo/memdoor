package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A task typed with no model opens the way to one: the /connect picker, the
// message kept for when a model answers. No dead end (2026-11-11).
func TestATaskWithNoModelOpensConnect(t *testing.T) {
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error {
		t.Fatal("nothing is sent to a gateway with no model")
		return nil
	})
	m.connectOps = ConnectOps{Connect: func(req ConnectRequest) (ConnectResult, error) { return ConnectResult{}, nil }}
	step := func(msg tea.Msg) { nm, _ := m.Update(msg); m = nm.(Model) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	m.status.Gate = "no model yet — no key? memdoor connect openrouter-login …"
	typeAndEnter(step, "add a hello.py that prints hi")
	if m.connect == nil {
		t.Fatal("the /connect picker did not open")
	}
	if m.heldPrompt != "add a hello.py that prints hi" {
		t.Fatalf("the message is not kept: %q", m.heldPrompt)
	}
	if !strings.Contains(notesOf(m), "connect one below") {
		t.Fatalf("no line says what the picker is for:\n%s", notesOf(m))
	}
}

// /model with a name searches, as /model-search does; auto, a rung, a model
// id and provider:model go to the route as before.
func TestModelWithANameSearches(t *testing.T) {
	for _, c := range []struct {
		args   []string
		search bool
	}{
		{[]string{"deepseek"}, true},
		{[]string{"glm", "flash"}, true},
		{[]string{"auto"}, false},
		{[]string{"AUTO"}, false},
		{[]string{"2"}, false},
		{[]string{"z-ai/glm-5.3-flash"}, false},
		{[]string{"z-ai/glm-5.3-flash", "throughput"}, false},
		{[]string{"groq:qwen/qwen3.8-27b"}, false},
		{nil, false},
	} {
		if got := modelSearchArg(c.args); got != c.search {
			t.Errorf("%q: search=%v, want %v", c.args, got, c.search)
		}
	}
	var asked string
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error { return nil })
	m.route = func(string) (string, error) { t.Fatal("a name went to the route"); return "", nil }
	m.modelCatalog = func(q string) ([]CatalogModel, error) { asked = q; return nil, nil }
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	typeAndEnter(step, "/model deepseek")
	if asked != "deepseek" {
		t.Fatalf("/model deepseek searched %q", asked)
	}
}
