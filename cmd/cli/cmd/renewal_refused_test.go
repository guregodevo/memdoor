package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// A generated login the gateway refuses is named in the lapsed session's
// message; no generated login at all leaves the message as it was.
func TestRefusedGeneratedLoginIsNamed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "invalid email or password"}`, http.StatusUnauthorized)
	}))
	defer gw.Close()
	gatewayAddr = strings.Replace(gw.URL, "127.0.0.1", "localhost", 1)

	if err := saveCredentials(&credentials{Token: "tok", Email: "hand@test.local", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}

	none := renewLocalSession()
	if got := withRefusedRenewal(sessionProblem(), none); strings.Contains(got.Error(), "login this machine generated") {
		t.Fatalf("no generated login on disk: the message must not mention one, got %v", got)
	}

	if err := os.WriteFile(homePath(localLoginFile), []byte(`{"email":"me@localhost","password":"p"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := withRefusedRenewal(sessionProblem(), renewLocalSession())
	for _, want := range []string{"expired", "login-direct --email hand@test.local", "me@localhost", "refused", "invalid email or password"} {
		if !strings.Contains(got.Error(), want) {
			t.Fatalf("want %q in the message, got %v", want, got)
		}
	}
}
