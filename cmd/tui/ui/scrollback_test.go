package ui

import (
	"github.com/charmbracelet/bubbles/viewport"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func scrollbackModel(t *testing.T) Model {
	t.Helper()
	m := NewModel("ws://localhost:0/ws", "demo", "chan", "", func(string, string, string, string) error { return nil })
	m.ready = true
	m.width, m.height = 100, 40
	m.viewport.Width, m.viewport.Height = 100, 30
	// NewModel seeds a welcome message; these tests count positions.
	m.messages = nil
	return m
}

func say(text string) Message {
	return Message{Role: "assistant", Content: text, Timestamp: time.Now()}
}

func ranTool(name, out string) Message {
	return Message{Role: "tool_call", ToolName: name, ToolInput: "{}", ToolOutput: out, Timestamp: time.Now()}
}

// The invariant that makes an inline program safe: the drawn view must be
// STRICTLY shorter than the terminal. A view exactly as tall scrolls the
// terminal on every repaint, and each scroll strands the previous version of
// the tail in the history — the same message twice, at two lengths. Asserted
// against m.height, not against liveBudget: a test measured against the
// constant it is testing passes just as happily when that constant is zero.
func TestTheDrawnViewIsShorterThanTheTerminal(t *testing.T) {
	m := scrollbackModel(t)
	flushed := false
	for i := 0; i < 60; i++ {
		m.messages = append(m.messages, say(marker(i)+"\n"+strings.Repeat("body line\n", 3)))
		var chunk string
		chunk, m.printedThrough = m.nextFlush()
		flushed = flushed || chunk != ""
		m.refreshFollow()

		if h := lipgloss.Height(m.View()); h >= m.height {
			t.Fatalf("after %d messages the view is %d rows on a %d-row terminal — it will scroll and tear",
				i+1, h, m.height)
		}
	}
	if !flushed {
		t.Fatal("nothing was ever flushed — the test proves nothing")
	}
}

// The hardest case for the bound: one message taller than the screen. It
// cannot be flushed — it is the live tail — so the viewport clamps to the
// space available, and that is the shape that fills the terminal exactly and
// scrolls on every repaint. (This asserts the invariant, not the headroom
// constant: View's own arithmetic already reserves a couple of rows, so
// zeroing headroom does not fail this. headroom is the explicit margin for the
// variable-height bars liveBudget deliberately does not subtract.)
func TestATallMessageStillLeavesHeadroom(t *testing.T) {
	m := scrollbackModel(t)
	m.messages = append(m.messages, say(strings.Repeat("a very long line of output\n", 200)))
	m.refreshFollow()
	if h := lipgloss.Height(m.View()); h >= m.height {
		t.Fatalf("a single oversized message draws %d rows on a %d-row terminal — "+
			"a view that exactly fills the terminal scrolls it on every repaint",
			h, m.height)
	}
}

// And the flush is what keeps it that way: without it the live region would
// simply grow until the viewport clamped it, which is the full-height view
// above.
func TestTheFlushKeepsTheLiveRegionBounded(t *testing.T) {
	m := scrollbackModel(t)
	for i := 0; i < 60; i++ {
		m.messages = append(m.messages, say(marker(i)+"\n"+strings.Repeat("body line\n", 3)))
		_, m.printedThrough = m.nextFlush()
	}
	if live := len(m.messages) - m.printedThrough; live > 12 {
		t.Errorf("%d messages still live after 60 — the live region is tracking the conversation, not the screen", live)
	}
	if h := lipgloss.Height(m.renderMessages()); h > m.height {
		t.Errorf("live transcript is %d rows on a %d-row terminal", h, m.height)
	}
}

// The newest message is still growing; freezing it is what puts two versions of
// it in the scrollback.
func TestTheNewestMessageIsNeverFlushed(t *testing.T) {
	m := scrollbackModel(t)
	m.height = 6 // a budget so small the flush wants to take everything
	for i := 0; i < 30; i++ {
		m.messages = append(m.messages, say(marker(i)))
		_, through := m.nextFlush()
		// The literal 1, not minLive: the newest message must stay live, and a
		// test phrased in terms of minLive still passes when minLive is zero.
		if through > len(m.messages)-1 {
			t.Fatalf("%d messages: flushed through %d — that freezes the live tail",
				len(m.messages), through)
		}
		m.printedThrough = through
	}
}

// A terminal we have not been told the size of yet: measuring against a zero
// budget would flush the entire transcript on the first update, before a single
// frame has been drawn.
func TestNothingFlushesBeforeTheFirstResize(t *testing.T) {
	m := scrollbackModel(t)
	m.height, m.width = 0, 0
	for i := 0; i < 20; i++ {
		m.messages = append(m.messages, say(marker(i)))
	}
	if _, through := m.nextFlush(); through != 0 {
		t.Fatalf("flushed through %d with no terminal size, want 0", through)
	}
}

// Scrollback order has to match transcript order, so a message that can still
// change blocks everything behind it — even messages that are themselves final.
func TestARunningToolBlocksTheFlush(t *testing.T) {
	m := scrollbackModel(t)
	m.messages = append(m.messages, say("first"))
	m.messages = append(m.messages, Message{Role: "tool_call", ToolName: "bash", ToolInput: "{}"}) // no output: running
	for i := 0; i < 30; i++ {
		m.messages = append(m.messages, say("after"))
	}
	if _, through := m.nextFlush(); through != 1 {
		t.Fatalf("flushed through %d, want 1 — the running tool at index 1 must stop the prefix", through)
	}

	m.messages[1].ToolOutput = "done"
	if _, through := m.nextFlush(); through <= 1 {
		t.Fatalf("after the tool finished, flushed through %d — the prefix should move past it", through)
	}
}

func TestAStreamingMessageIsNeverFlushed(t *testing.T) {
	m := scrollbackModel(t)
	m.messages = append(m.messages, Message{Role: "assistant", Content: "half a th", Streaming: true})
	for i := 0; i < 30; i++ {
		m.messages = append(m.messages, say("filler"))
	}
	if _, through := m.nextFlush(); through != 0 {
		t.Fatalf("flushed through %d, want 0 — index 0 is still streaming", through)
	}
}

// The invariant that makes the split safe: every message appears exactly once,
// either in what was printed or in what is still drawn. A message lost at the
// seam is invisible; one on both sides is printed twice.
func TestEveryMessageIsPrintedExactlyOnce(t *testing.T) {
	m := scrollbackModel(t)
	var printed strings.Builder

	for i := 0; i < 40; i++ {
		switch i % 3 {
		case 0:
			m.messages = append(m.messages, Message{Role: "user", Content: marker(i), Timestamp: time.Now()})
		case 1:
			m.messages = append(m.messages, ranTool("bash", marker(i)))
		default:
			m.messages = append(m.messages, say(marker(i)))
		}
		var chunk string
		chunk, m.printedThrough = m.nextFlush()
		printed.WriteString(chunk)
	}

	all := printed.String() + m.renderMessages()
	for i := 0; i < 40; i++ {
		if n := strings.Count(all, marker(i)); n != 1 {
			t.Errorf("message %d appears %d times across scrollback+viewport, want 1", i, n)
		}
	}
	if m.printedThrough == 0 {
		t.Fatal("nothing was ever flushed — the test proves nothing")
	}
}

// marker is a token that survives markdown rendering and word wrapping.
func marker(i int) string { return "MSGMARKER" + string(rune('a'+i/26)) + string(rune('a'+i%26)) }

// Once flushed, a message must be gone from the drawn view: leaving it there is
// the duplicate the terminal shows twice.
func TestFlushedMessagesLeaveTheViewport(t *testing.T) {
	m := scrollbackModel(t)
	for i := 0; i < 30; i++ {
		m.messages = append(m.messages, say(marker(i)))
	}
	chunk, through := m.nextFlush()
	m.printedThrough = through

	live := m.renderMessages()
	for i := 0; i < through; i++ {
		if strings.Contains(live, marker(i)) {
			t.Errorf("message %d was printed to scrollback but is still drawn", i)
		}
		if !strings.Contains(chunk, marker(i)) {
			t.Errorf("message %d was skipped over: not printed, not drawn", i)
		}
	}
	if !strings.Contains(live, marker(through)) {
		t.Errorf("message %d is neither printed nor drawn", through)
	}
}

// flushSettled advances the watermark and drops the frozen cache entries — they
// can never be rendered again, so holding them grows the cache forever.
func TestFlushEvictsFrozenCacheEntries(t *testing.T) {
	m := scrollbackModel(t)
	for i := 0; i < 30; i++ {
		m.messages = append(m.messages, say(marker(i)))
	}
	_ = m.renderMessages() // populate the cache
	if len(m.blocks) == 0 {
		t.Fatal("cache never populated — the test proves nothing")
	}
	m, cmd := m.flushSettled()
	if cmd == nil {
		t.Fatal("no print command")
	}
	for i := 0; i < m.printedThrough; i++ {
		if _, ok := m.blocks[i]; ok {
			t.Errorf("cache still holds frozen message %d", i)
		}
	}
}

// /clear reuses index 0, so a stale watermark would treat the new session's
// first message as already printed and never draw it.
func TestClearResetsTheWatermark(t *testing.T) {
	m := scrollbackModel(t)
	m.fresh = func(string) error { return nil }
	for i := 0; i < 30; i++ {
		m.messages = append(m.messages, say(marker(i)))
	}
	m, _ = m.flushSettled()
	if m.printedThrough == 0 {
		t.Fatal("nothing flushed — the test proves nothing")
	}

	cmd := m.runNetworkCmd([]string{"/clear"})
	if cmd == nil {
		t.Fatal("/clear did not ask the gateway")
	}
	nm, _ := m.Update(cmd())
	m = nm.(Model)
	if m.printedThrough != 0 {
		t.Errorf("watermark is %d after /clear, want 0", m.printedThrough)
	}
	if got := m.renderMessages(); !strings.Contains(got, "Cleared") {
		t.Errorf("the post-clear confirmation is not drawn:\n%s", got)
	}
}

// While a tool call is being written the transcript stops moving, because the
// half-written JSON is held back rather than shown as text. That is correct,
// and on its own it is indistinguishable from a hang: measured live
// 2026-08-30, the text stopped at "I'll make the calls." and did not move for
// six minutes. The activity bar has to name what is actually happening, and
// "Thinking" is the one thing it is NOT doing.
func TestTheBarNamesTheCallBeingWritten(t *testing.T) {
	m := scrollbackModel(t)
	m.isThinking = true
	m.thinkingStartTime = time.Now()

	if got := stripANSI(m.renderThinkingBar()); strings.Contains(got, "call") {
		t.Fatalf("the bar mentions a call before one is being written: %q", got)
	}

	m.jsonFilter.feed(`{"name":"bash","arg`)
	m.pendingCall = m.jsonFilter.PendingName()
	got := stripANSI(m.renderThinkingBar())
	// The bar uses the DISPLAY name ("Bash"), which is the point of
	// toolview.go's mapping — match case-insensitively.
	if !strings.Contains(strings.ToLower(got), "bash") {
		t.Errorf("the bar does not name the call being written: %q", got)
	}
	if strings.Contains(got, "Thinking") {
		t.Errorf("the bar still says Thinking while a call is being written: %q", got)
	}
}

// The name is not always available yet; the bar still has to say something
// rather than fall back to a label that is now wrong.
func TestTheBarSaysSomethingWithoutAName(t *testing.T) {
	m := scrollbackModel(t)
	m.isThinking = true
	m.thinkingStartTime = time.Now()
	m.pendingCall = pendingCallUnnamed

	got := stripANSI(m.renderThinkingBar())
	if !strings.Contains(strings.ToLower(got), "call") {
		t.Errorf("the bar does not say a call is being written: %q", got)
	}
}

// End to end through Update: streaming a half-written call must light the
// indicator, and completing the turn must clear it. A pendingCall left set
// would pin the bar to "Writing a … call" for the rest of the session.
func TestStreamingAHalfWrittenCallLightsAndClearsTheIndicator(t *testing.T) {
	var m tea.Model = NewModel("ws://localhost:0/ws", "w", "c", "", func(string, string, string, string) error { return nil })
	m, _ = m.Update(assistantStreamingMsg{content: "I'll make the calls.\n"})
	if got := m.(Model).pendingCall; got != "" {
		t.Fatalf("indicator lit on plain text: %q", got)
	}

	m, _ = m.Update(assistantStreamingMsg{content: `{"name":"bash","arg`})
	if got := m.(Model).pendingCall; got != "bash" {
		t.Errorf("pendingCall = %q while a bash call is half-written, want bash", got)
	}
	// The byte count must come from the UPDATE path, not be set by hand — a
	// mutation that stopped publishing it survived a test that did.
	if got := m.(Model).pendingCallBytes; got == 0 {
		t.Error("pendingCallBytes = 0 while a call is being held — the bar cannot show growth")
	}

	m, _ = m.Update(assistantStreamingMsg{content: `uments":{"command":"ls"}}`})
	if got := m.(Model).pendingCall; got != "" {
		t.Errorf("pendingCall = %q after the call completed, want empty", got)
	}
}

// The truncation case, which is how this was found: the model is cut off at
// the output cap MID-CALL, so the object never closes and the turn simply
// ends. Nothing in the streaming path can clear the indicator — there are no
// more deltas — so the bar would read "Writing a bash call…" for the rest of
// the session, describing something that stopped happening.
func TestATurnEndingMidCallClearsTheIndicator(t *testing.T) {
	var m tea.Model = NewModel("ws://localhost:0/ws", "w", "c", "", func(string, string, string, string) error { return nil })
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40}) // without a size the renderer produces nothing
	m, _ = m.Update(assistantStreamingMsg{content: `I'll make the calls.` + "\n" + `{"name":"bash","arg`})
	if got := m.(Model).pendingCall; got != "bash" {
		t.Fatalf("pendingCall = %q mid-call, want bash", got)
	}

	// The cap hits here: no closing brace ever arrives, the turn just ends.
	m, _ = m.Update(assistantResponseMsg{content: ""})
	if got := m.(Model).pendingCall; got != "" {
		t.Errorf("pendingCall = %q after the turn ended — the bar is pinned to a call that is no longer being written", got)
	}
	// And the held text must not be lost: an object that never closed was not a
	// call, and swallowing it would drop real output.
	if view := m.(Model).renderMessages(); !strings.Contains(view, "I'll make the calls") {
		t.Errorf("the streamed text was lost when the turn ended mid-call:\n%s", view)
	}
}

// The other half of that rule: when the final reply DOES carry text, it is
// authoritative and must replace what streamed — the streamed form is plain,
// the final one is the markdown-rendered answer.
func TestANonEmptyFinalReplyStillReplacesTheStreamedText(t *testing.T) {
	var m tea.Model = NewModel("ws://localhost:0/ws", "w", "c", "", func(string, string, string, string) error { return nil })
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m, _ = m.Update(assistantStreamingMsg{content: "partial ans"})
	m, _ = m.Update(assistantResponseMsg{content: "the complete answer"})

	msgs := m.(Model).messages
	last := msgs[len(msgs)-1]
	if last.Content != "the complete answer" {
		t.Errorf("final content is %q, want the complete answer", last.Content)
	}
}

// Mouse capture went with the altscreen, so a bar that says "click to jump"
// advertises a control that does nothing — worse than none, same rule as the
// removed chips.
func TestTheJumpBarDoesNotAdvertiseTheMouse(t *testing.T) {
	m := scrollbackModel(t)
	m.newBelow = 3
	view := stripANSI(m.View())
	if strings.Contains(strings.ToLower(view), "click") {
		t.Errorf("the jump bar tells the reader to click with no mouse capture:\n%s", view)
	}
	if !strings.Contains(view, "End") {
		t.Errorf("the jump bar lost its working affordance:\n%s", view)
	}
}

// The "Writing a … call" indicator is consulted INSIDE the activity bar, but
// the bar's own visibility test did not include it — so once thinking text
// had streamed (which clears isThinking), a held half-written call rendered
// no bar at all. The exact silence the indicator was built for, back again
// one layer up. Measured live 2026-08-31 12:24: mid-generation, text frozen,
// no bar.
func TestAHeldCallShowsTheBarEvenAfterThinkingCleared(t *testing.T) {
	m := scrollbackModel(t)
	m.isThinking = false // thinking text already streamed and cleared it
	m.pendingCall = "bash"

	view := stripANSI(m.View())
	if !strings.Contains(view, "Writing a Bash call") {
		t.Errorf("a held call renders no indicator once thinking cleared:\n%s", view)
	}
}

// pendingCall was cleared only by assistantResponseMsg — but a turn whose
// last message ended with a half-written object can finish through
// runCompleteMsg alone, and the indicator then counts forever. Measured live
// 2026-08-31 12:39: "Writing a glob call… (9m 0s)" pinned since a turn that
// ended nine minutes earlier, over a session doing something else entirely.
func TestARunCompleteClearsTheHeldCallIndicator(t *testing.T) {
	var m tea.Model = NewModel("ws://localhost:0/ws", "w", "c", "", func(string, string, string, string) error { return nil })
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m, _ = m.Update(assistantStreamingMsg{content: `{"name":"glob","arg`})
	if got := m.(Model).pendingCall; got != "glob" {
		t.Fatalf("pendingCall = %q mid-object, want glob", got)
	}

	m, _ = m.Update(runCompleteMsg{})
	if got := m.(Model).pendingCall; got != "" {
		t.Errorf("pendingCall = %q after the run completed — the bar counts a call from a finished turn", got)
	}
}

// "it keeps saying writing a bash tool call" (2026-08-31): the indicator was
// STATIC, so a call being streamed and a call stuck dead read identically.
// The held byte count grows while the model works — show it, and the bar
// becomes its own progress signal.
func TestTheWritingIndicatorShowsGrowth(t *testing.T) {
	m := scrollbackModel(t)
	m.isThinking = true
	m.thinkingStartTime = time.Now()
	m.jsonFilter.feed(`{"name":"bash","arguments":{"command":"` + strings.Repeat("echo 1;", 300))
	m.pendingCall = m.jsonFilter.PendingName()
	m.pendingCallBytes = m.jsonFilter.Held()

	got := stripANSI(m.renderThinkingBar())
	if !strings.Contains(got, "KB") && !strings.Contains(got, "B") {
		t.Errorf("the bar shows no size — a growing call and a stuck one read the same: %q", got)
	}
}

// Prose that streamed BEFORE a tool frame must not render again after it
// when the final reply repeats it (live 2026-09-03: the candidate
// list showed twice, above and below the transcribe frame). Only text the
// reader has not seen yet is added.
func TestFinalReplyDoesNotRepeatProseStreamedAroundAFrame(t *testing.T) {
	var m tea.Model = NewModel("ws://localhost:0/ws", "w", "c", "", func(string, string, string, string) error { return nil })
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	prose := "I've transcribed the file. Candidate moments: the freedom beat."
	m, _ = m.Update(assistantStreamingMsg{content: prose})
	m, _ = m.Update(toolCallStartMsg{toolID: "t1", toolName: "transcribe", toolInput: `{"path":"free30.mp4"}`})
	m, _ = m.Update(assistantResponseMsg{content: prose + "\n\nCut it into free30_short.mp4."})

	view := m.(Model).renderMessages()
	if n := strings.Count(view, "Candidate moments"); n != 1 {
		t.Errorf("streamed prose rendered %d times, want once:\n%s", n, view)
	}
	if !strings.Contains(view, "Cut it into free30_short.mp4.") {
		t.Errorf("the part that did not stream must still appear:\n%s", view)
	}
}

// THE WINDOW HAS ONE RIGHT EDGE (battle test, 2026-09-27). The header rule was
// drawn at the terminal's width and the input box's rules at its content's, so
// they disagreed by three columns at every size.
func TestTheRulesSpanTheSameWidth(t *testing.T) {
	for _, width := range []int{80, 100, 120, 200} {
		m := NewModel("", "", "", "", nil)
		m.width, m.height, m.connected = width, 24, true
		m.viewport = viewport.New(width, 10)
		m.input.SetWidth(width - 4)
		widths := map[int]bool{}
		for _, part := range []string{m.renderHeader(), m.renderFooter()} {
			for _, line := range strings.Split(part, "\n") {
				clean := stripANSI(line)
				if strings.Count(clean, "─") > 3 {
					widths[lipgloss.Width(clean)] = true
				}
			}
		}
		if len(widths) != 1 {
			t.Errorf("at %d columns the rules disagree: %v", width, widths)
			continue
		}
		for w := range widths {
			if w != width {
				t.Errorf("at %d columns a rule is %d wide", width, w)
			}
		}
	}
}
