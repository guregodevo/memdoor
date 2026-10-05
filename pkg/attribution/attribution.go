// Package attribution names this app to OpenRouter.
//
// THE CHANNEL IS OPENROUTER (Greg, 2026-09-27: "the strategy should be to
// promote our app through openrouter"). Their ranking is where this audience
// compares tools, and a request only counts toward an app when it carries these
// two headers. We are app_id 5145054, origin https://memdoor.ai/
// (docs/internal/OPS.md).
//
// ONE definition, imported by everything that talks to OpenRouter — the
// gateway's chat clients and the decision client on a person's own key. Two spellings would split
// us across two rows in the ranking, which is the whole thing we are trying to
// win.
//
// The headers carry the app's name and site. Nothing about the person, their
// code or their prompt, and OpenRouter sees the traffic either way.
// MEMDOOR_OPENROUTER_ATTRIBUTION=0 leaves them off.
package attribution

import (
	"net/http"
	"os"
	"strings"
)

const (
	// AppName is the X-Title OpenRouter lists us under.
	AppName = "Memdoor"
	// AppURL is the HTTP-Referer they scrape the card from: the title,
	// description and og:image at that address are what people see next to us.
	AppURL = "https://memdoor.ai"
)

// Off reports whether this machine has opted out.
func Off() bool {
	v := strings.TrimSpace(os.Getenv("MEMDOOR_OPENROUTER_ATTRIBUTION"))
	return v == "0" || strings.EqualFold(v, "off")
}

// Set names this app on a request bound for OpenRouter.
func Set(h http.Header) {
	if Off() {
		return
	}
	h.Set("HTTP-Referer", AppURL)
	h.Set("X-Title", AppName)
}

// SetIfOpenRouter names the app only when the request is actually going to
// OpenRouter. The decision client also speaks to TypeSafe directly and to a
// local Kev, where these headers mean nothing.
func SetIfOpenRouter(h http.Header, endpoint string) {
	if strings.Contains(strings.ToLower(endpoint), "openrouter.ai") {
		Set(h)
	}
}
