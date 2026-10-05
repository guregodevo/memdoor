package gateway

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/broadcast"
	"memdoor/gateway/infra"
	"memdoor/gateway/streaming"
	"memdoor/pkg/shared"
	"memdoor/tools"
)

// collect subscribes to the emitter and records every event.
func collect(ee *infra.EventEmitter) (*[]infra.AgentEvent, *sync.Mutex) {
	var mu sync.Mutex
	got := []infra.AgentEvent{}
	ee.OnEvent(func(e infra.AgentEvent) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	})
	return &got, &mu
}

// A LONG TOOL MUST KEEP SAYING IT IS RUNNING.
//
// Between "start" and "complete" the stream said nothing, so a ten-minute
// render and a dead turn looked identical on screen (2026-09-16).
func TestARunningToolKeepsSayingSo(t *testing.T) {
	ee := infra.NewEventEmitter(false)
	ar := &AgentRuntime{events: ee}
	got, mu := collect(ee)

	old := toolProgressEveryVar
	toolProgressEveryVar = 10 * time.Millisecond
	defer func() { toolProgressEveryVar = old }()

	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	stop := ar.watchToolProgress(session, "run-1", "tu-1", "run_tests")
	time.Sleep(60 * time.Millisecond)
	stop()

	mu.Lock()
	defer mu.Unlock()
	beats := 0
	for _, e := range *got {
		if e.Stream != infra.EventStreamTool || e.Data["event"] != toolEventProgress {
			continue
		}
		beats++
		if e.SessionID != session.ID {
			t.Fatalf("a heartbeat went to the wrong session: %q", e.SessionID)
		}
		if e.Data["tool"] != "run_tests" || e.Data["id"] != "tu-1" {
			t.Fatalf("a heartbeat does not name its tool call: %+v", e.Data)
		}
	}
	if beats < 2 {
		t.Fatalf("a tool running for six intervals beat %d times", beats)
	}
}

// A SPAWNED RUN'S WORK IS MIRRORED ONTO THE SCREEN THAT ASKED FOR IT.
// Its own session has no subscriber; the requester's does.
func TestASpawnedRunsProgressReachesItsRequester(t *testing.T) {
	ee := infra.NewEventEmitter(false)
	ar := &AgentRuntime{events: ee}
	got, mu := collect(ee)

	old := toolProgressEveryVar
	toolProgressEveryVar = 10 * time.Millisecond
	defer func() { toolProgressEveryVar = old }()

	parent := "workspace:w:channel:c"
	child := &Session{ID: "agent:planner:subagent:run-9", Metadata: map[string]interface{}{
		tools.RequesterSessionKey: parent,
	}}

	endTurn := ar.watchTurnForRequester(child, "run-9")
	stop := ar.watchToolProgress(child, "run-9", "tu-2", "run_tests")
	time.Sleep(40 * time.Millisecond)
	stop()
	endTurn()

	mu.Lock()
	defer mu.Unlock()
	var mirrored, named, done int
	for _, e := range *got {
		if e.Data["event"] != subagentEvent {
			continue
		}
		if e.SessionID != parent {
			t.Fatalf("the mirror went to %q, not to the requester", e.SessionID)
		}
		mirrored++
		if e.Data["agent"] != "planner" {
			t.Fatalf("the mirror does not say who is working: %+v", e.Data)
		}
		if e.Data["tool"] == "run_tests" {
			named++
		}
		if e.Data["state"] == "done" {
			done++
		}
	}
	if mirrored < 3 || named < 2 {
		t.Fatalf("the requester heard %d events, %d naming the tool", mirrored, named)
	}
	if done != 1 {
		t.Fatalf("the requester was told %d times the run was over; want exactly one", done)
	}
}

// The tool is named the MOMENT it starts, not one heartbeat later. A tool
// that runs three seconds would otherwise never be named at all.
func TestTheRequesterHearsTheToolImmediately(t *testing.T) {
	ee := infra.NewEventEmitter(false)
	ar := &AgentRuntime{events: ee}
	got, mu := collect(ee)

	old := toolProgressEveryVar
	toolProgressEveryVar = time.Hour // no heartbeat can fire during this test
	defer func() { toolProgressEveryVar = old }()

	child := &Session{ID: "agent:planner:subagent:run-9", Metadata: map[string]interface{}{
		tools.RequesterSessionKey: "workspace:w:channel:c",
	}}
	ar.watchToolProgress(child, "run-9", "tu-3", "run_tests")()

	mu.Lock()
	defer mu.Unlock()
	for _, e := range *got {
		if e.Data["event"] == subagentEvent && e.Data["tool"] == "run_tests" {
			return
		}
	}
	t.Fatalf("a short tool was never named to the requester: %+v", *got)
}

// An ordinary turn has no requester and must emit no mirror at all.
func TestAnOrdinaryTurnMirrorsNothing(t *testing.T) {
	ee := infra.NewEventEmitter(false)
	ar := &AgentRuntime{events: ee}
	got, mu := collect(ee)

	session := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	ar.watchTurnForRequester(session, "run-3")()

	mu.Lock()
	defer mu.Unlock()
	for _, e := range *got {
		if e.Data["event"] == subagentEvent {
			t.Fatalf("an ordinary turn emitted a subagent mirror: %+v", e.Data)
		}
	}
}

// screen is a websocket client that only remembers what it was sent.
type screen struct {
	mu     sync.Mutex
	frames [][]byte
}

func (s *screen) GetID() string { return "screen-1" }
func (s *screen) Send(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frames = append(s.frames, append([]byte(nil), b...))
	return nil
}

// THE WHOLE ROUTE, NOT THE CALL: emitter → broadcaster → the subscriber that
// is the TUI. The mirror is worth nothing if it lands on a session nobody is
// listening to, which is exactly what happened to the child's OWN events.
func TestTheMirrorLandsOnTheSubscribedScreen(t *testing.T) {
	ee := infra.NewEventEmitter(false)
	sm := broadcast.NewSubscriptionManager(false)
	streaming.ConnectInfraEventEmitter(ee, sm, false)

	watching := &screen{}
	sm.AddSubscriber(watching)
	parent := shared.NewChannelSessionID("demo", "chan")
	sm.SubscribeToSession(watching.GetID(), parent)

	oldEvery := toolProgressEveryVar
	toolProgressEveryVar = time.Hour
	defer func() { toolProgressEveryVar = oldEvery }()

	ar := &AgentRuntime{events: ee}
	child := &Session{ID: "agent:planner:subagent:run-7", Metadata: map[string]interface{}{
		tools.RequesterSessionKey: parent,
	}}
	ar.watchToolProgress(child, "run-7", "tu-7", "run_tests")

	watching.mu.Lock()
	defer watching.mu.Unlock()
	for _, raw := range watching.frames {
		var frame struct {
			Type string `json:"type"`
			Data struct {
				SessionID string                 `json:"session_id"`
				Stream    string                 `json:"stream"`
				Data      map[string]interface{} `json:"data"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &frame); err != nil {
			t.Fatalf("unreadable frame: %v", err)
		}
		if frame.Data.Data["event"] != subagentEvent {
			continue
		}
		if frame.Data.SessionID != parent {
			t.Fatalf("the frame was addressed to %q, not the watching screen", frame.Data.SessionID)
		}
		if frame.Data.Data["tool"] != "run_tests" || frame.Data.Data["agent"] != "planner" {
			t.Fatalf("the frame does not say who is doing what: %+v", frame.Data.Data)
		}
		return
	}
	t.Fatalf("the watching screen was sent nothing about the run it spawned (%d frames)", len(watching.frames))
}
