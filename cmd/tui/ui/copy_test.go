package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

func TestStartOfLastInteractions(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "tool_call", ToolName: "bash"},
		{Role: "assistant", Content: "a2"},
	}
	if got := startOfLastInteractions(msgs, 1); got != 2 {
		t.Errorf("last 1 interaction starts at %d, want 2", got)
	}
	if got := startOfLastInteractions(msgs, 2); got != 0 {
		t.Errorf("last 2 interactions start at %d, want 0", got)
	}
	if got := startOfLastInteractions(msgs, 9); got != 0 {
		t.Errorf("more than available should clamp to 0, got %d", got)
	}
}

func TestFormatTranscriptForCopy(t *testing.T) {
	out := formatTranscriptForCopy([]Message{
		{Role: "user", Content: "run the tests"},
		{Role: "tool_call", ToolName: "bash", ToolInput: `{"command":"go test ./..."}`, ToolOutput: "ok\n"},
		{Role: "assistant", Content: "Tests pass."},
		{Role: "assistant", IsThinking: true, Content: "INTERNAL_THINKING"}, // must be skipped
	})
	for _, want := range []string{"> run the tests", "⏺ Bash(go test ./...)", "  ok", "Tests pass."} {
		if !strings.Contains(out, want) {
			t.Errorf("copy text missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "INTERNAL_THINKING") {
		t.Errorf("thinking messages must not leak into the copy:\n%s", out)
	}
}

// /copy drives the handler end to end: it puts the last interaction on the
// clipboard (where the platform supports it) and posts a confirmation note.
func TestCopySlashCommand(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})

	m.messages = []Message{
		{Role: "user", Content: "first", Timestamp: time.Now()},
		{Role: "assistant", Content: "one", Timestamp: time.Now()},
		{Role: "user", Content: "second", Timestamp: time.Now()},
		{Role: "assistant", Content: "two", Timestamp: time.Now()},
		// The /copy invocation is echoed as the last user message before the
		// handler runs — it must NOT be what gets copied (regression: it was).
		{Role: "user", Content: "/copy", Timestamp: time.Now()},
	}

	expanded, handled := m.handleSlashCommand("/copy")
	if !handled || expanded != "" {
		t.Fatalf("/copy should be handled locally, got handled=%v expanded=%q", handled, expanded)
	}
	// A confirmation note was posted.
	last := m.messages[len(m.messages)-1]
	// Either outcome is the contract: "Copied …" where a clipboard exists,
	// "Couldn't copy …" where none does (CI has no xsel/xclip). The assertion
	// used to accept only the first and was red on every Linux run.
	if last.Role != "system" || !(strings.Contains(last.Content, "Copied") || strings.Contains(last.Content, "Couldn't copy")) {
		t.Errorf("expected a Copied/Couldn't-copy note, got %+v", last)
	}
	// Where the clipboard works (macOS pbcopy here), it holds the last interaction.
	if !clipboard.Unsupported {
		got, _ := clipboard.ReadAll()
		if !strings.Contains(got, "> second") || !strings.Contains(got, "two") {
			t.Errorf("clipboard should hold the last interaction, got:\n%s", got)
		}
		if strings.Contains(got, "first") {
			t.Errorf("/copy (last interaction) must not include the earlier turn:\n%s", got)
		}
		if strings.Contains(got, "/copy") {
			t.Errorf("/copy must not copy its own invocation, got:\n%s", got)
		}
	}
}
