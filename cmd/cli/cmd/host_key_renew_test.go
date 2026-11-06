package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// A lapsed session whose generated login the gateway refuses is renewed
// from the gateway's host key, with no password; the key is sent to the
// gateway on this machine only.
func TestALapsedSessionRenewsFromTheHostKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })

	var keySeen, emailSeen string
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			http.Error(w, `{"error": "invalid email or password"}`, http.StatusUnauthorized)
		case "/api/auth/host-login":
			var req struct{ Key, Email string }
			_ = json.NewDecoder(r.Body).Decode(&req)
			keySeen, emailSeen = req.Key, req.Email
			if req.Key != "the-key" {
				http.Error(w, `{"error": "that is not this gateway's host key"}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "fresh", "user": map[string]string{"email": "first@test.local"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer gw.Close()
	gatewayAddr = strings.Replace(gw.URL, "127.0.0.1", "localhost", 1)

	if err := saveCredentials(&credentials{Token: "old", Email: "hand@test.local", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(homePath(localLoginFile), []byte(`{"email":"me@localhost","password":"p"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	// No key on disk: the refusal stands, and the key is never asked for.
	if err := renewLocalSession(); err == nil || !strings.Contains(err.Error(), "refused") || keySeen != "" {
		t.Fatalf("without a key: want the login's refusal, got %v (key sent %q)", err, keySeen)
	}

	if err := os.WriteFile(homePath(hostKeyFile), []byte("the-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := renewLocalSession(); err != nil {
		t.Fatalf("with the key: %v", err)
	}
	if keySeen != "the-key" || emailSeen != "hand@test.local" {
		t.Fatalf("the key and the signed-in account are sent: key %q email %q", keySeen, emailSeen)
	}
	creds, err := loadCredentials()
	if err != nil || creds.Token != "fresh" || creds.Email != "first@test.local" || time.Until(creds.ExpiresAt) < 24*time.Hour {
		t.Fatalf("the new session is saved: %+v %v", creds, err)
	}
	if sessionProblem() != nil {
		t.Fatalf("the renewed session is live: %v", sessionProblem())
	}

	// A gateway elsewhere never sees this machine's key.
	keySeen = ""
	gatewayAddr = strings.Replace(gw.URL, "127.0.0.1", "box.example", 1)
	_ = renewLocalSession()
	if keySeen != "" {
		t.Fatalf("the key was sent to a gateway on another machine")
	}
}

// What a login shell prints around the key over ssh is not the key.
func TestHostKeyInShellOutput(t *testing.T) {
	key := strings.Repeat("ab", 32)
	if got := hostKeyIn("Welcome to the box\n" + key + "\n\nSession ended\n"); got != key {
		t.Fatalf("got %q", got)
	}
	if got := hostKeyIn("cat: ~/.memdoor/host-key: No such file\n"); got != "" {
		t.Fatalf("no key: got %q", got)
	}
}
