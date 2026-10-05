package gateway

import (
	"testing"
	"time"

	"memdoor/pkg/domain"
)

// A deadline alone cannot tell a dead turn from a long one. Measured live
// 2026-08-31 09:47: a turn 22 minutes in — two long generations, a truncation
// recovery, tool calls still landing — was declared LOST and a "reply was
// lost, re-send" announcement posted into the channel while it worked. The
// janitor now asks whether a turn for that (channel, agent) is executing
// RIGHT NOW and leaves running turns alone; the deadline still catches turns
// orphaned by a restart, which hold nothing.
func TestARunningTurnIsNotDeclaredLost(t *testing.T) {
	old := &domain.AgentTurn{TurnID: "t1", ChannelID: "chan-1", AgentID: "agent:coder",
		DispatchedAt: time.Now().Add(-40 * time.Minute)}
	dead := &domain.AgentTurn{TurnID: "t2", ChannelID: "chan-2", AgentID: "agent:coder",
		DispatchedAt: time.Now().Add(-40 * time.Minute)}

	activeOn := map[string]bool{"chan-1": true}
	lost := lostTurns([]*domain.AgentTurn{old, dead}, func(ch, agent string) bool { return activeOn[ch] })

	if len(lost) != 1 || lost[0].TurnID != "t2" {
		ids := []string{}
		for _, x := range lost {
			ids = append(ids, x.TurnID)
		}
		t.Fatalf("lost = %v, want exactly [t2] — t1 is still executing", ids)
	}
}

// No activity probe wired (nil) must behave like the old rule, not panic and
// not spare everything: losing the probe must not silence the janitor.
func TestNoProbeFallsBackToTheDeadline(t *testing.T) {
	stale := &domain.AgentTurn{TurnID: "t3", ChannelID: "c", AgentID: "a",
		DispatchedAt: time.Now().Add(-40 * time.Minute)}
	if lost := lostTurns([]*domain.AgentTurn{stale}, nil); len(lost) != 1 {
		t.Fatalf("with no probe, the stale turn was not declared lost")
	}
}
