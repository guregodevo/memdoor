package providers

import (
	"context"
	"errors"
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

// The guard replaces Go's redirect policy, so it keeps Go's limit, and says
// it in words: the host and what to check, no Go error and no URL.
func TestARedirectLoopStopsAtTenAndSaysSo(t *testing.T) {
	hops := make([]string, 20)
	for i := range hops {
		hops[i] = "https://api.anthropic.com/v1/messages"
	}
	client := providerHTTPClient(&http.Client{Transport: &redirectChain{hops: hops}, Timeout: 5 * time.Second})
	resp, err := client.Post("https://api.anthropic.com/v1/messages", "application/json", strings.NewReader("{}"))
	if err == nil {
		resp.Body.Close()
		t.Fatal("20 redirects were followed")
	}
	loop := redirectLoop(err)
	if loop == nil {
		t.Fatalf("not a redirect loop: %v", err)
	}
	want := "api.anthropic.com answered with a redirect 10 times in a row and never with a reply; check the provider's base URL"
	if loop.Error() != want {
		t.Fatalf("got  %q\nwant %q", loop, want)
	}
	if redirectLoop(errors.New("connection refused")) != nil {
		t.Fatal("another failure was read as a redirect loop")
	}
}

// A refusal from a host the request was redirected to says so: the person's
// key is set, and "no key was sent" alone does not tell them why. A refusal
// from the provider's own host, redirected on it or not, is left as it is.
func TestARefusalAfterARedirectOffTheHostSaysSo(t *testing.T) {
	refused := errors.New("HTTP 401 from the vendor: no API key was sent")
	cases := []struct {
		name string
		hops []string
		want string
	}{
		{"no redirect", nil, refused.Error()},
		{"same host", []string{"https://api.anthropic.com/v2/messages"}, refused.Error()},
		{"another host", []string{"https://evil.example/v1/messages"},
			refused.Error() + " (api.anthropic.com redirected the request to evil.example; the key is not sent to another host)"},
		{"two hops", []string{"https://api.anthropic.com/v2/messages", "https://x.api.anthropic.com/v2/messages"},
			refused.Error() + " (api.anthropic.com redirected the request to x.api.anthropic.com; the key is not sent to another host)"},
	}
	for _, tc := range cases {
		client := providerHTTPClient(&http.Client{Transport: &redirectChain{hops: append([]string(nil), tc.hops...)}})
		resp, err := client.Post("https://api.anthropic.com/v1/messages", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		resp.Body.Close()
		got := noteRedirect(resp, refused)
		if got.Error() != tc.want {
			t.Errorf("%s:\ngot  %q\nwant %q", tc.name, got, tc.want)
		}
		if !errors.Is(got, refused) {
			t.Errorf("%s: the refusal is no longer in the error", tc.name)
		}
	}
	if noteRedirect(nil, refused) != refused || noteRedirect(&http.Response{}, nil) != nil {
		t.Error("a missing response or a nil error changed the result")
	}
}
