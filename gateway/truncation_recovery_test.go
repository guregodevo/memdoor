package gateway

import (
	"testing"

	"memdoor/pkg/llm"
)

// A reply cut off at max_tokens did NOT finish. Whatever it was doing is
// incomplete and the tool call it was about to emit does not exist — so it must
// be distinguishable from a reply that chose to stop, or the turn ends on a
// half-written answer.
//
// Measured 2026-08-30: turns of exactly 16,384 output tokens
// taking 369s at 44 tok/s, ending with no tool call and nothing done, against
// healthy turns of 65-90 tokens. Nothing recovered, and nothing said it had
// happened.
func TestTruncationHasItsOwnStopReason(t *testing.T) {
	if llm.StopReasonMaxTokens == llm.StopReasonEndTurn {
		t.Fatal("a truncated reply must not look like one that ended normally")
	}
	if llm.StopReasonMaxTokens == llm.StopReasonToolUse {
		t.Fatal("a truncated reply must not look like a tool call")
	}
}

// The escalated cap must actually be BIGGER, or stage one of recovery is a
// no-op that just burns another full generation at the same limit.
func TestEscalatedCapIsLarger(t *testing.T) {
	base, esc := maxOutputTokens(""), escalatedMaxOutputTokens("")
	if esc <= base {
		t.Errorf("escalated cap %d must exceed the normal cap %d — retrying at the same "+
			"limit would hit it again after another full generation", esc, base)
	}
}

// The override applies to exactly one retry and nothing else.
func TestMaxOutputTokensOverrideIsScopedToTheRetry(t *testing.T) {
	ctx := t.Context()
	if got := maxOutputTokensFor(ctx, ""); got != maxOutputTokens("") {
		t.Errorf("a normal inference uses the normal cap, got %d", got)
	}
	esc := escalatedMaxOutputTokens("")
	if got := maxOutputTokensFor(withMaxOutputOverride(ctx, esc), ""); got != esc {
		t.Errorf("the escalated retry must use %d, got %d", esc, got)
	}
}
