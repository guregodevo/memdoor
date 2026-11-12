package gateway

import (
	"context"
	"testing"
)

// A stopping gateway interrupts the turns and runs in flight instead of
// waiting for them (#224).
func TestCancelAllWorkInterruptsTurnsAndRuns(t *testing.T) {
	s := &Server{}
	turn, cancelTurn := context.WithCancel(context.Background())
	s.registerRunCancel("w:c", cancelTurn)
	runCtx, cancelRun := context.WithCancel(context.Background())
	s.workflows().runs["r1"] = &workflowRun{state: "running", cancel: cancelRun}
	s.workflows().runs["r2"] = &workflowRun{state: "done", cancel: func() { t.Error("a finished run was cancelled") }}

	if n := s.cancelAllWork(); n != 2 {
		t.Fatalf("cancelled %d, want 2", n)
	}
	if turn.Err() == nil || runCtx.Err() == nil {
		t.Fatal("work in flight was not interrupted")
	}
}
