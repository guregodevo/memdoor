package ui

import (
	"encoding/json"
	"regexp"
	"strings"
)

// A tool call written as bare JSON must not be SHOWN as text.
//
// The gateway parses those into real tool calls (providers/toolproto.go: the
// untagged fallback), so the frame renders correctly — but the streamed deltas
// arrive before any of that, so the reader watches the raw object scroll past
// and then sees the frame for it. Measured 2026-08-30:
//
//	{"arguments": {"command": "new-program"}
//	                                            ,"name": "skill"}
//	⏺ skill(command: "new-program")
//
// The object is held from the moment its opening brace arrives until the
// braces balance, then dropped if it is a call and emitted if it is not — so
// JSON the model is genuinely showing the reader still appears.
// The held bytes are a []byte and NOT a strings.Builder: this filter is a
// VALUE field on Model, and Bubble Tea copies the whole Model on every Update.
// A Builder that has been written to panics the moment it is copied
// ("illegal use of non-zero Builder copied by value") — a crash that only
// fires once a stream contains a brace, so it survives every test that does
// not stream one.
type jsonCallFilter struct {
	buf   []byte // the candidate object, while it is incomplete
	depth int
	inStr bool
	esc   bool
	// afterCall: a call was just hidden; the comma that follows it is hidden too.
	afterCall bool
}

// feed returns the text to display, holding back any in-progress JSON object.
func (f *jsonCallFilter) feed(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if f.depth == 0 {
			if c == '{' {
				f.depth = 1
				f.buf = append(f.buf[:0], c)
				continue
			}
			// The comma between two halves of a split call is not prose
			// either: "}}," sat on the screen after the hidden call (2026-09-19).
			if f.afterCall && (c == ',' || c == ' ' || c == '\t') {
				continue
			}
			f.afterCall = false
			out.WriteByte(c)
			continue
		}

		f.buf = append(f.buf, c)
		switch {
		case f.esc:
			f.esc = false
		case c == '\\' && f.inStr:
			f.esc = true
		case c == '"':
			f.inStr = !f.inStr
		case f.inStr:
			// braces inside a string do not nest
		case c == '{':
			f.depth++
		case c == '}':
			f.depth--
			if f.depth == 0 {
				blob := string(f.buf)
				f.buf = f.buf[:0]
				if !looksLikeToolCall(blob) {
					out.WriteString(blob) // ordinary JSON: show it
				} else {
					f.afterCall = true
				}
			}
		}
	}
	return out.String()
}

// flush releases anything still held — an object that never closed was not a
// call, and swallowing it would lose real text.
func (f *jsonCallFilter) flush() string {
	if f.depth == 0 {
		return ""
	}
	held := string(f.buf)
	f.buf = f.buf[:0]
	f.depth, f.inStr, f.esc = 0, false, false
	return held
}

// looksLikeToolCall is the same shape the gateway accepts untagged: a non-empty
// name, an arguments object if present, and no other keys. Deliberately strict
// so JSON the model is explaining is not swallowed.
func looksLikeToolCall(blob string) bool {
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(blob), &raw) != nil {
		return false
	}
	for k := range raw {
		if k != "name" && k != "arguments" {
			return false
		}
	}
	var name string
	_ = json.Unmarshal(raw["name"], &name)
	args, hasArgs := raw["arguments"]
	if hasArgs && len(args) > 0 {
		var obj map[string]json.RawMessage
		if json.Unmarshal(args, &obj) != nil {
			return false
		}
	}
	// A NAMELESS HALF IS STILL A CALL. {"arguments":{"path":…}} is the
	// first object of a call split in two (the gateway splices them); shown,
	// it is the raw call on the screen (2026-09-19).
	return name != "" || hasArgs
}

// Held reports how many bytes of an in-progress object are being withheld.
//
// Holding is correct — a raw tool call must not scroll past as text — but
// holding SILENTLY is what froze the screen: measured live 2026-08-30, the
// reply stopped at "I'll make the calls." and nothing moved for six minutes
// while the model went on generating into the output cap. Nothing was wrong
// except that the one thing happening was invisible. The view uses this to say
// so.
func (f *jsonCallFilter) Held() int { return len(f.buf) }

// PendingName is the tool named by the object currently being held, or "" when
// the name has not arrived yet. Best-effort by design: the buffer is partial
// JSON, so it cannot be decoded, and a wrong guess in the indicator is worse
// than the generic label the caller falls back to.
func (f *jsonCallFilter) PendingName() string {
	if len(f.buf) == 0 {
		return ""
	}
	m := pendingNameRE.FindSubmatch(f.buf)
	if m == nil {
		return ""
	}
	return string(m[1])
}

var pendingNameRE = regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)
