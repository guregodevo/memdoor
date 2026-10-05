package gateway

import (
	"strings"
	"testing"
)

const metricsSchema = `{"type":"object","required":["ticker","net_margin_pct"],"properties":{"ticker":{"type":"string"},"net_margin_pct":{"type":"number"}}}`

// The typed answer: the last json block is found, checked, and kept compact;
// a mismatch says why in the validator's words; no JSON at all is an error.
func TestMatchOutputSchema(t *testing.T) {
	got, err := matchOutputSchema(metricsSchema, "Done.\n\n```json\n{\"ticker\": \"NVDA\", \"net_margin_pct\": 55.6}\n```\n")
	if err != nil || got != `{"net_margin_pct":55.6,"ticker":"NVDA"}` {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := matchOutputSchema(metricsSchema, "```json\n{\"ticker\": \"NVDA\", \"net_margin_pct\": \"55.6%\"}\n```"); err == nil || !strings.Contains(err.Error(), "net_margin_pct") {
		t.Fatalf("a string where a number is required must fail, naming the field: %v", err)
	}
	if _, err := matchOutputSchema(metricsSchema, "I computed it, the margin is high."); err == nil {
		t.Fatal("prose with no JSON must fail")
	}
	if got, err := matchOutputSchema(metricsSchema, `result: {"ticker":"AAPL","net_margin_pct":26.9} done`); err != nil || !strings.Contains(got, "AAPL") {
		t.Fatalf("a bare object is found too: %q %v", got, err)
	}
}
