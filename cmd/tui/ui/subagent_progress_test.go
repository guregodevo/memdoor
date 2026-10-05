package ui

import (
	"strings"
	"testing"
)

// THE SCREEN THAT ASKED FOR THE WORK MUST SEE THE WORK.
//
// A spawned run emits on its own session, so the screen that delegated used
// to go blank for the whole job — live 2026-09-16, ffmpeg rendering for
// minutes under a spinner still reading "Writing a Captions call — 129 B so
// far". The gateway now mirrors the child's activity onto the requester's
// session; this is the screen's half.
func TestASpawnedRunsWorkShowsOnTheScreenThatAskedForIt(t *testing.T) {
	m := NewModel("", "", "", "", nil)

	// Nothing delegated: the bar says nothing.
	if bar := stripANSI(m.renderThinkingBar()); strings.Contains(bar, "verifier") {
		t.Fatalf("an idle screen named a subagent: %q", bar)
	}

	next, _ := m.update(subagentWorkMsg{sessionID: "agent:verifier:subagent:run-1", agent: "verifier"})
	m = next.(Model)
	bar := stripANSI(m.renderThinkingBar())
	if !strings.Contains(bar, "verifier is working") {
		t.Fatalf("a spawned run that has not called a tool yet is invisible: %q", bar)
	}

	// Inside a long tool: the tool and ITS OWN clock, which is the number
	// that separates a long render from a hang.
	next, _ = m.update(subagentWorkMsg{sessionID: "agent:verifier:subagent:run-1", agent: "verifier", tool: "bash", seconds: 95})
	m = next.(Model)
	bar = stripANSI(m.renderThinkingBar())
	if !strings.Contains(bar, "verifier") || !strings.Contains(bar, "Bash") {
		t.Fatalf("the running tool is not named: %q", bar)
	}
	if !strings.Contains(bar, "1m") {
		t.Fatalf("the tool's elapsed is missing, so a hang and a render look alike: %q", bar)
	}

	// Done clears it: a label that outlives the run is a lie.
	next, _ = m.update(subagentWorkMsg{sessionID: "agent:verifier:subagent:run-1", agent: "verifier", done: true})
	m = next.(Model)
	if len(m.subagentWork) != 0 {
		t.Fatalf("a finished run is still on screen: %+v", m.subagentWork)
	}
}

// THE REQUESTER'S TURN ENDS THE MOMENT IT DELEGATES.
//
// sessions_spawn enqueues and returns, so the parent's run completes while
// the job it asked for is just beginning. Clearing the mirror on run-complete
// would blank the screen for exactly the ten minutes the work takes.
func TestDelegatedWorkSurvivesTheRequestersOwnTurn(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	next, tick := m.update(subagentWorkMsg{sessionID: "s1", agent: "verifier", tool: "bash", seconds: 3})
	m = next.(Model)
	if tick == nil {
		t.Fatal("no animation was started, so the label would freeze at its first value")
	}

	next, _ = m.update(runCompleteMsg{})
	m = next.(Model)
	if len(m.subagentWork) != 1 {
		t.Fatal("the requester's own turn ending erased the job it delegated")
	}
	if bar := stripANSI(m.renderThinkingBar()); !strings.Contains(bar, "verifier") {
		t.Fatalf("the bar went quiet while the delegated job ran: %q", bar)
	}
}
