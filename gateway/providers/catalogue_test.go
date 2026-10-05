package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Each reader reads its provider's metadata API as it is (shapes copied
// from the live answers of 2026-10-02): Anthropic's windows, caps and
// capabilities; Groq's window, cap, prices and modalities; Gemini's native
// limits; xAI's window from /models and modalities from /language-models.
func TestReadersReadEachProvidersOwnMetadata(t *testing.T) {
	clearProviderEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/anthropic/v1/models":
			if r.Header.Get("x-api-key") != "sk-ant" {
				w.WriteHeader(401)
				return
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"claude-haiku-4-5-20251001","display_name":"Claude Haiku 4.5","max_input_tokens":200000,"max_tokens":64000,"line":"haiku",
			  "capabilities":{"image_input":{"supported":true},"thinking":{"supported":true},"effort":{"supported":false}}},
			  {"id":"claude-sonnet-5","display_name":"Claude Sonnet 5","max_input_tokens":1000000,"max_tokens":128000,"capabilities":{"effort":{"supported":true}}}]}`)
		case r.URL.Path == "/anthropic/v1/models/claude-sonnet-5":
			_, _ = io.WriteString(w, `{"id":"claude-sonnet-5","display_name":"Claude Sonnet 5","max_input_tokens":1000000,"max_tokens":128000}`)
		case r.URL.Path == "/groq/openai/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"qwen/qwen3.8-27b","name":"Qwen 3.8 27B","active":true,"context_window":131072,"max_completion_tokens":16384,"input_modalities":["text","image"],"pricing":{"prompt":"0.0000008","completion":"0.000004"}},
			  {"id":"whisper-large-v3","active":true,"context_window":0,"max_completion_tokens":0}]}`)
		case r.URL.Path == "/gemini/v1beta/models":
			if r.URL.Query().Get("key") != "AIza" {
				w.WriteHeader(401)
				return
			}
			_, _ = io.WriteString(w, `{"models":[{"name":"models/gemini-2.5-pro","displayName":"Gemini 2.5 Pro","inputTokenLimit":1048576,"outputTokenLimit":65536,"supportedGenerationMethods":["generateContent"],"thinking":true},
			  {"name":"models/embedding-001","inputTokenLimit":2048,"supportedGenerationMethods":["embedContent"]}]}`)
		case r.URL.Path == "/gemini/v1beta/models/gemini-2.5-pro":
			_, _ = io.WriteString(w, `{"name":"models/gemini-2.5-pro","displayName":"Gemini 2.5 Pro","inputTokenLimit":1048576,"outputTokenLimit":65536,"supportedGenerationMethods":["generateContent"]}`)
		case r.URL.Path == "/xai/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"grok-4.5","context_length":500000,"aliases":["grok-4.5-latest"]}]}`)
		case r.URL.Path == "/xai/v1/language-models":
			_, _ = io.WriteString(w, `{"models":[{"id":"grok-4.5","input_modalities":["text","image"]}]}`)
		case r.URL.Path == "/deepseek/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"deepseek-flash","name":"DeepSeek Flash","context_window":128000,"max_output_tokens":32000,"effort":{"supported":true},"input_modalities":["text"]}]}`)
		case r.URL.Path == "/baseten/v1/models":
			_, _ = io.WriteString(w, `{"data":[{"id":"zai-org/GLM-5.3-Flash","name":"GLM 5.3 Flash","context_length":131072,"max_completion_tokens":65536,"input_modalities":["text"],"supported_features":["tools","reasoning"],"pricing":{"prompt":"0.0000001","completion":"0.0000005"}},
			  {"id":"some/embedder","context_length":8192,"supported_features":["embeddings"]}]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	ant := Provider{ID: "anthropic", API: APIAnthropic, Base: srv.URL + "/anthropic", Key: "sk-ant"}
	list, err := readerFor(ant).ListModels(ctx, ant)
	if err != nil || len(list) != 2 {
		t.Fatal(err, list)
	}
	if h := list[0]; h.ID != "claude-haiku-4-5-20251001" || h.Context != 200000 || h.MaxOutput != 64000 || !h.Thinking || h.Effort || strings.Join(h.Inputs, ",") != "text,image" {
		t.Fatalf("anthropic haiku: %+v", h)
	}
	if one, err := GetModel(ctx, ant, "claude-sonnet-5"); err != nil || one.Context != 1000000 || one.Provider != "anthropic" {
		t.Fatalf("anthropic per-model: %+v %v", one, err)
	}
	full, _ := ModelsOf(ctx, ant)
	if full[0].ContextSource != ContextFromProvider {
		t.Fatalf("marked as the provider's own: %+v", full[0])
	}

	groq := Provider{ID: "groq", API: APIChat, Base: srv.URL + "/groq/openai/v1", Key: "gsk"}
	list, err = readerFor(groq).ListModels(ctx, groq)
	if err != nil || len(list) != 2 {
		t.Fatal(err, list)
	}
	near := func(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }
	if q := list[0]; q.ID != "qwen/qwen3.8-27b" || q.Context != 131072 || q.MaxOutput != 16384 || !near(q.InPerM, 0.8) || !near(q.OutPerM, 4) || strings.Join(q.Inputs, ",") != "text,image" {
		t.Fatalf("groq: %+v", q)
	}

	gem := Provider{ID: "gemini", API: APIChat, Base: srv.URL + "/gemini/v1beta/openai", Key: "AIza"}
	list, err = readerFor(gem).ListModels(ctx, gem)
	if err != nil || len(list) != 1 || list[0].ID != "gemini-2.5-pro" || list[0].Context != 1048576 || list[0].MaxOutput != 65536 || !list[0].Thinking {
		t.Fatalf("gemini native, embeddings left out: %+v %v", list, err)
	}
	if one, err := readerFor(gem).GetModel(ctx, gem, "gemini-2.5-pro"); err != nil || one.Context != 1048576 {
		t.Fatalf("gemini per-model: %+v %v", one, err)
	}

	xai := Provider{ID: "xai", API: APIChat, Base: srv.URL + "/xai/v1", Key: "xai"}
	list, err = readerFor(xai).ListModels(ctx, xai)
	if err != nil || len(list) != 1 || list[0].Context != 500000 || strings.Join(list[0].Inputs, ",") != "text,image" {
		t.Fatalf("xai: %+v %v", list, err)
	}

	ds := Provider{ID: "deepseek", API: APIChat, Base: srv.URL + "/deepseek/v1", Key: "sk-ds"}
	list, err = readerFor(ds).ListModels(ctx, ds)
	if err != nil || len(list) != 1 || list[0].Context != 128000 || list[0].MaxOutput != 32000 || !list[0].Effort || !list[0].Thinking {
		t.Fatalf("deepseek: %+v %v", list, err)
	}
	bt := Provider{ID: "baseten", API: APIChat, Base: srv.URL + "/baseten/v1", Key: "bt"}
	list, err = readerFor(bt).ListModels(ctx, bt)
	if err != nil || len(list) != 1 || list[0].ID != "zai-org/GLM-5.3-Flash" || list[0].Context != 131072 || list[0].MaxOutput != 65536 || !list[0].Thinking || !near(list[0].InPerM, 0.1) {
		t.Fatalf("baseten, tool-capable only: %+v %v", list, err)
	}

	if _, isPlain := readerFor(Provider{ID: "openai", API: APIChat}).(openAIShapeReader); !isPlain {
		t.Fatal("OpenAI has nothing beyond the shape")
	}
	if _, isPlain := readerFor(Provider{ID: "corp", API: APIResponses}).(openAIShapeReader); !isPlain {
		t.Fatal("a company gateway reads the shape")
	}
}

// OpenRouter's catalogue answers a vendor's model by its normalized id:
// claude-haiku-4-5-20251001 ↔ anthropic/claude-haiku-4.5.
func TestCatalogueLimitsMatchAVendorsModel(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OPEN_ROUTER_API_KEY", "sk-or")
	old := OpenRouterModels
	OpenRouterModels = func(string) ([]Model, error) {
		return []Model{{ID: "anthropic/claude-haiku-4.5", Context: 200000, MaxOutput: 64000}, {ID: "openai/gpt-5-codex", Context: 400000, MaxOutput: 128000}, {ID: "x-ai/grok-4.5", Context: 500000}}, nil
	}
	defer func() { OpenRouterModels = old }()
	if l, ok := CatalogueLimits("anthropic", "claude-haiku-4-5-20251001"); !ok || l.Context != 200000 || l.Output != 64000 {
		t.Fatalf("anthropic by normalized id: %+v %v", l, ok)
	}
	if l, ok := CatalogueLimits("openai", "gpt-5-codex"); !ok || l.Context != 400000 {
		t.Fatalf("openai: %+v %v", l, ok)
	}
	if l, ok := CatalogueLimits("xai", "grok-4.5"); !ok || l.Context != 500000 {
		t.Fatalf("xai → x-ai: %+v %v", l, ok)
	}
	if _, ok := CatalogueLimits("openai", "gpt-nobody"); ok {
		t.Fatal("not in the catalogue")
	}
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	if _, ok := CatalogueLimits("openai", "gpt-5-codex"); ok {
		t.Fatal("no key, no catalogue")
	}
}
