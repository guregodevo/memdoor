package logs

import (
	"context"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

func TestQueryFromParamsReadsWhatTheGatewayTakes(t *testing.T) {
	q, err := QueryFromParams(url.Values{"since": {"2d"}, "regex": {"keep-alive"}, "limit": {"7"}, "session": {"s1"}, "run": {"r1"}})
	if err != nil {
		t.Fatal(err)
	}
	if q.Limit != 7 || q.MessageRegex != "keep-alive" || q.Session != "s1" || q.RunID != "r1" {
		t.Fatalf("%+v", q)
	}
	if q.Since != 48*time.Hour {
		t.Fatalf("2d: %v", q.Since)
	}
	if _, err := QueryFromParams(url.Values{"since": {"soon"}}); err == nil {
		t.Fatal("an unreadable since is refused")
	}
	e, err := ErrorsQueryFromParams(url.Values{})
	if err != nil || e.Limit != 20 || len(e.Levels) != 1 || e.Levels[0] != LevelError || e.Since != 24*time.Hour {
		t.Fatalf("errors defaults: %+v %v", e, err)
	}
}

// The file another process writes is read without a write: a regex query
// works, a write is refused, a missing file is an error.
func TestOpenReadOnlyReadsAndNeverWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	if _, err := OpenReadOnly(path); err == nil {
		t.Fatal("a missing file opens")
	}
	w, err := NewSQLiteStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	ev := NewEvent(EventStateChange, "Remote", "provider sent only keep-alive comments before any text; asking again")
	ev.Level = LevelWarn
	if err := w.WriteEvents(context.Background(), []*Event{ev, NewEvent(EventStateChange, "Gateway", "ready")}); err != nil {
		t.Fatal(err)
	}

	r, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	q, _ := QueryFromParams(url.Values{"regex": {"keep-alive|went silent"}, "since": {"1h"}})
	res, err := r.QueryEvents(context.Background(), q)
	if err != nil || len(res.Events) != 1 || res.Events[0].Component != "Remote" {
		t.Fatalf("%+v %v", res, err)
	}
	if err := r.WriteEvents(context.Background(), []*Event{NewEvent(EventStateChange, "x", "y")}); err == nil {
		t.Fatal("a read-only store wrote")
	}
}
