package tools

import (
	"strings"
	"testing"
)

func TestWebFetchJudgesALongPage(t *testing.T) {
	withDecisions(t, &scriptedDecisions{p: func(q string) float64 {
		if strings.Contains(q, "cold proof") {
			return 0.9
		}
		return 0.05
	}})
	var b strings.Builder
	for i := 0; i < 400; i++ {
		b.WriteString("Navigation link, cookie banner text, unrelated paragraph filler.\n\n")
	}
	b.WriteString("The cold proof overnight in the fridge develops flavour.\n")
	page := b.String()
	out := capExtractedFor(page, "https://example.com/bread", "What does the page say about the cold proof?")
	if !strings.Contains(out, "sections that matter") || !strings.Contains(out, "cold proof overnight") || len(out) > DefaultMaxExtractedChars+100 {
		t.Fatalf("judged: len %d\n%.300s", len(out), out)
	}
	// No task: the head, as before.
	if out := capExtractedFor(page, "https://example.com/bread", ""); strings.Contains(out, "cold proof overnight") || !strings.Contains(out, "truncated") {
		t.Fatal("unjudged page must be the head")
	}
}
