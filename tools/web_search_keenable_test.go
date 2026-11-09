package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

type keenableCall struct {
	path, title, apiKey string
	body                map[string]any
}

func fakeKeenable(t *testing.T, key string, status int, reply string) *keenableCall {
	t.Helper()
	got := &keenableCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got.body)
		got.path, got.title, got.apiKey = r.URL.Path, r.Header.Get("X-Keenable-Title"), r.Header.Get(keenableKeyHeader)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	prev := webSearchBackend
	SetWebSearchBackend(func() WebSearchBackend {
		return WebSearchBackend{Engine: WebSearchKeenable, Endpoint: srv.URL, Key: key, Headers: map[string]string{"X-Keenable-Title": "Memdoor"}}
	})
	t.Cleanup(func() { webSearchBackend = prev })
	return got
}

// Keyless: the public endpoint, the app named, no key header; the results
// are the Sources list the TUI already parses.
func TestKeenableKeylessReturnsTheSourcesList(t *testing.T) {
	got := fakeKeenable(t, "", 200, `{"query":"q","mode":"pro","results":[
		{"title":"Go 1.25 Release Notes","url":"https://go.dev/doc/go1.25","description":"","snippet":"Go 1.25   is released."},
		{"title":"dup","url":"https://go.dev/doc/go1.25","snippet":"x"},
		{"title":"","url":"https://go.dev/blog","description":"The Go Blog","snippet":""}]}`)
	out, err := WebSearch(json.RawMessage(`{"query":"latest go release","max_results":50}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.path != keenablePathPublic || got.title != "Memdoor" || got.apiKey != "" {
		t.Fatalf("keyless request: path %q title %q key %q", got.path, got.title, got.apiKey)
	}
	if got.body["query"] != "latest go release" || got.body["max_results"] != float64(webSearchMaxResults) {
		t.Fatalf("request body: %v", got.body)
	}
	answer, sources, ok := strings.Cut(out, "\n\nSources:")
	if !ok || !strings.Contains(answer, "Keenable") {
		t.Fatalf("no Sources list:\n%s", out)
	}
	for _, want := range []string{"[1] Go 1.25 Release Notes — https://go.dev/doc/go1.25", "    Go 1.25 is released.", "[2] https://go.dev/blog — https://go.dev/blog", "    The Go Blog"} {
		if !strings.Contains(sources, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "dup") {
		t.Fatalf("a URL listed twice is listed once:\n%s", out)
	}
	// The TUI's source line (cmd/tui/ui/toolview_frames.go webSourceRe).
	re := regexp.MustCompile(`^\[(\d+)\] (.*?) — (\S+)$`)
	n := 0
	for _, l := range strings.Split(sources, "\n") {
		if re.MatchString(l) {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("the TUI reads %d sources, want 2:\n%s", n, out)
	}
}

// A key goes to the keyed endpoint in X-API-Key.
func TestKeenableKeyGoesToTheKeyedEndpoint(t *testing.T) {
	got := fakeKeenable(t, "kn-test", 200, `{"results":[{"title":"t","url":"https://example.com/a","snippet":"s"}]}`)
	if _, err := WebSearch(json.RawMessage(`{"query":"x"}`)); err != nil {
		t.Fatal(err)
	}
	if got.path != keenablePathKeyed || got.apiKey != "kn-test" {
		t.Fatalf("keyed request: path %q key %q", got.path, got.apiKey)
	}
}

func TestKeenableNoResultsIsSaid(t *testing.T) {
	fakeKeenable(t, "", 200, `{"results":[]}`)
	if out, err := WebSearch(json.RawMessage(`{"query":"x"}`)); err == nil || !strings.Contains(err.Error(), "found nothing") {
		t.Fatalf("want nothing found, got %q, %v", out, err)
	}
}

func TestKeenableKeylessLimitNamesTheKey(t *testing.T) {
	fakeKeenable(t, "", http.StatusTooManyRequests, `{"error":"rate limited"}`)
	if _, err := WebSearch(json.RawMessage(`{"query":"x"}`)); err == nil || !strings.Contains(err.Error(), "KEENABLE_API_KEY") {
		t.Fatalf("want the key named, got %v", err)
	}
}
