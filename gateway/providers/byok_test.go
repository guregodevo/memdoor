package providers

import (
	"context"
	"encoding/json"
	"io"
	"memdoor/pkg/llm"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	sharedctx "memdoor/pkg/shared/context"
)

func TestByokEngineFromTheKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	if _, ok := ByokEngine(); ok {
		t.Fatal("no key: there must be no BYOK engine")
	}
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-test")
	re, ok := ByokEngine()
	if !ok || !re.Byok || re.APIKey != "sk-test" {
		t.Fatalf("the key must build the engine: %+v", re)
	}
	if re.Endpoint != byokEndpoint {
		t.Errorf("BYOK talks to OpenRouter itself, not %q", re.Endpoint)
	}
	// The ladder is resolved in the binary: rung 1 is the cheapest model.
	if got := re.ModelForTier("coder", 0); got != "z-ai/glm-5.3-flash" {
		t.Errorf("the coder's first rung is the cheapest model, got %q", got)
	}
	if got := re.ModelForTier("coder", 2); got != "z-ai/glm-5.3" {
		t.Errorf("the coder's top rung, got %q", got)
	}

	// There is no way back to a seat (2026-10-04): the old MEMDOOR_BYOK=0
	// switch is gone, and the person's key is always used.
	t.Setenv("MEMDOOR_BYOK", "0")
	if k := ByokKey(); k == "" {
		t.Errorf("the key is used whatever MEMDOOR_BYOK says")
	}
}

// The person's code must never reach a host that trains on it, and the
// cheapest qualifying host is the default — their own ordering wins.
func TestByokProviderPolicy(t *testing.T) {
	p := byokProviderPolicy(context.Background())
	if p["data_collection"] != "deny" {
		t.Error("every BYOK request must say data_collection: deny")
	}
	if p["require_parameters"] != true {
		t.Error("a host that drops tool parameters answers wrong: require_parameters")
	}
	// A 4-bit host failed 3 of 3 GLM turns (2026-09-30): never below fp8.
	q, _ := p["quantizations"].([]string)
	if !slices.Contains(q, "fp8") || !slices.Contains(q, "unknown") || slices.Contains(q, "fp4") || slices.Contains(q, "int4") {
		t.Errorf("quantizations = %v, want fp8-and-better plus unknown, never fp4/int4", q)
	}
	if p["sort"] != "price" {
		t.Errorf("the cheapest qualifying host by default, got %v", p["sort"])
	}

	ctx := context.WithValue(context.Background(), sharedctx.SortKey, "throughput")
	if p := byokProviderPolicy(ctx); p["sort"] != "throughput" {
		t.Errorf("the person's sort must win, got %v", p["sort"])
	}
	ctx = context.WithValue(context.Background(), sharedctx.SortKey, "default")
	if p := byokProviderPolicy(ctx); p["sort"] != nil {
		t.Errorf("sort=default leaves the choice to OpenRouter, got %v", p["sort"])
	}
	ctx = context.WithValue(context.Background(), sharedctx.OrderKey, "baidu, morph")
	p = byokProviderPolicy(ctx)
	order, _ := p["order"].([]string)
	if len(order) != 2 || order[0] != "baidu" || order[1] != "morph" {
		t.Fatalf("a hand-written host order must travel as given: %v", p["order"])
	}
	if p["sort"] != nil {
		t.Errorf("an explicit order is not re-sorted, got %v", p["sort"])
	}
	if p["allow_fallbacks"] != true {
		t.Error("an order must still allow the next host when the first is down")
	}
}

// The BYOK client carries the policy and the person's key, and a model they
// pinned wins over the rung.
func TestByokClientIsDirectAndPolicied(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-test")
	re, _ := ByokEngine()
	c, ok := newDirectOpenRouterClient(context.Background(), &re, "z-ai/glm-5.3").(*oaiClient)
	if !ok {
		t.Fatal("expected the OpenAI-compatible client")
	}
	if c.apiKey != "sk-test" {
		t.Error("the person's key is the credential")
	}
	if c.baseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("straight to OpenRouter, got %q", c.baseURL)
	}
	if c.model != "z-ai/glm-5.3" {
		t.Errorf("the model asked for, got %q", c.model)
	}
	if c.provider == nil || c.provider["data_collection"] != "deny" {
		t.Error("the request must carry the provider policy")
	}
}

// OpenRouter builds its public app ranking from these two headers. Without
// them a person's turns count for nobody (true until 2026-09-27), and a second
// spelling would split us across two rows.
func TestOpenRouterAttribution(t *testing.T) {
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	h := http.Header{}
	SetOpenRouterAttribution(h)
	if h.Get("X-Title") != "Memdoor" || h.Get("HTTP-Referer") != "https://memdoor.ai" {
		t.Fatalf("both headers, canonical values: %v", h)
	}
	for _, off := range []string{"0", "off", "OFF"} {
		t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", off)
		h := http.Header{}
		SetOpenRouterAttribution(h)
		if len(h) != 0 {
			t.Errorf("%q must leave the request unattributed: %v", off, h)
		}
	}
}

// The headers must reach the wire, not just the struct: this is the one that
// would have caught their absence (2026-09-27).
func TestByokRequestCarriesAttributionAndPolicy(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the meter ledger lives under $HOME
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	var got struct {
		title, referer, auth string
		body                 map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.title, got.referer = r.Header.Get("X-Title"), r.Header.Get("HTTP-Referer")
		got.auth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got.body)
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range []string{
			`{"choices":[{"index":0,"delta":{"role":"assistant","content":"ok"}}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":1,"total_tokens":11}}`,
		} {
			_, _ = io.WriteString(w, "data: "+ev+"\n\n")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	re := RemoteEngine{Endpoint: srv.URL + "/api/v1/chat/completions", APIKey: "sk-mine", Byok: true}
	c := newDirectOpenRouterClient(context.Background(), &re, "z-ai/glm-5.3-flash")
	if _, err := c.Messages().New(context.Background(), llm.MessageNewParams{
		MaxTokens: 16,
		Messages:  []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("say ok"))},
	}); err != nil {
		t.Fatal(err)
	}
	if got.title != "Memdoor" || got.referer != "https://memdoor.ai" {
		t.Errorf("OpenRouter must be told which app this is: title=%q referer=%q", got.title, got.referer)
	}
	if got.auth != "Bearer sk-mine" {
		t.Errorf("the person's own key is the credential, got %q", got.auth)
	}
	p, _ := got.body["provider"].(map[string]any)
	if p == nil || p["data_collection"] != "deny" {
		t.Errorf("the privacy policy must travel with the request: %v", got.body["provider"])
	}
}

// The model answering a turn is one answer for the client, the window and
// the log: the one pinned with /model, else the agent's rung.
func TestAnsweringModel(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-test")
	re, _ := ByokEngine()
	ctx := context.Background()
	if got := re.AnsweringModel(ctx, "coder"); got != "z-ai/glm-5.3-flash" {
		t.Errorf("rung 1, got %q", got)
	}
	if got := re.AnsweringModel(context.WithValue(ctx, sharedctx.TierKey, 2), "coder"); got != "z-ai/glm-5.3" {
		t.Errorf("rung 3, got %q", got)
	}
	pinned := context.WithValue(context.WithValue(ctx, sharedctx.TierKey, 2), sharedctx.ModelKey, "qwen/qwen3.6-plus")
	if got := re.AnsweringModel(pinned, "coder"); got != "qwen/qwen3.6-plus" {
		t.Errorf("a pin wins over the rung, got %q", got)
	}
	re.Model = ""
	if got := re.AnsweringModel(ctx, "planner"); got != byokDefaultModel {
		t.Errorf("an agent with no ladder on the own key, got %q", got)
	}
}
