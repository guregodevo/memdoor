package providers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// hostsTransport answers api.aimlapi.com with a redirect to next and records
// what every other host was sent.
type hostsTransport struct {
	next string
	got  http.Header
	host string
}

func (h *hostsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == "api.aimlapi.com" && h.next != "" {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {h.next}},
			Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}
	h.got, h.host = r.Header.Clone(), r.URL.Host
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: r}, nil
}

// A redirect off api.aimlapi.com carries neither X-AIMLAPI-Source nor the
// key to the host it points at, a subdomain included; one that stays on the
// host keeps both. Both clients that talk to it: the model list and a turn.
func TestAIMLAPIRedirectKeepsKeyAndSourceOnItsHost(t *testing.T) {
	t.Setenv("MEMDOOR_OPENROUTER_ATTRIBUTION", "")
	t.Setenv("MEMDOOR_VENDOR_HEADERS", "")
	p := Provider{ID: VendorAIMLAPI, API: APIChat, Base: aimlapiDefaultBase, Key: "aiml-secret"}
	cases := map[string]bool{
		"https://evil.example/v1/models":           false,
		"https://x.api.aimlapi.com/v1/models":      false,
		"https://api.aimlapi.com.evil.example/v1/": false,
		"https://api.aimlapi.com/v1/models?moved":  true,
	}
	for to, keeps := range cases {
		for name, client := range map[string]*http.Client{"list": newListClient(), "turn": oaiHTTPClient()} {
			tr := &hostsTransport{next: to}
			if keeps {
				client.Transport = &sameHostOnce{inner: tr}
			} else {
				client.Transport = tr
			}
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, modelsURL(p.Base), nil)
			for k, v := range bearer(p) {
				req.Header.Set(k, v)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s → %s: %v", name, to, err)
			}
			resp.Body.Close()
			src, auth := tr.got.Get("X-AIMLAPI-Source") != "", tr.got.Get("Authorization") != ""
			if src != keeps || auth != keeps {
				t.Errorf("%s → %s (%s): X-AIMLAPI-Source %v, Authorization %v, want %v", name, to, tr.host, src, auth, keeps)
			}
		}
	}
}

// sameHostOnce redirects the first request to inner.next on the same host,
// then lets inner record the second.
type sameHostOnce struct {
	inner *hostsTransport
	done  bool
}

func (s *sameHostOnce) RoundTrip(r *http.Request) (*http.Response, error) {
	if !s.done {
		s.done = true
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {s.inner.next}},
			Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	}
	s.inner.next = ""
	return s.inner.RoundTrip(r)
}
