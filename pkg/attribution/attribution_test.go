package attribution

import (
	"net/http"
	"testing"
)

// One spelling everywhere, or the ranking splits us across two rows — this is
// the channel (Greg, 2026-09-27: "the strategy should be to promote our app
// through openrouter").
func TestAttributionIsOneCanonicalPair(t *testing.T) {
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	h := http.Header{}
	Set(h)
	if h.Get("X-Title") != "Memdoor" || h.Get("HTTP-Referer") != "https://memdoor.ai" {
		t.Fatalf("the pair OpenRouter records: %v", h)
	}

	// Only when the request is actually going there: the decision client also
	// speaks to TypeSafe and to a local Kev.
	for endpoint, want := range map[string]bool{
		"https://openrouter.ai/api/alpha/decisions": true,
		"https://OpenRouter.ai/api/v1":              true,
		"https://api.typesafe.ai/v1/systemone":      false,
		"http://localhost:8080/v1/systemone":        false,
	} {
		h := http.Header{}
		SetIfOpenRouter(h, endpoint)
		if got := h.Get("X-Title") != ""; got != want {
			t.Errorf("%s: attributed=%v, want %v", endpoint, got, want)
		}
	}

	// And the opt-out is honoured on both paths.
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "0")
	h = http.Header{}
	Set(h)
	SetIfOpenRouter(h, "https://openrouter.ai/api/v1")
	if len(h) != 0 {
		t.Errorf("opting out must leave the request unattributed: %v", h)
	}
}
