package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Anyone can sign up (Greg, 2026-10-18): a register request with no invite
// token is not refused by the gate. The service behind it is nil here, so
// the request that passes the gate panics inside the service call; the
// gate itself answers before that, and a refusal is what this test fails on.
func TestRegisterHasNoInviteGate(t *testing.T) {
	a := &AuthGatewayAdapter{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register",
		strings.NewReader(`{"email":"new@example.com","username":"newbie","password":"longenough1","name":"New"}`))
	func() {
		defer func() { recover() }()
		a.HandleRegister(rec, req)
	}()
	if rec.Code == http.StatusForbidden {
		t.Fatalf("a sign-up with no invite was refused: %s", rec.Body.String())
	}
}

// The limiter is the only thing between the form and a script: the sixth
// sign-up from one address inside ten minutes is told to slow down.
func TestRegisterIsRateLimitedPerClient(t *testing.T) {
	a := &AuthGatewayAdapter{}
	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		last = httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{}`))
		req.RemoteAddr = "203.0.113.77:1234"
		func() {
			defer func() { recover() }()
			a.HandleRegister(last, req)
		}()
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("sixth sign-up from one address: got %d, want 429", last.Code)
	}
}
