package gateway

import (
	"context"
	"encoding/json"

	"memdoor/pkg/savings"
	"memdoor/tools"
)

// Per-turn tool narrowing — OpenClaw's before_prompt_build `toolsAllow`.
//
// A turn may carry an allow list that narrows the tools SUBMITTED to the model
// for that turn. It is not an enforcement boundary: executeTool still checks
// the agent's full palette, so a narrowed-out tool the model names anyway
// runs. Rules, as in OpenClaw:
//   - no list on the context: the submitted surface is unchanged;
//   - several contributors intersect;
//   - an empty list submits none of the narrowable tools.

type ctxKeyTurnToolsAllow struct{}

// withTurnToolsAllow narrows the turn's tool surface to allow, intersected
// with any narrowing already on ctx.
func withTurnToolsAllow(ctx context.Context, allow []string) context.Context {
	next := make(map[string]bool, len(allow))
	for _, n := range allow {
		next[n] = true
	}
	if prev, ok := ctx.Value(ctxKeyTurnToolsAllow{}).(map[string]bool); ok {
		for n := range next {
			if !toolAllowed(prev, n) {
				delete(next, n)
			}
		}
	}
	return context.WithValue(ctx, ctxKeyTurnToolsAllow{}, next)
}

// submittedTools applies the turn's narrowing, if any, to the agent's tools.
func submittedTools(ctx context.Context, all []tools.ToolDefinition) []tools.ToolDefinition {
	allow, ok := ctx.Value(ctxKeyTurnToolsAllow{}).(map[string]bool)
	if !ok {
		return all
	}
	out := make([]tools.ToolDefinition, 0, len(allow))
	for _, t := range all {
		if toolAllowed(allow, t.Name) {
			out = append(out, t)
		}
	}
	return out
}

// schemaBytes is what a tool palette costs in a request: its names,
// descriptions and JSON schemas, which travel on EVERY call of a turn. Used to
// price what the per-turn narrowing saved (pkg/savings).
func schemaBytes(defs []tools.ToolDefinition) int {
	n := 0
	for _, d := range defs {
		b, err := json.Marshal(map[string]any{"name": d.Name, "description": d.Description, "input_schema": d.InputSchema})
		if err != nil {
			continue
		}
		n += len(b)
	}
	return n
}

// recordToolboxSaving records what the narrowed palette kept out of ONE call.
// One entry per call rather than one per turn: the arithmetic is then exact
// with no bookkeeping to carry to the end of the turn, and a turn that ends
// early is not charged for calls it never made.
func recordToolboxSaving(full, submitted []tools.ToolDefinition, model string) {
	if len(submitted) == 0 || len(submitted) >= len(full) {
		return
	}
	savings.Record(savings.Entry{
		Kind: savings.KindToolbox, Calls: 1, Model: model,
		RawBytes: schemaBytes(full), KeptBytes: schemaBytes(submitted),
	})
}
