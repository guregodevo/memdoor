package gemini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"memdoor/pkg/decision"
)

func mustReq(t *testing.T) decision.Request {
	t.Helper()
	b, _ := decision.Boolean("Needs a shell?", "", "")
	c, _ := decision.Choice("Which tool?", []decision.Option{{Label: "bash", Description: "Run a command."}, {Label: "read", Description: "Read a file."}})
	s, _ := decision.Score("How severe?", []string{"cosmetic", "blocked"})
	return decision.Request{State: "run the tests", Questions: map[string]decision.Question{"shell": b, "tool": c, "sev": s}}
}

func TestEvaluateSendsTheDocumentedRequestAndReadsTheAnswers(t *testing.T) {
	var gotKey, gotPath string
	var got wireRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotPath = r.Header.Get("x-goog-api-key"), r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		_, _ = w.Write([]byte(`{"answers":{
			"shell":{"type":"noul","noul":0.97,"confidence":0.97},
			"tool":{"type":"choice","choice":"bash","probabilities":{"bash":0.99,"read":0.01},"confidence":0.99},
			"sev":{"type":"score","score":0.8,"probabilities":{"0":0.2,"1":0.8},"confidence":0.8}},
			"usage":{"total_input_tokens":10,"total_output_tokens":0,"total_tokens":10}}`))
	}))
	defer srv.Close()
	c, err := New(Config{BaseURL: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Evaluate(context.Background(), "", mustReq(t))
	if err != nil || !res.OK() {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if gotKey != "k" || gotPath != "/v1beta/decisions" || got.Model != DefaultModel || got.Input != "run the tests" {
		t.Fatalf("request: key=%q path=%q %+v", gotKey, gotPath, got)
	}
	if got.Questions["shell"].Type != "noul" || got.Questions["tool"].Type != "choice" || got.Questions["sev"].Type != "score" {
		t.Fatalf("question types: %+v", got.Questions)
	}
	a := res.Answers
	if a["shell"].ProbabilityTrue != 0.97 || a["tool"].Choice != "bash" || a["sev"].Score != 0.8 || a["sev"].ScoreProbabilities[1] != 0.8 {
		t.Fatalf("answers: %+v", a)
	}
}

func TestEvaluateClassifiesFailures(t *testing.T) {
	for code, want := range map[int]decision.Reason{
		400: decision.ReasonUnsupportedInput,
		429: decision.ReasonOverloaded,
		403: decision.ReasonNotConfigured,
		500: decision.ReasonProviderError,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		c, _ := New(Config{BaseURL: srv.URL, APIKey: "k"})
		res, err := c.Evaluate(context.Background(), "", mustReq(t))
		srv.Close()
		if err != nil || res.Reason != want {
			t.Fatalf("HTTP %d: reason %q err %v, want %q", code, res.Reason, err, want)
		}
		if code == 403 && c.Ready() {
			t.Fatal("a refused key must stand down")
		}
		if code != 403 && !c.Ready() {
			t.Fatalf("HTTP %d must not stand the client down", code)
		}
	}
}

func TestNewNeedsAKey(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("no key accepted")
	}
}
