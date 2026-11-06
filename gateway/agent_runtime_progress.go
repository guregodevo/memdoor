package gateway

import (
	"context"
	"sync"
	"time"

	"memdoor/gateway/infra"
	"memdoor/pkg/notes"
	"memdoor/pkg/shared"
	"memdoor/tools"
)

// While work is happening, say what it is.
//
// Two silences looked identical on screen and both read as a hang:
//
//  1. A tool that runs for minutes. The tool's "start" event arrives, the
//     "complete" arrives when it is over, and between them the stream says
//     nothing — a ten-minute render is indistinguishable from a dead turn.
//  2. A SPAWNED run. A sub-session gets its own run id and its own session
//     key, and every event is broadcast per session, so the screen that
//     asked for the work was subscribed to a session where nothing more
//     happened. Live 2026-09-16: a sub-session was running a long tool
//     while the window's spinner still showed the parent's last own act
//     before it delegated.
//
// So: a heartbeat on the running tool, and the same heartbeat MIRRORED onto
// the requester's session (the maintainer: "if it's not [stuck] we should display what
// is in progress… like adding an event emitter for subspawned"). The mirror
// is a separate event — "subagent" — so it never collides with the screen's
// own tool frames; it drives the status line, not the transcript.
const (
	// toolEventProgress / subagentEvent are the wire names of the two events
	// this file emits, beside the existing "start" / "complete".
	toolEventProgress = "progress"
	subagentEvent     = "subagent"
)

// modelWaitEvent is the beat sent while a model call is in flight. A local
// model on a CPU can read a 7,000-token prompt for eight minutes before its
// first token, and the stream says nothing in that time: the screen's stall
// watchdog took the silence for a lost run and cleared the spinner.
const modelWaitEvent = "waiting"

// modelWaitEveryVar is how often a model call in flight says so. Well under
// the screen's stall watchdog; a var so a test does not wait it out.
var modelWaitEveryVar = 30 * time.Second

// watchModelCall beats on the assistant stream until the returned stop
// function is called; when stop returns, no beat is sent any more. stop used
// to close the channel and return while a tick already taken (or one ready
// beside the close, select picks either) still went out: a beat after the
// call had returned (CI on main, 2026-11-06: 5 beats, then 6).
func (ar *AgentRuntime) watchModelCall(runID, sessionID string) func() {
	if ar == nil || ar.events == nil || runID == "" {
		return func() {}
	}
	done, exited := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(exited)
		ticker := time.NewTicker(modelWaitEveryVar)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				select {
				case <-done:
					return
				default:
				}
				ar.events.EmitEvent(runID, infra.EventStreamAssistant, sessionID, map[string]interface{}{
					"event": modelWaitEvent,
				})
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
		<-exited
	}
}

// toolProgressEveryVar is how often a running tool says it is still running.
// The events are tiny; the point is that the number on screen moves. A var so
// a test does not wait out five seconds a beat.
var toolProgressEveryVar = 5 * time.Second

// requesterOf returns the session key of the screen waiting on this session,
// and the name of the agent doing the work. Both are empty when the session
// IS the screen — a normal turn mirrors nothing.
func requesterOf(session *Session) (waiting, agent string) {
	if session == nil {
		return "", ""
	}
	v, ok := session.GetMetadataValue(tools.RequesterSessionKey)
	if !ok {
		return "", ""
	}
	key, _ := v.(string)
	if key == "" || key == session.ID {
		return "", ""
	}
	return key, shared.ParseSessionID(session.ID).GetAgentID()
}

// conversationOf is the conversation a turn's notes belong to: the screen's
// session, or for a sub-session the screen waiting on it, so what a
// delegated run notes its parent reads. "" outside any session.
func conversationOf(ctx context.Context) notes.ConversationKey {
	session, _ := ctx.Value(ctxSession).(*Session)
	if session == nil {
		return ""
	}
	if waiting, _ := requesterOf(session); waiting != "" {
		return notes.ConversationKey(waiting)
	}
	return notes.ConversationKey(session.ID)
}

// emitSubagentWork tells the waiting screen what the spawned run is doing.
// tool is empty between tool calls (the run is thinking); done clears it.
func (ar *AgentRuntime) emitSubagentWork(runID, waiting, child, agent, tool string, seconds int, done bool) {
	if ar == nil || ar.events == nil || waiting == "" {
		return
	}
	state := "running"
	if done {
		state = "done"
	}
	ar.events.EmitEvent(runID, infra.EventStreamTool, waiting, map[string]interface{}{
		"event":   subagentEvent,
		"state":   state,
		"agent":   agent,
		"session": child,
		"tool":    tool,
		"seconds": seconds,
	})
}

// watchTurnForRequester announces a spawned run to the screen that asked for
// it, for the whole length of the turn. Returns the stop function; it is safe
// to call on a session that was not spawned (it does nothing).
func (ar *AgentRuntime) watchTurnForRequester(session *Session, runID string) func() {
	waiting, agent := requesterOf(session)
	if waiting == "" {
		return func() {}
	}
	started := time.Now()
	ar.emitSubagentWork(runID, waiting, session.ID, agent, "", 0, false)
	var once sync.Once
	return func() {
		once.Do(func() {
			ar.emitSubagentWork(runID, waiting, session.ID, agent, "", int(time.Since(started).Seconds()), true)
		})
	}
}

// watchToolProgress beats while one tool runs: on the tool's own session, and
// on the requester's session when this run was spawned. Returns the stop
// function, which must be called when the tool returns.
func (ar *AgentRuntime) watchToolProgress(session *Session, runID, toolID, toolName string) func() {
	if ar == nil || ar.events == nil || session == nil {
		return func() {}
	}
	waiting, agent := requesterOf(session)
	started := time.Now()
	if waiting != "" {
		ar.emitSubagentWork(runID, waiting, session.ID, agent, toolName, 0, false)
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(toolProgressEveryVar)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				seconds := int(time.Since(started).Seconds())
				ar.events.EmitEvent(runID, infra.EventStreamTool, session.ID, map[string]interface{}{
					"event":   toolEventProgress,
					"id":      toolID,
					"tool":    toolName,
					"seconds": seconds,
				})
				if waiting != "" {
					ar.emitSubagentWork(runID, waiting, session.ID, agent, toolName, seconds, false)
				}
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			if waiting != "" {
				// The tool is over; the run is not. An empty tool leaves the
				// screen saying the agent is working, not that it is idle.
				ar.emitSubagentWork(runID, waiting, session.ID, agent, "", int(time.Since(started).Seconds()), false)
			}
		})
	}
}
