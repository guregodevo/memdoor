package providers

import (
	"os"
	"strings"

	"memdoor/tools"
)

// Keenable as the web_search engine (#168), opt-in: KEENABLE_API_KEY set, or
// MEMDOOR_WEB_SEARCH=keenable for the keyless endpoint. MEMDOOR_WEB_SEARCH set
// to anything else keeps OpenRouter even with the key. Neither set, OpenRouter
// stays the engine and api.keenable.ai is never contacted. Never in
// company-key mode: there web_search keeps answering that it has no key.
const (
	WebSearchEngineVar = "MEMDOOR_WEB_SEARCH"
	KeenableKeyVar     = "KEENABLE_API_KEY"
	keenableBase       = "https://api.keenable.ai"
	// keenableTitleHeader names the app; the keyless endpoint refuses a
	// request without it ("X-Keenable-Title header is required for
	// token-less requests").
	keenableTitleHeader = "X-Keenable-Title"
	keenableAppName     = "Memdoor"
)

func keenableSelected() bool {
	if VendorMode() {
		return false
	}
	if engine := strings.TrimSpace(os.Getenv(WebSearchEngineVar)); engine != "" {
		return strings.EqualFold(engine, tools.WebSearchKeenable)
	}
	return strings.TrimSpace(os.Getenv(KeenableKeyVar)) != ""
}

func keenableWebSearch() tools.WebSearchBackend {
	headers := map[string]string{"User-Agent": UserAgent(), keenableTitleHeader: keenableAppName}
	return tools.WebSearchBackend{
		Engine:   tools.WebSearchKeenable,
		Endpoint: keenableBase,
		Key:      strings.TrimSpace(os.Getenv(KeenableKeyVar)),
		Headers:  headers,
	}
}
