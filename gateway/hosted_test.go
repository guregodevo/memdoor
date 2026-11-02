package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MEMDOOR_HOSTED=off: a route of the hosted services answers 410 and says
// why; unset, the handler behind it answers as before.
func TestHostedOffAnswersGone(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	t.Setenv("MEMDOOR_HOSTED", "")
	rr := httptest.NewRecorder()
	hostedOr(ok).ServeHTTP(rr, httptest.NewRequest("GET", "/api/auth/login", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("hosted on: status %d, want 200", rr.Code)
	}

	t.Setenv("MEMDOOR_HOSTED", "off")
	rr = httptest.NewRecorder()
	hostedOr(ok).ServeHTTP(rr, httptest.NewRequest("POST", "/api/auth/login", nil))
	if rr.Code != http.StatusGone || !strings.Contains(rr.Body.String(), "is off") {
		t.Fatalf("hosted off: status %d body %q, want 410 and the reason", rr.Code, rr.Body.String())
	}
}

// Off, every served page carries the tag the site reads; on, the page is
// the build's own.
func TestHostedOffMarksThePageHead(t *testing.T) {
	page := []byte("<html><head><title>x</title></head><body></body></html>")
	t.Setenv("MEMDOOR_HOSTED", "")
	if got := renderIndex(page, pageMeta{Path: "/"}); string(got) != string(page) {
		t.Fatalf("hosted on: the page changed: %s", got)
	}
	t.Setenv("MEMDOOR_HOSTED", "off")
	if got := renderIndex(page, pageMeta{Path: "/"}); !strings.Contains(string(got), hostedMeta) {
		t.Fatalf("hosted off: the home must carry the tag: %s", got)
	}
	if got := renderIndex(page, pageMeta{Path: "/docs", Title: "d", Description: "d"}); !strings.Contains(string(got), hostedMeta) {
		t.Fatalf("hosted off: a doc page must carry the tag: %s", got)
	}
}
