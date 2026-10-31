package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memdoor/pkg/auth"
)

// loginFake signs in only with the right password.
type loginFake struct{ auth.Service }

func (loginFake) Login(_ context.Context, email, password string) (*auth.User, string, error) {
	if password != "right-password" {
		return nil, "", errors.New("invalid credentials")
	}
	u := &auth.User{}
	u.Email = email
	return u, "session-for-" + email, nil
}

func postLogin(a *AuthGatewayAdapter, remote, realIP, email, password string) int {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)))
	req.RemoteAddr = remote
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	a.HandleLogin(rec, req)
	return rec.Code
}

// Guessing one account from many addresses (memdoor.ai, 2026-10-29: 28 IPs)
// is stopped after five failures, whatever the address; the right password
// from the network is then refused too until the window passes, but the
// gateway's own machine still signs in.
func TestSignInIsLimitedPerAccountAcrossAddresses(t *testing.T) {
	a := NewAuthGatewayAdapter(loginFake{})
	const owner = "owner-limit@example.com"
	for i := 0; i < 5; i++ {
		ip := fmt.Sprintf("203.0.113.%d", 10+i)
		if code := postLogin(a, "127.0.0.1:4242", ip, owner, "guess"); code != http.StatusUnauthorized {
			t.Fatalf("guess %d from %s: %d, want 401", i+1, ip, code)
		}
	}
	if code := postLogin(a, "127.0.0.1:4242", "198.51.100.99", owner, "guess"); code != http.StatusTooManyRequests {
		t.Fatalf("the sixth guess from a fresh address: %d, want 429", code)
	}
	if code := postLogin(a, "127.0.0.1:4242", "198.51.100.98", owner, "right-password"); code != http.StatusTooManyRequests {
		t.Fatalf("the network waits out the window: %d, want 429", code)
	}
	if code := postLogin(a, "127.0.0.1:4242", "", owner, "right-password"); code != http.StatusOK {
		t.Fatalf("the gateway's own machine still signs in: %d, want 200", code)
	}
}

// Only failures add up: a sign-in that succeeds refills the account's tries.
func TestASuccessfulSignInRefillsTheAccount(t *testing.T) {
	a := NewAuthGatewayAdapter(loginFake{})
	const person = "person-limit@example.com"
	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			if code := postLogin(a, "127.0.0.1:4242", fmt.Sprintf("192.0.2.%d", round*10+i), person, "typo"); code != http.StatusUnauthorized {
				t.Fatalf("round %d typo %d: %d", round, i, code)
			}
		}
		if code := postLogin(a, "127.0.0.1:4242", fmt.Sprintf("192.0.2.%d", round*10+9), person, "right-password"); code != http.StatusOK {
			t.Fatalf("round %d: the right password after four typos: %d, want 200", round, code)
		}
	}
}
