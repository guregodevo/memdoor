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

// LLM Gateway's list, as GET /v1/models answered on 2026-11-09, cut to the
// cases the reader decides on: a chat model with tools and reasoning, one
// without reasoning or a stated reply cap, a model switched off, an image
// model that lists tools, a realtime model that answers in audio too, the
// "custom" route with no window, and an embedding model.
const llmGatewayList = `{"data":[
 {"id":"glm-5.3-flash","name":"GLM-5.3 Flash","context_length":1048576,"max_output":128000,
  "architecture":{"input_modalities":["text"],"output_modalities":["text"]},
  "supported_parameters":["temperature","max_tokens","tools","tool_choice","reasoning"],
  "pricing":{"prompt":"0.000000088","completion":"0.00000025"},"free":false},
 {"id":"glm-4.5-air","name":"GLM-4.5 Air","context_length":128000,
  "architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},
  "supported_parameters":["temperature","tools","tool_choice"],
  "pricing":{"prompt":"0.2e-6","completion":"1.1e-6"},"free":false},
 {"id":"o1-old","name":"o1","context_length":200000,"max_output":100000,"deactivated_at":"2026-01-01T00:00:00.000Z",
  "architecture":{"input_modalities":["text"],"output_modalities":["text"]},
  "supported_parameters":["tools","reasoning"],"pricing":{"prompt":"15e-6","completion":"60e-6"}},
 {"id":"glm-image","name":"GLM Image","context_length":2000,"max_output":4096,
  "architecture":{"input_modalities":["text"],"output_modalities":["text","image"]},
  "supported_parameters":["tools","tool_choice"],"pricing":{"prompt":"0","completion":"0"}},
 {"id":"gpt-realtime","name":"GPT Realtime","context_length":32000,
  "architecture":{"input_modalities":["text","audio"],"output_modalities":["text","audio"]},
  "supported_parameters":["tools"],"pricing":{"prompt":"4e-6","completion":"16e-6"}},
 {"id":"custom","name":"Custom Model","context_length":0,
  "architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},
  "supported_parameters":["temperature","max_tokens"],"pricing":{"prompt":"0","completion":"0"}},
 {"id":"bge-m3","name":"BGE M3","context_length":8192,
  "architecture":{"input_modalities":["text"],"output_modalities":["embedding"]},
  "supported_parameters":[],"pricing":{"prompt":"0.01e-6","completion":"0"}}]}`

func llmGatewayServer(t *testing.T, auth *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if auth != nil {
			*auth = r.Header.Get("Authorization")
		}
		_, _ = io.WriteString(w, llmGatewayList)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The reader keeps the models a turn can run on: text out, tools, a window,
// not switched off.
func TestLLMGatewayListsChatModelsWithTools(t *testing.T) {
	var auth string
	srv := llmGatewayServer(t, &auth)
	p := Provider{ID: VendorLLMGateway, API: APIChat, Base: srv.URL + "/v1", Key: "llmgtwy_key"}
	list, err := readerFor(p).ListModels(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer llmgtwy_key" {
		t.Fatalf("the list is read on the key: %q", auth)
	}
	if len(list) != 2 || list[0].ID != "glm-4.5-air" && list[1].ID != "glm-4.5-air" {
		t.Fatalf("only chat models with tools that are on: %+v", list)
	}
	byID := map[string]Model{list[0].ID: list[0], list[1].ID: list[1]}
	glm, air := byID["glm-5.3-flash"], byID["glm-4.5-air"]
	if glm.Context != 1048576 || glm.MaxOutput != 128000 || !glm.Tools || !glm.Effort || !glm.Thinking || !nearly(glm.InPerM, 0.088) || !nearly(glm.OutPerM, 0.25) || glm.Currency != "" {
		t.Fatalf("glm-5.3-flash as its list states it: %+v", glm)
	}
	if air.Context != 128000 || air.MaxOutput != 0 || air.Effort || !nearly(air.InPerM, 0.2) || !nearly(air.OutPerM, 1.1) || strings.Join(air.Inputs, "+") != "text+image" {
		t.Fatalf("glm-4.5-air, its prices in exponent form: %+v", air)
	}
}

// Through the registry: LLMGATEWAY_API_KEY connects the built-in provider and
// every window is the provider's own.
func TestLLMGatewayFromTheEnvironment(t *testing.T) {
	clearProviderEnv(t)
	srv := llmGatewayServer(t, nil)
	if p, ok := FindProvider(VendorLLMGateway); !ok || p.Connected() || p.Base != llmGatewayDefaultBase {
		t.Fatalf("built in, not connected without a key: %+v %v", p, ok)
	}
	t.Setenv("LLMGATEWAY_API_KEY", "llmgtwy_key")
	t.Setenv("LLMGATEWAY_BASE_URL", srv.URL+"/v1")
	p, ok := FindProvider(VendorLLMGateway)
	if !ok || !p.Connected() || p.KeySource != "env LLMGATEWAY_API_KEY" || p.Name != "LLM Gateway" {
		t.Fatalf("connected from the environment: %+v", p)
	}
	list, err := ModelsOf(context.Background(), p)
	if err != nil || len(list) != 2 {
		t.Fatalf("models: %+v %v", list, err)
	}
	for _, m := range list {
		if m.ContextSource != ContextFromProvider || m.Provider != VendorLLMGateway {
			t.Fatalf("%s: the window from LLM Gateway itself: %+v", m.ID, m)
		}
	}
}

// LLMGATEWAY_API_KEY and MEMDOOR_MODEL alone run a gateway where LLM Gateway
// is the only host contacted. It comes after every other vendor key.
func TestLLMGatewayAsTheCompanysVendor(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MEMDOOR_VENDOR", "")
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "")
	t.Setenv("LLMGATEWAY_BASE_URL", "")
	t.Setenv("LLMGATEWAY_API_KEY", "llmgtwy_company")
	if _, ok, reason := VendorEngine(); ok || !strings.Contains(reason, "LLMGATEWAY_API_KEY") {
		t.Fatalf("without MEMDOOR_MODEL it says why: ok=%v reason=%q", ok, reason)
	}
	t.Setenv("MEMDOOR_MODEL", "glm-5.3-flash")
	re, ok, _ := VendorEngine()
	if !ok || re.Vendor != VendorLLMGateway || re.Endpoint != llmGatewayDefaultBase || re.Model != "glm-5.3-flash" || re.CtxLen != llmGatewayDefaultContext {
		t.Fatalf("llmgateway engine: %+v", re)
	}
	if got := providerOfEngine(&re); got != VendorLLMGateway {
		t.Fatalf("the engine names its provider: %q", got)
	}
	t.Setenv("EUROUTER_API_KEY", "eur_company")
	if re, _, _ := VendorEngine(); re.Vendor != VendorEURouter {
		t.Fatalf("an EUrouter key set before keeps answering: %+v", re)
	}
}

// A turn on LLM Gateway names the app by X-Source, the header its apps page
// reads, whether connected or the company's key.
func TestATurnOnLLMGatewayCarriesXSource(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("MEMDOOR_VENDOR", "")
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "")
	t.Setenv("MEMDOOR_ATTRIBUTION", "")
	t.Setenv("MEMDOOR_ATTRIBUTION_HEADERS", "")
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	t.Setenv("LLMGATEWAY_API_KEY", "llmgtwy_company")
	t.Setenv("LLMGATEWAY_BASE_URL", "")
	t.Setenv("MEMDOOR_MODEL", "glm-5.3-flash")
	fromEnv, ok, _ := VendorEngine()
	if !ok {
		t.Fatal("the vendor engine")
	}
	connected := Provider{ID: VendorLLMGateway, API: APIChat, Base: llmGatewayDefaultBase, Key: "llmgtwy_connected"}.Engine("glm-5.3-flash")
	for name, re := range map[string]RemoteEngine{"company key": fromEnv, "connected": connected} {
		c, isOAI := newVendorClient(context.Background(), &re, re.Model).(*oaiClient)
		if !isOAI {
			t.Fatalf("%s: LLM Gateway speaks chat/completions: %+v", name, re)
		}
		capture := &captureTransport{}
		c.httpClient = &http.Client{Transport: capture}
		if _, err := c.Messages().New(context.Background(), llm.MessageNewParams{MaxTokens: 8, Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}); err != nil {
			t.Fatal(err)
		}
		r := capture.req
		if r.URL.String() != llmGatewayDefaultBase+"/chat/completions" || r.Header.Get("X-Source") != "memdoor.ai" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer llmgtwy_") {
			t.Errorf("%s: %s, X-Source %q", name, r.URL, r.Header.Get("X-Source"))
		}
	}
}
