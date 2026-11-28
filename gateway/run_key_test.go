package gateway

import (
	"context"
	"testing"

	sharedctx "memdoor/pkg/shared/context"
)

func TestEveryStepOfAWorkflowRunHasTheRunsKey(t *testing.T) {
	run := &workflowRun{ID: "single-lcb-2026-11-28T113439@20261128-113439-2", Workflow: "single-lcb"}
	for _, task := range []string{"read", "answer", "grade"} {
		ctx := context.WithValue(context.Background(), sharedctx.SessionIDKey, run.taskSession(task))
		ctx = context.WithValue(ctx, ctxRunID, "run-9-coder") // an agent step's turn has its own run id
		if got := runKey(ctx); got != run.runKey() {
			t.Fatalf("step %s: key %q, want %q", task, got, run.runKey())
		}
	}
}

func TestATurnOutsideAWorkflowIsItsOwnRun(t *testing.T) {
	ctx := context.WithValue(context.Background(), sharedctx.SessionIDKey, "workspace:w:channel:c:agent:coder")
	if got := runKey(ctx); got != "workspace:w:channel:c:agent:coder" {
		t.Fatalf("no run id: %q", got)
	}
	ctx = context.WithValue(ctx, ctxRunID, "run-4-coder")
	if got := runKey(ctx); got != "run-4-coder" {
		t.Fatalf("with a run id: %q", got)
	}
}
