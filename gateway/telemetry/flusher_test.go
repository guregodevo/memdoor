package telemetry_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/telemetry"
)

// fakeClient records every Send and returns a configurable error.
// Thread-safe — the flusher invokes Send from its background goroutine.
type fakeClient struct {
	mu      sync.Mutex
	calls   int32
	batches []telemetry.Batch
	sendErr error
}

func (c *fakeClient) Send(ctx context.Context, b telemetry.Batch) error {
	atomic.AddInt32(&c.calls, 1)
	c.mu.Lock()
	c.batches = append(c.batches, b)
	c.mu.Unlock()
	return c.sendErr
}

func (c *fakeClient) snapshot() []telemetry.Batch {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]telemetry.Batch, len(c.batches))
	copy(out, c.batches)
	return out
}

func TestFlusher_ShipsBatchOnTick(t *testing.T) {
	ring := telemetry.NewRing(100)
	ring.Enqueue([]*logs.Event{evt(logs.LevelError, "boom")})
	client := &fakeClient{}
	f := telemetry.NewFlusher(ring, client, telemetry.FlusherConfig{
		Interval: 50 * time.Millisecond,
		BatchMax: 10,
		HostID:   "host-1",
		Version:  "v-test",
	}, nil)
	f.Start(context.Background())
	defer f.Stop()

	// Wait for at least one tick.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&client.calls) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if atomic.LoadInt32(&client.calls) == 0 {
		t.Fatal("expected at least one Send call within 2s, got 0")
	}
	batches := client.snapshot()
	if len(batches[0].Events) != 1 || batches[0].Events[0].Message != "boom" {
		t.Errorf("first batch should carry one 'boom' event, got %v", batches[0])
	}
	if batches[0].HostID != "host-1" || batches[0].Version != "v-test" {
		t.Errorf("envelope fields not propagated: %+v", batches[0])
	}
}

func TestFlusher_StopDrainsRemaining(t *testing.T) {
	ring := telemetry.NewRing(100)
	ring.Enqueue([]*logs.Event{evt(logs.LevelError, "x")})
	client := &fakeClient{}
	f := telemetry.NewFlusher(ring, client, telemetry.FlusherConfig{
		Interval: 1 * time.Hour, // effectively no tick
		BatchMax: 10,
	}, nil)
	f.Start(context.Background())
	f.Stop()

	if atomic.LoadInt32(&client.calls) == 0 {
		t.Error("expected final drain on Stop to ship pending batch, got 0 calls")
	}
}

func TestFlusher_NoEventsNoSend(t *testing.T) {
	ring := telemetry.NewRing(100)
	client := &fakeClient{}
	f := telemetry.NewFlusher(ring, client, telemetry.FlusherConfig{
		Interval: 30 * time.Millisecond,
	}, nil)
	f.Start(context.Background())
	time.Sleep(100 * time.Millisecond)
	f.Stop()
	if atomic.LoadInt32(&client.calls) != 0 {
		t.Errorf("expected 0 Send calls with empty ring, got %d", client.calls)
	}
}

func TestFlusher_SendErrorDoesNotKillLoop(t *testing.T) {
	ring := telemetry.NewRing(100)
	ring.Enqueue([]*logs.Event{evt(logs.LevelError, "a")})
	client := &fakeClient{sendErr: errors.New("network down")}
	f := telemetry.NewFlusher(ring, client, telemetry.FlusherConfig{
		Interval: 30 * time.Millisecond,
		BatchMax: 10,
	}, nil)
	f.Start(context.Background())

	time.Sleep(150 * time.Millisecond)

	// Re-enqueue more events after the failure; the loop should still
	// be alive and ship them on next tick.
	ring.Enqueue([]*logs.Event{evt(logs.LevelError, "b")})
	time.Sleep(100 * time.Millisecond)

	f.Stop()
	if atomic.LoadInt32(&client.calls) < 2 {
		t.Errorf("expected ≥2 Send attempts (loop survived first failure), got %d", client.calls)
	}
}

func TestFlusher_DoubleStartPanics(t *testing.T) {
	f := telemetry.NewFlusher(telemetry.NewRing(10), &fakeClient{}, telemetry.FlusherConfig{
		Interval: 1 * time.Hour,
	}, nil)
	f.Start(context.Background())
	defer f.Stop()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on second Start")
		}
	}()
	f.Start(context.Background())
}
