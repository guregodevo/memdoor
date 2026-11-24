package gateway

import (
	"testing"

	"memdoor/pkg/llm"
)

// A SCRUBBED REPLY IS RETRIED WITH THE REASON. A bare blank is retried as
// is. Mutation check: return "" for every stop and the nudge is gone.
func TestAScrubbedReplyIsRetriedWithTheReason(t *testing.T) {
	if got := emptyRetryNudge(llm.StopReasonScrubbed); got != scrubbedNudge {
		t.Fatalf("a scrubbed reply gets the nudge: %q", got)
	}
	if got := emptyRetryNudge(llm.StopReasonEndTurn); got != "" {
		t.Fatalf("a bare blank changes nothing: %q", got)
	}
}
