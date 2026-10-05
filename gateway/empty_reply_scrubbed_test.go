package gateway

import (
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

// A SCRUBBED REPLY IS RETRIED WITH THE REASON. A bare blank is retried as
// is. Mutation check: return extra for every stop and the nudge is gone.
func TestAScrubbedReplyIsRetriedWithTheReason(t *testing.T) {
	if got := emptyRetryPrompt("be brief", llm.StopReasonScrubbed); !strings.HasPrefix(got, "be brief") || !strings.Contains(got, "could not be read") {
		t.Fatalf("the nudge follows the prompt: %q", got)
	}
	if got := emptyRetryPrompt("be brief", llm.StopReasonEndTurn); got != "be brief" {
		t.Fatalf("a bare blank changes nothing: %q", got)
	}
	if got := emptyRetryPrompt("", llm.StopReasonScrubbed); !strings.Contains(got, "<tool_call>") {
		t.Fatalf("no prompt, the nudge alone: %q", got)
	}
}
