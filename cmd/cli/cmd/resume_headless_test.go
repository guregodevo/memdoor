package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// The headless transcript is oldest first, the person's lines marked, the
// agent's plain, empty messages skipped: what a pipe wants.
func TestHeadlessTranscript(t *testing.T) {
	msgs := []map[string]any{
		{"created_at": "2026-10-11T02:00:02Z", "author_id": "agent:coder", "author_type": "agent", "content": map[string]any{"text": "Fixed Add.\n✓ checked: 1 file changed"}},
		{"created_at": "2026-10-11T02:00:00Z", "author_id": "user-1", "content": map[string]any{"text": "@coder fix calc.go"}},
		{"created_at": "2026-10-11T02:00:01Z", "author_id": "agent:coder", "content": map[string]any{"text": "  "}},
	}
	var out bytes.Buffer
	writeTranscript(&out, transcriptLines(msgs))
	want := "> fix calc.go\n\nFixed Add.\n✓ checked: 1 file changed\n"
	if out.String() != want {
		t.Fatalf("transcript:\n%q\nwant\n%q", out.String(), want)
	}
	if strings.Count(out.String(), "\n\n") != 1 {
		t.Fatalf("one blank line between messages:\n%q", out.String())
	}
}
