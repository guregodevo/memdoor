package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunDoneTellsTheCompanyGatewayTheRunIsOver(t *testing.T) {
	got := make(chan *http.Request, 1)
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r
		w.WriteHeader(http.StatusOK)
	}))
	defer gw.Close()
	if err := postRunDone(gw.URL+"/v1", "tok", "workflow:lcb:2026-11-28T113439@20261128-113439-2"); err != nil {
		t.Fatal(err)
	}
	r := <-got
	if r.Method != http.MethodPost || r.URL.Path != "/v1/run/done" {
		t.Fatalf("%s %s", r.Method, r.URL.Path)
	}
	if run := r.URL.Query().Get("run"); run != "workflow:lcb:2026-11-28T113439@20261128-113439-2" {
		t.Fatalf("run query %q", run)
	}
	if h := r.Header.Get("X-Memdoor-Run"); h != "workflow:lcb:2026-11-28T113439@20261128-113439-2" {
		t.Fatalf("X-Memdoor-Run %q", h)
	}
	if a := r.Header.Get("Authorization"); a != "Bearer tok" {
		t.Fatalf("Authorization %q", a)
	}
}

func TestRunDoneSendsNothingWithoutACompanyGateway(t *testing.T) {
	t.Setenv("AI_GATEWAY_BASE_URL", "")
	RunDone("run-1") // must not panic or dial anything
}
