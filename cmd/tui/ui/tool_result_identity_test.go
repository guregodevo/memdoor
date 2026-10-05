package ui

import (
	"testing"
	"time"
)

// A tool RESULT must land on the frame that asked for it.
//
// Matching on tool NAME cannot tell concurrent calls apart: the old code took
// the last unfilled frame with that name, so with three bash calls in one reply
// the first result landed on the third frame. Measured 2026-08-30 — the frame
// labelled `Bash(ls -la && pwd && python3 --version)` rendered "bash: python:
// command not found", which was another call's output entirely. Three frames,
// three results, every one attached to the wrong call.
func TestConcurrentToolResultsLandOnTheirOwnFrames(t *testing.T) {
	m := Model{activeTools: map[string]time.Time{}}

	// Three bash calls in flight, exactly as the model emitted them.
	for _, c := range []struct{ id, input string }{
		{"toolu_1", `{"command":"ls -la"}`},
		{"toolu_2", `{"command":"new-program"}`},
		{"toolu_3", `{"command":"python"}`},
	} {
		mm, _ := m.Update(toolCallStartMsg{toolID: c.id, toolName: "bash", toolInput: c.input})
		m = mm.(Model)
	}
	if len(m.messages) != 3 {
		t.Fatalf("want 3 frames, got %d", len(m.messages))
	}

	// Results arrive — the first one completing is NOT the first one started.
	for _, c := range []struct{ id, out string }{
		{"toolu_3", "bash: python: command not found"},
		{"toolu_1", "total 0"},
		{"toolu_2", "bash: new-program: command not found"},
	} {
		mm, _ := m.Update(toolCallCompleteMsg{toolID: c.id, toolName: "bash", toolOutput: c.out})
		m = mm.(Model)
	}

	want := map[string]string{
		"toolu_1": "total 0",
		"toolu_2": "bash: new-program: command not found",
		"toolu_3": "bash: python: command not found",
	}
	for _, msg := range m.messages {
		if msg.Role != "tool_call" {
			continue
		}
		if got := msg.ToolOutput; got != want[msg.ToolID] {
			t.Errorf("frame %s (%s) shows %q — that is another call's output; want %q",
				msg.ToolID, msg.ToolInput, got, want[msg.ToolID])
		}
	}
}

// A gateway too old to send an id must still render: fall back to name
// matching rather than showing nothing.
func TestResultsStillAttachWithoutAnID(t *testing.T) {
	m := Model{activeTools: map[string]time.Time{}}
	mm, _ := m.Update(toolCallStartMsg{toolName: "bash", toolInput: `{"command":"ls"}`})
	m = mm.(Model)
	mm, _ = m.Update(toolCallCompleteMsg{toolName: "bash", toolOutput: "total 0"})
	m = mm.(Model)

	for _, msg := range m.messages {
		if msg.Role == "tool_call" && msg.ToolOutput != "total 0" {
			t.Errorf("without an id the result must still attach, got %q", msg.ToolOutput)
		}
	}
}
