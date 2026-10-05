package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// A streaming bash tool renders a live tail while running, then collapses to the
// compact result with a Took line when it completes.
func TestBashLiveTailRendersThenCollapses(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})

	// Tool starts, then streams three lines.
	step(toolCallStartMsg{toolName: "bash", toolInput: `{"command":"tail -f log"}`})
	// Each update carries the output SO FAR (cumulative), matching the gateway.
	step(toolOutputDeltaMsg{toolName: "bash", chunk: "line 1\n"})
	step(toolOutputDeltaMsg{toolName: "bash", chunk: "line 1\nline 2\n"})
	step(toolOutputDeltaMsg{toolName: "bash", chunk: "line 1\nline 2\nline 3\n"})

	running := stripANSI(m.renderMessages())
	if !strings.Contains(running, "line 3") {
		t.Errorf("live tail did not show the newest line:\n%s", running)
	}
	if !strings.Contains(running, "live · 3 lines") {
		t.Errorf("live-pane header missing:\n%s", running)
	}
	if !strings.Contains(running, "Elapsed:") {
		t.Errorf("Elapsed counter missing while running:\n%s", running)
	}
	if strings.Contains(running, "Took:") {
		t.Errorf("Took must not appear while still running:\n%s", running)
	}

	// Completion collapses the pane and shows Took.
	// (Make the elapsed non-zero so Took renders.)
	for i := range m.messages {
		if m.messages[i].Role == "tool_call" && m.messages[i].ToolName == "bash" {
			m.messages[i].Timestamp = time.Now().Add(-2 * time.Second)
		}
	}
	step(toolCallCompleteMsg{toolName: "bash", toolOutput: "line 1\nline 2\nline 3\n"})

	done := stripANSI(m.renderMessages())
	if strings.Contains(done, "live · ") {
		t.Errorf("live pane should be gone after completion:\n%s", done)
	}
	if !strings.Contains(done, "Took:") {
		t.Errorf("Took missing after completion:\n%s", done)
	}
	if strings.Contains(done, "Elapsed:") {
		t.Errorf("Elapsed should be replaced by Took after completion:\n%s", done)
	}
	if !strings.Contains(done, "line 3") {
		t.Errorf("final output missing:\n%s", done)
	}
}

// The tail is bounded to liveTailLines: a burst of many lines shows only the
// newest N, so the frame never blows up the transcript.
func TestBashLiveTailIsBounded(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})
	step(toolCallStartMsg{toolName: "bash", toolInput: `{"command":"seq 100"}`})

	// Stream 100 lines, each update carrying the cumulative buffer.
	var buf strings.Builder
	for i := 1; i <= 100; i++ {
		buf.WriteString("row " + itoaTest(i) + "\n")
		step(toolOutputDeltaMsg{toolName: "bash", chunk: buf.String()})
	}

	out := stripANSI(m.renderMessages())
	if !strings.Contains(out, "row 100") {
		t.Errorf("newest line missing:\n%s", out)
	}
	if strings.Contains(out, "row 1\n") && !strings.Contains(out, "row 100") {
		t.Error("should not show the oldest lines once past the window")
	}
	if !strings.Contains(out, "live · 100 lines") {
		t.Errorf("total-line count header wrong:\n%s", out)
	}
	// only ~liveTailLines rows should be drawn in the pane
	drawn := strings.Count(out, "row ")
	if drawn > liveTailLines+1 {
		t.Errorf("pane drew %d rows, expected at most %d", drawn, liveTailLines)
	}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// The live tail, Elapsed, and Took are gated on the generic Streamed flag (set
// when deltas arrive), NOT on the tool being "bash" — so ANY tool registered as
// a streamingExecutor renders the same way. Drive a non-bash tool name and
// assert the same UI appears.
func TestLiveTailWorksForAnyStreamingTool(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	step := func(msg tea.Msg) { nm, cmd := m.Update(msg); m = nm.(Model); runCmd(cmd) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(websocketConnectedMsg{})

	step(toolCallStartMsg{toolName: "web_search", toolInput: `{"query":"go"}`})
	step(toolOutputDeltaMsg{toolName: "web_search", chunk: "result a\n"})
	step(toolOutputDeltaMsg{toolName: "web_search", chunk: "result a\nresult b\n"})

	running := stripANSI(m.renderMessages())
	if !strings.Contains(running, "result b") || !strings.Contains(running, "live · 2 lines") {
		t.Errorf("non-bash streaming tool did not render a live tail:\n%s", running)
	}
	if !strings.Contains(running, "Elapsed:") {
		t.Errorf("Elapsed missing for a non-bash streaming tool:\n%s", running)
	}

	for i := range m.messages {
		if m.messages[i].Role == "tool_call" {
			m.messages[i].Timestamp = time.Now().Add(-3 * time.Second)
		}
	}
	step(toolCallCompleteMsg{toolName: "web_search", toolOutput: "result a\nresult b\n"})
	done := stripANSI(m.renderMessages())
	if !strings.Contains(done, "Took:") {
		t.Errorf("Took missing for a non-bash streaming tool:\n%s", done)
	}
}

// A completed command's frame must keep the TAIL, not the head: counting 1..10
// with an 8-line cap used to show 1-8 and hide 9 and 10 — the newest lines, and
// exactly the ones the user watched stream past.
func TestCompletedOutputKeepsTheTail(t *testing.T) {
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, itoaTest(i))
	}
	out := stripANSI(resultBlock(strings.Join(lines, "\n"), 8, false))

	if !strings.Contains(out, "\n     9") || !strings.Contains(out, "\n     10") {
		t.Errorf("the newest lines (9, 10) must survive the cap:\n%s", out)
	}
	if !strings.Contains(out, "earlier lines") {
		t.Errorf("expected an elision note for the hidden head:\n%s", out)
	}
	// Expanded shows everything, including the head.
	full := stripANSI(resultBlock(strings.Join(lines, "\n"), 8, true))
	if !strings.Contains(full, "\n     1\n") && !strings.HasPrefix(strings.TrimSpace(full), "⎿ 1") {
		t.Errorf("ctrl+o expand should show the first line too:\n%s", full)
	}
}
