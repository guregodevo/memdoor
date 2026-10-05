package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/subagents"
)

func TestSubagentFailureNamesTheTimeout(t *testing.T) {
	status, text := subagentFailure(context.DeadlineExceeded, 15*time.Minute)
	if status != subagents.OutcomeTimeout {
		t.Fatalf("status = %q, want timeout", status)
	}
	if !strings.Contains(text, "15m0s") || !strings.Contains(text, "runTimeoutSeconds") {
		t.Fatalf("text does not tell the limit: %q", text)
	}
	wrapped := errors.Join(errors.New("inference"), context.DeadlineExceeded)
	if s, _ := subagentFailure(wrapped, time.Minute); s != subagents.OutcomeTimeout {
		t.Fatalf("wrapped deadline: status = %q", s)
	}
}

func TestSubagentFailureKeepsOtherErrors(t *testing.T) {
	status, text := subagentFailure(errors.New("the brain refused"), time.Minute)
	if status != subagents.OutcomeError || text != "the brain refused" {
		t.Fatalf("got %q %q", status, text)
	}
	if s, _ := subagentFailure(context.DeadlineExceeded, 0); s != subagents.OutcomeError {
		t.Fatalf("a deadline with no limit set is not the requester's timeout: %q", s)
	}
}
