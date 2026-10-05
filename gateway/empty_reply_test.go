package gateway

import (
	"strings"
	"testing"
)

// A BLANK TURN IS NEVER THE ANSWER. An empty reply is retried once whether
// or not a tool has run this turn (the "continue" after the first blank had
// run none and was never retried), never twice, and never for a prose agent;
// when the retry is blank too the window gets a sentence, not silence.
func TestAnEmptyReplyIsRetriedOnceThenSaidOutLoud(t *testing.T) {
	if !emptyReplyRetryWanted("", false, true) {
		t.Fatal("an empty reply with no tool run this turn is still retried")
	}
	if !emptyReplyRetryWanted("  \n", false, true) {
		t.Fatal("whitespace is empty")
	}
	if emptyReplyRetryWanted("", true, true) {
		t.Fatal("retried once already: not again")
	}
	if emptyReplyRetryWanted("", false, false) {
		t.Fatal("a prose agent's tool-only turn ends silently by design")
	}
	if emptyReplyRetryWanted("done.", false, true) {
		t.Fatal("a reply with words is an answer")
	}
	if !strings.Contains(emptyReplyNotice, "Ask again") || !strings.Contains(emptyReplyNotice, "nothing was done") {
		t.Fatalf("the notice says what happened and what to do: %q", emptyReplyNotice)
	}
}
