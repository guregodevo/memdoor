package context

import (
	"strings"
	"testing"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

// The window is read when a turn is checked, not when the checker is built.
// Live 2026-09-27: the checker was built before the engine was written and
// held the local 32K profile, so a 1.3M-token model compacted twice in one
// review. The mutation check: read pc.limits in Check and the second check
// still refuses.
func TestPreflightFollowsTheEngineServingTheTurn(t *testing.T) {
	if err := logs.InitGlobalLogger(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	window := 0
	SetRemoteWindow(func(string) int { return window })
	t.Cleanup(func() { SetRemoteWindow(nil) })

	pc, err := NewPreflightChecker("default", nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	big := tokensOf(40_000)
	if r, _ := pc.CheckQuick(string(big), nil, nil); r.CanProceed {
		t.Fatalf("40K tokens should not fit the placeholder window (limit %d)", r.EffectiveLimit)
	}
	window = 262_144
	if r, _ := pc.CheckQuick(string(big), nil, nil); !r.CanProceed {
		t.Fatalf("40K tokens must fit a 262K engine, got limit %d", r.EffectiveLimit)
	}
}

// tokensOf is a system prompt of about n tokens.
func tokensOf(n int) string {
	b := make([]byte, 4*n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// The window is the ANSWERING model's, and compaction starts at 60% of it but
// never past CompactionCeiling. Live 2026-09-29: every model on the person's
// own key, 128K to 1.3M, was measured against one fixed 262K. The mutation
// checks: measure against the checker's own model (the pinned small model then
// never compacts), or drop the ceiling (the 1M model carries 210K).
func TestPreflightMeasuresTheAnsweringModel(t *testing.T) {
	if err := logs.InitGlobalLogger(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	SetRemoteWindow(func(model string) int {
		switch model {
		case "small/128k":
			return 131_072
		case "big/1m":
			return 1_048_576
		}
		return 262_144
	})
	t.Cleanup(func() { SetRemoteWindow(nil) })
	pc, err := NewPreflightChecker("default", nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	check := func(model string, tokens int) *CheckResult {
		t.Helper()
		r, _ := pc.Check("", "", model, tokensOf(tokens), nil, nil)
		return r
	}

	if r := check("small/128k", 90_000); !r.ShouldCompact || !r.CanProceed {
		t.Fatalf("90K on a 128K model is past 60%%: compact=%v proceed=%v limit=%d", r.ShouldCompact, r.CanProceed, r.EffectiveLimit)
	}
	if r := check("big/1m", 90_000); r.ShouldCompact {
		t.Fatalf("90K on a 1M model is far under 60%%, limit %d", r.EffectiveLimit)
	}
	if r := check("big/1m", 210_000); !r.ShouldCompact || !r.CanProceed {
		t.Fatalf("210K is past the ceiling on any window: compact=%v proceed=%v", r.ShouldCompact, r.CanProceed)
	}
	if r := check("small/128k", 140_000); r.CanProceed {
		t.Fatal("140K cannot go to a 128K model")
	}
	if r := check("big/1m", 140_000); !r.CanProceed {
		t.Fatal("140K fits a 1M model")
	}
	if r := check("", 90_000); r.EffectiveLimit != 262_144-16_384 {
		t.Fatalf("no model named: the engine's window, got %d", r.EffectiveLimit)
	}
}

// /context says where compaction starts: the same number the check uses.
func TestCompactAtIsTheChecksThreshold(t *testing.T) {
	if err := logs.InitGlobalLogger(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	SetRemoteWindow(func(string) int { return 131_072 })
	t.Cleanup(func() { SetRemoteWindow(nil) })
	pc, err := NewPreflightChecker("default", nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := pc.Check("", "", "m", tokensOf(100), nil, nil)
	if at := pc.CompactAt("m"); at != r.CompactAt || at != (131_072-16_384)*60/100 {
		t.Fatalf("CompactAt %d, the check's %d", at, r.CompactAt)
	}
}

// Tool output is counted apart: it is what compaction gives up first.
func TestCountToolOutputTokens(t *testing.T) {
	tc := NewTokenCounter("m", false)
	msgs := []llm.MessageParam{
		llm.NewUserMessage(llm.NewTextBlock(strings.Repeat("q", 4000))),
		llm.NewUserMessage(llm.NewToolResultBlock("t1", strings.Repeat("x", 8000), false)),
	}
	if got, want := tc.CountToolOutputTokens(msgs), tc.EstimateTokens(strings.Repeat("x", 8000)); got != want {
		t.Fatalf("only the tool result: %d, want %d", got, want)
	}
}

// compaction.thresholdTokens takes precedence over the percentage, never past
// the effective limit, and /context says which rule is in force.
func TestAFixedThresholdTakesPrecedence(t *testing.T) {
	if err := logs.InitGlobalLogger(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	SetRemoteWindow(func(string) int { return 131_072 })
	t.Cleanup(func() { SetRemoteWindow(nil) })
	pc, err := NewPreflightChecker("default", nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(pc.CompactRule(), "60% of the window") {
		t.Fatalf("the default rule: %s", pc.CompactRule())
	}
	pc.SetThresholdTokens(20_000)
	if at := pc.CompactAt("m"); at != 20_000 || pc.CompactRule() != "thresholdTokens" {
		t.Fatalf("a fixed trigger: %d (%s)", at, pc.CompactRule())
	}
	if r, _ := pc.Check("", "", "m", tokensOf(21_000), nil, nil); !r.ShouldCompact {
		t.Fatal("past the fixed trigger it compacts")
	}
	pc.SetThresholdTokens(10_000_000)
	if at := pc.CompactAt("m"); at != 131_072-16_384 {
		t.Fatalf("never past the effective limit: %d", at)
	}
}

// The per-call cap never exceeds the model's own output cap, and the
// ceiling travels with the limits for the escalation to stop at.
func TestOutputCapFollowsTheModelsOwn(t *testing.T) {
	SetRemoteWindow(func(string) int { return 131072 })
	SetRemoteOutput(func(model string) int {
		if model == "small" {
			return 4096
		}
		if model == "big" {
			return 65536
		}
		return 0
	})
	t.Cleanup(func() { SetRemoteWindow(nil); SetRemoteOutput(nil) })
	if l, _ := GetModelLimits("small"); l.MaxOutputTokens != 4096 || l.MaxOutputCeiling != 4096 || l.CompactionReserve != 4096 {
		t.Fatalf("a 4k model: %+v", l)
	}
	if l, _ := GetModelLimits("big"); l.MaxOutputTokens != 16384 || l.MaxOutputCeiling != 65536 {
		t.Fatalf("a 64k model keeps the 16k per-call default, ceiling 64k: %+v", l)
	}
	if l, _ := GetModelLimits("unknown"); l.MaxOutputTokens != 16384 || l.MaxOutputCeiling != 0 {
		t.Fatalf("unknown cap: as before: %+v", l)
	}
}
