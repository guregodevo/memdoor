package ui

import (
	"strings"
	"testing"
)

// ONE PRODUCT, TWO FRONT ENDS, AND BOTH NAME THE MODEL — NEVER THE MONEY.
// The window shows where the work is, what the brain is doing and which
// model answers (Greg, 2026-09-26: the product routes each agent to a
// model, so the person sees which one). What it costs and what is left stay
// the operator's business, behind admin auth on the broker (2026-09-05).
func TestTheFooterNamesTheModelAndNoMoneyInEitherFrontEnd(t *testing.T) {
	full := Status{Branch: "main",
		Model: "deepseek/deepseek-v4.1-flash"}
	line := full.line()
	for _, leak := range []string{"0.40", "18.80", "/hr"} {
		if strings.Contains(line, leak) {
			t.Fatalf("leaked %q: %q", leak, line)
		}
	}
	if !strings.Contains(line, "main") || !strings.Contains(line, "deepseek-v4.1-flash") {
		t.Fatalf("dropped what the window is for: %q", line)
	}
}
