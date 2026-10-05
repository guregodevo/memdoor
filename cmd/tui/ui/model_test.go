package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// update runs one Update and returns the concrete Model back (Update returns the
// tea.Model interface).
func update(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestFormatElapsed(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{12 * time.Second, "12s"},
		{59 * time.Second, "59s"},
		{90 * time.Second, "1m 30s"},
		{2*time.Minute + 5*time.Second, "2m 5s"},
		{-3 * time.Second, "0s"}, // clock skew never prints garbage
	}
	for _, c := range cases {
		if got := formatElapsed(c.d); got != c.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestFormatTokensShort(t *testing.T) {
	for in, want := range map[int]string{0: "0", 950: "950", 1000: "1.0k", 12800: "12.8k"} {
		if got := formatTokensShort(in); got != want {
			t.Errorf("formatTokensShort(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestThinkingBarNoOverflow locks the elapsed-timer fix: a zero start time used
// to make time.Since overflow int64 and render "9223372036s".
func TestThinkingBarNoOverflow(t *testing.T) {
	m := NewModel("", "", "", "", nil)

	// Zero start time (the bug trigger) must render 0s, never the overflow.
	m.isThinking = true
	if got := m.renderThinkingBar(); strings.Contains(got, "9223372036") || !strings.Contains(got, "0s") {
		t.Errorf("zero-start thinking bar = %q; want a 0s elapsed and no overflow", got)
	}

	// A real backdated start renders the real elapsed.
	m.thinkingStartTime = time.Now().Add(-3 * time.Second)
	if got := m.renderThinkingBar(); !strings.Contains(got, "3s") || strings.Contains(got, "9223372036") {
		t.Errorf("3s-old thinking bar = %q; want a 3s elapsed and no overflow", got)
	}
}

// TestEscInterrupts: Esc while a turn is running stops it and notes it, instead
// of the old behavior (quitting the app).
func TestEscInterrupts(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m.isThinking = true

	m = update(m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.isThinking {
		t.Error("Esc should clear isThinking")
	}
	if !m.interrupted {
		t.Error("Esc should set interrupted")
	}
	if len(m.messages) == 0 {
		t.Fatal("Esc should append an interrupt note")
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "system" || !strings.Contains(last.Content, "Interrupted") {
		t.Errorf("last message = %+v; want a system 'Interrupted' note", last)
	}
}

// TestInterruptedDropsEvents: once interrupted, the in-flight run's events are
// ignored until the next turn.
func TestInterruptedDropsEvents(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m.interrupted = true

	before := len(m.messages)
	m = update(m, assistantStreamingMsg{content: "late token"})

	if len(m.messages) != before {
		t.Errorf("interrupted model appended a streamed message (%d -> %d); want it dropped",
			before, len(m.messages))
	}
}

// TestRenderMessageGlyphs: assistant/user are plain; the tool_call keeps the ⏺
// status marker (grey running / white done / red error).
func TestRenderMessageGlyphs(t *testing.T) {
	m := NewModel("", "", "", "", nil)

	// Assistant text is plain now — no leading bullet.
	if got := m.renderMessage(Message{Role: "assistant", Content: "hi"}); strings.Contains(got, "⏺") {
		t.Errorf("assistant render = %q; want NO bullet (plain)", got)
	}
	if got := m.renderMessage(Message{Role: "user", Content: "hi"}); !strings.Contains(got, ">") {
		t.Errorf("user render = %q; want a > prompt", got)
	}
	// Tool calls keep the ⏺ status marker.
	// A COMPLETED tool call shows the ⏺ status marker.
	if got := m.renderMessage(Message{Role: "tool_call", ToolName: "web_search", ToolInput: "{}", ToolOutput: "done"}); !strings.Contains(got, "⏺") {
		t.Errorf("completed tool render = %q; want an ⏺ status marker", got)
	}
}

// TestEnterSubmitsTrimmed: Enter sends the message, trimmed — no trailing
// newline from the textarea, and the input resets.
func TestEnterSubmitsTrimmed(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m = update(m, websocketConnectedMsg{}) // offline submits queue instead of dispatching
	m.input.SetValue("hello\n")            // what the textarea would leave on Enter

	before := len(m.messages)
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.messages) != before+1 {
		t.Fatalf("Enter should append the user message (%d -> %d)", before, len(m.messages))
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "user" || last.Content != "hello" {
		t.Errorf("submitted = %+v; want a user 'hello' (trimmed, no newline)", last)
	}
	if strings.TrimSpace(m.input.Value()) != "" {
		t.Errorf("input not reset after Enter: %q", m.input.Value())
	}
}

// TestEnterEmptyDoesNothing: Enter on whitespace-only input is a no-op (never
// submits a lone newline).
func TestEnterEmptyDoesNothing(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m.input.SetValue("   \n")

	before := len(m.messages)
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.messages) != before {
		t.Errorf("Enter on blank input appended a message; want a no-op")
	}
}

// TestContextHeaderShowsTokens: the persistent token usage indicator.
func TestContextHeaderShowsTokens(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m.width = 120
	m.contextTokens = 12800
	m.contextLimit = 28672

	got := m.renderHeader()
	if !strings.Contains(got, "ctx") || !strings.Contains(got, "12.8k") || !strings.Contains(got, "45%") {
		t.Errorf("header = %q; want '12.8k/28.7k · 45%% ctx'", got)
	}
}
