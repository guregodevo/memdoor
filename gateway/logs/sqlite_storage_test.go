package logs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// Regex queries must work on EVERY pooled connection — the REGEXP function is
// installed via ConnectHook. The prior single-connection registration made
// concurrent regex queries randomly fail with "no such function: REGEXP"
// (a week of phantom empty log results traced back to it).
func TestRegexpWorksOnEveryPooledConnection(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStorage(dir + "/logs.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	ev := NewEvent(EventStateChange, "T", "needle-xyz present").WithLevel(LevelInfo)
	if err := s.WriteEvents(ctx, []*Event{ev}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.QueryEvents(ctx, &Query{MessageRegex: "needle-.yz", Limit: 5})
			if err != nil {
				errs <- err
				return
			}
			if len(res.Events) != 1 {
				errs <- fmt.Errorf("want 1 event, got %d", len(res.Events))
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// The log has a size ceiling: over it, the oldest events go until the file
// is well under, the newest stay, and the space is given back. An age-only
// rule let a chatty scheduler write 40 GB in a night (2026-10-26).
func TestTheLogHasASizeCeiling(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSQLiteStorage(dir + "/logs.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	long := strings.Repeat("x", 2000)
	base := time.Now().Add(-time.Hour)
	for batch := 0; batch < 20; batch++ {
		evs := make([]*Event, 0, 100)
		for i := 0; i < 100; i++ {
			ev := NewEvent(EventStateChange, "T", fmt.Sprintf("row %d %s", batch*100+i, long)).WithLevel(LevelInfo)
			ev.Timestamp = base.Add(time.Duration(batch*100+i) * time.Second)
			evs = append(evs, ev)
		}
		if err := s.WriteEvents(ctx, evs); err != nil {
			t.Fatal(err)
		}
	}
	s.reclaim(ctx)
	before := s.FileSize()
	ceiling := before / 2
	n, err := s.PruneToSize(ctx, ceiling)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatalf("nothing pruned from %d bytes under a %d ceiling", before, ceiling)
	}
	if after := s.FileSize(); after > ceiling {
		t.Fatalf("%d bytes after the prune, ceiling %d (was %d)", after, ceiling, before)
	}
	res, err := s.QueryEvents(ctx, &Query{MessageRegex: "^row 1999 ", Limit: 1})
	if err != nil || len(res.Events) != 1 {
		t.Fatalf("the newest row is gone (%v, %d events)", err, len(res.Events))
	}
	res, err = s.QueryEvents(ctx, &Query{MessageRegex: "^row 0 ", Limit: 1})
	if err != nil || len(res.Events) != 0 {
		t.Fatalf("the oldest row survived (%v, %d events)", err, len(res.Events))
	}
	if n, err := s.PruneToSize(ctx, ceiling); err != nil || n != 0 {
		t.Fatalf("a second sweep under the ceiling pruned %d (%v)", n, err)
	}
}
