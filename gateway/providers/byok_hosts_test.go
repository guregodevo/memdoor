package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

func resetSticky(t *testing.T, eps []Endpoint) {
	t.Helper()
	sticky.Lock()
	sticky.hosts, sticky.slugs, sticky.fetched = map[string]stickyEntry{}, map[string]map[string]string{}, map[string]time.Time{}
	sticky.Unlock()
	old := fetchEndpoints
	fetchEndpoints = func(string, string) ([]Endpoint, error) { return eps, nil }
	t.Cleanup(func() { fetchEndpoints = old })
}

// The host that answered is asked first on the next request, by its slug,
// with fallback left on; a person's own order is never replaced.
func TestTheServingHostIsAskedFirstNextTime(t *testing.T) {
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	resetSticky(t, []Endpoint{{ProviderName: "AtlasCloud", Tag: "atlas-cloud/fp8"}, {ProviderName: "Together", Tag: "together"}})
	var sent []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Provider map[string]any `json:"provider"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		sent = append(sent, body.Provider)
		sse(w, `{"provider":"AtlasCloud","choices":[{"index":0,"delta":{"content":"OK"}}]}`,
			`{"provider":"AtlasCloud","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	}))
	defer srv.Close()
	c := &oaiClient{apiKey: "k", baseURL: srv.URL, httpClient: srv.Client(), model: "deepseek/deepseek-v4.1-flash",
		provider: map[string]any{"sort": "price", "data_collection": "deny"}, attribute: true}
	ask := func() {
		if _, err := c.Messages().New(context.Background(), llm.MessageNewParams{MaxTokens: 8,
			Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}}); err != nil {
			t.Fatal(err)
		}
	}
	ask()
	deadline := time.Now().Add(2 * time.Second)
	for stickyHost("deepseek/deepseek-v4.1-flash") == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	ask()
	if _, ok := sent[0]["order"]; ok {
		t.Fatalf("first request already had an order: %v", sent[0])
	}
	if got := sent[1]["order"]; !reflect.DeepEqual(got, []any{"atlas-cloud"}) || sent[1]["allow_fallbacks"] != true || sent[1]["sort"] != "price" {
		t.Fatalf("second request provider = %v, want order [atlas-cloud] with fallbacks and price sort", sent[1])
	}
	own := map[string]any{"order": []string{"together"}}
	if got := withStickyHost(own, "deepseek/deepseek-v4.1-flash"); !reflect.DeepEqual(got["order"], []string{"together"}) {
		t.Fatalf("a person's order was replaced: %v", got)
	}
}

func TestAnUnknownHostOrAnOldPinIsNotUsed(t *testing.T) {
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	resetSticky(t, []Endpoint{{ProviderName: "AtlasCloud", Tag: "atlas-cloud/fp8"}})
	noteServedHost("k", "m/x", "NotListed")
	if h := stickyHost("m/x"); h != "" {
		t.Fatalf("a host the listing does not name was pinned: %q", h)
	}
	noteServedHost("k", "m/x", "AtlasCloud")
	sticky.Lock()
	e := sticky.hosts["m/x"]
	e.at = time.Now().Add(-stickyHostFor - time.Minute)
	sticky.hosts["m/x"] = e
	sticky.Unlock()
	if h := stickyHost("m/x"); h != "" {
		t.Fatalf("an expired pin was used: %q", h)
	}
}

// One fallback answering is not a move: under parallel load it would re-pin
// every conversation at once and throw away all their prompt caches. Two
// answers in a row from the same other host do move the pin. A host that
// answers its own request leaves a half-built move untouched.
func TestASingleFallbackDoesNotMoveThePin(t *testing.T) {
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	resetSticky(t, []Endpoint{{ProviderName: "AtlasCloud", Tag: "atlas-cloud/fp8"}, {ProviderName: "Together", Tag: "together"}})
	noteServedHost("k", "m/x", "AtlasCloud")
	if h := stickyHost("m/x"); h != "atlas-cloud" {
		t.Fatalf("pin = %q, want atlas-cloud", h)
	}
	noteServedHost("k", "m/x", "Together")
	if h := stickyHost("m/x"); h != "atlas-cloud" {
		t.Fatalf("one fallback moved the pin to %q", h)
	}
	noteServedHost("k", "m/x", "AtlasCloud")
	noteServedHost("k", "m/x", "Together")
	if h := stickyHost("m/x"); h != "atlas-cloud" {
		t.Fatalf("two fallbacks apart from each other moved the pin to %q", h)
	}
	noteServedHost("k", "m/x", "Together")
	if h := stickyHost("m/x"); h != "together" {
		t.Fatalf("two fallbacks in a row did not move the pin: %q", h)
	}
}
