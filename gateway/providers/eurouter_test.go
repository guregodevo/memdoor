package providers

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/pkg/llm"
	"memdoor/pkg/price"
	sharedctx "memdoor/pkg/shared/context"
)

// EUrouter's list, as GET /api/v1/models answered on 2026-11-08, cut to the
// cases the reader decides on: a chat model with tools priced in dollars, one
// priced in euros, a chat model without tools, and a speech model.
const euRouterList = `{"object":"list","data":[
 {"id":"glm-5.3-flash","name":"GLM 5.3 Flash","context_length":1048576,
  "architecture":{"input_modalities":["text"]},"top_provider":{"context_length":1048576,"max_completion_tokens":131072},
  "supported_parameters":["max_tokens","tools","tool_choice","reasoning_effort","stream"],"supported_api_endpoints":["chat.completions","responses"],
  "pricing":{"prompt":"0.0000001","completion":"0.00000035","currency":"USD"}},
 {"id":"mistral-small-3.2-24b","name":"Mistral Small 3.2","context_length":131072,
  "architecture":{"input_modalities":["text","image"]},"top_provider":{"context_length":131072,"max_completion_tokens":131072},
  "supported_parameters":["tools","tool_choice"],"supported_api_endpoints":["chat.completions"],
  "pricing":{"prompt":"0.00000009","completion":"0.00000028","currency":"EUR"}},
 {"id":"plain-chat","context_length":32000,"supported_parameters":["max_tokens"],"supported_api_endpoints":["chat.completions"],
  "pricing":{"prompt":"0","completion":"0","currency":"USD"}},
 {"id":"pianissimo-sv","context_length":1,"supported_parameters":["language","stream"],"supported_api_endpoints":["audio.transcriptions"],
  "pricing":{"prompt":"0","completion":"0","currency":"EUR"}}]}`

func euRouterServer(t *testing.T, auth *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if auth != nil {
			*auth = r.Header.Get("Authorization")
		}
		_, _ = io.WriteString(w, euRouterList)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func nearly(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// The reader keeps the models a turn can run on, each price in the currency
// EUrouter states it in: dollars blank, as every other provider's.
func TestEURouterListsChatModelsWithToolsInTheirOwnCurrency(t *testing.T) {
	var auth string
	srv := euRouterServer(t, &auth)
	p := Provider{ID: VendorEURouter, API: APIChat, Base: srv.URL + "/api/v1", Key: "eur_key"}
	list, err := readerFor(p).ListModels(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer eur_key" {
		t.Fatalf("the list is read on the key: %q", auth)
	}
	if len(list) != 2 || list[0].ID != "glm-5.3-flash" || list[1].ID != "mistral-small-3.2-24b" {
		t.Fatalf("only chat models with tools: %+v", list)
	}
	glm, mistral := list[0], list[1]
	if glm.Context != 1048576 || glm.MaxOutput != 131072 || !glm.Tools || !glm.Effort || !glm.Thinking || !nearly(glm.InPerM, 0.1) || !nearly(glm.OutPerM, 0.35) || glm.Currency != "" {
		t.Fatalf("glm-5.3-flash as its list states it, in dollars: %+v", glm)
	}
	if !nearly(mistral.InPerM, 0.09) || !nearly(mistral.OutPerM, 0.28) || mistral.Currency != price.EUR || mistral.Effort || strings.Join(mistral.Inputs, "+") != "text+image" {
		t.Fatalf("mistral-small in euros: %+v", mistral)
	}
	if b, _ := json.Marshal(glm); strings.Contains(string(b), "currency") {
		t.Fatalf("a dollar price serialises as before: %s", b)
	}
}

// Through the registry: EUROUTER_API_KEY connects the built-in provider, every
// window is the provider's own, and a euro price reaches /api/models as euros.
func TestEURouterFromTheEnvironment(t *testing.T) {
	clearProviderEnv(t)
	srv := euRouterServer(t, nil)
	if p, ok := FindProvider(VendorEURouter); !ok || p.Connected() || p.Base != euRouterDefaultBase {
		t.Fatalf("built in, not connected without a key: %+v %v", p, ok)
	}
	t.Setenv("EUROUTER_API_KEY", "eur_key")
	t.Setenv("EUROUTER_BASE_URL", srv.URL+"/api/v1")
	p, ok := FindProvider(VendorEURouter)
	if !ok || !p.Connected() || p.KeySource != "env EUROUTER_API_KEY" || p.Name != "EUrouter" {
		t.Fatalf("connected from the environment: %+v", p)
	}
	list, err := ModelsOf(context.Background(), p)
	if err != nil || len(list) != 2 {
		t.Fatalf("models: %+v %v", list, err)
	}
	for _, m := range list {
		if m.ContextSource != ContextFromProvider || m.Provider != VendorEURouter {
			t.Fatalf("%s: the window from EUrouter itself: %+v", m.ID, m)
		}
	}
	if list[1].Currency != price.EUR || !nearly(list[1].InPerM, 0.09) {
		t.Fatalf("a euro price kept in euros: %+v", list[1])
	}
}

// A company that keeps its models in the EU sets EUROUTER_API_KEY and
// MEMDOOR_MODEL: EUrouter answers and nothing else is contacted. A key
// already there keeps its place: EUrouter comes after every other vendor.
func TestEURouterAsTheCompanysVendor(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MEMDOOR_VENDOR", "")
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "")
	t.Setenv("EUROUTER_BASE_URL", "")
	t.Setenv("EUROUTER_API_KEY", "eur_company")
	if _, ok, reason := VendorEngine(); ok || !strings.Contains(reason, "EUROUTER_API_KEY") {
		t.Fatalf("without MEMDOOR_MODEL it says why: ok=%v reason=%q", ok, reason)
	}
	t.Setenv("MEMDOOR_MODEL", "glm-5.3-flash")
	re, ok, _ := VendorEngine()
	if !ok || re.Vendor != VendorEURouter || re.Endpoint != euRouterDefaultBase || re.Model != "glm-5.3-flash" || re.CtxLen != euRouterDefaultContext {
		t.Fatalf("eurouter engine: %+v", re)
	}
	if got := providerOfEngine(&re); got != VendorEURouter {
		t.Fatalf("the engine names its provider: %q", got)
	}
	t.Setenv("GROQ_API_KEY", "gsk_company")
	if re, _, _ := VendorEngine(); re.Vendor != VendorGroq {
		t.Fatalf("a Groq key set before keeps answering: %+v", re)
	}
}

// captureTransport answers a chat request without the network and keeps it.
type captureTransport struct{ req *http.Request }

func (c *captureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.req = r
	body := "data: " + `{"choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}` + "\n\ndata: [DONE]\n\n"
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

// A turn on EUrouter names the app the way EUrouter's rankings read it
// (HTTP-Referer + X-EUrouter-Title) and carries the company's attribution
// fields, whether EUrouter was connected with `memdoor connect` or is the
// company's vendor key. MEMDOOR_OPENROUTER_ATTRIBUTION=off drops the app
// headers and keeps the company's.
func TestATurnOnEURouterCarriesTheAttribution(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MEMDOOR_VENDOR", "")
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "")
	t.Setenv("MEMDOOR_ATTRIBUTION", "user=dev@corp.example;team=eu-data")
	t.Setenv("MEMDOOR_ATTRIBUTION_HEADERS", "")
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	t.Setenv("EUROUTER_API_KEY", "eur_company")
	t.Setenv("EUROUTER_BASE_URL", "")
	t.Setenv("MEMDOOR_MODEL", "deepseek-v4.1-flash-1017")
	fromEnv, ok, _ := VendorEngine()
	if !ok {
		t.Fatal("the vendor engine")
	}
	connected := Provider{ID: VendorEURouter, API: APIChat, Base: euRouterDefaultBase, Key: "eur_connected"}.Engine("deepseek-v4.1-flash-1017")
	send := func(re RemoteEngine) *http.Request {
		t.Helper()
		c, isOAI := newVendorClient(context.Background(), &re, re.Model).(*oaiClient)
		if !isOAI {
			t.Fatalf("EUrouter speaks chat/completions: %+v", re)
		}
		capture := &captureTransport{}
		c.httpClient = &http.Client{Transport: capture}
		ctx := context.WithValue(context.Background(), sharedctx.SessionIDKey, "workspace:eu:channel:c1")
		if _, err := c.Messages().New(ctx, llm.MessageNewParams{MaxTokens: 8, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}); err != nil {
			t.Fatal(err)
		}
		return capture.req
	}
	for name, re := range map[string]RemoteEngine{"company key": fromEnv, "connected": connected} {
		r := send(re)
		want := map[string]string{
			"HTTP-Referer":      "https://memdoor.ai",
			"X-EUrouter-Title":  "Memdoor",
			"X-Memdoor-User":    "dev@corp.example",
			"X-Memdoor-Team":    "eu-data",
			"X-Memdoor-Session": "workspace:eu:channel:c1",
		}
		for h, v := range want {
			if got := r.Header.Get(h); got != v {
				t.Errorf("%s: %s = %q, want %q", name, h, got, v)
			}
		}
		if r.URL.String() != euRouterDefaultBase+"/chat/completions" || !strings.HasPrefix(r.Header.Get("User-Agent"), "Memdoor/") || r.Header.Get("X-OpenRouter-Categories") != "" {
			t.Errorf("%s: %s, User-Agent %q, OpenRouter's category %q", name, r.URL, r.Header.Get("User-Agent"), r.Header.Get("X-OpenRouter-Categories"))
		}
	}
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "off")
	r := send(connected)
	if r.Header.Get("X-EUrouter-Title") != "" || r.Header.Get("HTTP-Referer") != "" || r.Header.Get("X-Memdoor-Team") != "eu-data" {
		t.Errorf("opted out: the app headers go, the company's stay: %v", r.Header)
	}
}
