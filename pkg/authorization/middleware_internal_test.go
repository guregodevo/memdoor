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
		req.Header.Set("X-Internal-Service", shared.InternalServiceToken())
		if c.header != "" {
			req.Header.Set("X-Forwarded-Workspace", c.header)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		if seen != c.want {
			t.Fatalf("header %q: workspace %q, want %q", c.header, seen, c.want)
		}
	}
}

// Loopback alone is not the gateway: any other program on the machine (a
// sign-up's coder's bash, a hosted /remote program) sent X-Internal-Service
// with any value and was "system:internal" to every route until 2026-10-30.
func TestALoopbackCallerWithoutTheProcessTokenIsNotInternal(t *testing.T) {
	m := NewAuthMiddleware(nil)
	var actor shared.ActorID
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor = GetActorID(r.Context())
	}))
	for _, v := range []string{"agents", "1", "ops", shared.InternalServiceToken() + "x"} {
		actor = ""
		req := httptest.NewRequest("GET", "/api/admin/users", nil)
		req.RemoteAddr = "127.0.0.1:4242"
		req.Header.Set("X-Internal-Service", v)
		h.ServeHTTP(httptest.NewRecorder(), req)
		if actor != "" {
			t.Fatalf("X-Internal-Service %q from loopback became %q", v, actor)
		}
	}
	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	req.RemoteAddr = "203.0.113.9:4242"
	req.Header.Set("X-Internal-Service", shared.InternalServiceToken())
	h.ServeHTTP(httptest.NewRecorder(), req)
	if actor != "" {
		t.Fatalf("the token from the network became %q", actor)
	}
}
