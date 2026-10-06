package providers

import (
	"net/http"
	"strings"
)

// Every request to a provider names the app: a User-Agent of
// "Memdoor/<version> (+https://memdoor.ai)" on all of them (what a provider
// sees in its logs and partner dashboards; Go's default says only
// Go-http-client), and on OpenRouter the two headers its apps page
// (openrouter.ai/apps) lists apps by. Never the person, never the key.
const (
	appReferer = "https://memdoor.ai"
	appTitle   = "Memdoor"
)

var appVersion = "dev"

// SetAppVersion is called once at start with the build's version.
func SetAppVersion(v string) {
	if v != "" {
		appVersion = v
	}
}

// UserAgent is the app's name on the wire.
func UserAgent() string { return "Memdoor/" + appVersion + " (+" + appReferer + ")" }

func isOpenRouterHost(host string) bool {
	return host == "openrouter.ai" || strings.HasSuffix(host, ".openrouter.ai")
}

// SetAppHeaders names the app on a request: the User-Agent everywhere, the
// OpenRouter pair on OpenRouter.
func SetAppHeaders(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Set("User-Agent", UserAgent())
	if req.URL != nil && isOpenRouterHost(req.URL.Host) {
		req.Header.Set("HTTP-Referer", appReferer)
		req.Header.Set("X-Title", appTitle)
	}
}

// AppHeaders is the same identity as a map, for callers that carry headers
// that way.
func AppHeaders(endpoint string) map[string]string {
	h := map[string]string{"User-Agent": UserAgent()}
	if strings.Contains(endpoint, "openrouter.ai") {
		h["HTTP-Referer"] = appReferer
		h["X-Title"] = appTitle
	}
	return h
}
