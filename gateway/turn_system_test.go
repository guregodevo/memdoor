package gateway

import (
	"context"
	"errors"
	"testing"
)

// A TURN BUILDS ITS SYSTEM PROMPT ONCE. Later calls send the first build's
// bytes even when a build would now differ (the hour turned, AGENTS.md was
// edited). Mutation check: build on every call and the second call sees "B".
func TestATurnSendsTheSystemPromptItBuiltFirst(t *testing.T) {
	ts := turnSystemOf(withTurnSystem(context.Background()))
	builds, next := 0, "A"
	build := func() (string, error) { builds++; return next, nil }
	first, _ := ts.builtOnce(build)
	next = "B"
	second, _ := ts.builtOnce(build)
	if first != "A" || second != "A" || builds != 1 {
		t.Fatalf("got %q then %q after %d builds, want A, A after 1", first, second, builds)
	}
	if turnSystemOf(context.Background()) != nil {
		t.Fatal("a context with no turn has no turn system prompt")
	}
}

// A failed build is not kept: the turn's next call builds again.
func TestAFailedSystemPromptBuildIsTriedAgain(t *testing.T) {
	ts := turnSystemOf(withTurnSystem(context.Background()))
	if _, err := ts.builtOnce(func() (string, error) { return "", errors.New("no workspace") }); err == nil {
		t.Fatal("the error is returned")
	}
	if got, _ := ts.builtOnce(func() (string, error) { return "A", nil }); got != "A" {
		t.Fatalf("the next call builds: %q", got)
	}
}

// changed is false for the turn's first call and for the same prompt again,
// true when a call sends another one.
func TestASystemPromptChangeWithinATurnIsSeen(t *testing.T) {
	ts := turnSystemOf(withTurnSystem(context.Background()))
	for i, c := range []struct {
		prompt string
		want   bool
	}{{"A", false}, {"A", false}, {"A\n\nnudge", true}, {"A\n\nnudge", false}, {"A", true}} {
		if got := ts.changed(c.prompt); got != c.want {
			t.Fatalf("call %d %q: changed %v, want %v", i, c.prompt, got, c.want)
		}
	}
}
