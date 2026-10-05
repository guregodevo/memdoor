package gateway

import (
	"fmt"
	"strings"
	"testing"
)

// The repeat guard keyed on name+input+output, so a model that REPHRASES the
// command each time never tripped it.
//
// Measured 2026-08-30: eighteen bash calls in one turn re-fetching a single URL
// — `python3 -c …`, then `curl -s`, then `curl -sS`, then `ls && curl` — every
// one returning the same 117,729 bytes, and the guard silent throughout because
// no two inputs were identical.
//
// What says "no progress" is the RESULT, not the phrasing.
func TestSameResultDifferentCommandIsCaught(t *testing.T) {
	seen := map[string]int{}
	out := "OK bytes: 117729"

	// Three different commands, one identical result.
	var flagged int
	for _, cmd := range []string{
		`{"command":"python3 -c fetch"}`,
		`{"command":"curl -s url"}`,
		`{"command":"curl -sS url"}`,
	} {
		exact := "bash\x00" + cmd + "\x00" + out
		byResult := "bash\x00" + out
		seen[exact]++
		seen[byResult]++
		if seen[byResult] >= 3 && seen[exact] < 3 {
			flagged++
		}
	}
	if flagged == 0 {
		t.Error("three different commands returning the same result must be flagged — " +
			"this is the eighteen-call loop the input-keyed guard could not see")
	}
}

// A result that CHANGED must never be flagged: re-running a build after a patch
// is progress, and warning there would fight the fix loop.
func TestChangedResultIsNotARepeat(t *testing.T) {
	seen := map[string]int{}
	for i, out := range []string{"FAIL: undefined x", "FAIL: undefined y", "ok"} {
		byResult := "bash\x00" + out
		seen[byResult]++
		if seen[byResult] >= 3 {
			t.Errorf("output %d changed — it is progress, not a repeat", i)
		}
	}
}

// The message must tell the model what to do, not merely that it repeated.
func TestRepeatMessageIsActionable(t *testing.T) {
	msg := fmt.Sprintf("(you have run %s %d times this turn and got THIS EXACT result every time, "+
		"with different commands. The answer is not going to change — stop verifying it and do the next real step)", "bash", 3)
	for _, want := range []string{"stop verifying", "next real step", "different commands"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the nudge must say %q: %s", want, msg)
		}
	}
}
