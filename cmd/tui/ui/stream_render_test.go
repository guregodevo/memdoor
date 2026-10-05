package ui

import (
	"strings"
	"testing"
)

// A reply that is STILL ARRIVING must render verbatim, not through markdown.
//
// Markdown re-wraps to the viewport width, and doing that to incomplete text on
// every delta — forty-odd times a second — reflowed it differently each frame:
// words split across lines, spaces inserted mid-word, blank blocks. Measured
// 2026-08-30 mid-stream: "The us / er wa / nts a / pr / ogra / m that".
func TestStreamingTextRendersVerbatim(t *testing.T) {
	m := Model{width: 80}
	partial := "def f():\n    return 1"

	live := m.renderMessage(Message{Role: "assistant", Content: partial, Streaming: true})
	if !strings.Contains(live, "    return 1") {
		t.Errorf("indentation must survive while streaming, got:\n%s", live)
	}
	for _, word := range []string{"def f():", "return 1"} {
		if !strings.Contains(live, word) {
			t.Errorf("streaming render broke up %q:\n%s", word, live)
		}
	}
}

// The model writes MARKDOWN, so a streaming reply is rendered as markdown too —
// otherwise the reader watches raw "**bold**" and backticks arrive.
//
// This was briefly rendered verbatim on the theory that the renderer was too
// expensive to run per token. The real cost was rebuilding the WHOLE transcript
// on each repaint, which the block cache removes; skipping markdown only made
// the output worse.
func TestStreamingTextIsRenderedAsMarkdown(t *testing.T) {
	m := Model{width: 80, blocks: blockCache{}}
	out := m.renderMessage(Message{Role: "assistant", Content: "**bold** text", Streaming: true})
	if strings.Contains(out, "**bold**") {
		t.Errorf("raw markdown must not reach the reader while streaming: %q", out)
	}
	if !strings.Contains(out, "bold") {
		t.Errorf("the words must survive: %q", out)
	}
}

// The final response settles every message, so nothing stays verbatim forever.
func TestFinalResponseSettlesStreaming(t *testing.T) {
	m := Model{}
	mm, _ := m.Update(assistantStreamingMsg{content: "partial"})
	m = mm.(Model)
	if !m.messages[len(m.messages)-1].Streaming {
		t.Fatal("precondition: a streamed message is marked streaming")
	}
	mm, _ = m.Update(assistantResponseMsg{content: "partial done"})
	m = mm.(Model)
	for _, msg := range m.messages {
		if msg.Streaming {
			t.Error("the final response must settle every message, or it never renders as markdown")
		}
	}
}
