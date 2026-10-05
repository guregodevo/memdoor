package ui

import (
	"strings"
	"testing"
)

// The model's <think> block streams as ordinary deltas, and the reply is only
// classified as reasoning AFTER the turn ends. So a streamed turn rendered the
// entire thought process as the answer — measured 2026-08-30, pages of "Let me
// think about what this means" sitting above the actual reply.
func TestReasoningAndAnswerSeparateWhileStreaming(t *testing.T) {
	var s thinkStreamSplitter
	var think, answer string
	for _, chunk := range []string{"<think>weighing ", "options</think>Here is ", "the answer."} {
		tk, an := s.next(chunk)
		think += tk
		answer += an
	}
	if think != "weighing options" {
		t.Errorf("reasoning = %q, want %q", think, "weighing options")
	}
	if answer != "Here is the answer." {
		t.Errorf("answer = %q, want %q", answer, "Here is the answer.")
	}
}

// A marker can be SPLIT ACROSS DELTAS — "</thi" then "nk>". Emitting the
// fragment as text would print raw markup mid-sentence and leave the splitter
// permanently in the wrong mode.
func TestMarkerSplitAcrossChunks(t *testing.T) {
	var s thinkStreamSplitter
	var think, answer string
	for _, chunk := range []string{"<thi", "nk>reasoning", " here</thi", "nk>answer"} {
		tk, an := s.next(chunk)
		think += tk
		answer += an
	}
	if think != "reasoning here" {
		t.Errorf("reasoning = %q", think)
	}
	if answer != "answer" {
		t.Errorf("answer = %q — a split marker must never reach the screen", answer)
	}
}

// The orphan closer this model actually emits: the chat template opens the
// block, so the reply STARTS mid-thought and the first marker seen is the
// closer.
func TestOrphanCloserStartsInThinking(t *testing.T) {
	s := thinkStreamSplitter{inThink: true}
	tk, an := s.next("planning the work</think>Done.")
	if tk != "planning the work" {
		t.Errorf("reasoning = %q", tk)
	}
	if an != "Done." {
		t.Errorf("answer = %q", an)
	}
}

// A reply with no markers at all is entirely the answer — the common case must
// not be held back or reclassified.
func TestPlainTextIsAllAnswer(t *testing.T) {
	var s thinkStreamSplitter
	tk, an := s.next("just the answer")
	if tk != "" {
		t.Errorf("no reasoning expected, got %q", tk)
	}
	if an != "just the answer" {
		t.Errorf("answer = %q", an)
	}
}

// Text ending in "<" must not be swallowed forever: it is held only until the
// next chunk decides what it was.
func TestAmbiguousTailIsHeldThenReleased(t *testing.T) {
	var s thinkStreamSplitter
	_, an1 := s.next("a < b")
	_, an2 := s.next(" and c")
	if got := an1 + an2; got != "a < b and c" {
		t.Errorf("held text must be released, got %q", got)
	}
}

// Protocol litter must never reach the screen while streaming.
//
// scrubTagLitter removes these from a COMPLETE reply, but streamed deltas never
// pass through it. Measured 2026-08-30: a model repeating its own closing tag
// rendered a wall of "</tool_call>" live, while the finished reply for the same
// turn was clean — which is why this only appeared once streaming worked.
func TestProtocolLitterIsStrippedFromTheStream(t *testing.T) {
	var s thinkStreamSplitter
	var answer string
	for _, chunk := range []string{"Here is the plan.", "</tool_call>", "\n</tool_call>", " Done."} {
		_, an := s.next(chunk)
		answer += an
	}
	if strings.Contains(answer, "tool_call") {
		t.Errorf("protocol litter reached the transcript: %q", answer)
	}
	if !strings.Contains(answer, "Here is the plan.") || !strings.Contains(answer, "Done.") {
		t.Errorf("real text must survive the scrub: %q", answer)
	}
}

// A litter tag SPLIT across deltas must not leak either — the half-tag is the
// thing a naive filter prints.
func TestSplitLitterTagDoesNotLeak(t *testing.T) {
	var s thinkStreamSplitter
	var answer string
	for _, chunk := range []string{"text", "</tool", "_call>", "more"} {
		_, an := s.next(chunk)
		answer += an
	}
	if answer != "textmore" {
		t.Errorf("a split litter tag must vanish entirely, got %q", answer)
	}
}

// Litter inside a think block is dropped too, without ending the block.
func TestLitterInsideThinkingDoesNotEndIt(t *testing.T) {
	var s thinkStreamSplitter
	var think, answer string
	for _, chunk := range []string{"<think>plan", "</tool_call>", " more</think>ans"} {
		tk, an := s.next(chunk)
		think += tk
		answer += an
	}
	if think != "plan more" {
		t.Errorf("reasoning = %q — litter must not end the think block", think)
	}
	if answer != "ans" {
		t.Errorf("answer = %q", answer)
	}
}

// An unclosed <think> must not swallow the whole reply.
//
// The thinking block renders clipped to a few lines, so routing everything into
// it made the screen go blank apart from a dim stub — measured 2026-08-30, on a
// runaway that opened <think> and never closed it. Past a sane length the closer
// is simply lost, and showing the text beats hiding it.
func TestUnclosedThinkDoesNotSwallowTheReply(t *testing.T) {
	var s thinkStreamSplitter
	var think, answer string

	tk, an := s.next("<think>")
	think += tk
	answer += an
	for i := 0; i < 40; i++ { // 40 x 500 chars, well past the bound
		tk, an = s.next(strings.Repeat("x", 500))
		think += tk
		answer += an
	}

	if answer == "" {
		t.Error("an unclosed think block swallowed the entire reply — the screen goes blank")
	}
	if len(think) > maxThinkRun+1000 {
		t.Errorf("reasoning kept growing unbounded: %d chars", len(think))
	}
}

// A normal think block, well under the bound, still routes entirely to
// reasoning — the bound must not clip ordinary turns.
func TestOrdinaryThinkBlockIsUnaffected(t *testing.T) {
	var s thinkStreamSplitter
	reasoning := strings.Repeat("thinking about it. ", 50) // ~950 chars
	think, answer := s.next("<think>" + reasoning + "</think>the answer")
	if think != reasoning {
		t.Errorf("ordinary reasoning must route intact, got %d of %d chars", len(think), len(reasoning))
	}
	if answer != "the answer" {
		t.Errorf("answer = %q", answer)
	}
}

// The live-stream litter list must at least cover what the stored-reply
// scrubber covers for container tags: "</functions>\n</tools>" reached the
// screen twice on 2026-08-31 because only the singular forms were listed.
func TestPluralContainerTagsAreStreamLitter(t *testing.T) {
	var s thinkStreamSplitter
	_, answer := s.next("before </functions>\n</tools> after")
	for _, tag := range []string{"</functions>", "</tools>"} {
		if strings.Contains(answer, tag) {
			t.Errorf("%s reached the streamed text: %q", tag, answer)
		}
	}
	if !strings.Contains(answer, "before") || !strings.Contains(answer, "after") {
		t.Errorf("real text lost: %q", answer)
	}
}

// The Qwen3-Coder XML tool call must not reach the screen: neither its
// openers (which carry a name, so the bare litter tags never matched them)
// nor its body. The frame renders the call; the stream shows only the prose.
func TestToolCallMarkupIsHiddenFromTheStream(t *testing.T) {
	var s thinkStreamSplitter
	var answer string
	chunks := []string{"Transcribing now.\n", "<tool_call>\n<function=transcribe>\n<parameter=path>\n", "nasa.mp4\n</parameter>\n", "</function>\n</tool_call>\n", "Done."}
	for _, c := range chunks {
		_, an := s.next(c)
		answer += an
	}
	if strings.Contains(answer, "function") || strings.Contains(answer, "nasa.mp4") || strings.Contains(answer, "parameter") {
		t.Errorf("tool-call markup or body reached the transcript: %q", answer)
	}
	if !strings.Contains(answer, "Transcribing now.") || !strings.Contains(answer, "Done.") {
		t.Errorf("prose around the call must survive: %q", answer)
	}
}

// Without the <tool_call> wrapper, and with the opener split across deltas.
func TestFunctionMarkupWithoutWrapperIsHidden(t *testing.T) {
	var s thinkStreamSplitter
	var answer string
	for _, c := range []string{"ok <func", "tion=clip_cut>\n<parameter=start>78</parameter>\n<parameter=end>108</parameter>\n</function> then"} {
		_, an := s.next(c)
		answer += an
	}
	if answer != "ok  then" {
		t.Errorf("got %q", answer)
	}
}

// A call block whose closer never comes must not swallow the reply.
func TestUnclosedCallDoesNotSwallowTheReply(t *testing.T) {
	var s thinkStreamSplitter
	var answer string
	body := strings.Repeat("x", maxCallRun+10)
	for _, c := range []string{"<function=write_file>", body, " tail"} {
		_, an := s.next(c)
		answer += an
	}
	if !strings.HasSuffix(answer, " tail") {
		t.Errorf("text after a runaway call block must show, got %d bytes ending %q", len(answer), answer[max(0, len(answer)-10):])
	}
}

// A final "text" event flagged appended carries only what did not stream:
// it lands UNDER the streamed answer. Unflagged, the old contract holds and
// the final text replaces the streamed draft. Live 2026-10-04: a zero-tool
// turn's whole streamed reply vanished behind "(note: this turn used no
// tools …)".
func TestAnAppendedTailDoesNotEraseTheStreamedAnswer(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", nil)
	m.appendStream("assistant", "My answer, in one go: Kolibri is built by ablations.")
	mm, _ := m.Update(assistantResponseMsg{content: "(note: this turn used no tools — nothing was read, written, or verified)", appended: true})
	m = mm.(Model)
	got := m.messages[len(m.messages)-1].Content
	if !strings.Contains(got, "Kolibri is built") || !strings.Contains(got, "no tools") {
		t.Fatalf("the tail must join the answer, not replace it: %q", got)
	}
	// Nothing streamed first: the appended tail is the whole message.
	m2 := NewModel("http://x", "ws/demo", "chan", "", nil)
	mm2, _ := m2.Update(assistantResponseMsg{content: "✓ checked: 1 file changed", appended: true})
	m2 = mm2.(Model)
	if got := m2.messages[len(m2.messages)-1].Content; got != "✓ checked: 1 file changed" {
		t.Fatalf("no streamed text: the tail stands alone, got %q", got)
	}
	// Unflagged final text still replaces the streamed draft (a rewrite).
	m3 := NewModel("http://x", "ws/demo", "chan", "", nil)
	m3.appendStream("assistant", "The draft answer.")
	mm3, _ := m3.Update(assistantResponseMsg{content: "The repaired answer."})
	m3 = mm3.(Model)
	if got := m3.messages[len(m3.messages)-1].Content; got != "The repaired answer." {
		t.Fatalf("a rewritten final replaces the draft, got %q", got)
	}
}
