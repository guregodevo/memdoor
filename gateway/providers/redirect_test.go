package providers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// redirectChain answers each request with a redirect to the next address in
// hops, then 200, and records the headers the last request carried.
type redirectChain struct {
	hops []string
	got  http.Header
}

func (c *redirectChain) RoundTrip(r *http.Request) (*http.Response, error) {
	if len(c.hops) > 0 {
		next := c.hops[0]
		c.hops = c.hops[1:]
		return &http.Response{StatusCode: http.StatusTemporaryRedirect, Header: http.Header{"Location": {next}},
			Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}
	c.got = r.Header.Clone()
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: r}, nil
}

// A redirect that stays on the host keeps every header. One that leaves it,
// to a subdomain or to plain http too, keeps the content headers only: no
// key under any name, no app header, no company header. For every provider
// and for each client that sends a key.
func TestARedirectOffTheHostLeavesTheKeyBehind(t *testing.T) {
	secret := []string{"Authorization", "x-api-key", "anthropic-version", "X-AIMLAPI-Source", "HTTP-Referer", "X-Title", "X-Acme-Cost-Center"}
	kept := []string{"Content-Type", "Accept", "User-Agent"}
	cases := []struct {
		name  string
		from  string
		hops  []string
		keeps bool
	}{
		{"same host", "https://api.aimlapi.com/v1/models", []string{"https://api.aimlapi.com/v2/models"}, true},
		{"same host, other path", "https://api.anthropic.com/v1/messages", []string{"https://api.anthropic.com/v1/messages?moved"}, true},
		{"http to https", "http://gateway.acme.example/v1/responses", []string{"https://gateway.acme.example/v1/responses"}, true},
		{"another host", "https://api.anthropic.com/v1/messages", []string{"https://evil.example/v1/messages"}, false},
		{"a subdomain", "https://api.aimlapi.com/v1/models", []string{"https://x.api.aimlapi.com/v1/models"}, false},
		{"a parent domain", "https://api.openai.com/v1/models", []string{"https://openai.com/v1/models"}, false},
		{"a look-alike", "https://openrouter.ai/api/v1/models", []string{"https://openrouter.ai.evil.example/api/v1/models"}, false},
		{"https to http", "https://api.openai.com/v1/models", []string{"http://api.openai.com/v1/models"}, false},
		{"away and back", "https://openrouter.ai/api/v1/models", []string{"https://evil.example/", "https://openrouter.ai/api/v1/models"}, false},
	}
	clients := map[string]func() *http.Client{
		"turn":    oaiHTTPClient,
		"keyless": keylessHTTPClient,
		"list":    newListClient,
		"other":   func() *http.Client { return providerHTTPClient(&http.Client{}) },
	}
	for _, tc := range cases {
		for name, newClient := range clients {
			chain := &redirectChain{hops: append([]string(nil), tc.hops...)}
			client := newClient()
			client.Transport = chain
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, tc.from, strings.NewReader("{}"))
			if err != nil {
				t.Fatal(err)
			}
			for _, h := range append(append([]string(nil), secret...), kept...) {
				req.Header.Set(h, "value")
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s, %s: %v", tc.name, name, err)
			}
			resp.Body.Close()
			for _, h := range secret {
				if got := chain.got.Get(h) != ""; got != tc.keeps {
					t.Errorf("%s, %s: %s sent %v, want %v", tc.name, name, h, got, tc.keeps)
				}
			}
			for _, h := range kept {
				if chain.got.Get(h) == "" {
					t.Errorf("%s, %s: %s was dropped", tc.name, name, h)
				}
			}
		}
	}
}

// The guard replaces Go's redirect policy, so it keeps Go's limit.
func TestARedirectLoopStopsAtTen(t *testing.T) {
	hops := make([]string, 20)
	for i := range hops {
		hops[i] = "https://api.aimlapi.com/v1/models"
	}
	client := providerHTTPClient(&http.Client{Transport: &redirectChain{hops: hops}, Timeout: 5 * time.Second})
	resp, err := client.Get("https://api.aimlapi.com/v1/models")
	if err == nil {
		resp.Body.Close()
		t.Fatal("20 redirects were followed")
	}
	if !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("got %v", err)
	}
}
