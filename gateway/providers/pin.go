package providers

import "strings"

// UnreachablePin says why a model pinned as "provider:model" cannot answer:
// it names a provider this gateway knows but holds no key for. "" when the
// pin can be tried. The turn then answers on the agent's ladder and says so,
// instead of handing the pin's id to whichever provider answers (live
// 2026-11-11: a pin on aimlapi, whose key was not in the environment of the
// gateway /update restarted, went to ChatGPT as "aimlapi:deepseek/…" and came
// back HTTP 400 model_not_found).
func UnreachablePin(pin string) string {
	want, _ := SplitPin(pin)
	if want == "" {
		return ""
	}
	p, ok := FindProvider(want)
	if !ok || p.Connected() {
		return ""
	}
	switch src := p.KeySource; {
	case strings.Contains(src, KeyNameNotSet):
		return want + " is not connected: " + strings.SplitN(src, " (", 2)[0]
	case strings.HasPrefix(src, "env "):
		return want + " is not connected: " + strings.TrimPrefix(src, "env ") + KeyNameNotSet
	}
	return want + " is not connected: this gateway has no key for it"
}
