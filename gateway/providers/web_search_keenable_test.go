package providers

import (
	"memdoor/internal/testenv"
	"testing"

	"memdoor/tools"
)

// Unset, OpenRouter stays the engine; MEMDOOR_WEB_SEARCH=keenable switches
// it, except in company-key mode (#168).
func TestWebSearchEngineIsOptIn(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv(KeenableKeyVar, "")
	t.Cleanup(func() { _ = ClearRemoteEngine() })
	_ = ClearRemoteEngine()

	t.Setenv(WebSearchEngineVar, "")
	if be := WebSearch(); be.Engine != "" || be.Endpoint != byokEndpoint {
		t.Fatalf("unset: %+v, want OpenRouter", be)
	}

	t.Setenv(WebSearchEngineVar, "Keenable")
	be := WebSearch()
	if be.Engine != tools.WebSearchKeenable || be.Endpoint != keenableBase || be.Key != "" || be.Headers[keenableTitleHeader] != keenableAppName {
		t.Fatalf("keenable: %+v", be)
	}
	t.Setenv(KeenableKeyVar, " kn-test ")
	if be := WebSearch(); be.Key != "kn-test" {
		t.Fatalf("the key: %q", be.Key)
	}

	// The key alone selects Keenable; an explicit other engine keeps OpenRouter.
	t.Setenv(WebSearchEngineVar, "")
	if be := WebSearch(); be.Engine != tools.WebSearchKeenable || be.Key != "kn-test" {
		t.Fatalf("KEENABLE_API_KEY alone: %+v", be)
	}
	t.Setenv(WebSearchEngineVar, "openrouter")
	if be := WebSearch(); be.Engine != "" {
		t.Fatalf("MEMDOOR_WEB_SEARCH=openrouter with a Keenable key: %+v", be)
	}
	t.Setenv(WebSearchEngineVar, "")

	if err := SetRemoteEngine(vendorEngine(VendorAnthropic, anthropicDefaultBase, "k", anthropicDefaultModel, 1000)); err != nil {
		t.Fatal(err)
	}
	if be := WebSearch(); be.Engine != "" {
		t.Fatalf("company-key mode must keep OpenRouter's backend: %+v", be)
	}
}
