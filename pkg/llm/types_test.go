package llm

import "testing"

// A tool result is never an empty text block: a Claude request refuses the
// whole message for it (live 2026-11-18, a grep with no matches), so an
// empty result is written as "(no output)" — the words every shape already
// understands as "the tool said nothing".
func TestAnEmptyToolResultSaysSo(t *testing.T) {
	b := NewToolResultBlock("tu_1", "", false)
	if b.OfToolResult.Content[0].OfText.Text != "(no output)" {
		t.Fatalf("empty result: %+v", b.OfToolResult)
	}
	if got := NewToolResultBlock("tu_2", "a.go", false).OfToolResult.Content[0].OfText.Text; got != "a.go" {
		t.Fatalf("a real result is untouched: %q", got)
	}
}
