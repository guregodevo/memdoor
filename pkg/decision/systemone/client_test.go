package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"memdoor/pkg/attribution"
	"memdoor/pkg/decision"
)

func req(t *testing.T) decision.Request {
	t.Helper()
	dept, _ := decision.Choice("Which team?", []decision.Option{{Label: "billing", Description: "Payments"}, {Label: "tech", Description: "Bugs"}})
	refund, _ := decision.Boolean("Asking for money back?", "", "")
	anger, _ := decision.Score("How angry?", []string{"calm", "annoyed", "furious"})
	return decision.Request{State: "charged twice", Questions: map[string]decision.Question{"dept": dept, "refund": refund, "anger": anger}}
}

func TestConfigRules(t *testing.T) {
	if _, err := New(Config{BaseURL: "https://api.typesafe.ai"}); err == nil {
		t.Fatal("remote without key accepted")
	}
	c, err := New(Config{BaseURL: "http://127.0.0.1:8009/"})
	if err != nil || !c.Ready() || c.ID() != ProviderID {
		t.Fatalf("local without key: %v ready=%v", err, c != nil && c.Ready())
	}
	if _, err := New(Config{BaseURL: "ftp://x"}); err == nil {
		t.Fatal("non-http URL accepted")
	}
	if c, _ := New(Config{BaseURL: "https://api.typesafe.ai/", APIKey: "k"}); c.Endpoint() != "https://api.typesafe.ai/v1/systemone" {
		t.Fatalf("origin should get /v1/systemone appended: %s", c.Endpoint())
	}
	if c, _ := New(Config{BaseURL: "https://openrouter.ai/api/alpha/decisions", APIKey: "k"}); c.Endpoint() != "https://openrouter.ai/api/alpha/decisions" {
		t.Fatalf("full endpoint should be used verbatim: %s", c.Endpoint())
	}
}

func TestRoundTrip(t *testing.T) {
	var got wireRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("path %s auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "jev-1.13.0",
			"answers": map[string]any{
				"dept":   map[string]any{"type": "choice", "choice": "billing", "probabilities": map[string]float64{"billing": 0.9, "tech": 0.1}, "confidence": 0.9},
				"refund": map[string]any{"type": "noul", "noul": 0.8},
				"anger":  map[string]any{"type": "score", "score": 1.4, "probabilities": map[string]float64{"0": 0.1, "1": 0.4, "2": 0.5}, "confidence": 0.5},
			},
			"usage": map[string]int{"input_tokens": 10, "output_tokens": 0},
		})
	}))
	defer srv.Close()
	c, err := New(Config{BaseURL: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Evaluate(context.Background(), "", req(t))
	if err != nil || !res.OK() {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if got.Model != DefaultModel || got.Questions["refund"].Type != "noul" || got.Questions["dept"].Type != "choice" {
		t.Fatalf("wire request: %+v", got)
	}
	if crit, _ := got.Questions["dept"].Criteria.(map[string]any); crit["billing"] != "Payments" {
		t.Fatalf("choice criteria: %+v", got.Questions["dept"].Criteria)
	}
	if a := res.Answers["dept"]; a.Choice != "billing" || a.Probabilities["tech"] != 0.1 {
		t.Fatalf("choice answer: %+v", a)
	}
	if a := res.Answers["refund"]; a.ProbabilityTrue != 0.8 || a.Confidence != 0.8 {
		t.Fatalf("boolean answer: %+v", a)
	}
	if a := res.Answers["anger"]; a.Score != 1.4 || len(a.ScoreProbabilities) != 3 || a.ScoreProbabilities[2] != 0.5 {
		t.Fatalf("score answer: %+v", a)
	}
	if res.Model != "jev-1.13.0" || res.Provider != ProviderID {
		t.Fatalf("meta: %+v", res)
	}
}

func TestHTTPStatusClassification(t *testing.T) {
	for code, want := range map[int]decision.Reason{
		422: decision.ReasonUnsupportedInput, 413: decision.ReasonUnsupportedInput,
		429: decision.ReasonOverloaded, 529: decision.ReasonOverloaded,
		402: decision.ReasonNotConfigured, 501: decision.ReasonNotConfigured,
		401: decision.ReasonNotConfigured, 403: decision.ReasonNotConfigured, 404: decision.ReasonNotConfigured,
		500: decision.ReasonProviderError,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		c, _ := New(Config{BaseURL: srv.URL, APIKey: "k"})
		res, err := c.Evaluate(context.Background(), "", req(t))
		srv.Close()
		if err != nil || res.Status != decision.StatusUnavailable || res.Reason != want {
			t.Errorf("HTTP %d -> %+v err=%v, want %s", code, res, err, want)
		}
	}
}

// If there is a Jev model, use it, otherwise don't (Greg, 2026-10-03): a key
// the endpoint refuses stands the client down, so a turn does not ask again on
// every call; a transient failure does not.
func TestARefusedKeyStandsDown(t *testing.T) {
	code := http.StatusInternalServerError
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL, APIKey: "k"})
	_, _ = c.Evaluate(context.Background(), "", req(t))
	if !c.Ready() {
		t.Fatal("a server error is transient: still ready")
	}
	code = http.StatusForbidden
	_, _ = c.Evaluate(context.Background(), "", req(t))
	if c.Ready() {
		t.Fatal("a refused key must stand down")
	}
	c.refusedAt = time.Now().Add(-RefusalHold - time.Second)
	if !c.Ready() {
		t.Fatal("after the hold it is asked again")
	}
}

func TestMissingAnswerIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "x", "answers": map[string]any{}})
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL, APIKey: "k"})
	if _, err := c.Evaluate(context.Background(), "", req(t)); err == nil {
		t.Fatal("missing answers accepted")
	}
}

// recorder answers every call from memory and keeps the headers it was asked to
// send, so the test can look at a real request without a real network.
type recorder struct {
	hdr http.Header
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.hdr = req.Header.Clone()
	body, _ := json.Marshal(map[string]any{
		"model": "jev-1.13.0",
		"answers": map[string]any{
			"dept":   map[string]any{"type": "choice", "choice": "billing", "probabilities": map[string]float64{"billing": 1}, "confidence": 1},
			"refund": map[string]any{"type": "noul", "noul": 0.5},
			"anger":  map[string]any{"type": "score", "score": 1, "probabilities": map[string]float64{"1": 1}, "confidence": 1},
		},
	})
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}, nil
}

// THE DECISION CALL COUNTS TOWARD THE APP. Someone on the free tier pays the
// decision model with their own OpenRouter key, and that call is a call we made:
// without these two headers OpenRouter credits it to nobody, and the ranking we
// are trying to climb never sees it (pkg/attribution). A local Kev or TypeSafe
// endpoint has no such ranking, so it is left alone.
func TestDecisionCallsAreAttributedOnOpenRouterOnly(t *testing.T) {
	for _, tc := range []struct {
		name, base string
		attributed bool
	}{
		{"openrouter", "https://openrouter.ai/api/alpha/decisions", true},
		{"typesafe", "https://api.typesafe.ai/v1/systemone", false},
		{"local kev", "http://127.0.0.1:8009/v1/systemone", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			c, err := New(Config{BaseURL: tc.base, APIKey: "k", HTTPClient: &http.Client{Transport: rec}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.Evaluate(context.Background(), "", req(t)); err != nil {
				t.Fatal(err)
			}
			title, referer := rec.hdr.Get("X-Title"), rec.hdr.Get("HTTP-Referer")
			if tc.attributed {
				if title != attribution.AppName || referer != attribution.AppURL {
					t.Fatalf("not attributed: X-Title=%q HTTP-Referer=%q", title, referer)
				}
				return
			}
			if title != "" || referer != "" {
				t.Fatalf("attributed a non-OpenRouter endpoint: X-Title=%q HTTP-Referer=%q", title, referer)
			}
		})
	}
}
