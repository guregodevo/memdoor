package shared

import "testing"

func TestAccountHostTrusted(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://memdoor.ai/billing":   true,
		"http://localhost:18789":       true,
		"http://127.0.0.1:18790/x":     true,
		"http://memdoor.ai":            false, // the token never over plain http
		"https://memdoor.ai.evil.com":  false,
		"https://evil.example/billing": false,
		"not a url \x7f":               false,
	} {
		if got := AccountHostTrusted(raw); got != want {
			t.Errorf("%q: %v, want %v", raw, got, want)
		}
	}
}
