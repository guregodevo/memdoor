package gateway

import (
	"context"
	"strings"
	"testing"

	"memdoor/gateway/providers"
	sharedctx "memdoor/pkg/shared/context"
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

// Blank twice on a rung, the ladder's next rung gets one try: the next
// tier and its model, unless the person pinned a model, the ladder is at
// its last rung (a clamped tier is the same model), or no engine answers.
func TestEmptyTwiceClimbsToTheNextRung(t *testing.T) {
	if err := providers.SetRemoteEngine(providers.RemoteEngine{Model: "x", Endpoint: "http://e", AgentLadders: map[string][]string{"coder": {"a", "b", "c"}}}); err != nil {
		t.Fatal(err)
	}
	defer providers.ClearRemoteEngine()

	if rung, m := nextRungAfterEmpty(context.Background(), "coder"); rung != 1 || m != "b" {
		t.Fatalf("from the first rung: got %d %q, want 1 b", rung, m)
	}
	second := context.WithValue(context.Background(), sharedctx.TierKey, 1)
	if rung, m := nextRungAfterEmpty(second, "coder"); rung != 2 || m != "c" {
		t.Fatalf("from the second rung: got %d %q, want 2 c", rung, m)
	}
	last := context.WithValue(context.Background(), sharedctx.TierKey, 2)
	if _, m := nextRungAfterEmpty(last, "coder"); m != "" {
		t.Fatalf("the last rung has nothing above it, got %q", m)
	}
	pinned := context.WithValue(context.Background(), sharedctx.ModelKey, "z")
	if _, m := nextRungAfterEmpty(pinned, "coder"); m != "" {
		t.Fatalf("a pinned model is the person's choice, got %q", m)
	}
}
