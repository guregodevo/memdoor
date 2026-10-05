package logs

import (
	"context"
	"fmt"
	"sync"
	"testing"
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
