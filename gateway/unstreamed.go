package gateway

import "strings"

// What the window never saw.
//
// The window learns a turn's text from the per-round stream events; the
// final reply the runner hands to the websocket writer is a struct the
// writer cannot send. So everything appended AFTER the rounds — the
// empty-reply notice, "Stopped: the same tool kept failing", the API-call
// limit, the honesty notes — was posted to the channel and never shown:
// Sara's brief ended on a notice that only the channel had (2026-09-19).
// The tail beyond what streamed is emitted as one more text event.

// unstreamedTail is what the final text says beyond the rounds that
// streamed, and whether that is ONLY a tail to add under them. A final text
// that no longer begins with what streamed was rewritten (a suppressed
// draft, a repaired citation) and is shown whole (isTail false: the window
// replaces its draft). The distinction travels with the event: a tail sent
// without it REPLACED the streamed answer in the window — a zero-tool turn's
// whole reply vanished behind "(note: this turn used no tools …)"
// (live 2026-10-04).
func unstreamedTail(final, streamed string) (text string, isTail bool) {
	f := strings.TrimSpace(final)
	s := strings.TrimSpace(streamed)
	if s == "" {
		return f, false
	}
	if strings.HasPrefix(f, s) {
		return strings.TrimSpace(f[len(s):]), true
	}
	return f, false
}
