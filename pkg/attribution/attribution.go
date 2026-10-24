// Package attribution names this app to the providers that can tell apps
// apart, never the person and never the key.
//
// THE CHANNEL IS THE PROVIDERS' OWN APP PAGES (Greg, 2026-09-27: "the strategy
// should be to promote our app through openrouter"; 2026-10-24: every provider
// that can recognise the app must). A request counts toward an app only when
// it carries the headers that provider reads:
//
//   - OpenRouter (openrouter.ai/apps, app_id 5145054, docs/internal/OPS.md),
//     Vercel AI Gateway (its apps leaderboard), Requesty (private analytics) and
//     LLM Gateway (as a fallback) read HTTP-Referer + X-Title.
//   - LLM Gateway lists apps by X-Source (llmgateway.io/apps).
//   - EUrouter lists apps by HTTP-Referer + X-EUrouter-Title.
//   - Google asks every partner integration for x-goog-api-client
//     "company-product/version", "-oai" on the OpenAI-compatible endpoint
//     (ai.google.dev/gemini-api/docs/partner-integration); it is segmentation
//     on their side, not a public page.
//
// Anthropic, OpenAI, xAI, DeepSeek, Baseten, Groq and the rest document no
// app header (researched 2026-10-24, docs/features/PROVIDERS.md); they see the
// User-Agent the gateway sets and nothing more. A vendor key in company mode
// therefore carries only that and the company's own MEMDOOR_VENDOR_HEADERS.
//
// ONE definition, imported by everything that talks to a provider — the
// gateway's chat clients and the decision client on a person's own key. Two
// spellings would split us across two rows in a ranking, which is the whole
// thing we are trying to win. MEMDOOR_OPENROUTER_ATTRIBUTION=0 leaves every
// app header off (the User-Agent stays: it names the software, as any client's
// does).
package attribution

import (
	"net/http"
	"os"
	"strings"
)

const (
	// AppName is the X-Title the app pages list us under.
	AppName = "Memdoor"
	// AppURL is the HTTP-Referer they scrape the card from: the title,
	// description and og:image at that address are what people see next to us.
	AppURL = "https://memdoor.ai"
	// AppSource is LLM Gateway's X-Source, a bare domain.
	AppSource = "memdoor.ai"

	HeaderReferer       = "HTTP-Referer"
	HeaderTitle         = "X-Title"
	HeaderGoogleClient  = "x-goog-api-client"
	HeaderLLMGateway    = "X-Source"
	HeaderEUrouterTitle = "X-EUrouter-Title"
)

// pairHosts read HTTP-Referer + X-Title.
var pairHosts = []string{"openrouter.ai", "ai-gateway.vercel.sh", "requesty.ai", "llmgateway.io", "eurouter.ai"}

var version = "dev"

// SetVersion is called once at start with the build's version; Google's
// header carries it.
func SetVersion(v string) {
	if v = strings.TrimSpace(v); v != "" {
		version = v
	}
}

// GoogleClient is the x-goog-api-client value, company-product/version; the
// -oai suffix marks the OpenAI-compatible endpoint, as Google's partner page
// asks.
func GoogleClient() string { return "memdoor-oai/" + version }

// Off reports whether this machine has opted out.
func Off() bool {
	v := strings.TrimSpace(os.Getenv("MEMDOOR_OPENROUTER_ATTRIBUTION"))
	return v == "0" || strings.EqualFold(v, "off")
}

// Set names this app on a request bound for OpenRouter or another host that
// reads the pair.
func Set(h http.Header) {
	if Off() {
		return
	}
	h.Set(HeaderReferer, AppURL)
	h.Set(HeaderTitle, AppName)
}

// Headers is what a request to endpoint carries to name the app: nothing for
// a host that documents no app header.
func Headers(endpoint string) map[string]string {
	if Off() {
		return nil
	}
	host := Host(endpoint)
	out := map[string]string{}
	for _, p := range pairHosts {
		if isHost(host, p) {
			out[HeaderReferer], out[HeaderTitle] = AppURL, AppName
			break
		}
	}
	switch {
	case isHost(host, "googleapis.com"):
		out[HeaderGoogleClient] = GoogleClient()
	case isHost(host, "llmgateway.io"):
		out[HeaderLLMGateway] = AppSource
	case isHost(host, "eurouter.ai"):
		out[HeaderEUrouterTitle] = AppName
	}
	return out
}

// SetFor names the app on a request bound for endpoint, with the headers that
// host reads. The decision client also speaks to TypeSafe directly and to a
// local Kev, where no app header means anything.
func SetFor(h http.Header, endpoint string) {
	for name, value := range Headers(endpoint) {
		h.Set(name, value)
	}
}

// Host is the lower-cased host of an endpoint or a bare host, port dropped.
func Host(endpoint string) string {
	if i := strings.Index(endpoint, "://"); i >= 0 {
		endpoint = endpoint[i+3:]
	}
	if i := strings.IndexAny(endpoint, "/?#"); i >= 0 {
		endpoint = endpoint[:i]
	}
	if i := strings.LastIndex(endpoint, ":"); i >= 0 && !strings.Contains(endpoint[i:], "]") {
		endpoint = endpoint[:i]
	}
	return strings.ToLower(endpoint)
}

func isHost(host, name string) bool { return host == name || strings.HasSuffix(host, "."+name) }
