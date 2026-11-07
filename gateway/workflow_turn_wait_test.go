package gateway

import (
	"context"
	"testing"
	"time"
)

// A task's own clock is the wait on its turn when it is longer than the
// gateway's backstop: the backstop cut 90-minute steps at 31 minutes.
func TestTurnWaitNeverUndercutsTheTasksClock(t *testing.T) {
	if got := turnWait(context.Background()); got != jobWaitTimeout() {
		t.Fatalf("no clock: the backstop, got %s", got)
	}
	long, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	if got := turnWait(long); got < 90*time.Minute {
		t.Fatalf("a 90m clock: at least 90m, got %s", got)
	}
	short, cancel2 := context.WithTimeout(context.Background(), time.Minute)
	defer cancel2()
	if got := turnWait(short); got != jobWaitTimeout() {
		t.Fatalf("a shorter clock ends the turn itself; the wait stays the backstop, got %s", got)
	}
}
