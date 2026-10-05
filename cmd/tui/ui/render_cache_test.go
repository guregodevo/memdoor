package ui

import (
	"strings"
	"testing"
)

// renderMessages rebuilt EVERY message on every call, and streaming calls it
// once per token: a transcript of ~90 messages at 45 tokens/second meant
// thousands of renders a second — glamour markdown for every settled reply, a
// full diff for every tool frame — to change one line at the bottom.
func TestSettledMessagesAreNotRebuilt(t *testing.T) {
	m := Model{width: 80, blocks: blockCache{}}
	for i := 0; i < 20; i++ {
		m.messages = append(m.messages, Message{Role: "assistant", Content: "settled reply"})
	}
	m.messages = append(m.messages, Message{Role: "assistant", Content: "", Streaming: true})

	_ = m.renderMessages() // prime
	before := len(m.blocks)

	// One token arrives on the LAST message.
	last := len(m.messages) - 1
	m.messages[last].Content += "tok"
	out := m.renderMessages()

	if len(m.blocks) != before {
		t.Errorf("cache grew from %d to %d — settled messages should be reused", before, len(m.blocks))
	}
	if !strings.Contains(out, "tok") {
		t.Error("the changed message must still re-render")
	}
	if strings.Count(out, "settled reply") != 20 {
		t.Errorf("all 20 settled messages must still appear, got %d", strings.Count(out, "settled reply"))
	}
}

// A message that CHANGES must re-render, or the screen goes stale — the failure
// mode a cache introduces if its key is too coarse.
func TestChangedMessageIsRebuilt(t *testing.T) {
	m := Model{width: 80, blocks: blockCache{}}
	m.messages = []Message{{Role: "assistant", Content: "one"}}
	first := m.renderMessages()

	m.messages[0].Content = "two"
	second := m.renderMessages()

	if first == second {
		t.Error("changing the content must change the render")
	}
	if !strings.Contains(second, "two") {
		t.Errorf("the new content must appear: %q", second)
	}
}

// The key must cover everything renderMessage reads — a tool frame's result and
// the expand toggle included, or ctrl+o would appear to do nothing.
func TestKeyCoversToolResultAndExpansion(t *testing.T) {
	msg := Message{Role: "tool_call", ToolName: "bash", ToolInput: `{"command":"ls"}`}
	base := blockKey(msg, 80, false)

	withOutput := msg
	withOutput.ToolOutput = "a.go"
	if blockKey(withOutput, 80, false) == base {
		t.Error("a tool result must change the key, or the frame renders forever empty")
	}
	if blockKey(msg, 80, true) == base {
		t.Error("the expand toggle must change the key, or ctrl+o does nothing")
	}
	if blockKey(msg, 100, false) == base {
		t.Error("width must change the key — the render wraps to it")
	}
}
