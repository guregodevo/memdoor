package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeSearch serves one chat-completions answer and records the request.
func fakeSearch(t *testing.T, reply string) *map[string]any {
	t.Helper()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("the key was not sent: %q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	prev := webSearchBackend
	SetWebSearchBackend(func() WebSearchBackend {
		return WebSearchBackend{Endpoint: srv.URL, Key: "sk-test", Model: "z-ai/glm-5.3-flash"}
	})
	t.Cleanup(func() { webSearchBackend = prev })
	return &got
}

// One request with OpenRouter's search server tool; the answer comes back
// with its sources numbered, each once.
func TestWebSearchAnswersWithNumberedSources(t *testing.T) {
	got := fakeSearch(t, `{"choices":[{"message":{"content":"Go 1.25 is out.","annotations":[
		{"type":"url_citation","url_citation":{"url":"https://go.dev/doc/go1.25","title":"Go 1.25 Release Notes","content":"Go 1.25   is released."}},
		{"type":"url_citation","url_citation":{"url":"https://go.dev/doc/go1.25","title":"dup","content":"x"}},
		{"type":"url_citation","url_citation":{"url":"https://go.dev/blog","title":"The Go Blog","content":""}}]}}]}`)
	out, err := WebSearch(json.RawMessage(`{"query":"latest go release","max_results":50}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Go 1.25 is out.", "[1] Go 1.25 Release Notes — https://go.dev/doc/go1.25", "    Go 1.25 is released.", "[2] The Go Blog — https://go.dev/blog"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "dup") {
		t.Fatalf("a source cited twice is listed once:\n%s", out)
	}
	tool := (*got)["tools"].([]any)[0].(map[string]any)
	params := tool["parameters"].(map[string]any)
	if tool["type"] != "openrouter:web_search" || params["engine"] != webSearchEngine || params["max_results"] != float64(webSearchMaxResults) || params["max_uses"] != float64(1) {
		t.Fatalf("request tools: %v", tool)
	}
	if (*got)["model"] != "z-ai/glm-5.3-flash" {
		t.Fatalf("model: %v", (*got)["model"])
	}
}

// An answer without a single source is the model answering from memory:
// refused, not passed off as a search.
func TestWebSearchRefusesAnAnswerWithoutSources(t *testing.T) {
	fakeSearch(t, `{"choices":[{"message":{"content":"I believe Go 1.22 is the latest.","annotations":[]}}]}`)
	if out, err := WebSearch(json.RawMessage(`{"query":"latest go release"}`)); err == nil || !strings.Contains(err.Error(), "without a single source") {
		t.Fatalf("want a refusal, got %q, %v", out, err)
	}
}

// No key, no search — said plainly.
func TestWebSearchNeedsAKey(t *testing.T) {
	prev := webSearchBackend
	SetWebSearchBackend(func() WebSearchBackend { return WebSearchBackend{} })
	t.Cleanup(func() { webSearchBackend = prev })
	if _, err := WebSearch(json.RawMessage(`{"query":"x"}`)); err == nil || !strings.Contains(err.Error(), "OPEN_ROUTER_API_KEY") {
		t.Fatalf("want the missing key named, got %v", err)
	}
}
