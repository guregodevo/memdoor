package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"memdoor/gateway"
)

// The gateway's /compact answer decodes field for field: a renamed field
// decodes to zero, and the line would say "Nothing worth compacting".
func TestCompactReplyMatchesTheGateway(t *testing.T) {
	b, _ := json.Marshal(gateway.CompactResult{BeforeTokens: 18_200, AfterTokens: 6_100, BeforeMessages: 17, AfterMessages: 5, Summarized: true, Written: true})
	var r compactReply
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r != (compactReply{18_200, 6_100, 17, 5, true, true}) {
		t.Fatalf("drift: %+v from %s", r, b)
	}
	if line := r.line("keep the API"); !strings.Contains(line, "18.2k → 6.1k tokens, 17 → 5 messages") || !strings.Contains(line, "starts from its summary") {
		t.Fatalf("the line: %s", line)
	}
	if line := (compactReply{BeforeTokens: 900, AfterTokens: 900, BeforeMessages: 4, AfterMessages: 4}).line(""); !strings.HasPrefix(line, "Nothing worth compacting") {
		t.Fatalf("nothing to do says so: %s", line)
	}
	// Stubbing was enough: a focus had no summary to lead, and the line says so.
	stubbed := compactReply{BeforeTokens: 18_900, AfterTokens: 10_600, BeforeMessages: 28, AfterMessages: 28}
	if line := stubbed.line("keep what each file does"); !strings.Contains(line, "stubbed") || !strings.Contains(line, "focus was not needed") {
		t.Fatalf("the focus is not silently dropped: %s", line)
	}
}
