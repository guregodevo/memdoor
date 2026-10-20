package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/pkg/auth"
)

// registerFake is the auth service behind the register handler here: it
// keeps who registered and signs them in; the handler calls nothing else.
type registerFake struct {
	auth.Service
	registered []string
	workspaces []string
	refuse     string // when set, every Register fails with it
}

func (f *registerFake) Register(_ context.Context, email, username, _, name, workspaceID string) (*auth.User, error) {
	if f.refuse != "" {
		return nil, errors.New(f.refuse)
	}
	f.registered = append(f.registered, email)
	f.workspaces = append(f.workspaces, workspaceID)
	u := &auth.User{}
	u.ID, u.Email, u.Username, u.Name, u.WorkspaceID = "human:"+email, email, username, name, workspaceID
	return u, nil
}

func (f *registerFake) Login(_ context.Context, email, _ string) (*auth.User, string, error) {
	u := &auth.User{}
	u.Email = email
	return u, "session-for-" + email, nil
}

func postRegister(a *AuthGatewayAdapter, from, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	req.RemoteAddr = from
	a.HandleRegister(rec, req)
	return rec
}

// Anyone can sign up (Greg, 2026-10-18): a request with no invite token
// registers the account and answers signed in.
func TestRegisterHasNoInviteGate(t *testing.T) {
	fake := &registerFake{}
	a := NewAuthGatewayAdapter(fake)
	rec := postRegister(a, "198.51.100.9:1234",
		`{"email":"new@example.com","username":"newbie","password":"longenough1","name":"New"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("a sign-up with no invite: got %d %s", rec.Code, rec.Body.String())
	}
	var resp AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil || resp.Token == "" || resp.User.Email != "new@example.com" {
		t.Fatalf("the answer is the signed-in user and a token: %+v %v", resp, err)
	}
	if len(fake.registered) != 1 || fake.registered[0] != "new@example.com" {
		t.Fatalf("registered: %v", fake.registered)
	}
}

// The limiter is the only thing between the form and a script: the sixth
// sign-up from one address inside ten minutes is told to slow down, and
// is never registered.
func TestRegisterIsRateLimitedPerClient(t *testing.T) {
	fake := &registerFake{}
	a := NewAuthGatewayAdapter(fake)
	const from = "203.0.113.77:1234"
	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		last = postRegister(a, from, `{"email":"u`+string(rune('a'+i))+`@example.com","username":"u","password":"longenough1","name":"U"}`)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("sixth sign-up from one address: got %d, want 429", last.Code)
	}
	if len(fake.registered) != 5 {
		t.Fatalf("five registered before the limiter, got %d", len(fake.registered))
	}
}
