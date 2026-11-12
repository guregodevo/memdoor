package cmd

import (
	"bytes"
	"encoding/json"
	"memdoor/internal/testenv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/cmd/tui/ui"
	"memdoor/gateway/providers"
	"memdoor/pkg/modelkey"
)

// fakeConnectGateway answers /api/providers/connect with answer(req) and
// records every request.
func fakeConnectGateway(t *testing.T, answer func(ui.ConnectRequest) ui.ConnectResult) *[]ui.ConnectRequest {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	var got []ui.ConnectRequest
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/providers/connect" {
			http.NotFound(w, r)
			return
		}
		var req ui.ConnectRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		got = append(got, req)
		_ = json.NewEncoder(w).Encode(answer(req))
	}))
	t.Cleanup(gw.Close)
	gatewayAddr = gw.URL
	return &got
}

// A key in this shell goes by its NAME, so nothing is copied; when the
// gateway cannot read that name, the value follows. Never the value first
// (the mutation check: send k.Value up front and the first request carries it).
func TestConnectFoundSendsTheNameThenTheValue(t *testing.T) {
	value := "sk-or-v1-" + strings.Repeat("ab", 32)
	got := fakeConnectGateway(t, func(req ui.ConnectRequest) ui.ConnectResult {
		if req.Key == "MY_OR_KEY" {
			return ui.ConnectResult{Error: "MY_OR_KEY" + providers.KeyNameNotSet + " (…)"}
		}
		return ui.ConnectResult{OK: true, ID: req.ID}
	})
	res, err := connectFound(modelkey.Found{Providers: []modelkey.Provider{modelkey.ProviderOpenRouter}, Name: "MY_OR_KEY", Value: value})
	if err != nil || !res.OK {
		t.Fatalf("not connected: %v %+v", err, res)
	}
	if len(*got) != 2 || (*got)[0].Key != "MY_OR_KEY" || (*got)[1].Key != value {
		t.Fatalf("want the name, then the value; got %+v", *got)
	}
	if r := (*got)[0]; r.ID != "openrouter" || r.API != "openrouter" || r.Base != "https://openrouter.ai/api/v1" {
		t.Fatalf("the request is the openrouter kind's: %+v", r)
	}
}

// A key read from a .env file is not in the gateway's environment: the
// value goes at once. A key whose shape fits two providers tries each.
func TestConnectFoundFileKeyAndTwoProviders(t *testing.T) {
	value := "sk-" + strings.Repeat("0f", 16)
	got := fakeConnectGateway(t, func(req ui.ConnectRequest) ui.ConnectResult {
		if req.ID == "openai" {
			return ui.ConnectResult{OK: true, ID: "openai"}
		}
		return ui.ConnectResult{Error: "401"}
	})
	res, err := connectFound(modelkey.Found{Providers: []modelkey.Provider{modelkey.ProviderDeepSeek, modelkey.ProviderOpenAI}, Name: "KEY", File: ".env", Value: value})
	if err != nil || !res.OK || res.ID != "openai" {
		t.Fatalf("want openai after deepseek refused: %v %+v", err, res)
	}
	if len(*got) != 2 || (*got)[0].ID != "deepseek" || (*got)[0].Key != value || (*got)[1].Key != value {
		t.Fatalf("want deepseek then openai, each with the value: %+v", *got)
	}
}

// Every provider a key can be found for is one `memdoor connect` knows.
func TestEveryFoundProviderIsAConnectKind(t *testing.T) {
	for _, p := range []modelkey.Provider{modelkey.ProviderOpenRouter, modelkey.ProviderAnthropic, modelkey.ProviderOpenAI, modelkey.ProviderGemini, modelkey.ProviderGroq, modelkey.ProviderXAI, modelkey.ProviderDeepSeek, modelkey.ProviderBaseten, modelkey.ProviderTypeSafe} {
		if connectKindOf(p) == nil {
			t.Errorf("%s is no connect kind", p)
		}
	}
	for name, p := range modelkey.ProviderNames() {
		if kind := connectKindOf(p); kind == nil || (kind.KeyVar != "" && kind.KeyVar != name && name != "MEMDOOR_SYSTEMONE_API_KEY") {
			t.Errorf("%s names %s, whose kind reads %+v", name, p, kind)
		}
	}
}

func TestPickFound(t *testing.T) {
	for answer, want := range map[string]int{"": 0, "y": 0, "YES": 0, "2": 1, "3": 2} {
		if got, ok := pickFound(answer, 3); !ok || got != want {
			t.Errorf("pickFound(%q) = %d %v, want %d", answer, got, ok, want)
		}
	}
	for _, answer := range []string{"n", "no", "0", "4", "x"} {
		if _, ok := pickFound(answer, 3); ok {
			t.Errorf("pickFound(%q) connects; it must not", answer)
		}
	}
}

// The keys the gateway already reads from its environment under the same
// variable are set aside (OpenRouter's two names are one); a key under
// another name, a provider not connected, or a gateway elsewhere is offered
// (the mutation check: compare the names exactly and the OpenRouter key under
// OPENROUTER_API_KEY is offered while the gateway reads OPEN_ROUTER_API_KEY).
func TestKeysTheGatewayAlreadyUsesAreSetAside(t *testing.T) {
	saved, savedAddr := gatewayKeySources, gatewayAddr
	t.Cleanup(func() { gatewayKeySources, gatewayAddr = saved, savedAddr })
	gatewayKeySources = func() map[string]string {
		return map[string]string{"openrouter": "env OPEN_ROUTER_API_KEY", "groq": "env GROQ_API_KEY", "anthropic": "env ANTHROPIC_API_KEY", "openai": "kept in providers.json"}
	}
	found := []modelkey.Found{
		{Providers: []modelkey.Provider{modelkey.ProviderOpenRouter}, Name: "OPENROUTER_API_KEY", File: ".envrc"},
		{Providers: []modelkey.Provider{modelkey.ProviderGroq}, Name: "GROQ_API_KEY"},
		{Providers: []modelkey.Provider{modelkey.ProviderAnthropic}, Name: "MY_ANTHROPIC"},
		{Providers: []modelkey.Provider{modelkey.ProviderOpenAI}, Name: "OPENAI_API_KEY"},
		{Providers: []modelkey.Provider{modelkey.ProviderDeepSeek}, Name: "DEEPSEEK_API_KEY"},
	}
	gatewayAddr = "http://localhost:18789"
	offer, inUse := splitInUse(found)
	if len(inUse) != 2 || inUse[0].Name != "OPENROUTER_API_KEY" || inUse[1].Name != "GROQ_API_KEY" {
		t.Fatalf("in use: %+v", inUse)
	}
	if len(offer) != 3 {
		t.Fatalf("offered: %+v", offer)
	}
	gatewayAddr = "http://box.example:18789"
	if offer, inUse := splitInUse(found); len(offer) != 5 || inUse != nil {
		t.Fatalf("a gateway elsewhere: every key offered, got %d offered, %d in use", len(offer), len(inUse))
	}
}

// The list: the keys that would change something, then the sign-ins under
// their own heading (never buried: live 2026-10-31 Sign in with OpenRouter
// was 6th, under five keys already in use), then the kinds that take a key;
// numbers right-aligned, a long name's hint on its own line; with no key
// found, 1 (Enter) is Sign in with OpenRouter.
func TestTheConnectListPutsTheSignInsBeforeTheKeyedKinds(t *testing.T) {
	var out bytes.Buffer
	found := []modelkey.Found{{Providers: []modelkey.Provider{modelkey.ProviderDeepSeek, modelkey.ProviderOpenAI}, Named: true, Name: "DEEPSEEK_API_KEY", File: ".env"}}
	inUse := []modelkey.Found{{Providers: []modelkey.Provider{modelkey.ProviderOpenRouter}, Name: "OPENROUTER_API_KEY"}}
	menu := connectMenu(found, inUse, &out)
	s := out.String()
	if menu[0].found == nil || menu[1].kind == nil || menu[1].kind.ID != ui.KindOpenRouterLogin || menu[2].kind.ID != ui.KindChatGPT {
		t.Fatalf("order: found, then the sign-ins:\n%s", s)
	}
	for _, want := range []string{"Already connected from your environment: OpenRouter.", "Found on this machine:", "   1. DeepSeek             .env: DEEPSEEK_API_KEY", "Sign in, no key:", "   2. Sign in with OpenRouter", "Or a provider's key:"} {
		if !strings.Contains(s, want) {
			t.Fatalf("lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "DeepSeek or OpenAI") || strings.Contains(s, ". OpenRouter           OPENROUTER_API_KEY") {
		t.Fatalf("a named key is its provider's alone, and a key in use is not numbered:\n%s", s)
	}
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, "Local runtime") && strings.Contains(l, "blank finds") {
			t.Fatalf("a long name's hint goes on the next line: %q", l)
		}
	}
	out.Reset()
	if menu := connectMenu(nil, nil, &out); menu[0].kind == nil || menu[0].kind.ID != ui.KindOpenRouterLogin || len(menu) != len(ui.ConnectKinds) {
		t.Fatalf("no key found: 1 is Sign in with OpenRouter, every kind listed once:\n%s", out.String())
	}
}
