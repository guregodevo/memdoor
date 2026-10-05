package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A stand-in for billing's GET /v1/me: "acct-good" is workspace ws-1, any
// other token is refused. calls counts what reached it.
func fakeBilling(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/me" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Header.Get("Authorization") {
		case "Bearer acct-good":
			_ = json.NewEncoder(w).Encode(map[string]any{"email": "a@b.c", "workspace": "ws-1", "plan": "pro"})
		case "Bearer acct-free":
			_ = json.NewEncoder(w).Encode(map[string]any{"email": "f@b.c", "workspace": "ws-2", "plan": "free"})
		default:
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestRelayAuthLocalTokenSkipsBilling(t *testing.T) {
	billing, calls := fakeBilling(t)
	auth := newRelayAuth(relayUsers{"tok-a": "alice"}, billing.URL)
	if id, err := auth.ValidateToken("tok-a"); err != nil || id != "alice" {
		t.Fatalf("local token: %q %v", id, err)
	}
	if calls.Load() != 0 {
		t.Fatalf("a gateway token must not reach billing: %d calls", calls.Load())
	}
}

// The account token is the one the TUI sends: billing names it, the id is
// prefixed so it can never be a gateway user's id, and a reconnect is
// answered from the cache.
func TestRelayAuthAccountTokenThroughBilling(t *testing.T) {
	billing, calls := fakeBilling(t)
	auth := newRelayAuth(relayUsers{}, billing.URL+"/")
	for range 2 {
		if id, err := auth.ValidateToken("acct-good"); err != nil || id != "acct:ws-1" {
			t.Fatalf("account token: %q %v", id, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("billing must be asked once and cached: %d calls", calls.Load())
	}

	a := auth.(*relayAuth)
	later := time.Now().Add(relayAccountTTL + time.Second)
	a.now = func() time.Time { return later }
	if _, err := auth.ValidateToken("acct-good"); err != nil || calls.Load() != 2 {
		t.Fatalf("an expired entry must ask billing again: %v, %d calls", err, calls.Load())
	}
}

func TestRelayAuthRefusesBadTokensAndDownBilling(t *testing.T) {
	billing, calls := fakeBilling(t)
	auth := newRelayAuth(relayUsers{}, billing.URL)
	for range 2 {
		if id, err := auth.ValidateToken("acct-bad"); err == nil || id != "" {
			t.Fatalf("a refused token was let in: %q", id)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("a failure must never be cached: %d calls", calls.Load())
	}
	if _, err := auth.ValidateToken(""); err == nil {
		t.Fatal("an empty token was let in")
	}

	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	if id, err := newRelayAuth(relayUsers{}, down.URL).ValidateToken("acct-good"); err == nil || id != "" {
		t.Fatalf("an unreachable billing service let a terminal in: %q", id)
	}
}

// End to end: the relay, guarded by this validator, lets a terminal holding
// an account token attach, and refuses one billing does not know.
func TestRelayAttachesATerminalWithAnAccountToken(t *testing.T) {
	billing, _ := fakeBilling(t)
	hub := NewRemoteRelayHub()
	mux := http.NewServeMux()
	mux.Handle("/api/relay", hub.HandleRelayWS(newRelayAuth(relayUsers{}, billing.URL)))
	mux.HandleFunc("/api/relay/browser", hub.HandleRelayBrowser)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close(); hub.Shutdown() })

	if c, code := dialTerminal(t, srv, "acct-bad"); c != nil || code != http.StatusUnauthorized {
		t.Fatalf("unknown account token: %d", code)
	}
	if c, code := dialTerminal(t, srv, "acct-good"); c == nil {
		t.Fatalf("account token refused: %d", code)
	}
	if b, code := dialBrowser(t, srv, relayTestProof); b == nil {
		t.Fatalf("browser could not join the account's session: %d", code)
	}
}

// REMOTE IS PRO ON THE HOSTED RELAY (2026-10-04): a free account is refused
// with what to do; a Pro one gets in; a gateway's own user is never asked.
func TestRelayAuthRefusesAFreeAccount(t *testing.T) {
	billing, _ := fakeBilling(t)
	auth := newRelayAuth(relayUsers{"tok-a": "alice"}, billing.URL)
	if _, err := auth.ValidateToken("acct-free"); err == nil || !strings.Contains(err.Error(), "Pro") || !strings.Contains(err.Error(), "subscribe") {
		t.Fatalf("a free account is refused, told how to get it: %v", err)
	}
	if id, err := auth.ValidateToken("acct-good"); err != nil || id != "acct:ws-1" {
		t.Fatalf("a Pro account gets in: %q %v", id, err)
	}
	if id, err := auth.ValidateToken("tok-a"); err != nil || id != "alice" {
		t.Fatalf("a self-hosted gateway's own user is not asked for a plan: %q %v", id, err)
	}
}

// The hosted relay says WHY in words: a free account's terminal gets 402
// with the sentence the window prints, not a bare "unauthorized".
func TestRelayTellsAFreeAccountWhy(t *testing.T) {
	billing, _ := fakeBilling(t)
	h := NewRemoteRelayHub()
	srv := httptest.NewServer(h.HandleRelayWS(newRelayAuth(relayUsers{}, billing.URL)))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/relay?id=x", nil)
	req.Header.Set("Authorization", "Bearer acct-free")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusPaymentRequired || !strings.Contains(string(b), "Pro") {
		t.Fatalf("status %d body %q", resp.StatusCode, b)
	}
}
