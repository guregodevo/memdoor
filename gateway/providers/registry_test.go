package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{"AI_GATEWAY_BASE_URL", "AI_GATEWAY_TOKEN", "AI_GATEWAY_API", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "OPENAI_API_KEY", "OPENAI_BASE_URL", "GEMINI_API_KEY", "GEMINI_BASE_URL", "XAI_API_KEY", "BASETEN_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "GROK_API_KEY", "OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY", "MEMDOOR_MODEL", "MEMDOOR_VENDOR_HEADERS", "MEMDOOR_BYOK"} {
		t.Setenv(v, "")
	}
	t.Setenv("HOME", t.TempDir())
	_ = ClearRemoteEngine()
	t.Cleanup(func() { _ = ClearRemoteEngine() })
	modelLists.mu.Lock()
	modelLists.at, modelLists.list = map[string]time.Time{}, map[string][]Model{}
	modelLists.mu.Unlock()
	// No reference catalogue in these tests: the host is unreachable and
	// the cache empty, so a window the provider does not state is the default.
	modelsDevURL = "http://127.0.0.1:1/api.json"
	modelsDev.mu.Lock()
	modelsDev.doc, modelsDev.at = nil, time.Time{}
	modelsDev.mu.Unlock()
}

// Providers, each with its list (Greg, 2026-10-02): the built-in ones are
// listed whether or not a key is set — locked without one — and the file's
// come after them. A credential is a literal, an env var's NAME, or a
// command's stdout.
func TestProvidersAreBuiltInsThenTheFile(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant")
	t.Setenv("CORP_TOKEN", "pat-from-env")
	t.Setenv("AI_GATEWAY_BASE_URL", "https://ai-gateway.corp.example/ai")
	t.Setenv("AI_GATEWAY_TOKEN", "pat-1")
	t.Setenv("MEMDOOR_MODEL", "corp-slug")
	if err := SaveProvider(ProviderSpec{ID: "lab", Name: "Lab proxy", API: APIChat, Base: "https://lab.example/v1/", Key: "CORP_TOKEN"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveProvider(ProviderSpec{ID: "shell", API: APIChat, Base: "https://sh.example/v1", Key: "!printf from-a-command"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveProvider(ProviderSpec{ID: "literal", API: APIResponses, Base: "https://lit.example", Key: "sk-literal-123"}); err != nil {
		t.Fatal(err)
	}
	path, _ := providersPath()
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("providers.json must be 0600: %v %v", st, err)
	}
	byID := map[string]Provider{}
	var order []string
	for _, p := range Providers() {
		byID[p.ID] = p
		order = append(order, p.ID)
	}
	if order[0] != "gateway" || order[1] != "anthropic" || order[len(order)-3] != "lab" {
		t.Fatalf("order: %v", order)
	}
	if g := byID["gateway"]; !g.Connected() || g.API != APIResponses || g.Key != "pat-1" || len(g.Models) != 1 || g.Models[0].ID != "corp-slug" || !g.BuiltIn {
		t.Fatalf("gateway: %+v", g)
	}
	if a := byID["anthropic"]; !a.Connected() || a.KeySource != "env ANTHROPIC_API_KEY" {
		t.Fatalf("anthropic: %+v", a)
	}
	if o := byID["openai"]; o.Connected() || o.KeySource != "" {
		t.Fatalf("openai without a key is listed, locked: %+v", o)
	}
	if l := byID["lab"]; l.Key != "pat-from-env" || l.KeySource != "env CORP_TOKEN" || l.Base != "https://lab.example/v1" || l.Name != "Lab proxy" {
		t.Fatalf("an env var's NAME resolves: %+v", l)
	}
	if s := byID["shell"]; s.Key != "from-a-command" || s.KeySource != "command" {
		t.Fatalf("!command resolves to its stdout: %+v", s)
	}
	if l := byID["literal"]; l.Key != "sk-literal-123" || l.KeySource != "providers.json" {
		t.Fatalf("a literal stays: %+v", l)
	}
	if err := RemoveProvider("shell"); err != nil {
		t.Fatal(err)
	}
	if _, ok := FindProvider("shell"); ok {
		t.Fatal("removed")
	}
	if b, _ := os.ReadFile(path); !json.Valid(b) || string(b) == "" {
		t.Fatal("the file stays valid JSON")
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
}

// A provider's list comes from its own endpoint — OpenAI-shaped or
// Anthropic-shaped — declared models first, cached; a gateway whose list
// endpoint answers nothing keeps its declared models.
func TestModelsOfReadsEachProvidersList(t *testing.T) {
	clearProviderEnv(t)
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"|"+r.Header.Get("Authorization")+"|"+r.Header.Get("x-api-key"))
		switch r.URL.Path {
		case "/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"gpt-5","context_length":400000},{"id":"gpt-5-mini","max_model_len":128000}]}`)
		case "/anthropic/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"claude-sonnet-5","display_name":"Claude Sonnet 5"},{"id":"claude-opus-5-5","display_name":"Claude Opus 5.5"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	oai := Provider{ID: "openai", API: APIChat, Base: srv.URL + "/v1", Key: "sk-1", Context: 1}
	list, err := ModelsOf(ctx, oai)
	if err != nil || len(list) != 2 || list[0].ID != "gpt-5" || list[0].Context != 400000 || list[1].Context != 128000 || list[0].Provider != "openai" {
		t.Fatalf("openai list: %+v %v", list, err)
	}
	ant := Provider{ID: "anthropic", API: APIAnthropic, Base: srv.URL + "/anthropic", Key: "sk-ant", Context: 200000}
	list, err = ModelsOf(ctx, ant)
	if err != nil || len(list) != 2 || list[0].Name != "Claude Opus 5.5" || list[1].Context != 200000 {
		t.Fatalf("anthropic list: %+v %v", list, err)
	}
	// A declared model the list also names takes the list's figures.
	declared := Provider{ID: "declared", API: APIChat, Base: srv.URL + "/v1", Key: "sk-1", Models: []Model{{ID: "gpt-5", Name: "gpt-5"}}}
	list, err = ModelsOf(ctx, declared)
	if err != nil || list[0].ID != "gpt-5" || list[0].Context != 400000 || list[0].ContextSource != ContextFromProvider {
		t.Fatalf("declared, then listed: %+v %v", list, err)
	}
	gw := Provider{ID: "gateway", API: APIResponses, Base: srv.URL + "/nowhere", Key: "pat", Models: []Model{{ID: "corp-slug", Name: "corp-slug"}}}
	list, err = ModelsOf(ctx, gw)
	if err != nil || len(list) != 1 || list[0].ID != "corp-slug" {
		t.Fatalf("a gateway that returns none keeps its declared model: %+v %v", list, err)
	}
	if _, err := ModelsOf(ctx, Provider{ID: "locked", API: APIChat, Base: srv.URL}); err == nil {
		t.Fatal("a provider without a key has no list")
	}
	if len(paths) != 4 || paths[0] != "/v1/models|Bearer sk-1|" || paths[1] != "/anthropic/v1/models||sk-ant" || paths[2] != "/v1/models|Bearer sk-1|" {
		t.Fatalf("list calls: %v", paths)
	}
	ModelsOf(ctx, oai) // cached: no fifth call
	if len(paths) != 4 {
		t.Fatalf("the list is cached: %v", paths)
	}
	ForgetModels("openai")
	ModelsOf(ctx, oai)
	if len(paths) != 5 {
		t.Fatal("forgotten, read again")
	}
}

func TestSearchModelsAndFindModel(t *testing.T) {
	clearProviderEnv(t)
	list := []Model{{ID: "z-ai/glm-5.3", Name: "GLM 5.3"}, {ID: "openai/gpt-5", Name: "GPT-5"}, {ID: "glm-4", Name: "old"}}
	if got := SearchModels(list, "glm", 10); len(got) != 2 || got[0].ID != "glm-4" || got[1].ID != "z-ai/glm-5.3" {
		t.Fatalf("prefix first, then contains: %+v", got)
	}
	if got := SearchModels(list, "", 2); len(got) != 2 {
		t.Fatalf("limit: %+v", got)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"id":"shared-model"}]}`)
	}))
	defer srv.Close()
	t.Setenv("OPENAI_API_KEY", "sk")
	t.Setenv("OPENAI_BASE_URL", srv.URL+"/v1")
	if err := SaveProvider(ProviderSpec{ID: "corp", API: APIChat, Base: srv.URL + "/v1", Key: "pat"}); err != nil {
		t.Fatal(err)
	}
	p, m, ok := FindModel(context.Background(), "shared-model")
	if !ok || p.ID != "openai" || m.ID != "shared-model" {
		t.Fatalf("first connected provider that lists it: %+v %+v %v", p, m, ok)
	}
	// With corp active, corp wins the tie.
	corp, _ := FindProvider("corp")
	if err := SetRemoteEngine(corp.Engine("shared-model")); err != nil {
		t.Fatal(err)
	}
	if ActiveProviderID() != "corp" {
		t.Fatalf("active: %q", ActiveProviderID())
	}
	if p, _, _ := FindModel(context.Background(), "shared-model"); p.ID != "corp" {
		t.Fatalf("the active provider wins a tie: %+v", p)
	}
	if _, _, ok := FindModel(context.Background(), "nobody-has-it"); ok {
		t.Fatal("unknown")
	}
	// The explicit form names the provider; the model comes back bare.
	if p, m, ok := FindModel(context.Background(), "openai:shared-model"); !ok || p.ID != "openai" || m.ID != "shared-model" {
		t.Fatalf("provider:model pins that provider: %+v %+v %v", p, m, ok)
	}
	if _, _, ok := FindModel(context.Background(), "corp:nobody-has-it"); ok {
		t.Fatal("a provider that does not list it")
	}
	if prov, bare := SplitPin("z-ai/glm-5.3"); prov != "" || bare != "z-ai/glm-5.3" {
		t.Fatalf("no prefix: %q %q", prov, bare)
	}
	if _, isOAI := corp.Client(context.Background(), "shared-model").(*oaiClient); !isOAI {
		t.Fatal("a chat provider's client")
	}
}

// A built-in connected through /connect is kept in the file under its own
// id and is there at the next start, even with nothing in the environment;
// the environment wins when it has one.
func TestAConnectedBuiltInSurvivesARestart(t *testing.T) {
	clearProviderEnv(t)
	if err := SaveProvider(ProviderSpec{ID: "anthropic", API: APIAnthropic, Base: "https://api.anthropic.com", Key: "sk-ant-typed"}); err != nil {
		t.Fatal(err)
	}
	p, ok := FindProvider("anthropic")
	if !ok || !p.Connected() || p.Key != "sk-ant-typed" || p.KeySource != "providers.json" || !p.BuiltIn || p.Name != "Anthropic" {
		t.Fatalf("the built-in, connected from the file: %+v", p)
	}
	n := 0
	for _, q := range Providers() {
		if q.ID == "anthropic" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("listed once: %d", n)
	}
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-env")
	if p, _ := FindProvider("anthropic"); p.Key != "sk-ant-env" || p.KeySource != "env ANTHROPIC_API_KEY" {
		t.Fatalf("the environment first: %+v", p)
	}
}
