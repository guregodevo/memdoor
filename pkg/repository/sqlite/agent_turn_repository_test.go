package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"memdoor/pkg/domain"
)

func turnRepoForTest(t *testing.T) *agentTurnRepository {
	t.Helper()
	f, err := NewSQLiteFactory(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f.AgentTurns().(*agentTurnRepository)
}

func TestTurnLedgerIdempotency(t *testing.T) {
	r := turnRepoForTest(t)
	ctx := context.Background()

	mk := func(id string) *domain.AgentTurn {
		return &domain.AgentTurn{TurnID: id, ChannelID: "ch", AgentID: "coder",
			TriggerMessageID: 42, State: domain.TurnDispatched, DispatchedAt: time.Now()}
	}
	created, err := r.Begin(ctx, mk("t1"))
	if err != nil || !created {
		t.Fatalf("first begin must create: %v %v", created, err)
	}
	// Same trigger re-delivered → duplicate detected, NOT a second turn.
	created, err = r.Begin(ctx, mk("t2"))
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("duplicate trigger must not create a second turn")
	}

	// running only from dispatched; first terminal wins.
	if err := r.SetState(ctx, "t1", domain.TurnRunning, "", 0); err != nil {
		t.Fatal(err)
	}
	if err := r.SetState(ctx, "t1", domain.TurnResponded, "", 777); err != nil {
		t.Fatal(err)
	}
	// A late failure hook must NOT downgrade the recorded response.
	if err := r.SetState(ctx, "t1", domain.TurnFailed, "late error", 0); err != nil {
		t.Fatal(err)
	}
	turns, err := r.Since(ctx, "ch", time.Now().Add(-time.Hour))
	if err != nil || len(turns) != 1 {
		t.Fatalf("want 1 turn: %v %v", len(turns), err)
	}
	if turns[0].State != domain.TurnResponded || turns[0].ResponseMessageID != 777 {
		t.Fatalf("terminal state must be immutable: %+v", turns[0])
	}
}

func TestTurnJanitorFindsStuck(t *testing.T) {
	r := turnRepoForTest(t)
	ctx := context.Background()
	old := &domain.AgentTurn{TurnID: "old", ChannelID: "ch", AgentID: "coder",
		TriggerMessageID: 1, State: domain.TurnDispatched, DispatchedAt: time.Now().Add(-time.Hour)}
	if _, err := r.Begin(ctx, old); err != nil {
		t.Fatal(err)
	}
	stuck, err := r.Stuck(ctx, time.Now().Add(-20*time.Minute))
	if err != nil || len(stuck) != 1 {
		t.Fatalf("janitor must find the stuck turn: %v %v", len(stuck), err)
	}
	if err := r.SetState(ctx, "old", domain.TurnLost, "no terminal state within deadline", 0); err != nil {
		t.Fatal(err)
	}
	stuck, _ = r.Stuck(ctx, time.Now().Add(-20*time.Minute))
	if len(stuck) != 0 {
		t.Fatal("lost turns must not be re-flagged")
	}
}
