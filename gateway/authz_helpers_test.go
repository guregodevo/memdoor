package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"memdoor/pkg/authorization"
)

func realAuthz() *authorization.Service {
	return authorization.NewService(&mockMembershipRepo{}, &mockChannelRepo{}, &mockBuddyRepo{}, nil)
}

func TestRequireAdmin(t *testing.T) {
	authz := realAuthz()

	t.Run("anonymous is 401", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/x", nil)
		if requireAdmin(rr, req, authz) {
			t.Error("anonymous request was allowed")
		}
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("code = %d, want 401", rr.Code)
		}
	})

	t.Run("non-admin is 403", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := authenticatedRequest("POST", "/x", nil, "guest:mallory")
		if requireAdmin(rr, req, authz) {
			t.Error("non-admin request was allowed")
		}
		if rr.Code != http.StatusForbidden {
			t.Errorf("code = %d, want 403", rr.Code)
		}
	})

	t.Run("system:internal is admin", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := authenticatedRequest("POST", "/x", nil, "system:internal")
		if !requireAdmin(rr, req, authz) {
			t.Errorf("system:internal was denied; code = %d", rr.Code)
		}
	})

	t.Run("nil authz service bypasses (test-harness convention)", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := authenticatedRequest("POST", "/x", nil, "guest:mallory")
		if !requireAdmin(rr, req, nil) {
			t.Error("nil authz should bypass the admin check")
		}
	})
}

// isInternalActor must be driven by the middleware-set context actor, NOT by
// the raw X-Internal-Service header — otherwise a remote attacker forging the
// header would be treated as internal (the email/invite open-relay bug).
func TestIsInternalActor(t *testing.T) {
	t.Run("forged raw header alone is not internal", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/email/send", nil)
		req.Header.Set("X-Internal-Service", "1") // attacker-supplied, no loopback context
		if isInternalActor(req) {
			t.Error("a forged X-Internal-Service header was treated as internal")
		}
	})

	t.Run("system:internal context actor is internal", func(t *testing.T) {
		// This actor is only ever set by AuthMiddleware for loopback calls.
		req := authenticatedRequest("POST", "/api/email/send", nil, "system:internal")
		if !isInternalActor(req) {
			t.Error("system:internal context actor was not recognized as internal")
		}
	})

	t.Run("ordinary authenticated user is not internal", func(t *testing.T) {
		req := authenticatedRequest("POST", "/api/email/send", nil, "human:alice")
		if isInternalActor(req) {
			t.Error("a normal user was treated as internal")
		}
	})
}
