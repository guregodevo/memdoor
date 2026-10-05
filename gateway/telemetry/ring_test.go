package telemetry_test

import (
	"testing"

	"memdoor/gateway/logs"
	"memdoor/gateway/telemetry"
)

func evt(level logs.Level, msg string) *logs.Event {
	e := logs.NewEvent(logs.EventError, "test", msg)
	e.Level = level
	return e
}

func TestRing_EnqueueDrainRoundTrip(t *testing.T) {
	r := telemetry.NewRing(10)
	r.Enqueue([]*logs.Event{evt(logs.LevelWarn, "a"), evt(logs.LevelWarn, "b")})
	out := r.Drain(10)
	if len(out) != 2 {
		t.Fatalf("expected 2 events, got %d", len(out))
	}
	if out[0].Message != "a" || out[1].Message != "b" {
		t.Errorf("order broken: %s, %s", out[0].Message, out[1].Message)
	}
}

func TestRing_DrainEmptyReturnsNil(t *testing.T) {
	r := telemetry.NewRing(10)
	if out := r.Drain(10); out != nil {
		t.Errorf("expected nil, got %v", out)
	}
}

func TestRing_OverwritesOldestWhenFull(t *testing.T) {
	r := telemetry.NewRing(3)
	for _, m := range []string{"a", "b", "c", "d", "e"} {
		r.Enqueue([]*logs.Event{evt(logs.LevelWarn, m)})
	}
	stats := r.Stats()
	if stats.Pending != 3 {
		t.Errorf("expected Pending=3 at cap, got %d", stats.Pending)
	}
	if stats.Dropped != 2 {
		t.Errorf("expected Dropped=2 (a, b overwritten), got %d", stats.Dropped)
	}
	out := r.Drain(10)
	got := []string{out[0].Message, out[1].Message, out[2].Message}
	want := []string{"c", "d", "e"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("oldest-3 order broken: got %v, want %v", got, want)
		}
	}
}

func TestRing_PartialDrain(t *testing.T) {
	r := telemetry.NewRing(10)
	for _, m := range []string{"a", "b", "c", "d", "e"} {
		r.Enqueue([]*logs.Event{evt(logs.LevelWarn, m)})
	}
	out := r.Drain(2)
	if len(out) != 2 || out[0].Message != "a" || out[1].Message != "b" {
		t.Errorf("expected drain to take oldest 2 (a, b), got %v", messages(out))
	}
	stats := r.Stats()
	if stats.Pending != 3 {
		t.Errorf("expected 3 remaining, got %d", stats.Pending)
	}
	rest := r.Drain(10)
	if len(rest) != 3 || rest[0].Message != "c" {
		t.Errorf("expected next drain c,d,e, got %v", messages(rest))
	}
}

func TestRing_PeekDoesNotRemove(t *testing.T) {
	r := telemetry.NewRing(10)
	r.Enqueue([]*logs.Event{evt(logs.LevelWarn, "a"), evt(logs.LevelWarn, "b")})
	peek := r.Peek(10)
	if len(peek) != 2 {
		t.Fatalf("Peek expected 2 events, got %d", len(peek))
	}
	if r.Stats().Pending != 2 {
		t.Errorf("Peek must not remove; Pending=%d", r.Stats().Pending)
	}
}

func TestRing_NewRingZeroPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on capacity 0")
		}
	}()
	telemetry.NewRing(0)
}

func TestLevelFilter_DropsBelowMin(t *testing.T) {
	inner := telemetry.NewRing(100)
	filter := &telemetry.LevelFilter{Inner: inner, Min: logs.LevelWarn}
	filter.Enqueue([]*logs.Event{
		evt(logs.LevelDebug, "d"),
		evt(logs.LevelInfo, "i"),
		evt(logs.LevelWarn, "w"),
		evt(logs.LevelError, "e"),
	})
	out := inner.Drain(10)
	got := messages(out)
	if len(got) != 2 || got[0] != "w" || got[1] != "e" {
		t.Errorf("LevelWarn filter expected [w, e], got %v", got)
	}
}

func TestLevelFilter_UnknownLevelGetsDropped(t *testing.T) {
	// Documents the intentional behavior: events with a Level that
	// isn't in {DEBUG, INFO, WARN, ERROR} hit the zero value in
	// levelOrder (= 0) and get filtered out by any min ≥ INFO. Catches
	// programmer errors (someone emitting with empty string Level)
	// rather than silently shipping them to the monitoring inbox.
	inner := telemetry.NewRing(10)
	filter := &telemetry.LevelFilter{Inner: inner, Min: logs.LevelWarn}
	weird := logs.NewEvent(logs.EventError, "test", "weird")
	weird.Level = "" // not a known Level constant
	filter.Enqueue([]*logs.Event{weird, evt(logs.LevelError, "real")})
	out := inner.Drain(10)
	if len(out) != 1 || out[0].Message != "real" {
		t.Errorf("expected only the ERROR event to pass, got %v", messages(out))
	}
}

func TestLevelFilter_PassesThroughOnEmpty(t *testing.T) {
	inner := telemetry.NewRing(10)
	filter := &telemetry.LevelFilter{Inner: inner, Min: logs.LevelWarn}
	filter.Enqueue(nil)
	if inner.Stats().Pending != 0 {
		t.Errorf("expected 0, got %d", inner.Stats().Pending)
	}
}

func TestNullSink_DropsSilently(t *testing.T) {
	s := telemetry.NewNullSink()
	s.Enqueue([]*logs.Event{evt(logs.LevelError, "x")})
	if got := s.Drain(10); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if stats := s.Stats(); stats.Pending != 0 || stats.Capacity != 0 {
		t.Errorf("expected empty stats, got %+v", stats)
	}
}

func messages(events []*logs.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Message
	}
	return out
}
