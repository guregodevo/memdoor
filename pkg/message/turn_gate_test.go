package message

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// One agent, one channel, one turn at a time. Measured live 2026-08-30: the
// user typed "continue" while the coder's turn was still generating; the
// gateway dispatched it immediately, and TWO turns streamed into the same
// transcript at once — text interleaved mid-word, protocol markers torn apart,
// raw <tool_call> JSON on screen, and both spending tokens.
func TestASecondTurnWaitsForTheFirst(t *testing.T) {
	g := newTurnGate()
	var running int32
	var maxRunning int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := g.acquire("chan-1", "coder")
			defer release()
			n := atomic.AddInt32(&running, 1)
			for {
				old := atomic.LoadInt32(&maxRunning)
				if n <= old || atomic.CompareAndSwapInt32(&maxRunning, old, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&running, -1)
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&maxRunning); got != 1 {
		t.Errorf("%d turns ran at once for the same channel+agent, want 1", got)
	}
}

// Different channels — and different agents in one channel — stay concurrent:
// serializing those would make one slow turn block the whole gateway.
func TestIndependentTurnsStayConcurrent(t *testing.T) {
	g := newTurnGate()
	release1 := g.acquire("chan-1", "coder")
	defer release1()

	done := make(chan struct{})
	go func() {
		r := g.acquire("chan-2", "coder") // different channel
		r()
		r = g.acquire("chan-1", "planner") // different agent, same channel
		r()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("an unrelated turn was blocked behind chan-1/coder")
	}
}

// The wiring, not just the gate: NewService must construct it, and the
// dispatch path must hold it for the duration of a turn. A gate that exists
// but is never acquired serializes nothing — which is exactly how the live
// incident happened (there was no gate at all).
func TestServiceConstructsTheGate(t *testing.T) {
	s := NewService(nil, nil, nil, nil, nil, nil, nil, nil)
	if s.turnGate == nil {
		t.Fatal("NewService left turnGate nil — every dispatch runs ungated")
	}
}

// The janitor needs to ask "is a turn for this pair running RIGHT NOW?"
// before declaring one lost. Measured live 2026-08-31 09:47: a legitimate
// 22-minute turn — two long generations, a truncation recovery, tool calls
// still landing — was declared LOST on a dispatch-time deadline and a wrong
// "reply was lost, re-send" announcement was posted while it worked.
func TestHeldReportsALiveTurn(t *testing.T) {
	g := newTurnGate()
	if g.held("chan-1", "coder") {
		t.Fatal("held() true before any turn started")
	}
	release := g.acquire("chan-1", "coder")
	if !g.held("chan-1", "coder") {
		t.Error("held() false while a turn is running")
	}
	if g.held("chan-1", "planner") || g.held("chan-2", "coder") {
		t.Error("held() leaked across agent or channel")
	}
	release()
	if g.held("chan-1", "coder") {
		t.Error("held() true after the turn released")
	}
}

// And the service exposes it, duck-typed for the gateway's janitor.
func TestServiceReportsActiveTurns(t *testing.T) {
	s := NewService(nil, nil, nil, nil, nil, nil, nil, nil)
	if s.TurnActive("chan-1", "coder") {
		t.Fatal("TurnActive true on an idle service")
	}
	release := s.turnGate.acquire("chan-1", "coder")
	defer release()
	if !s.TurnActive("chan-1", "coder") {
		t.Error("TurnActive false while the gate is held")
	}
}
