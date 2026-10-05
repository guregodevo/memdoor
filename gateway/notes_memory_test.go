package gateway

import (
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

// THE CODER REMEMBERS THROUGH NOTES (Greg, 2026-09-27: "notes was pretty
// handy", "we should have it in coder tool palette"). It is in the palette,
// and it survives per-turn tool routing — a routed turn that dropped the
// memory tool would forget exactly when the decision model is on.
func TestTheCoderHasNotesEvenWhenRouted(t *testing.T) {
	if !strings.Contains(coderToolPalette, `"notes"`) {
		t.Fatal("the coder's palette must carry notes")
	}
	kept := false
	for _, name := range builtinToolRouting["coder"].AlwaysAllow {
		if name == "notes" {
			kept = true
		}
	}
	if !kept {
		t.Fatal("notes must be offered on every routed coder turn")
	}
}

// THE PRE-COMPACTION FLUSH KEEPS WHAT IT SAYS. Its reply used to be thrown
// away. Now the reply is the note — and saying nothing is a real answer that
// writes nothing.
func TestFlushReplyText(t *testing.T) {
	text := func(s string) *llm.Message {
		return &llm.Message{Content: []llm.ContentBlockUnion{{Type: "text", Text: s}}}
	}
	if got := flushReplyText(text("- build: go build ./...\n- RULE: no new deps")); !strings.Contains(got, "go build") {
		t.Errorf("a reply worth keeping is kept: %q", got)
	}
	for _, silent := range []string{"NO_REPLY", "  no_reply.  ", "`NO_REPLY`", "", "   "} {
		if got := flushReplyText(text(silent)); got != "" {
			t.Errorf("%q means nothing to keep, got %q", silent, got)
		}
	}
	// A tool call in the reply is not text; only the words are kept.
	m := &llm.Message{Content: []llm.ContentBlockUnion{{Type: "tool_use", Name: "bash"}, {Type: "text", Text: "- tests: go test ./..."}}}
	if got := flushReplyText(m); got != "- tests: go test ./..." {
		t.Errorf("only the text is a note, got %q", got)
	}
}
