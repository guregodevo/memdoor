package gateway

import (
	"encoding/json"
	"regexp"
	"sync"
)

// User-configurable tool guards — permission gates as workspace CONFIG, not
// permission modes. The runtime stays always-auto; a guard is a rule the
// workspace admin sets once ("never let an agent run sudo", "never touch
// .env") that blocks a matching tool call with a teaching error the model can
// act on. Complements the hardcoded catastrophic-command net in executeTool:
// that one protects the MACHINE and is not configurable; these protect the
// PROJECT and are.
//
// Stored as workspace setting `tool_guards`, a JSON array:
//
//	[{"tool":"bash","pattern":"\\bsudo\\b","message":"sudo is off-limits here"},
//	 {"tool":"*","pattern":"\\.env\\b"}]
//
// `tool` is a tool name or "*"; `pattern` is a Go regexp matched against the
// raw input JSON; `message` is optional teaching text.
type toolGuard struct {
	Tool    string `json:"tool"`
	Pattern string `json:"pattern"`
	Message string `json:"message,omitempty"`

	re *regexp.Regexp
}

// guardCache caches compiled rule sets keyed by the raw settings value, so the
// per-call path costs one settings read plus a map hit; a settings change is a
// new key and compiles once.
var guardCache sync.Map // raw string → []toolGuard

// parseToolGuards compiles the settings value into guard rules. Invalid JSON
// or an invalid regexp disables that rule (never the whole set) — a typo in
// one rule must not silently drop the others.
func parseToolGuards(raw string) []toolGuard {
	if raw == "" {
		return nil
	}
	if cached, ok := guardCache.Load(raw); ok {
		return cached.([]toolGuard)
	}
	var rules []toolGuard
	_ = json.Unmarshal([]byte(raw), &rules)
	out := rules[:0]
	for _, r := range rules {
		if r.Pattern == "" {
			continue
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		r.re = re
		out = append(out, r)
	}
	guardCache.Store(raw, out)
	return out
}

// guardBlocks returns the teaching error for the first rule matching this
// call, or "" when no guard applies.
func guardBlocks(rules []toolGuard, tool string, input []byte) string {
	for _, r := range rules {
		if r.Tool != "" && r.Tool != "*" && r.Tool != tool {
			continue
		}
		if r.re.Match(input) {
			msg := r.Message
			if msg == "" {
				msg = "this call matches a workspace guard rule (pattern " + r.Pattern + ")"
			}
			return "blocked by workspace tool guard: " + msg + ". Choose a different approach; if the action is truly needed, ask the user to do it or to lift the guard"
		}
	}
	return ""
}
