package providers

import (
	"net/http"
	"strings"
	"testing"
)

func TestEveryProviderSeesTheAppAndOnlyTheHostsThatRankAppsGetThePair(t *testing.T) {
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	SetAppVersion("1.2.3")
	for url, pair := range map[string]bool{
		"https://openrouter.ai/api/v1/chat/completions":    true,
		"https://ai-gateway.vercel.sh/v1/chat/completions": true,
		"https://api.anthropic.com/v1/messages":            false,
		"https://api.openai.com/v1/responses":              false,
		"https://inference.baseten.co/v1/chat/completions": false,
	} {
		req, _ := http.NewRequest(http.MethodPost, url, nil)
		SetAppHeaders(req)
		if got := req.Header.Get("X-Title") == "Memdoor" && req.Header.Get("HTTP-Referer") == "https://memdoor.ai"; got != pair {
			t.Fatalf("%s: pair=%v, want %v: %v", url, got, pair, req.Header)
		}
		if ua := req.Header.Get("User-Agent"); !strings.HasPrefix(ua, "Memdoor/1.2.3") || !strings.Contains(ua, "memdoor.ai") {
			t.Fatalf("%s: every provider sees the app's name: %q", url, ua)
		}
		if m := AppHeaders(url); m["User-Agent"] == "" || (m["X-Title"] == "Memdoor") != pair {
			t.Fatalf("%s: the map form agrees with the request form: %v", url, m)
		}
	}
	g, _ := http.NewRequest(http.MethodPost, "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", nil)
	SetAppHeaders(g)
	if g.Header.Get("x-goog-api-client") != "memdoor-oai/1.2.3" || g.Header.Get("X-Title") != "" {
		t.Fatalf("Google gets its partner header and not the pair: %v", g.Header)
	}
}
