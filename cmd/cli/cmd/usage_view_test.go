package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memdoor/pkg/savings"
)

// /usage shows the month's turns, their price on the person's own key and
// what the decision model kept out — never the billing lines it used to
// print ("The good brain has been working for 0 min", "1 of 1 seat in use").
func TestUsageShowsTheReceiptAndThePrice(t *testing.T) {
	m := meterMonth{Requests: 12, In: 480_000, Out: 9_000, Cost: 0.42, Priced: 12,
		ByModel: []meterModel{{model: "z-ai/glm-5.3-flash", requests: 12, in: 480_000}}}
	s := savings.Summary{Month: "2026-09", JudgedRaw: 400_000, JudgedKept: 40_000, BytesPerToken: 4}
	out := usageView("**Pro** · your own OpenRouter key · the decision model is on", true, m, s)
	for _, want := range []string{"12 model requests", "12 requests", "glm-5.3-flash", "kept **≈", "memdoor savings", "priced on 12"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	for _, never := range []string{"good brain", "seat in use", "ran on the seat"} {
		if strings.Contains(out, never) {
			t.Fatalf("/usage must not say %q:\n%s", never, out)
		}
	}
}

// With no decision model (no OpenRouter key, no decision key) jgrep/jread are not
// offered, so "kept nothing out yet: it works through jgrep, jread…" would read
// as if the model were running and empty — it is simply not there. Since
// 2026-10-03 the person's own key runs it, so that is the way on to name.
func TestUsageWithoutADecisionModelSaysHowToTurnItOn(t *testing.T) {
	m := meterMonth{Requests: 3, In: 1_000, Out: 100, ByModel: []meterModel{{model: "z-ai/glm-5.3-flash", requests: 3, in: 1_000}}}
	out := usageView("**Your company's key**", false, m, savings.Summary{Month: "2026-09", BytesPerToken: 4})
	if !strings.Contains(out, "The decision model is off") || !strings.Contains(out, "memdoor connect typesafe") {
		t.Fatalf("with no decision model the hint names memdoor connect typesafe:\n%s", out)
	}
	if strings.Contains(out, "jgrep") {
		t.Fatalf("no decision model, no jgrep/jread pitch:\n%s", out)
	}
	for _, header := range []string{"**Your own OpenRouter key** · the decision model is on", "**Pro** · the decision model is on"} {
		kept := usageView(header, true, m, savings.Summary{Month: "2026-09"})
		if !strings.Contains(kept, "The decision model has kept nothing out yet this month: it works through jgrep, jread and the per-turn toolbox.") {
			t.Fatalf("the model runs:\n%s", kept)
		}
	}
}

// Only lines OpenRouter priced carry a price; any other line (an old broker
// line, a vendor's) counts as a request but never as a price.
func TestMeterPricesOnlyTheirOwnKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".memdoor"), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"ts":"2026-09-27T10:00:00Z","model":"z-ai/glm-5.3-flash","input_tokens":100,"cost_usd":0.5,"engine":"https://memdoor.ai/billing/v1/brain"}
{"ts":"2026-09-27T10:01:00Z","model":"z-ai/glm-5.3-flash","input_tokens":100,"cost_usd":0.01,"engine":"https://openrouter.ai/api/v1"}
`
	if err := os.WriteFile(filepath.Join(home, ".memdoor", "meter.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	m := readMeterMonth(time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	if m.Requests != 2 || m.Priced != 1 || m.Cost != 0.01 {
		t.Fatalf("want 2 requests, 1 priced at $0.01; got %+v", m)
	}
}

// One model is one row: the broker named it without the vendor, OpenRouter
// with it, and /usage showed "qwen3.8-omni-flash" and "qwen/qwen3.8-omni-flash"
// as two models (2026-09-29).
func TestUsageCountsOneModelOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".memdoor"), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"ts":"2026-09-27T10:00:00Z","model":"qwen3.8-omni-flash","input_tokens":100}
{"ts":"2026-09-27T10:01:00Z","model":"qwen/qwen3.8-omni-flash","input_tokens":50}
{"ts":"2026-09-27T10:02:00Z","model":"z-ai/glm-5.3-flash","input_tokens":10}
`
	if err := os.WriteFile(filepath.Join(home, ".memdoor", "meter.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	m := readMeterMonth(time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	if len(m.ByModel) != 2 {
		t.Fatalf("two models, two rows: %+v", m.ByModel)
	}
	if top := m.ByModel[0]; top.model != "qwen/qwen3.8-omni-flash" || top.requests != 2 || top.in != 150 {
		t.Fatalf("one row, the full name, both requests: %+v", top)
	}
	out := usageView("**Pro**", true, m, savings.Summary{Month: "2026-09", EarlyStops: 1, JudgedRaw: 400, BytesPerToken: 4})
	if !strings.Contains(out, "3 model requests") || !strings.Contains(out, "1 request ·") || !strings.Contains(out, "1 turn ended") {
		t.Fatalf("requests, and the singular when there is one:\n%s", out)
	}
}
