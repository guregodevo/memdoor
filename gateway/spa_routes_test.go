package gateway

import "testing"

func TestUnknownPathsAreNotRoutes(t *testing.T) {
	for _, p := range []string{"/", "/pricing", "/features", "/workflows", "/docs", "/docs/security", "/r/abc", "/s/xyz", "/pro/checkout", "/topup", "/login", "/download/"} {
		if !isSPARoute(p) {
			t.Errorf("%s is a route of the app", p)
		}
	}
	for _, p := range []string{"/cyberlaw", "/cyberlaw/legislation", "/localllm/gptq", "/hackernews/economics", "/this-does-not-exist", "/pricingx", "/docsx"} {
		if isSPARoute(p) {
			t.Errorf("%s is not a route and must 404", p)
		}
	}
}
