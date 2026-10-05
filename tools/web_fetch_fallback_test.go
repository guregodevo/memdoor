package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A site that refuses the plain client is read through the browser.
func TestWebFetchFallsBackToTheBrowserOn403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := browserPageText
	browserPageText = func(url string) (string, error) { return "The article, as a browser sees it.", nil }
	t.Cleanup(func() { browserPageText = old })
	in, _ := json.Marshal(WebFetchInput{URL: srv.URL + "/story"})
	out, err := WebFetch(in)
	if err != nil {
		t.Fatalf("a 403 must be read through the browser, got %v", err)
	}
	if !strings.Contains(out, "as a browser sees it") {
		t.Fatalf("got %q", out)
	}
	browserPageText = func(url string) (string, error) { return "", nil }
	if _, err := WebFetch(in); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("with nothing from the browser the refusal stands: %v", err)
	}
}
