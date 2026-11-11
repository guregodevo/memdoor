package cmd

import "testing"

type frame struct {
	delta      string
	whole      string
	appendTail bool
	toolsSince bool
}

func printed(frames []frame) string {
	var t turnText
	out := ""
	for _, f := range frames {
		if f.delta != "" {
			out += t.delta(f.delta)
			continue
		}
		out += t.whole(f.whole, f.appendTail, f.toolsSince)
	}
	return out
}

// The headless cast's second take: a draft streamed and was suppressed, the
// final round streamed, and the gateway sent the final whole (the window
// replaces its draft). The answer is printed once, then the receipt.
func TestTurnTextFinalAfterASuppressedDraftIsNotRepeated(t *testing.T) {
	got := printed([]frame{
		{delta: "Draft."}, {whole: "Draft."},
		{delta: "Fixed."}, {whole: "Fixed."},
		{whole: "Fixed.\n\n✓ checked: 1 file changed"},
	})
	if want := "Draft.Fixed.\n\n✓ checked: 1 file changed"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A round's whole text after its deltas, and an appended tail, as before.
func TestTurnTextStreamedRoundAndAppendedTail(t *testing.T) {
	got := printed([]frame{
		{delta: "Done"}, {delta: "."}, {whole: "Done."},
		{whole: "✓ checked: 1 file changed", appendTail: true},
	})
	if want := "Done.\n✓ checked: 1 file changed"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A provider that sends no deltas: each round's whole text is printed.
func TestTurnTextUnstreamedRounds(t *testing.T) {
	got := printed([]frame{
		{whole: "Reading."},
		{whole: "Done.", toolsSince: true},
	})
	if want := "Reading.Done."; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A final that says something else than what streamed is printed whole, on
// its own line.
func TestTurnTextRewrittenFinalStartsOnItsOwnLine(t *testing.T) {
	got := printed([]frame{
		{delta: "First answer."}, {whole: "First answer."},
		{whole: "Stopped: the same tool kept failing."},
	})
	if want := "First answer.\nStopped: the same tool kept failing."; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
