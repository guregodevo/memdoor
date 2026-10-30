package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// The gateway's settings are its operator's: on memdoor.ai (2026-10-26) a
// probe read every workspace's settings with no token, and a sign-up — admin
// of a workspace of its own — could have rewritten tool_guards for all. No
// token, the network, or a proxy in front is refused before anything is read.
func TestWorkspaceSettingsAreTheMachineOperatorsOnly(t *testing.T) {
	s := &Server{}
	signedIn := authorization.WithActorID(context.Background(), shared.ActorID("human:stranger"))
	for _, c := range []struct {
		name, method, addr, real string
		ctx                      context.Context
		want                     int
	}{
		{"anonymous read from the network", http.MethodGet, "203.0.113.9:4242", "", context.Background(), http.StatusUnauthorized},
		{"anonymous read through the proxy", http.MethodGet, "127.0.0.1:4242", "203.0.113.9", context.Background(), http.StatusUnauthorized},
		{"anonymous read on the machine", http.MethodGet, "127.0.0.1:4242", "", context.Background(), http.StatusUnauthorized},
		{"signed-in read through the proxy", http.MethodGet, "127.0.0.1:4242", "203.0.113.9", signedIn, http.StatusForbidden},
		{"signed-in write from the network", http.MethodPut, "203.0.113.9:4242", "", signedIn, http.StatusForbidden},
		{"signed-in write through the proxy", http.MethodPut, "127.0.0.1:4242", "203.0.113.9", signedIn, http.StatusForbidden},
	} {
		req := httptest.NewRequest(c.method, "/api/workspace/settings", strings.NewReader(`{"tool_guards":"[]"}`))
		req = req.WithContext(c.ctx)
		req.RemoteAddr = c.addr
		if c.real != "" {
			req.Header.Set("X-Real-IP", c.real)
		}
		rec := httptest.NewRecorder()
		s.handleWorkspaceSettings(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, rec.Code, rec.Body.String(), c.want)
		}
	}
}
