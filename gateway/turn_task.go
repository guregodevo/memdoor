package gateway

import (
	"context"
	"encoding/json"
)

// turnTaskKey carries the request a turn is working on.
type turnTaskKey struct{}

// turnTaskTools judge their output against the turn's task when a decision
// model answers: read_file past 32 KB returns the sections that matter,
// notes returns the sections that help, web_fetch past its budget returns
// the page's sections that matter instead of its head, and a read-only bash
// command past 2 KB returns the sections that matter (tools/agent.go
// bashReadOutput). The task goes in the input as
// turn_task — like cwd, in no schema — so it is per call, for every agent,
// and safe with concurrent turns (a process-wide brief is not).
var turnTaskTools = map[string]bool{"bash": true, "read_file": true, "notes": true, "web_fetch": true}

const turnTaskMax = 2000

func withTurnTask(ctx context.Context, tool string, input json.RawMessage) json.RawMessage {
	if !turnTaskTools[tool] {
		return input
	}
	task, _ := ctx.Value(turnTaskKey{}).(string)
	if task == "" {
		return input
	}
	var in map[string]any
	if json.Unmarshal(input, &in) != nil || in == nil {
		return input
	}
	if _, set := in["turn_task"]; set {
		return input
	}
	in["turn_task"] = clipHead(task, turnTaskMax)
	out, err := json.Marshal(in)
	if err != nil {
		return input
	}
	return out
}

// withHarness sets fields of a tool's input that belong to the harness (whose
// notes, which instructions file, which transcript). It always overwrites:
// the model chooses none of them.
func withHarness(input json.RawMessage, fields map[string]any) json.RawMessage {
	var in map[string]any
	if json.Unmarshal(input, &in) != nil || in == nil {
		return input
	}
	for k, v := range fields {
		in[k] = v
	}
	out, err := json.Marshal(in)
	if err != nil {
		return input
	}
	return out
}
