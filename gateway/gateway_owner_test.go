package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// A conversation's key is the caller's when it names the caller's own
// workspace; any other key is the gateway's own workspace's, never a
// stranger's (a sign-up started turns on any conversation by its key until
// 2026-10-30).
func TestSessionIsCallersOnlyInTheirOwnWorkspace(t *testing.T) {
	person := func(ws, slug string) context.Context {
		ctx := authorization.WithActorID(context.Background(), shared.ActorID("human:p"))
		return shared.WithExecutionContext(ctx, &shared.ExecutionContext{ActorID: "human:p", WorkspaceID: ws, WorkspaceSlug: slug})
	}
	for _, c := range []struct {
		name string
		ctx  context.Context
		key  string
		want bool
	}{
		{"own workspace by id", person("ws-a", "a"), "workspace:ws-a:channel:c1", true},
		{"own workspace by slug", person("ws-a", "a"), "workspace:a:channel:c1", true},
		{"another workspace", person("ws-a", "a"), "workspace:b:channel:c1", false},
		{"a bare key, no gateway workspace to be in", person("ws-a", "a"), "session_main", false},
		{"no context at all", authorization.WithActorID(context.Background(), shared.ActorID("human:p")), "workspace:a:channel:c1", false},
		{"in-process tool call", authorization.WithActorID(context.Background(), shared.ActorID("system:internal")), "workspace:b:channel:c1", true},
	} {
		if got := sessionIsCallers(c.ctx, nil, c.key); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// Membership is the first account's workspace: no token is 401, anyone
// else 403, the in-process caller passes.
func TestGatewayMemberIsTheFirstAccountsWorkspace(t *testing.T) {
	s := &Server{}
	try := func(ctx context.Context) int {
		req := httptest.NewRequest(http.MethodGet, "/api/fs", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		if s.gatewayMember(rec, req) {
			return http.StatusOK
		}
		return rec.Code
	}
	if code := try(context.Background()); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}
	stranger := shared.WithExecutionContext(authorization.WithActorID(context.Background(), "human:s"), &shared.ExecutionContext{ActorID: "human:s", WorkspaceID: "s"})
	if code := try(stranger); code != http.StatusForbidden {
		t.Fatalf("a sign-up: %d", code)
	}
	if code := try(authorization.WithActorID(context.Background(), "system:internal")); code != http.StatusOK {
		t.Fatalf("in-process: %d", code)
	}
}
