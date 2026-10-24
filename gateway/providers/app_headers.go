package providers

import (
	"net/http"

	"memdoor/pkg/attribution"
)

// Every request to a provider names the app, never the person and never the
// key: a User-Agent of "Memdoor/<version> (+https://memdoor.ai)" on all of
// them (what a provider sees in its logs and partner dashboards; Go's default
// says only Go-http-client), and the app headers a host reads on the hosts
// that read them — pkg/attribution owns which host reads what.
var appVersion = "dev"

// SetAppVersion is called once at start with the build's version.
func SetAppVersion(v string) {
	if v != "" {
		appVersion = v
		attribution.SetVersion(v)
	}
}

// UserAgent is the app's name on the wire.
func UserAgent() string { return "Memdoor/" + appVersion + " (+" + attribution.AppURL + ")" }

// SetAppHeaders names the app on a request: the User-Agent everywhere, the
// host's own app headers where it has them.
func SetAppHeaders(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Set("User-Agent", UserAgent())
	if req.URL != nil {
		attribution.SetFor(req.Header, req.URL.Host)
	}
}

// AppHeaders is the same identity as a map, for callers that carry headers
// that way.
func AppHeaders(endpoint string) map[string]string {
	h := map[string]string{"User-Agent": UserAgent()}
	for name, value := range attribution.Headers(endpoint) {
		h[name] = value
	}
	return h
}
