package cmd

import (
	"encoding/json"
	"memdoor/internal/testenv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// EVERY COMMAND RENEWS A REFUSED SESSION (#88): a request the gateway refuses
// is renewed from this machine's generated login and sent again, once; with
// no generated login (an account made by hand) the refusal stands and no
// sign-in is tried. `memdoor workflow status` answered "Authentication
// required" on a lapsed session while `memdoor run` renewed it.
func TestARefusedSessionRenewsOnceAndRetries(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	logins, calls := 0, 0
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			logins++
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "fresh", "user": map[string]string{"email": "greg@localhost"}})
		case "/api/workflows/runs":
			calls++
			if r.Header.Get("Authorization") != "Bearer fresh" {
				http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"runs": []}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer gw.Close()
	gatewayAddr = strings.Replace(gw.URL, "127.0.0.1", "localhost", 1)

	lapsed := func() {
		t.Helper()
		if err := saveCredentials(&credentials{Token: "old", Email: "greg@localhost", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}

	// An account made by hand: nothing on disk renews it.
	lapsed()
	var out map[string]any
	if err := NewClient().GetJSON("/api/workflows/runs", &out); err == nil || logins != 0 {
		t.Fatalf("no generated login: the refusal stands and no sign-in is tried (err=%v, logins=%d)", err, logins)
	}

	// The machine's own user: renewed once, the request answered.
	rec, _ := json.Marshal(localLogin{Email: "greg@localhost", Password: "generated"})
	if err := writeHomeFile(localLoginFile, rec, 0o600); err != nil {
		t.Fatal(err)
	}
	lapsed()
	calls = 0
	if err := NewClient().GetJSON("/api/workflows/runs", &out); err != nil {
		t.Fatalf("a refused session renews and the request is answered: %v", err)
	}
	if logins != 1 || calls != 2 {
		t.Fatalf("one renewal and one retry: logins=%d calls=%d", logins, calls)
	}
	creds, err := loadCredentials()
	if err != nil || creds.Token != "fresh" || creds.ExpiresAt.Before(time.Now().Add(365*24*time.Hour)) {
		t.Fatalf("the renewed session is saved, with no 30-day end: %+v %v", creds, err)
	}
}

// No session saved at all (no ~/.memdoor/credentials.json): a gateway on
// this machine is signed in to from this machine's own login, and the request
// answered. `memdoor logs` answered "authentication required" with the
// generated login beside it (2026-11-12). With nothing on disk to sign in
// with, the refusal stands.
func TestNoSavedSessionSignsInFromThisMachine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	logins, calls := 0, 0
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			logins++
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "fresh", "user": map[string]string{"email": "greg@localhost"}})
		case "/api/logs/query":
			calls++
			if r.Header.Get("Authorization") != "Bearer fresh" {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"events": []}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer gw.Close()
	gatewayAddr = strings.Replace(gw.URL, "127.0.0.1", "localhost", 1)

	var out map[string]any
	if err := NewClient().GetJSON("/api/logs/query", &out); err == nil || logins != 0 {
		t.Fatalf("nothing to sign in with: the refusal stands (err=%v, logins=%d)", err, logins)
	}

	rec, _ := json.Marshal(localLogin{Email: "greg@localhost", Password: "generated"})
	if err := writeHomeFile(localLoginFile, rec, 0o600); err != nil {
		t.Fatal(err)
	}
	calls = 0
	if err := NewClient().GetJSON("/api/logs/query", &out); err != nil {
		t.Fatalf("no session saved: signed in from this machine and answered: %v", err)
	}
	if logins != 1 || calls != 2 {
		t.Fatalf("one sign-in and one retry: logins=%d calls=%d", logins, calls)
	}
	if creds, err := loadCredentials(); err != nil || creds.Token != "fresh" {
		t.Fatalf("the session is saved: %+v %v", creds, err)
	}
}
