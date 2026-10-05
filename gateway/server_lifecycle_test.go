package gateway

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Only the allowed pages may call the gateway from a browser: a foreign site
// gets no CORS permission at all, never "*" (2026-10-04).
func TestCORSAllowsOnlyItsOrigins(t *testing.T) {
	h := addCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for origin, want := range map[string]string{
		"https://evil.example":  "",
		"https://memdoor.ai":    "https://memdoor.ai",
		"http://localhost:5173": "http://localhost:5173",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
		req.Header.Set("Origin", origin)
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != want {
			t.Fatalf("%s: allow-origin %q, want %q", origin, got, want)
		}
	}
}

// A key added with `memdoor connect` is a model: the status must not say
// "none" (which blocks every turn in the window) because no OpenRouter or
// vendor engine is active. A decisions-only provider answers no turn.
func TestAConnectedProviderIsAModel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, k := range []string{"OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "XAI_API_KEY", "GROK_API_KEY", "BASETEN_API_KEY", "GROQ_API_KEY", "DEEPSEEK_API_KEY", "AI_GATEWAY_BASE_URL", "MEMDOOR_SYSTEMONE_API_KEY", "TYPESAFE_API_KEY"} {
		t.Setenv(k, "")
	}
	s := &Server{}
	if got := s.BrainStatus().State; got != "none" {
		t.Fatalf("no key anywhere: %q", got)
	}
	dir := filepath.Join(home, ".memdoor")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "providers.json"), []byte(`{"providers":[{"id":"typesafe","api":"decisions","base":"https://api.typesafe.ai","key":"ts-key"}]}`), 0o600)
	if got := s.BrainStatus().State; got != "none" {
		t.Fatalf("a decisions-only provider answers no turn: %q", got)
	}
	os.WriteFile(filepath.Join(dir, "providers.json"), []byte(`{"providers":[{"id":"deepseek","api":"chat","base":"https://api.deepseek.com","key":"sk-test"}]}`), 0o600)
	if got := s.BrainStatus().State; got != "ready" {
		t.Fatalf("a connected chat provider is a model: %q", got)
	}
}
