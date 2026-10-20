package authorization

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"memdoor/pkg/shared"
)

// A tool call from inside the gateway names the run's workspace and is
// scoped to it; without one it sees the whole gateway (2026-10-20).
func TestAnInternalCallIsScopedToTheWorkspaceItNames(t *testing.T) {
	m := NewAuthMiddleware(nil)
	var seen string
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = shared.WorkspaceFromContext(r.Context())
	}))
	for _, c := range []struct{ header, want string }{{"bob", "bob"}, {"", ""}} {
		req := httptest.NewRequest("GET", "/api/channels", nil)
		req.RemoteAddr = "127.0.0.1:4242"
		req.Header.Set("X-Internal-Service", "agents")
		if c.header != "" {
			req.Header.Set("X-Forwarded-Workspace", c.header)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		if seen != c.want {
			t.Fatalf("header %q: workspace %q, want %q", c.header, seen, c.want)
		}
	}
}
