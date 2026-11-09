package attribution

import (
	"net/http"
	"testing"
)

// One spelling everywhere, or a ranking splits us across two rows — this is
// the channel (the maintainer, 2026-09-27: "the strategy should be to promote our app
// through openrouter"; 2026-10-24: every provider that can tell apps apart).
func TestAttributionIsOneCanonicalPair(t *testing.T) {
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	h := http.Header{}
	Set(h)
	if h.Get("X-Title") != "Memdoor" || h.Get("HTTP-Referer") != "https://memdoor.ai" {
		t.Fatalf("the pair OpenRouter records: %v", h)
	}

	// Only to a host that reads the pair: the decision client also speaks to
	// TypeSafe and to a local Kev, and a vendor key sees only the User-Agent.
	for endpoint, want := range map[string]bool{
		"https://openrouter.ai/api/alpha/decisions":        true,
		"https://OpenRouter.ai/api/v1":                     true,
		"https://ai-gateway.vercel.sh/v1/chat/completions": true,
		"https://api.llmgateway.io/v1":                     true,
		"https://api.eurouter.ai/api/v1":                   true,
		"https://router.requesty.ai/v1":                    true,
		"https://api.typesafe.ai/v1/systemone":             false,
		"http://localhost:8080/v1/systemone":               false,
		"https://api.anthropic.com/v1/messages":            false,
		"https://api.openai.com/v1/responses":              false,
		"https://inference.baseten.co/v1":                  false,
	} {
		h := http.Header{}
		SetFor(h, endpoint)
		if got := h.Get("X-Title") != ""; got != want {
			t.Errorf("%s: attributed=%v, want %v", endpoint, got, want)
		}
	}

	// And the opt-out is honoured on both paths.
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "0")
	h = http.Header{}
	Set(h)
	SetFor(h, "https://openrouter.ai/api/v1")
	SetFor(h, "https://generativelanguage.googleapis.com/v1beta/openai")
	if len(h) != 0 {
		t.Errorf("opting out must leave the request unattributed: %v", h)
	}
}

// X-AIMLAPI-Source goes to AI/ML API's API host and to nothing that merely
// resembles it: not a subdomain, a parent, a lookalike or a path.
func TestAIMLAPISourceOnlyOnItsExactHost(t *testing.T) {
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	for endpoint, want := range map[string]bool{
		"https://api.aimlapi.com/v1":                  true,
		"https://api.aimlapi.com/v1/chat/completions": true,
		"https://API.AIMLAPI.com/v1":                  true,
		"https://api.aimlapi.com:443/v1":              true,
		"api.aimlapi.com":                             true,
		"https://aimlapi.com/v1":                      false,
		"https://eu.api.aimlapi.com/v1":               false,
		"https://api.aimlapi.com.example.org/v1":      false,
		"https://notapi.aimlapi.com/v1":               false,
		"https://example.org/api.aimlapi.com/v1":      false,
		"https://example.org/v1?host=api.aimlapi.com": false,
		"https://api.aimlapi.com@example.org/v1":      false,
		"https://openrouter.ai/api/v1":                false,
		"http://localhost:8080/v1/chat/completions":   false,
	} {
		h := http.Header{}
		SetFor(h, endpoint)
		if got := h.Get(HeaderAIMLAPISource) == "agent/memdoor"; got != want {
			t.Errorf("%s: source=%v, want %v: %v", endpoint, got, want, h)
		}
	}
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "0")
	if got := Headers("https://api.aimlapi.com/v1"); len(got) != 0 {
		t.Errorf("opting out leaves it off: %v", got)
	}
}

func TestTheHostsWithAHeaderOfTheirOwn(t *testing.T) {
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	SetVersion("1.2.3")
	for _, c := range []struct{ endpoint, header, want string }{
		{"https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", HeaderGoogleClient, "memdoor-oai/1.2.3"},
		{"https://api.llmgateway.io/v1/chat/completions", HeaderLLMGateway, "memdoor.ai"},
		{"https://api.eurouter.ai/api/v1/chat/completions", HeaderEUrouterTitle, "Memdoor"},
	} {
		if got := Headers(c.endpoint)[c.header]; got != c.want {
			t.Errorf("%s: %s = %q, want %q", c.endpoint, c.header, got, c.want)
		}
	}
	// #145: OpenRouter's category lists read X-OpenRouter-Categories; no other host gets it.
	if got := Headers("https://openrouter.ai/api/v1/chat/completions")[HeaderCategories]; got != "cli-agent" {
		t.Errorf("OpenRouter: %s = %q, want cli-agent", HeaderCategories, got)
	}
	h := http.Header{}
	Set(h)
	if h.Get(HeaderCategories) != "cli-agent" {
		t.Errorf("Set: %s = %q, want cli-agent", HeaderCategories, h.Get(HeaderCategories))
	}
	for _, e := range []string{"https://ai-gateway.vercel.sh/v1", "https://api.eurouter.ai/api/v1", "https://api.anthropic.com/v1/messages"} {
		if Headers(e)[HeaderCategories] != "" {
			t.Errorf("%s gets OpenRouter's category header", e)
		}
	}
	for _, h := range []string{HeaderGoogleClient, HeaderLLMGateway, HeaderEUrouterTitle} {
		if Headers("https://api.anthropic.com/v1/messages")[h] != "" {
			t.Errorf("another host's header is not sent to Anthropic: %s", h)
		}
	}
	if Host("https://API.EUrouter.ai:443/api/v1?x=1") != "api.eurouter.ai" || Host("openrouter.ai") != "openrouter.ai" {
		t.Errorf("Host: %q %q", Host("https://API.EUrouter.ai:443/api/v1?x=1"), Host("openrouter.ai"))
	}
}
