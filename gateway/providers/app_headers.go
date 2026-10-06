package providers

import (
	"net/http"
	"strings"
)

// OpenRouter lists the apps that identify themselves (openrouter.ai/apps):
// two headers on every request, naming the app and its site — never the
// person, never the key. On any other host they are not sent.
const (
	appReferer = "https://memdoor.ai"
	appTitle   = "Memdoor"
)

func isOpenRouterHost(host string) bool {
	return host == "openrouter.ai" || strings.HasSuffix(host, ".openrouter.ai")
}

// SetAppHeaders adds the app's identity to a request bound for OpenRouter.
func SetAppHeaders(req *http.Request) {
	if req == nil || req.URL == nil || !isOpenRouterHost(req.URL.Host) {
		return
	}
	req.Header.Set("HTTP-Referer", appReferer)
	req.Header.Set("X-Title", appTitle)
}

// AppHeaders is the same identity as a map, for callers that carry headers
// that way; empty for any host but OpenRouter.
func AppHeaders(endpoint string) map[string]string {
	if !strings.Contains(endpoint, "openrouter.ai") {
		return nil
	}
	return map[string]string{"HTTP-Referer": appReferer, "X-Title": appTitle}
}
