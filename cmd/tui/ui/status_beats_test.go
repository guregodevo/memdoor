package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"memdoor/gateway/infra"
)

type sinkSpy struct{ got []tea.Msg }

func (s *sinkSpy) Send(m tea.Msg) { s.got = append(s.got, m) }

// The status beats reach the screen through the LIVE path (OnAgentEvent →
// consumer → handler), with the exact shape the gateway emits
// (agent_runtime_progress.go). The old test fed the handler directly, so it
// stayed green after the path that decoded these events was deleted.
func TestStatusBeatsReachTheScreen(t *testing.T) {
	spy := &sinkSpy{}
	a := NewTUIEventAdapter(nil)
	a.SetSink(spy)
	a.OnAgentEvent(infra.AgentEvent{RunID: "r1", Seq: 1, Stream: infra.EventStreamTool, SessionID: "s",
		Data: map[string]interface{}{"event": "progress", "id": "t1", "tool": "bash", "seconds": 45}})
	a.OnAgentEvent(infra.AgentEvent{RunID: "r1", Seq: 2, Stream: infra.EventStreamTool, SessionID: "s",
		Data: map[string]interface{}{"event": "subagent", "state": "running", "agent": "coder", "session": "agent:coder:subagent:run-1", "tool": "apply_patch", "seconds": 12}})
	a.OnAgentEvent(infra.AgentEvent{RunID: "r1", Seq: 3, Stream: infra.EventStreamTool, SessionID: "s",
		Data: map[string]interface{}{"event": "subagent", "state": "done", "agent": "coder", "session": "agent:coder:subagent:run-1", "tool": "", "seconds": 80}})
	if len(spy.got) != 3 {
		t.Fatalf("want 3 messages, got %d: %#v", len(spy.got), spy.got)
	}
	if p, ok := spy.got[0].(toolProgressMsg); !ok || p.toolName != "bash" || p.seconds != 45 {
		t.Errorf("progress: got %#v", spy.got[0])
	}
	if w, ok := spy.got[1].(subagentWorkMsg); !ok || w.agent != "coder" || w.tool != "apply_patch" || w.seconds != 12 || w.done || w.sessionID != "agent:coder:subagent:run-1" {
		t.Errorf("subagent running: got %#v", spy.got[1])
	}
	if w, ok := spy.got[2].(subagentWorkMsg); !ok || !w.done {
		t.Errorf("subagent done: got %#v", spy.got[2])
	}

	// And the screen shows it: the status line names the spawned run's work.
	m := NewModel("", "ws", "ch", "", nil)
	next, _ := m.update(spy.got[1])
	m = next.(Model)
	if len(m.subagentWork) != 1 || m.subagentWork["agent:coder:subagent:run-1"].tool != "apply_patch" {
		t.Fatalf("the screen did not record the spawned run's work: %#v", m.subagentWork)
	}
}

// The gateway's "waiting" beat (a model call in flight) reaches the screen and
// keeps the stall watchdog from clearing the spinner of a turn whose local
// model reads its prompt for minutes.
func TestTheModelWaitBeatKeepsTheTurnAlive(t *testing.T) {
	spy := &sinkSpy{}
	a := NewTUIEventAdapter(nil)
	a.SetSink(spy)
	a.OnAgentEvent(infra.AgentEvent{RunID: "r1", Seq: 1, Stream: infra.EventStreamAssistant, SessionID: "s",
		Data: map[string]interface{}{"event": "waiting"}})
	if len(spy.got) != 1 {
		t.Fatalf("want 1 message, got %d: %#v", len(spy.got), spy.got)
	}
	if _, ok := spy.got[0].(modelWaitMsg); !ok {
		t.Fatalf("waiting: got %#v", spy.got[0])
	}

	m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	m.isThinking = true
	m.turnRunning = true
	m.thinkingStartTime = time.Now().Add(-watchdogStallTimeout - time.Minute)
	m.lastEventAt = m.thinkingStartTime
	step(spy.got[0])
	step(thinkingTickMsg(time.Now()))
	if !m.isThinking || !m.turnRunning {
		t.Fatal("a turn still waiting on its model was cleared as stalled")
	}
}
