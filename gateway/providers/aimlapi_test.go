package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

// AI/ML API's list (GET /v1/models?include=all, shape of 2026-11-07), cut to
// the cases the reader decides on: chat with no tools, chat with tools and no
// name, an image model and a responses route that list tools, and the fast
// model with reasoning, a reply cap and every kind of price unit.
const aimlapiList = `{"data":[
 {"id":"a/no-tools","type":"openai/chat-completions","info":{"name":"No tools","contextLength":1000},"capabilities":["streaming"]},
 {"id":"a/first","type":"openai/chat-completions","info":{"contextLength":2000},"capabilities":["tools"]},
 {"id":"a/picture","type":"openai/image-generations","info":{"name":"Picture"},"capabilities":["tools"]},
 {"id":"a/responses","type":"openai/responses/submit","info":{"name":"Responses"},"capabilities":["tools"]},
 {"id":"z-ai/glm-5.3-flash","type":"openai/chat-completions",
  "info":{"name":"GLM 5.3 Flash","contextLength":1048576,"outputMax":131072},
  "modalities":{"input":["image","text"]},"capabilities":["reasoning","tools"],
  "pricing":{"units":[
    {"name":"token","content":"text","origin":"provided","phase":"inference","price":0.15,"per":1000000},
    {"name":"token","content":"text","origin":"generated","phase":"inference","price":0.5,"per":1000000},
    {"name":"token","content":"text","origin":"generated","phase":"reasoning","price":9,"per":1000000},
    {"name":"token","content":"text","origin":"cached","phase":"inference","price":0.03,"per":1000000},
    {"name":"call","content":"structured","origin":"generated","phase":"inference","price":7,"per":1}]}}]}`

func aimlapiServer(t *testing.T, query *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if query != nil {
			*query = r.URL.RawQuery
		}
		_, _ = io.WriteString(w, aimlapiList)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The catalogue lists every kind of model together: chat with tools is kept,
// the fast model leads, and the figures come from its own fields.
func TestAIMLAPIReaderKeepsToolCapableChat(t *testing.T) {
	var query string
	srv := aimlapiServer(t, &query)
	p := Provider{ID: VendorAIMLAPI, API: APIChat, Base: srv.URL + "/v1", Key: "k"}
	list, err := readerFor(p).ListModels(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if query != "include=all" {
		t.Fatalf("capabilities and prices need include=all, asked %q", query)
	}
	if len(list) != 2 || list[0].ID != aimlapiFastModel || list[1].ID != "a/first" {
		t.Fatalf("chat with tools, the fast model first: %+v", list)
	}
	m := list[0]
	if m.Name != "GLM 5.3 Flash" || m.Context != 1048576 || m.MaxOutput != 131072 || !m.Tools || !m.Thinking || len(m.Inputs) != 2 || m.InPerM != 0.15 || m.OutPerM != 0.5 {
		t.Fatalf("the model's own figures: %+v", m)
	}
	if list[1].Name != "a/first" || list[1].Thinking {
		t.Fatalf("no name falls back to the id, no reasoning stays off: %+v", list[1])
	}
}

// Through the registry: AIMLAPI_API_KEY connects the built-in provider, listed
// after Vercel so no existing /connect number moves, and every window is the
// provider's own.
func TestAIMLAPIFromTheEnvironment(t *testing.T) {
	clearProviderEnv(t)
	srv := aimlapiServer(t, nil)
	if p, ok := FindProvider(VendorAIMLAPI); !ok || p.Connected() || p.Base != aimlapiDefaultBase || p.API != APIChat || !p.BuiltIn {
		t.Fatalf("built in, not connected without a key: %+v %v", p, ok)
	}
	at := map[string]int{}
	for i, p := range Providers() {
		at[p.ID] = i
	}
	if at[VendorAIMLAPI] != at[VendorVercel]+1 {
		t.Fatalf("right after Vercel: %v", at)
	}
	t.Setenv("AIMLAPI_API_KEY", "aiml-key")
	t.Setenv("AIMLAPI_BASE_URL", srv.URL+"/v1")
	p, ok := FindProvider(VendorAIMLAPI)
	if !ok || !p.Connected() || p.KeySource != "env AIMLAPI_API_KEY" || p.Name != "AI/ML API" {
		t.Fatalf("connected from the environment: %+v", p)
	}
	list, err := ModelsOf(context.Background(), p)
	if err != nil || len(list) != 2 {
		t.Fatalf("models: %+v %v", list, err)
	}
	for _, m := range list {
		if m.ContextSource != ContextFromProvider || m.Provider != VendorAIMLAPI {
			t.Fatalf("%s: the window from AI/ML API itself: %+v", m.ID, m)
		}
	}
}

// AIMLAPI_API_KEY and MEMDOOR_MODEL alone run a gateway where AI/ML API is
// the only host contacted. It comes after every other vendor key.
func TestAIMLAPIAsTheCompanysVendor(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MEMDOOR_VENDOR", "")
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "")
	t.Setenv("AIMLAPI_BASE_URL", "")
	t.Setenv("AIMLAPI_API_KEY", "aiml-company")
	if _, ok, reason := VendorEngine(); ok || !strings.Contains(reason, "AIMLAPI_API_KEY") {
		t.Fatalf("without MEMDOOR_MODEL it says why: ok=%v reason=%q", ok, reason)
	}
	t.Setenv("MEMDOOR_MODEL", aimlapiFastModel)
	re, ok, _ := VendorEngine()
	if !ok || re.Vendor != VendorAIMLAPI || re.Endpoint != aimlapiDefaultBase || re.Model != aimlapiFastModel || re.CtxLen != aimlapiDefaultContext {
		t.Fatalf("aimlapi engine: %+v", re)
	}
	if got := providerOfEngine(&re); got != VendorAIMLAPI {
		t.Fatalf("the engine names its provider: %q", got)
	}
	t.Setenv("AIMLAPI_BASE_URL", "https://proxy.example/v1/")
	if re, _, _ := VendorEngine(); re.Endpoint != "https://proxy.example/v1" {
		t.Fatalf("AIMLAPI_BASE_URL replaces the base: %+v", re)
	}
	t.Setenv("LLMGATEWAY_API_KEY", "llmgtwy_company")
	if re, _, _ := VendorEngine(); re.Vendor != VendorLLMGateway {
		t.Fatalf("an LLM Gateway key set before keeps answering: %+v", re)
	}
}

// A turn on AI/ML API names the app by X-AIMLAPI-Source, whether connected or
// the company's key, and carries no other host's app header.
func TestATurnOnAIMLAPICarriesItsSource(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MEMDOOR_VENDOR", "")
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "")
	t.Setenv("MEMDOOR_ATTRIBUTION", "")
	t.Setenv("MEMDOOR_ATTRIBUTION_HEADERS", "")
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	t.Setenv("AIMLAPI_API_KEY", "aiml-company")
	t.Setenv("AIMLAPI_BASE_URL", "")
	t.Setenv("MEMDOOR_MODEL", aimlapiFastModel)
	fromEnv, ok, _ := VendorEngine()
	if !ok {
		t.Fatal("the vendor engine")
	}
	connected := Provider{ID: VendorAIMLAPI, API: APIChat, Base: aimlapiDefaultBase, Key: "aiml-connected"}.Engine(aimlapiFastModel)
	for name, re := range map[string]RemoteEngine{"company key": fromEnv, "connected": connected} {
		c, isOAI := newVendorClient(context.Background(), &re, re.Model).(*oaiClient)
		if !isOAI {
			t.Fatalf("%s: AI/ML API speaks chat/completions: %+v", name, re)
		}
		capture := &captureTransport{}
		c.httpClient = &http.Client{Transport: capture}
		if _, err := c.Messages().New(context.Background(), llm.MessageNewParams{MaxTokens: 8, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}); err != nil {
			t.Fatal(err)
		}
		r := capture.req
		if r.URL.String() != aimlapiDefaultBase+"/chat/completions" || r.Header.Get("X-AIMLAPI-Source") != "agent/memdoor" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer aiml-") {
			t.Errorf("%s: %s, X-AIMLAPI-Source %q", name, r.URL, r.Header.Get("X-AIMLAPI-Source"))
		}
		if r.Header.Get("X-Title") != "" || r.Header.Get("X-Source") != "" {
			t.Errorf("%s: another host's app header: %v", name, r.Header)
		}
	}
}
