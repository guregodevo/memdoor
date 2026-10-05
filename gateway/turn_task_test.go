package gateway

import (
	"context"
	"encoding/json"
	"testing"
)

func TestWithTurnTask(t *testing.T) {
	ctx := context.WithValue(context.Background(), turnTaskKey{}, "make a short about sourdough")
	var in map[string]any
	json.Unmarshal(withTurnTask(ctx, "read_file", json.RawMessage(`{"path":"a.json"}`)), &in)
	if in["turn_task"] != "make a short about sourdough" || in["path"] != "a.json" {
		t.Fatalf("read_file: %v", in)
	}
	if got := string(withTurnTask(ctx, "apply_patch", json.RawMessage(`{"input":"x"}`))); got != `{"input":"x"}` {
		t.Fatalf("apply_patch must be untouched: %s", got)
	}
	// bash gets the task: a read-only command past 2 KB is judged against it.
	in = nil
	json.Unmarshal(withTurnTask(ctx, "bash", json.RawMessage(`{"command":"sed -n 1,400p a.go"}`)), &in)
	if in["turn_task"] != "make a short about sourdough" || in["command"] != "sed -n 1,400p a.go" {
		t.Fatalf("bash: %v", in)
	}
	if got := string(withTurnTask(context.Background(), "notes", json.RawMessage(`{"read":true}`))); got != `{"read":true}` {
		t.Fatalf("no task: %s", got)
	}
}
