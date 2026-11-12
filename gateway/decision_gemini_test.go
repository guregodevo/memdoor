package gateway

import (
	"testing"

	"memdoor/gateway/providers"
	"memdoor/internal/testenv"
)

func geminiEngine(t *testing.T, vendor string) {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	t.Setenv("GEMINI_API_KEY", "k")
	t.Setenv("GEMINI_BASE_URL", "")
	t.Cleanup(func() { _ = providers.ClearRemoteEngine() })
	if err := providers.SetRemoteEngine(providers.RemoteEngine{Name: providers.VendorEngineName, Vendor: vendor, Endpoint: "https://x", APIKey: "k", Model: "m"}); err != nil {
		t.Fatal(err)
	}
}

func TestGeminiJudgesOnAGeminiVendorKey(t *testing.T) {
	geminiEngine(t, providers.VendorGoogle)
	if geminiFromEnv(nil) == nil {
		t.Fatal("a Gemini vendor key must judge on Gemini")
	}
}

func TestGeminiLeavesEveryOtherSetupAlone(t *testing.T) {
	geminiEngine(t, providers.VendorGoogle)
	if geminiFromEnv(readyStub(true)) != nil {
		t.Fatal("a decision provider already judging must win")
	}
	t.Setenv("GEMINI_BASE_URL", "https://gateway.example/v1")
	if geminiFromEnv(nil) != nil {
		t.Fatal("a company gateway is not known to serve /decisions")
	}
	t.Setenv("GEMINI_BASE_URL", "")
	if err := providers.SetRemoteEngine(providers.RemoteEngine{Name: providers.VendorEngineName, Vendor: providers.VendorAnthropic, Endpoint: "https://x", APIKey: "k", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if geminiFromEnv(nil) != nil {
		t.Fatal("another vendor's key must never reach Google")
	}
	_ = providers.ClearRemoteEngine()
	if geminiFromEnv(nil) != nil {
		t.Fatal("no engine: decisions stay off")
	}
}
