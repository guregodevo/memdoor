package queue

import (
	"testing"
	"time"
)

// A job that never returns must not hold a stopping gateway (#224).
func TestStopWithinGivesUpOnAJobThatNeverReturns(t *testing.T) {
	qm := NewQueueManager(nil, false)
	release := make(chan struct{})
	defer close(release)
	qm.wg.Add(1)
	go func() {
		defer qm.wg.Done()
		<-release
	}()
	start := time.Now()
	if qm.StopWithin(200 * time.Millisecond) {
		t.Fatal("a stuck worker was reported stopped")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("StopWithin waited %s", time.Since(start))
	}
}

func TestStopWithinReturnsAtOnceWhenIdle(t *testing.T) {
	qm := NewQueueManager(nil, false)
	qm.Start()
	if !qm.StopWithin(2 * time.Second) {
		t.Fatal("an idle manager did not stop")
	}
}
