package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"memdoor/gateway/providers"
)

// /connect probes before it keeps: the model list, then one tiny call; the
// API shape is found by trying; a refused token fails in words with advice;
// --probe keeps nothing.
func TestConnectProbesThenKeeps(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CORP_TOKEN", "pat-ok")
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer pat-ok" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"code":"unauthorized","message":"bad token"}}`)
			return
		}
		switch r.URL.Path {
		case "/ai/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"corp-claude"},{"id":"corp-gpt"}]}`)
		case "/ai/v1/responses":
			// One object, not a stream: a gateway may answer so; the client
			// reads it as the final object.
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"r","model":"corp-claude","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	res := connectProvider(context.Background(), connectRequest{ID: "corp", Name: "Company AI gateway", Base: srv.URL + "/ai/", Key: "CORP_TOKEN", Probe: true})
	if !res.OK || res.API != providers.APIResponses || res.Models != 2 || res.Tested != "corp-claude" || res.Answer != "ok" || res.Saved {
		t.Fatalf("probe: %+v", res)
	}
	if len(calls) != 2 || calls[0] != "GET /ai/v1/models" || calls[1] != "POST /ai/v1/responses" {
		t.Fatalf("the list, then one call: %v", calls)
	}
	if _, ok := providers.FindProvider("corp"); ok {
		t.Fatal("--probe keeps nothing")
	}

	res = connectProvider(context.Background(), connectRequest{ID: "corp", Name: "Company AI gateway", Base: srv.URL + "/ai", Key: "CORP_TOKEN"})
	if !res.OK || !res.Saved {
		t.Fatalf("connect: %+v", res)
	}
	p, ok := providers.FindProvider("corp")
	if !ok || p.API != providers.APIResponses || p.Key != "pat-ok" || p.KeySource != "env CORP_TOKEN" {
		t.Fatalf("kept, with the key resolved from the env var's NAME: %+v", p)
	}
	home, _ := os.UserHomeDir()
	if b, err := os.ReadFile(home + "/.memdoor/providers.json"); err != nil || !strings.Contains(string(b), `"key": "CORP_TOKEN"`) || strings.Contains(string(b), "pat-ok") {
		t.Fatalf("the file holds the NAME, never the value: %s %v", b, err)
	}

	res = connectProvider(context.Background(), connectRequest{ID: "corp2", Base: srv.URL + "/ai", Key: "wrong-token"})
	if res.OK || !strings.Contains(res.Error, "401") || !strings.Contains(res.Advice, "refused the token") {
		t.Fatalf("a refused token, in words with advice: %+v", res)
	}
	res = connectProvider(context.Background(), connectRequest{ID: "Bad id!", Base: srv.URL, Key: "x"})
	if res.OK || !strings.Contains(res.Error, "short id") {
		t.Fatalf("id rule: %+v", res)
	}
	res = connectProvider(context.Background(), connectRequest{ID: "nowhere", Base: srv.URL + "/nothing", Key: "CORP_TOKEN", Model: "m"})
	if res.OK || res.Advice == "" {
		t.Fatalf("a wrong path gets advice: %+v", res)
	}
}

// /api/models groups by provider and keeps the flat list the picker reads
// at one row per id: two providers in front of the same catalogue (live
// 2026-10-02: OpenRouter and a custom entry on its base) show each model
// once in the picker and twice in the groups.
func TestModelsAreGroupedByProviderAndFlatOncePerID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, v := range []string{"AI_GATEWAY_BASE_URL", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(v, "")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"id":"shared-a"},{"id":"shared-b"}]}`)
	}))
	defer srv.Close()
	t.Setenv("OPENAI_API_KEY", "sk")
	t.Setenv("OPENAI_BASE_URL", srv.URL+"/v1")
	if err := providers.SaveProvider(providers.ProviderSpec{ID: "proxy", API: providers.APIChat, Base: srv.URL + "/v1", Key: "pat"}); err != nil {
		t.Fatal(err)
	}
	providers.ForgetModels("openai")
	providers.ForgetModels("proxy")
	out, ok := providerModelsResponse(context.Background(), "shared", 40)
	if !ok {
		t.Fatal("two connected providers")
	}
	flat := out["models"].([]providers.Model)
	if len(flat) != 2 || flat[0].ID != "shared-a" || flat[1].ID != "shared-b" {
		t.Fatalf("flat, once per id: %+v", flat)
	}
	counted := 0
	for _, g := range out["providers"].([]map[string]any) {
		if ms, ok := g["models"].([]providers.Model); ok {
			counted += len(ms)
		}
	}
	if counted != 4 {
		t.Fatalf("the groups keep both providers' lists: %d", counted)
	}
	// A query that names a provider is that provider's whole list.
	out, _ = providerModelsResponse(context.Background(), "proxy", 40)
	flat = out["models"].([]providers.Model)
	if len(flat) != 2 || flat[0].Provider != "proxy" || flat[1].Provider != "proxy" {
		t.Fatalf("by provider: %+v", flat)
	}
	for _, g := range out["providers"].([]map[string]any) {
		if g["id"] == "openai" && g["count"] != 2 {
			t.Fatalf("each group still says how many it lists: %v", g)
		}
	}
}

// The first model of a provider's list may be refused for itself (no host
// at the precision floor, an unknown id): the probe tries the next ones,
// and a refusal of every model still connects the key, since the provider
// answered about the model and not about us.
func TestConnectProbeOutlivesARefusedFirstModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var tested, hits []string
	refuseAll := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"int4-only"},{"id":"served"}]}`)
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Model string `json:"model"`
			}
			_ = json.Unmarshal(body, &req)
			tested = append(tested, req.Model)
			if req.Model == "int4-only" || refuseAll {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"error":{"message":"No endpoints found for the request with quantization: int8,fp8,fp16,bf16","code":404}}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"c","model":"served","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	res := connectProvider(context.Background(), connectRequest{ID: "or", Base: srv.URL + "/v1", Key: "sk-x", Probe: true})
	if !res.OK || res.Tested != "served" {
		t.Fatalf("the next model answers: %+v (tested %v, hits %v)", res, tested, hits)
	}
	refuseAll, tested = true, nil
	res = connectProvider(context.Background(), connectRequest{ID: "or", Base: srv.URL + "/v1", Key: "sk-x", Probe: true})
	if !res.OK || !strings.Contains(res.Answer, "the key and URL answered") || len(tested) < 2 {
		t.Fatalf("every model refused still proves the key: %+v (tested %v)", res, tested)
	}
}

// A local runtime (Ollama, llama.cpp, janis, LM Studio) is connected with no
// key (2026-10-25): the probe sends no Authorization header, the provider
// counts as connected, and providers.json records keyless, never a token.
func TestConnectALocalRuntimeNeedsNoKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"qwen3-8b"}]}`)
		case "/v1/systemone":
			_, _ = io.WriteString(w, `{"model":"qwen3-8b","answers":{"q":{"type":"noul","noul":0.9,"confidence":0.9}}}`)
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"id\":\"c\",\"model\":\"qwen3-8b\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\n")
			_, _ = io.WriteString(w, "data: {\"id\":\"c\",\"model\":\"qwen3-8b\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	res := connectProvider(context.Background(), connectRequest{ID: "local", Name: "Local runtime", API: providers.APIChat, Base: srv.URL + "/v1"})
	if res.OK || !strings.Contains(res.Error, "token or key") {
		t.Fatalf("without keyless a key is still required: %+v", res)
	}
	// A blank base finds the runtime among the default addresses.
	saved := localRuntimeBases
	localRuntimeBases = []string{"http://localhost:1/v1", srv.URL + "/v1"}
	defer func() { localRuntimeBases = saved }()
	res = connectProvider(context.Background(), connectRequest{ID: "local", Name: "Local runtime", API: providers.APIChat, Keyless: true})
	if !res.OK || !res.Saved || res.Tested != "qwen3-8b" || res.Answer != "ok" || !res.Decisions || res.Base != srv.URL+"/v1" {
		t.Fatalf("keyless connect on a blank base, decisions too: %+v", res)
	}
	localRuntimeBases = []string{"http://localhost:1/v1"}
	if r := connectProvider(context.Background(), connectRequest{ID: "none", Keyless: true}); r.OK || !strings.Contains(r.Error, "no runtime answers") || !strings.Contains(r.Advice, "janis serve") {
		t.Fatalf("nothing listening is said in words, with what to start: %+v", r)
	}
	if dp, ok := providers.DecisionProvider(); !ok || dp.ID != "local-decisions" || !dp.Keyless || dp.Key != "" || dp.Base != srv.URL {
		t.Fatalf("the runtime's decisions are the decision provider, keyless, on the runtime's origin: %+v %v", dp, ok)
	}
	for _, a := range auth {
		if a != "" {
			t.Fatalf("no Authorization header on a keyless runtime, got %q", a)
		}
	}
	p, ok := providers.FindProvider("local")
	if !ok || !p.Connected() || p.Key != "" || !p.Keyless || p.KeySource != providers.KeySourceNone {
		t.Fatalf("kept as connected with no key: %+v", p)
	}
	home, _ := os.UserHomeDir()
	if b, err := os.ReadFile(home + "/.memdoor/providers.json"); err != nil || !strings.Contains(string(b), `"keyless": true`) {
		t.Fatalf("the file says keyless: %s %v", b, err)
	}
}
