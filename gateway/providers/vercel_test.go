package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Vercel AI Gateway's list, as GET /v1/models answered on 2026-11-09, cut to
// the cases the reader decides on: a language model with tools and an effort
// option, one with a reasoning toggle only, one past its deprecated_at, a
// language model without tools, and an embedding model.
const vercelList = `{"object":"list","data":[
 {"id":"alibaba/qwen-3-14b","name":"Qwen3-14B","type":"language","context_window":40960,"max_tokens":16384,
  "modalities":{"input":["text"],"output":["text"]},
  "supported_parameters":["max_tokens","tools","tool_choice","reasoning","include_reasoning"],
  "reasoning_options":[{"type":"toggle"}],
  "pricing":{"input":"0.00000012","output":"0.00000024"}},
 {"id":"openai/gpt-6-mini","name":"GPT-6 mini","type":"language","context_window":400000,"max_tokens":128000,
  "modalities":{"input":["text","image"],"output":["text"]},
  "supported_parameters":["max_tokens","tools","tool_choice","reasoning"],
  "reasoning_options":[{"type":"effort","values":["low","medium","high"]}],
  "pricing":{"input":"0.00000025","output":"0.000002","input_cache_read":"0.000000025"}},
 {"id":"bytedance/seed-1.6","name":"Seed 1.6","type":"language","context_window":256000,"max_tokens":32000,
  "supported_parameters":["tools"],"deprecated_at":1700000000000,
  "pricing":{"input":"0.00000025","output":"0.000002"}},
 {"id":"some/plain","name":"Plain","type":"language","context_window":8192,"supported_parameters":["max_tokens"],
  "pricing":{"input":"0","output":"0"}},
 {"id":"openai/text-embedding-3-small","name":"Embeddings","type":"embedding","supported_parameters":["tools"],
  "pricing":{"input":"0.00000002"}}]}`

func vercelServer(t *testing.T, auth *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if auth != nil {
			*auth = r.Header.Get("Authorization")
		}
		_, _ = io.WriteString(w, vercelList)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The reader keeps the language models that call tools and are not past
// their deprecation, with the window, cap and price the list states.
func TestVercelListsLanguageModelsWithTools(t *testing.T) {
	var auth string
	srv := vercelServer(t, &auth)
	p := Provider{ID: VendorVercel, API: APIChat, Base: srv.URL + "/v1", Key: "vck_key"}
	list, err := readerFor(p).ListModels(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer vck_key" {
		t.Fatalf("the list is read on the key: %q", auth)
	}
	if len(list) != 2 || list[0].ID != "alibaba/qwen-3-14b" || list[1].ID != "openai/gpt-6-mini" {
		t.Fatalf("only language models with tools, none deprecated: %+v", list)
	}
	qwen, gpt := list[0], list[1]
	if qwen.Context != 40960 || qwen.MaxOutput != 16384 || !qwen.Tools || !qwen.Thinking || qwen.Effort || !nearly(qwen.InPerM, 0.12) || !nearly(qwen.OutPerM, 0.24) {
		t.Fatalf("qwen-3-14b as the list states it, a toggle and no effort: %+v", qwen)
	}
	if !gpt.Effort || !gpt.Thinking || strings.Join(gpt.Inputs, "+") != "text+image" || !nearly(gpt.OutPerM, 2) {
		t.Fatalf("gpt-6-mini with an effort option: %+v", gpt)
	}
}

// AI_GATEWAY_API_KEY (Vercel's name) or VERCEL_API_KEY connects the built-in
// provider; VERCEL_BASE_URL replaces the base, never AI_GATEWAY_BASE_URL.
func TestVercelFromTheEnvironment(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("VERCEL_BASE_URL", "")
	srv := vercelServer(t, nil)
	if p, ok := FindProvider(VendorVercel); !ok || p.Connected() || p.Base != vercelDefaultBase {
		t.Fatalf("built in, not connected without a key: %+v %v", p, ok)
	}
	t.Setenv("VERCEL_API_KEY", "a-vercel-deploy-token")
	if p, _ := FindProvider(VendorVercel); p.Connected() || p.Key != "" {
		t.Fatalf("a deploy token under VERCEL_API_KEY is no model key: %+v", p)
	}
	alias := "vck_" + strings.Repeat("Ab", 28)
	t.Setenv("VERCEL_API_KEY", alias)
	if p, _ := FindProvider(VendorVercel); !p.Connected() || p.Key != alias || p.KeySource != "env VERCEL_API_KEY" {
		t.Fatalf("an AI Gateway key under VERCEL_API_KEY connects it: %+v", p)
	}
	t.Setenv("AI_GATEWAY_API_KEY", "vck_docs")
	t.Setenv("VERCEL_BASE_URL", srv.URL+"/v1")
	p, ok := FindProvider(VendorVercel)
	if !ok || p.Key != "vck_docs" || p.KeySource != "env AI_GATEWAY_API_KEY" || p.Name != "Vercel AI Gateway" {
		t.Fatalf("Vercel's own name comes first: %+v", p)
	}
	list, err := ModelsOf(context.Background(), p)
	if err != nil || len(list) != 2 {
		t.Fatalf("models: %+v %v", list, err)
	}
	for _, m := range list {
		if m.ContextSource != ContextFromProvider || m.Provider != VendorVercel {
			t.Fatalf("%s: the window from Vercel itself: %+v", m.ID, m)
		}
	}
}

// AI_GATEWAY_API_KEY + MEMDOOR_MODEL alone run on Vercel. With the company's
// AI_GATEWAY_BASE_URL in the same shell, the company gateway answers on its
// own token, and Vercel's key is never taken for that token.
func TestTheCompanyGatewayAndVercelReadNeitherOthersVariables(t *testing.T) {
	clearProviderEnv(t)
	for _, v := range []string{"MEMDOOR_VENDOR", "MEMDOOR_MODEL_CONTEXT", "VERCEL_BASE_URL", "ACME_AI_GATEWAY_TOKEN"} {
		t.Setenv(v, "")
	}
	t.Setenv("AI_GATEWAY_API_KEY", "vck_company")
	t.Setenv("MEMDOOR_MODEL", "alibaba/qwen-3-14b")
	re, ok, _ := VendorEngine()
	if !ok || re.Vendor != VendorVercel || re.Endpoint != vercelDefaultBase || re.APIKey != "vck_company" || re.CtxLen != vercelDefaultContext {
		t.Fatalf("vercel engine: %+v", re)
	}
	if got := providerOfEngine(&re); got != VendorVercel {
		t.Fatalf("the engine names its provider: %q", got)
	}

	t.Setenv("AI_GATEWAY_BASE_URL", "https://ai-gateway.corp.example/ai")
	if _, ok, reason := VendorEngine(); ok || !strings.Contains(reason, "AI_GATEWAY_API_KEY is Vercel AI Gateway's key") {
		t.Fatalf("no company token: Vercel's key is not taken for it, and it says so: ok=%v reason=%q", ok, reason)
	}
	if p, _ := FindProvider("gateway"); p.Connected() || p.Key != "" {
		t.Fatalf("the company gateway holds no Vercel key: %+v", p)
	}

	t.Setenv("AI_GATEWAY_TOKEN", "corp-token")
	re, ok, _ = VendorEngine()
	if !ok || re.Vendor != VendorGateway || re.Endpoint != "https://ai-gateway.corp.example/ai" || re.APIKey != "corp-token" {
		t.Fatalf("the company gateway comes first, on its own token: %+v", re)
	}
	if p, _ := FindProvider(VendorVercel); p.Base != vercelDefaultBase || p.Key != "vck_company" {
		t.Fatalf("Vercel keeps its own base and key: %+v", p)
	}
}
