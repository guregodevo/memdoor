package message

import "sync"

// turnGate serializes agent turns per (channel, agent).
//
// Every incoming message dispatches its agent work on its own goroutine, and
// nothing else orders them: the turn ledger dedupes the SAME trigger message,
// but a NEW message for an agent already mid-turn ran immediately. Measured
// live 2026-08-30: "continue" typed while the coder was still generating put
// TWO turns on one channel at once — their deltas interleaved mid-word in the
// transcript, tore the <think>/<tool_call> markers apart so raw protocol
// leaked to the screen, and both spent tokens in parallel.
//
// One turn at a time per channel+agent, in arrival order; different channels
// and different agents stay concurrent — serializing those would let one slow
// turn block the whole gateway.
type turnGate struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newTurnGate() *turnGate {
	return &turnGate{locks: make(map[string]*sync.Mutex)}
}

// acquire blocks until the (channel, agent) slot is free and returns the
// release. Entries live for the process's lifetime: the key space is channels
// × agents actually used, which is small, and dropping a mutex that a later
// goroutine might still look up is how keyed locks go wrong.
func (g *turnGate) acquire(channelID, agentID string) func() {
	key := channelID + "\x00" + agentID
	g.mu.Lock()
	l, ok := g.locks[key]
	if !ok {
		l = &sync.Mutex{}
		g.locks[key] = l
	}
	g.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// held reports whether a turn for (channel, agent) is running right now.
//
// Consumers use it to tell a WORKING turn from a dead one: the turn janitor
// declared a legitimate 22-minute turn lost on a dispatch-time deadline while
// its tool calls were still landing (live 2026-08-31). TryLock is a probe, not
// an acquisition — on success the lock is released immediately.
func (g *turnGate) held(channelID, agentID string) bool {
	key := channelID + "\x00" + agentID
	g.mu.Lock()
	l, ok := g.locks[key]
	g.mu.Unlock()
	if !ok {
		return false
	}
	if l.TryLock() {
		l.Unlock()
		return false
	}
	return true
}
