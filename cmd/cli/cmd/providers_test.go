package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A bare pin that two connected providers list says where it went and how
// to pin it on the other (live 2026-10-10: the plan's model went to the API key).
func TestABarePinListedTwiceNamesTheOtherProvider(t *testing.T) {
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"providers":[{"id":"openai","connected":true,"models":[{"id":"gpt-5.6-sol"}]},{"id":"chatgpt","connected":true,"models":[{"id":"gpt-5.6-sol"},{"id":"gpt-6.1-sol"}]},{"id":"groq","connected":false,"models":[{"id":"gpt-5.6-sol"}]}]}`)
	}))
	defer srv.Close()
	gatewayAddr = srv.URL
	if got := alsoListedBy("gpt-5.6-sol", "openai"); !strings.Contains(got, "Served by openai") || !strings.Contains(got, "/model chatgpt:gpt-5.6-sol") || strings.Contains(got, "groq") {
		t.Fatalf("the other connected provider is named, a disconnected one is not: %q", got)
	}
	if got := alsoListedBy("gpt-6.1-sol", "chatgpt"); got != "" {
		t.Fatalf("an id one provider lists says nothing: %q", got)
	}
	if got := alsoListedBy("chatgpt:gpt-5.6-sol", "chatgpt"); got != "" {
		t.Fatalf("a prefixed pin is already explicit: %q", got)
	}
}
