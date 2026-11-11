package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	out, ok := providerModelsResponse(context.Background(), "shared", "", 40)
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
	// q matches model ids and names, never a provider's name: "proxy"
	// finds no model, as the proxy lists shared-a and shared-b.
	out, _ = providerModelsResponse(context.Background(), "proxy", "", 40)
	flat = out["models"].([]providers.Model)
	if len(flat) != 0 {
		t.Fatalf("a query equal to a provider id searches models, not the provider: %+v", flat)
	}
	// provider= with no q is that provider's whole list, past the limit (a
	// bare /model-search picks a provider, then its models).
	out, _ = providerModelsResponse(context.Background(), "", "proxy", 1)
	flat = out["models"].([]providers.Model)
	if len(flat) != 2 || flat[0].Provider != "proxy" || flat[1].Provider != "proxy" {
		t.Fatalf("by provider: %+v", flat)
	}
	if status, _ := scopedProviderFailure(out, "proxy"); status != 0 {
		t.Fatalf("a connected provider with a list is no failure: %d", status)
	}
	if status, msg := scopedProviderFailure(out, "nobody"); status != http.StatusNotFound || !strings.Contains(msg, "memdoor connect nobody") {
		t.Fatalf("an unknown provider says how to connect it: %d %q", status, msg)
	}
	// Without provider=, an empty q is still held to the limit.
	out, _ = providerModelsResponse(context.Background(), "", "", 1)
	for _, g := range out["providers"].([]map[string]any) {
		if ms, ok := g["models"].([]providers.Model); ok && len(ms) > 1 {
			t.Fatalf("an empty q keeps the limit: %v", g)
		}
	}
	for _, g := range out["providers"].([]map[string]any) {
		if g["id"] == "openai" && g["count"] != 2 {
			t.Fatalf("each group still says how many it lists: %v", g)
		}
	}
}

// A provider whose list call failed answers its error, not an empty list
// the window would read as "no model matches".
func TestAScopedProviderFailureIsItsError(t *testing.T) {
	out := map[string]any{"providers": []map[string]any{
		{"id": "deepseek", "connected": true, "error": "401 refused"},
		{"id": "anthropic", "connected": false},
	}}
	if status, msg := scopedProviderFailure(out, "deepseek"); status != http.StatusBadGateway || !strings.Contains(msg, "401 refused") {
		t.Fatalf("the list call's error: %d %q", status, msg)
	}
	if status, _ := scopedProviderFailure(out, "anthropic"); status != http.StatusNotFound {
		t.Fatalf("not connected: %d", status)
	}
	if status, _ := scopedProviderFailure(out, ""); status != 0 {
		t.Fatalf("no provider asked for: %d", status)
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

// A key on an account with no credit yet (a new OpenRouter sign-up) is a good
// key: the probe's 402 connects it and says what to do, instead of "not
// connected" (the mutation check: drop hasNoCredit and res.OK is false).
func TestConnectAKeyWithNoCreditYet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_, _ = io.WriteString(w, `{"data":[{"id":"some/model"}]}`)
			return
		}
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = io.WriteString(w, `{"error":{"code":402,"message":"Insufficient credits. Add more using https://openrouter.ai/settings/credits"}}`)
	}))
	defer srv.Close()
	res := connectProvider(context.Background(), connectRequest{ID: "fresh", Name: "Fresh", API: providers.APIChat, Base: srv.URL + "/v1", Key: "sk-or-v1-fresh", Probe: true})
	if !res.OK || !strings.Contains(res.Answer, "no credit") {
		t.Fatalf("want connected with the no-credit note, got %+v", res)
	}
}

// The no-credit note names the provider the key is for: OpenRouter's page
// for OpenRouter, the host for any other (an LLM Gateway account was told to
// add credit at openrouter.ai, 2026-11-09).
func TestTheNoCreditNoteNamesTheProvider(t *testing.T) {
	for base, want := range map[string]string{
		"https://openrouter.ai/api/v1": "openrouter.ai/settings/credits",
		"https://api.llmgateway.io/v1": "add some at llmgateway.io)",
		"https://inference.example/v1": "add some at inference.example)",
	} {
		if got := noCreditAnswer(providers.Provider{API: providers.APIChat, Base: base}); !strings.Contains(got, want) {
			t.Errorf("%s: %q, want %q", base, got, want)
		}
	}
	if got := noCreditAnswer(providers.Provider{API: providers.APIChat, Base: "https://api.llmgateway.io/v1"}); strings.Contains(got, "openrouter") {
		t.Errorf("LLM Gateway is not sent to OpenRouter: %q", got)
	}
}

// Vercel AI Gateway takes a good key and serves nothing until the account
// has a card on file (403 customer_verification_required, live 2026-11-09):
// the key connects and the answer says why no model replied. A refused key
// (401) is still not connected.
func TestConnectAKeyOnAnAccountWithNoCardYet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"alibaba/qwen-3-14b","type":"language","supported_parameters":["tools"]}]}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer vck_good" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"message":"Invalid API key","type":"authentication_error"}}`)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"message":"AI Gateway requires a valid credit card on file to service requests.","type":"customer_verification_required"}}`)
	}))
	defer srv.Close()
	res := connectProvider(context.Background(), connectRequest{ID: "vercel", Name: "Vercel AI Gateway", API: providers.APIChat, Base: srv.URL + "/v1", Key: "vck_good", Probe: true})
	if !res.OK || !strings.Contains(res.Answer, "card on file") {
		t.Fatalf("want connected with the card note, got %+v", res)
	}
	if res := connectProvider(context.Background(), connectRequest{ID: "vercel", API: providers.APIChat, Base: srv.URL + "/v1", Key: "vck_wrong", Probe: true}); res.OK {
		t.Fatalf("a refused key is not connected: %+v", res)
	}
}

// The menu behind `connect local` lists every default address whose model list
// answers, in order, and names a runtime by its default port.
func TestAnsweringRuntimesListsEachThatAnswers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_, _ = io.WriteString(w, `{"data":[{"id":"a"},{"id":"b"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer empty.Close()

	saved := localRuntimeBases
	defer func() { localRuntimeBases = saved }()
	localRuntimeBases = []string{"http://localhost:1/v1", empty.URL + "/v1", srv.URL + "/v1", srv.URL + "/v1"}
	found, tried := answeringRuntimes(context.Background())
	if len(tried) != 4 {
		t.Fatalf("every address is tried: %v", tried)
	}
	if len(found) != 2 || found[0].Base != srv.URL+"/v1" || found[0].Models != 2 || found[0].Name != "a model server" {
		t.Fatalf("only an address that lists models is offered, models counted: %+v", found)
	}

	localRuntimeBases = []string{"http://localhost:1/v1"}
	if found, _ := answeringRuntimes(context.Background()); len(found) != 0 {
		t.Fatalf("nothing listening offers nothing: %+v", found)
	}
	for port, name := range map[string]string{"8090": "janis", "11434": "Ollama", "8080": "llama.cpp", "1234": "LM Studio", "8000": "vLLM"} {
		if runtimeNames[port] != name {
			t.Errorf("port %s is %q, not %q", port, runtimeNames[port], name)
		}
	}
}

// Connecting a local runtime from a terminal keeps its tested model as the
// person's pin, so the next conversation runs on it and not on an older pin.
func TestConnectingALocalRuntimeSelectsItsModel(t *testing.T) {
	dir := t.TempDir()
	old := persistentPinPath
	persistentPinPath = func() string { return filepath.Join(dir, ".memdoor", "model.json") }
	t.Cleanup(func() { persistentPinPath = old })

	if err := writePersistentPin("human:me", &persistentPin{Model: "deepseek/deepseek-v4.1-flash-1017"}); err != nil {
		t.Fatal(err)
	}
	connected := connectResult{OK: true, Saved: true, ID: "local", Tested: "Qwen3-1.7B-4bit"}

	for name, req := range map[string]connectRequest{
		"not asked for":    {Keyless: true},
		"a keyed provider": {Select: true},
	} {
		res := connected
		selectConnected("human:me", req, &res)
		if res.Selected != "" || readPersistentPin("human:me").Model != "deepseek/deepseek-v4.1-flash-1017" {
			t.Fatalf("%s must leave the pin alone: %+v %+v", name, res, readPersistentPin("human:me"))
		}
	}
	for name, res := range map[string]connectResult{
		"a probe only":  {OK: true, ID: "local", Tested: "m"},
		"a failed test": {ID: "local", Tested: "m", Saved: true},
		"no model":      {OK: true, Saved: true, ID: "local"},
	} {
		selectConnected("human:me", connectRequest{Keyless: true, Select: true}, &res)
		if res.Selected != "" {
			t.Fatalf("%s must not select: %+v", name, res)
		}
	}

	res := connected
	selectConnected("human:me", connectRequest{Keyless: true, Select: true}, &res)
	if res.Selected != "local:Qwen3-1.7B-4bit" {
		t.Fatalf("the tested model is reported as selected: %+v", res)
	}
	if p := readPersistentPin("human:me"); p == nil || p.Model != "local:Qwen3-1.7B-4bit" {
		t.Fatalf("the older pin is replaced by the local model: %+v", p)
	}
	if readPersistentPin("human:other") != nil {
		t.Fatal("another person's pin is not written")
	}
}
