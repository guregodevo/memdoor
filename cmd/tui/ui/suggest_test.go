package ui

import (
	"strings"
	"testing"
)

// A RUNNING TURN ALWAYS SAYS ESC STOPS IT — between rounds too, when no
// flag says "thinking". Mutation check: key the line on isThinking alone
// and the bottom goes blank mid-turn.
func TestARunningTurnAlwaysSaysEscStopsIt(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m.ready = true
	m.turnRunning = true // sent, a tool done, the brain retrying: no other flag set
	if v := m.View(); !strings.Contains(v, "esc interrupt") {
		t.Fatal("a running turn names the key that stops it")
	}
	m.turnRunning = false
	if v := m.View(); strings.Contains(v, "esc interrupt") {
		t.Fatal("an idle window does not")
	}
}

// THE CODER'S EMPTY BOX SHOWS THE NEXT THING (Greg, 2026-09-27: "contextual
// hint in the input. the next action. The hint for free user to upgrade. The
// hint for slash command that can be useful with jev"). One line, from what
// just happened; and an upgrade line is information Tab can never send.
func TestTheCoderHintIsTheNextThing(t *testing.T) {
	coder := func(plan string, msgs ...Message) Model {
		m := NewPage(PageConfig{GatewayAddr: "http://x", Workspace: "w", ChannelID: "c", Agent: "coder", Plan: plan})
		m.messages = append(m.messages, msgs...)
		return m
	}
	ask := Message{Role: "user", Content: "fix the bug"}
	patch := Message{Role: "tool_call", ToolName: "apply_patch"}
	answer := Message{Role: "assistant", Content: "Fixed, and the tests pass."}

	// Before anything has happened: nothing (the welcome line covers it).
	if text, _ := coder("free").hint(); text != "" {
		t.Errorf("an empty session shows no hint, got %q", text)
	}
	// After a change: the next action, and Tab sends it.
	text, send := coder("pro", ask, patch, answer).hint()
	if !strings.Contains(text, "Commit") || send != text {
		t.Errorf("a turn that changed code suggests committing it: %q / %q", text, send)
	}
	// After a turn, any plan: the slash command that shows what Jev saved
	// (the decision model is not Pro, 2026-10-03).
	for _, plan := range []string{"pro", "free"} {
		text, send = coder(plan, ask, answer).hint()
		if !strings.Contains(text, "/usage") || send != "/usage" {
			t.Errorf("%s: the hint is /usage and Tab runs it: %q / %q", plan, text, send)
		}
	}
	// Unknown plan: no nudge on a guess.
	if text, _ := coder("", ask, answer).hint(); text != "" {
		t.Errorf("an unknown plan shows nothing, got %q", text)
	}
	// Typing, or a turn running: the box belongs to the person.
	typing := coder("free", ask, answer)
	typing.input.SetValue("now add a test")
	if text, _ := typing.hint(); text != "" {
		t.Errorf("no hint over what the person is typing, got %q", text)
	}
	running := coder("free", ask, answer)
	running.turnRunning = true
	if text, _ := running.hint(); text != "" {
		t.Errorf("no hint while a turn runs, got %q", text)
	}
}

// No decision model: the empty input says how to connect one, and Tab
// sends nothing (Greg, 2026-10-03: "we should have a hint for users to
// connect").
func TestTheHintSaysHowToConnectDecisions(t *testing.T) {
	m := NewPage(PageConfig{GatewayAddr: "http://x", Workspace: "w", ChannelID: "c", Agent: "coder", Plan: "free", DecisionsOff: true})
	m.messages = append(m.messages, Message{Role: "user", Content: "what does x do"}, Message{Role: "assistant", Content: "it does y"})
	text, send := m.hint()
	if !strings.Contains(text, "memdoor connect typesafe") || send != "" {
		t.Fatalf("hint %q send %q", text, send)
	}
}
