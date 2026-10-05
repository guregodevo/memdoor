package ui

import (
	"regexp"
	"strings"
	"time"
)

// thinkStreamSplitter separates a streamed reply into REASONING and ANSWER as
// the chunks arrive.
//
// The model's <think> block streams as ordinary text deltas, and the reply is
// only classified as reasoning after the turn ends (providers.replyContent). A
// streamed turn therefore rendered the entire thought process as the answer —
// measured 2026-08-30, pages of "Let me think about what this means" above the
// actual reply.
//
// The hard part is that a marker can be SPLIT ACROSS DELTAS: "</thi" arrives in
// one chunk and "nk>" in the next. So a partial marker at the end of a chunk is
// held back rather than emitted, and joined with whatever comes next.
type thinkStreamSplitter struct {
	inThink bool
	pending string // a possible partial marker held from the previous chunk
	// thinkLen is how much has gone to the reasoning block since it opened.
	// A block that never closes must not swallow the whole reply — see
	// maxThinkRun.
	thinkLen int
	// calls is the stack of open tool-call blocks (their opener kinds). While
	// it is non-empty the text is a tool call's body and never reaches the
	// screen: the gateway renders the parsed call as a frame. See callOpeners.
	calls   []string
	callLen int
}

// maxCallRun bounds a tool-call block whose closer never arrives; past it the
// text is shown rather than hidden, like maxThinkRun. Tool bodies can be a
// whole file (write_file), so the bound is generous.
const maxCallRun = 64000

// callOpeners are the Qwen3-Coder XML tool-call tags, which carry the NAME in
// the tag ("<function=read_file>", "<parameter=path>") and so never match
// the bare litter tags below. Live 2026-09-03 a call streamed as
//
//	<function=read_file>
//	<parameter=path>
//	main.go
//	⏺ Read(main.go)
//
// — the closers were scrubbed as litter and the openers, with their body,
// were shown as text above the frame. An opener hides everything up to its
// closer (or </tool_call>, which ends every open block).
var callOpeners = map[string]string{
	"<function=":  "</function>",
	"<invoke ":    "</invoke>",
	"<parameter=": "</parameter>",
}

// maxThinkRun bounds how much text a single <think> block may absorb before the
// splitter decides the closer was lost and treats the rest as the answer.
//
// Without it an unclosed opener routed EVERYTHING to reasoning, and the thinking
// block renders clipped to a few lines — so a runaway that opened <think> and
// never closed it made the screen go blank apart from a dim stub (measured
// 2026-08-30). Reasoning is normally hundreds to a couple of thousand
// characters; well past that the marker is simply broken, and showing the text
// beats hiding it.
const maxThinkRun = 8000

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

// litterTags are protocol markers that must never reach the screen.
//
// scrubTagLitter removes these from a COMPLETE reply (providers/toolproto.go),
// but streamed deltas never pass through it — they go straight to the
// transcript. So a model repeating its own closing tag rendered a wall of
// "</tool_call>" live, measured 2026-08-30. The same text was scrubbed in the
// finished reply, which is why this only appeared once streaming worked.
var litterTags = []string{
	"</tool_call>", "<tool_call>",
	"</invoke>", "<invoke>",
	"</function>", "<function>",
	"</anthropic>",
	// Seen live 2026-08-30: the model closes a <parameter> it never opened,
	// leaving a bare tag in the middle of the reply.
	"</parameter>", "<parameter>",
	"</tool_response>", "<tool_response>",
	// Plural container tags, live 2026-08-31: "</functions>\n</tools>" echoed
	// twice into a reply — the tool-LIST encoding of another harness's prompt.
	"</functions>", "<functions>",
	"</tools>", "<tools>",
}

// allMarkers is every string the scanner must not split across a chunk
// boundary: emitting half of one prints raw markup mid-sentence.
func allMarkers() []string {
	all := append([]string{thinkOpen, thinkClose}, litterTags...)
	for o := range callOpeners {
		all = append(all, o)
	}
	return all
}

// closesCall reports whether tag closes the innermost open tool-call block.
func (s *thinkStreamSplitter) closesCall(tag string) bool {
	if len(s.calls) == 0 {
		return false
	}
	if tag == "</tool_call>" {
		return true
	}
	return callOpeners[s.calls[len(s.calls)-1]] == tag
}

// next consumes one delta and returns the reasoning and answer text within it.
// Protocol litter is dropped; think markers toggle which side the text goes to.
func (s *thinkStreamSplitter) next(chunk string) (think, answer string) {
	buf := s.pending + chunk
	s.pending = ""

	emit := func(text string) {
		if text == "" {
			return
		}
		if len(s.calls) > 0 {
			// A tool call's body: the frame shows it, the stream must not.
			s.callLen += len(text)
			if s.callLen > maxCallRun {
				s.calls, s.callLen = nil, 0
				answer += text
			}
			return
		}
		if s.inThink {
			// A think block this long has lost its closer. Stop hiding the
			// reply behind a clipped block and show the rest as the answer.
			if s.thinkLen+len(text) > maxThinkRun {
				s.inThink = false
				s.thinkLen = 0
				answer += text
				return
			}
			s.thinkLen += len(text)
			think += text
			return
		}
		answer += text
	}

	for buf != "" {
		// Earliest marker of any kind wins, so a </think> before a </tool_call>
		// is handled in the order the model wrote them.
		at, found := -1, ""
		for _, m := range allMarkers() {
			if i := strings.Index(buf, m); i >= 0 && (at < 0 || i < at || (i == at && len(m) > len(found))) {
				at, found = i, m
			}
		}
		if at < 0 {
			keep := partialSuffix(buf, allMarkers()...)
			emit(buf[:len(buf)-keep])
			s.pending = buf[len(buf)-keep:]
			return think, answer
		}
		emit(buf[:at])
		buf = buf[at+len(found):]
		switch {
		case found == thinkOpen:
			s.inThink = true
			s.thinkLen = 0
		case found == thinkClose:
			s.inThink = false
			s.thinkLen = 0
		case callOpeners[found] != "":
			s.calls = append(s.calls, found)
		case s.closesCall(found):
			if found == "</tool_call>" {
				s.calls = nil
			} else {
				s.calls = s.calls[:len(s.calls)-1]
			}
			if len(s.calls) == 0 {
				s.callLen = 0
			}
		default: // litter — dropped, state unchanged
		}
	}
	return think, answer
}

// partialSuffix returns how many trailing bytes of buf could be the start of
// either marker — the amount to hold back until the next chunk arrives.
func partialSuffix(buf string, markers ...string) int {
	longest := 0
	for _, m := range markers {
		for n := len(m) - 1; n > 0; n-- {
			if n > len(buf) {
				continue
			}
			if strings.HasSuffix(buf, m[:n]) && n > longest {
				longest = n
			}
		}
	}
	return longest
}

// appendStream adds streamed text to the last message of this role, or starts a
// new one. Reasoning and answer therefore accumulate into separate blocks even
// though they arrive interleaved on one stream.
func (m *Model) appendStream(role, text string) {
	if n := len(m.messages); n > 0 && m.messages[n-1].Role == role && !m.messages[n-1].IsThinking {
		// TWO ROUNDS WITH NO FRAME BETWEEN THEM — a retry after a stopped
		// reply — streamed into one paragraph: "…for context!The last line
		// shows…" (the drone reel, 2026-09-19). A closed round ends its
		// paragraph; the next one starts its own.
		if role == "assistant" && m.roundClosed && strings.TrimSpace(m.messages[n-1].Content) != "" {
			text = "\n\n" + strings.TrimLeft(text, " \t\r\n")
		}
		m.roundClosed = false
		m.messages[n-1].Content += text
		m.messages[n-1].Streaming = true
		return
	}
	m.roundClosed = false
	m.messages = append(m.messages, Message{Role: role, Content: text, Streaming: true, Timestamp: time.Now()})
}

// settleStreaming marks every message as finished arriving, so they render as
// markdown again rather than verbatim.
func (m *Model) settleStreaming() {
	for i := range m.messages {
		if m.messages[i].Role == "workflow" {
			continue // a graph is live while its run is (workflowGraphMessage), not while a turn is
		}
		m.messages[i].Streaming = false
		if m.messages[i].Role == "assistant" {
			m.messages[i].Content = stripCallTags(m.messages[i].Content)
		}
	}
}

// stripCallTags removes the tool-call protocol tags a raw stream carries:
// invisible once rendered as markdown, but words to a comparison and
// litter in a verbatim view.
func stripCallTags(s string) string {
	if !strings.ContainsAny(s, "<{}") {
		return s
	}
	// Both call forms: the JSON one's tags, and the XML-parameter one's
	// <function=x> / <parameter=y> — whose "</parameter>" and a stray "}"
	// were the words that made a round print twice (Sara's brief,
	// 2026-09-19, the trace).
	return callMarkup.ReplaceAllString(s, "")
}

var callMarkup = regexp.MustCompile(`</?(?:tool_call|function(?:=[^>]*)?|parameter(?:=[^>]*)?)>`)

// words are what a comparison counts: the text without call markup, split
// on space, with tokens made only of braces and commas dropped.
func words(s string) []string {
	var out []string
	for _, w := range strings.Fields(stripCallTags(s)) {
		if strings.Trim(w, "{}[],") == "" {
			continue
		}
		out = append(out, w)
	}
	return out
}
