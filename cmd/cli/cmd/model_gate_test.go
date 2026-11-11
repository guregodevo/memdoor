package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A command that needs a model (workflow try, the editor door) says how to
// connect one when the gateway has none, in the window's words, and says
// nothing when a model answers.
func TestModelGateNamesTheWayToAModel(t *testing.T) {
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	state := "none"
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/llm/burst" {
			_, _ = io.WriteString(w, `{"brain":{"state":"`+state+`"}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer gw.Close()
	gatewayAddr = gw.URL

	err := modelGate(NewClient())
	if err == nil || err.Error() != brainGate("none") || !strings.Contains(err.Error(), "openrouter-login") {
		t.Fatalf("no model: %v", err)
	}
	state = "ready"
	if err := modelGate(NewClient()); err != nil {
		t.Fatalf("a model answers: %v", err)
	}
}
