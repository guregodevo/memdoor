package context

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A context with steady progress outlives its idle timeout many times over;
// the same context left alone ends, with the cause saying why.
func TestIdleTimeoutEndsOnlyWithoutProgress(t *testing.T) {
	ctx, cancel := WithIdleTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	for i := 0; i < 8; i++ { // 240ms of work, 4x the idle timeout
		time.Sleep(30 * time.Millisecond)
		Progress(ctx)
	}
	if ctx.Err() != nil {
		t.Fatalf("ended while making progress: %v", context.Cause(ctx))
	}
	time.Sleep(120 * time.Millisecond)
	if ctx.Err() == nil || !errors.Is(context.Cause(ctx), ErrNoProgress) {
		t.Fatalf("still running after no progress: err %v, cause %v", ctx.Err(), context.Cause(ctx))
	}
}

// Progress under an inner timeout keeps the outer one alive too: the turn's
// own backstop sits under the message service's net.
func TestProgressReachesTheOuterTimeout(t *testing.T) {
	outer, cancelOuter := WithIdleTimeout(context.Background(), 60*time.Millisecond)
	defer cancelOuter()
	inner, cancelInner := WithIdleTimeout(outer, time.Hour)
	defer cancelInner()
	for i := 0; i < 8; i++ {
		time.Sleep(30 * time.Millisecond)
		Progress(inner)
	}
	if outer.Err() != nil {
		t.Fatalf("the outer net ended while the inner work progressed: %v", context.Cause(outer))
	}
}

// Without an idle timeout, Progress does nothing.
func TestProgressWithoutIdleTimeout(t *testing.T) {
	Progress(context.Background())
}
