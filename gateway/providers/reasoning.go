package providers

import (
	"context"

	sharedctx "memdoor/pkg/shared/context"
)

// Reasoning effort. With no reasoning parameter a model thinks as its host
// defaults, and GLM 5.3 Flash defaults to its maximum (omp's catalogue,
// classes/glm.kdl): 16-46K tokens of thinking before a plan or a first tool
// call were measured on it (2026-09-29/30). Every turn now carries an effort
// (turn_effort.go: the person's Shift+Tab, else the decision model, else
// high), sent as OpenRouter's reasoning.effort.
//
// Only to a model whose catalogue entry lists "reasoning": requests also say
// require_parameters, so a parameter no host takes would leave no host.

var reasoningSupport func(model string) bool

// SetReasoningSupport wires the catalogue's answer to "does model take
// reasoning.effort".
func SetReasoningSupport(fn func(model string) bool) { reasoningSupport = fn }

// reasoningFor is the request's reasoning field for model, nil when none.
func reasoningFor(ctx context.Context, model string) map[string]any {
	effort, _ := ctx.Value(sharedctx.EffortKey).(string)
	if effort == "" || reasoningSupport == nil || !reasoningSupport(model) {
		return nil
	}
	return map[string]any{"effort": effort}
}
