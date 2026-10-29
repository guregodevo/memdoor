package gateway

import "testing"

func TestUnknownPathsAreNotRoutes(t *testing.T) {
	for _, p := range []string{"/", "/enterprise", "/features", "/workflows", "/docs", "/docs/security", "/compare/claude-code", "/r/abc", "/s/xyz", "/pro/checkout", "/topup", "/login", "/download/", "/welcome"} {
		if !isSPARoute(p) {
			t.Errorf("%s is a route of the app", p)
		}
	}
	if legacyRedirects["/how-it-works"] != "/docs/how-it-works" {
		t.Fatal("/how-it-works moves to the docs")
	}
	if legacyRedirects["/pricing"] != "/enterprise" {
		t.Fatal("/pricing moves to /enterprise")
	}
	for _, p := range []string{"/cyberlaw", "/cyberlaw/legislation", "/localllm/gptq", "/hackernews/economics", "/this-does-not-exist", "/pricingx", "/docsx", "/compare/zzz"} {
		if isSPARoute(p) {
			t.Errorf("%s is not a route and must 404", p)
		}
	}
}
