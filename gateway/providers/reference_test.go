package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

const modelsDevFixture = `{"anthropic":{"models":{"claude-haiku-4-5":{"limit":{"context":200000,"output":64000}},"claude-sonnet-5":{"limit":{"context":1000000,"output":128000},"cost":{"input":3,"output":15}}}},
"google":{"models":{"gemini-2.5-pro":{"limit":{"context":1048576,"output":65536}}}},
"groq":{"models":{"openai/gpt-oss-120b":{"limit":{"context":131072,"output":65536}}}}}`

func referenceServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, modelsDevFixture)
	}))
	old := modelsDevURL
	modelsDevURL = srv.URL + "/api.json"
	t.Cleanup(func() { srv.Close(); modelsDevURL = old })
	modelsDev.mu.Lock()
	modelsDev.doc, modelsDev.at = nil, time.Time{}
	modelsDev.mu.Unlock()
	return srv, &hits
}

// The reference answers a model's window by exact id, by the id without
// its date suffix, by the id without a vendor prefix; it is read once and
// kept on disk.
func TestReferenceLimitsAndTheirCache(t *testing.T) {
	clearProviderEnv(t)
	_, hits := referenceServer(t)
	ctx := context.Background()
	if l, ok := ReferenceLimits(ctx, "anthropic", "claude-haiku-4-5-20251001"); !ok || l.Context != 200000 || l.Output != 64000 {
		t.Fatalf("date suffix dropped: %+v %v", l, ok)
	}
	if _, ok := ReferenceLimits(ctx, "anthropic", "claude-haiku-4-5-2025-10-01"); !ok {
		t.Fatal("a dashed date suffix is dropped too")
	}
	if l, ok := ReferenceLimits(ctx, "anthropic", "claude-sonnet-5"); !ok || l.Context != 1000000 {
		t.Fatalf("exact: %+v %v", l, ok)
	}
	if l, ok := ReferenceLimits(ctx, "gemini", "models/gemini-2.5-pro"); !ok || l.Context != 1048576 {
		t.Fatalf("vendor prefix dropped, provider mapped to google: %+v %v", l, ok)
	}
	if _, ok := ReferenceLimits(ctx, "groq", "qwen/unknown"); ok {
		t.Fatal("not in the catalogue")
	}
	if _, ok := ReferenceLimits(ctx, "anthropic", "claude-sonnet-5-codex-max"); ok {
		t.Fatal("no family guessing: a variant the catalogue does not name is unknown")
	}
	if _, ok := ReferenceLimits(ctx, "gateway", "corp-slug"); ok {
		t.Fatal("a company gateway has no reference")
	}
	if *hits != 1 {
		t.Fatalf("read once: %d", *hits)
	}
	home, _ := os.UserHomeDir()
	if st, err := os.Stat(home + "/.memdoor/models-dev.json"); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("kept on disk, 0600: %v %v", st, err)
	}
}

// In company-key mode the host is never contacted: the disk copy answers
// when there is one, the defaults when there is none.
func TestReferenceNeverContactsTheHostOnACompanyKey(t *testing.T) {
	clearProviderEnv(t)
	_, hits := referenceServer(t)
	if err := SetRemoteEngine(vendorEngine(VendorGateway, "https://ai-gateway.corp.example/ai", "pat", "slug", 0)); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReferenceLimits(context.Background(), "anthropic", "claude-sonnet-5"); ok || *hits != 0 {
		t.Fatalf("no copy, no fetch: ok=%v hits=%d", ok, *hits)
	}
	home, _ := os.UserHomeDir()
	_ = os.MkdirAll(home+"/.memdoor", 0o700)
	if err := os.WriteFile(home+"/.memdoor/models-dev.json", []byte(modelsDevFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	modelsDev.mu.Lock()
	modelsDev.doc = nil
	modelsDev.mu.Unlock()
	if l, ok := ReferenceLimits(context.Background(), "anthropic", "claude-sonnet-5"); !ok || l.Context != 1000000 || *hits != 0 {
		t.Fatalf("the disk copy answers, still no fetch: %+v %v hits=%d", l, ok, *hits)
	}
}

// A provider's list that states no window gets the reference's.
func TestModelsOfFillsTheWindowFromTheReference(t *testing.T) {
	clearProviderEnv(t)
	referenceServer(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"id":"claude-sonnet-5","display_name":"Claude Sonnet 5"},{"id":"claude-mystery-9"}]}`)
	}))
	defer srv.Close()
	list, err := ModelsOf(context.Background(), Provider{ID: "anthropic", API: APIAnthropic, Base: srv.URL, Key: "k", Context: 200000})
	if err != nil || len(list) != 2 {
		t.Fatal(err, list)
	}
	byID := map[string]Model{}
	for _, m := range list {
		byID[m.ID] = m
	}
	if m := byID["claude-sonnet-5"]; m.Context != 1000000 || m.MaxOutput != 128000 || m.InPerM != 3 || m.OutPerM != 15 {
		t.Fatalf("from the reference, price included: %+v", m)
	}
	if byID["claude-mystery-9"].Context != 200000 || byID["claude-mystery-9"].ContextSource != ContextDefault {
		t.Fatalf("unknown to the reference: the provider's default, marked a guess: %+v", byID["claude-mystery-9"])
	}
	if byID["claude-sonnet-5"].ContextSource != ContextFromReference {
		t.Fatalf("marked as the reference's: %+v", byID["claude-sonnet-5"])
	}
	ForgetAllModels()
	if _, ok := ReferenceAge(); ok {
		t.Fatal("a refresh forgets the disk copy too")
	}
}
