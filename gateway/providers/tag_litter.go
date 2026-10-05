package providers

import (
	"regexp"
	"strings"
)

// scrubTagLitter removes protocol markup from a reply before it becomes an
// assistant message.
//
// This is not cosmetic — it BREAKS A FEEDBACK LOOP. Whatever survives here is
// stored in the conversation and shown back to the model on the next turn, so a
// stray marker becomes an example the model imitates and emits more of.
//
// Measured 2026-08-30: the list below lacked "</parameter>", and a session ended
// up with an assistant message whose entire text was "</parameter>". The next
// turn produced it repeatedly, to the token cap. The model was reading its own
// debris back.
//
// So the list must cover every dialect the parser tolerates and every marker
// this model has been seen to borrow — a tag stripped from the SCREEN but left
// in the CONVERSATION still teaches the model to write it.
func scrubTagLitter(s string) string {
	for _, tag := range litterTags {
		s = strings.ReplaceAll(s, tag, "")
	}
	// A tag CUT SHORT: a runaway stopped mid-marker leaves "</anth" at the end,
	// or writes it malformed as "</anth>", and the exact list above misses
	// both — the fragment reached the screen and the stored transcript.
	s = cutLitter.ReplaceAllStringFunc(s, func(m string) string {
		name := strings.Trim(m, "</>")
		for _, tag := range litterTags {
			if full := strings.Trim(tag, "</>"); len(name) < len(full) && strings.HasPrefix(full, name) {
				return ""
			}
		}
		return m
	})
	return strings.TrimSpace(s)
}

// cutLitter is a closing tag, or a tag left open at the very end of a reply,
// whose name may be a litter tag's cut short (scrubTagLitter checks which).
var cutLitter = regexp.MustCompile(`</[a-z_]{3,}>|</?[a-z_]{3,}$`)

// litterTags are the protocol markers scrubTagLitter removes.
var litterTags = []string{
	"</tool_call>", "<tool_call>",
	"</tool_calls>", "<tool_calls>", // plural: never ours
	"</invoke>", "<invoke>",
	"</function>", "<function>",
	"</parameter>", "<parameter>",
	"</tool_response>", "<tool_response>",
	"</anthropic>",
	// Plural container tags, measured live 2026-08-31: a reply carried
	// "</functions>\n</tools>" twice — the tool-LIST encoding of some
	// other harness's prompt, echoed back. Singular was here; plural
	// walked through and reached both the screen and the stored reply.
	"</functions>", "<functions>",
	"</tools>", "<tools>",
	// NOT "</think>": stripThink needs that marker to separate reasoning
	// from the answer, and scrubbing it first destroys the split. Removed
	// again within minutes of adding it (2026-08-30) — the think tags are
	// load-bearing, not litter.
}
