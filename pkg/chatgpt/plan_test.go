package chatgpt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeOpenAI is auth.openai.com for a test: it issues a client id on the
// first sign-in, checks PKCE on the exchange, rotates the refresh token, and
// signs ID tokens with a key its JWKS publishes.
type fakeOpenAI struct {
	srv       *httptest.Server
	key       *rsa.PrivateKey
	mu        sync.Mutex
	exchanges []url.Values
	refreshes []url.Values
	refuse    string // a token-endpoint error code to answer with, when set
	nonce     string // the nonce the next ID token carries (read from the authorize URL)
	clientID  string // the audience of the next ID token
	challenge string // the PKCE challenge the authorize URL carried
}

func newFakeOpenAI(t *testing.T) *fakeOpenAI {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeOpenAI{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/api/accounts/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.refuse != "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": f.refuse, "error_description": "no"})
			return
		}
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			f.exchanges = append(f.exchanges, r.PostForm)
			if r.PostForm.Get("code") != "c1" || pkceChallenge(r.PostForm.Get("code_verifier")) != f.challenge {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "pkce"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-1", "refresh_token": "rt-1", "id_token": f.idToken(t, f.clientID, f.nonce),
				"token_type": "Bearer", "expires_in": 3600, "scope": Scopes})
		case "refresh_token":
			f.refreshes = append(f.refreshes, r.PostForm)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at-" + r.PostForm.Get("refresh_token"), "refresh_token": "rt-next", "token_type": "Bearer", "expires_in": 3600})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeOpenAI) endpoints() Endpoints {
	return Endpoints{Issuer: f.srv.URL, Authorize: f.srv.URL + "/api/accounts/authorize", Token: f.srv.URL + "/api/accounts/oauth/token",
		JWKS: f.srv.URL + "/.well-known/jwks.json", Resource: "https://api.example/v1"}
}

func (f *fakeOpenAI) idToken(t *testing.T, aud, nonce string) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	c, _ := json.Marshal(map[string]any{"iss": f.srv.URL, "sub": "user-7", "aud": aud, "exp": time.Now().Add(time.Hour).Unix(), "nonce": nonce, "email": "dev@example.com"})
	signed := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	sum := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, 0, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// browse is the person approving in the browser: the authorize URL's state
// and challenge are read, and the callback is hit with a code bound to the
// challenge (so the exchange's PKCE check can pass) and the issued client id.
func (f *fakeOpenAI) browse(t *testing.T, l *Login, issued string) url.Values {
	t.Helper()
	u, err := url.Parse(l.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	f.mu.Lock()
	f.nonce, f.clientID, f.challenge = q.Get("nonce"), issued, q.Get("code_challenge")
	f.mu.Unlock()
	cb := q.Get("redirect_uri") + "?" + url.Values{"code": {"c1"}, "state": {q.Get("state")}, "client_id": {issued}}.Encode()
	resp, err := http.Get(cb)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("callback: HTTP %d", resp.StatusCode)
	}
	return q
}

func TestSignInKeepsTheIssuedClientAndVerifiesTheIDToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeOpenAI(t)
	store := NewStore(t.TempDir())
	l, err := BeginLogin(context.Background(), f.endpoints(), store, f.srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	q := f.browse(t, l, "oaiapp_123")
	// The first sign-in registers: the dynamic client, the app's name, the
	// host id, the full scope set, PKCE S256, a resource.
	if q.Get("client_id") != "dynamic_agent_client" || q.Get("agent_name_hint") != AgentName || !strings.HasPrefix(q.Get("ext_agent_host_id"), "urn:uuid:") ||
		q.Get("scope") != Scopes || q.Get("code_challenge_method") != "S256" || q.Get("resource") != "https://api.example/v1" || !strings.HasPrefix(q.Get("redirect_uri"), "http://127.0.0.1:") || !strings.HasSuffix(q.Get("redirect_uri"), "/callback") {
		t.Fatalf("authorize URL: %v", q)
	}
	c, err := l.Wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.ClientID != "oaiapp_123" || c.Email != "dev@example.com" || c.Subject != "user-7" || c.AccessToken != "at-1" || c.RefreshToken != "rt-1" || c.HostID != q.Get("ext_agent_host_id") {
		t.Fatalf("credential: %+v", c)
	}
	if len(f.exchanges) != 1 || f.exchanges[0].Get("client_id") != "oaiapp_123" || f.exchanges[0].Get("resource") != "https://api.example/v1" || f.exchanges[0].Get("code_verifier") == "" {
		t.Fatalf("exchange: %v", f.exchanges)
	}
	kept, err := store.Load()
	if err != nil || kept == nil || kept.AccessToken != "at-1" {
		t.Fatalf("kept: %+v %v", kept, err)
	}
	if st, err := os.Stat(store.path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("the credential file is owner-only: %v %v", st, err)
	}
	if b, _ := os.ReadFile(store.path); strings.Contains(string(b), "at-1") || strings.Contains(string(b), "rt-1") {
		t.Fatal("tokens are encrypted on disk")
	}
	// A second sign-in on the same host uses the issued client id, the same
	// host id, and no registration hint.
	l2, err := BeginLogin(context.Background(), f.endpoints(), store, f.srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()
	q2, _ := url.Parse(l2.AuthURL)
	if q2.Query().Get("client_id") != "oaiapp_123" || q2.Query().Get("agent_name_hint") != "" || q2.Query().Get("ext_agent_host_id") != c.HostID {
		t.Fatalf("second sign-in: %v", q2.Query())
	}
}

func TestTheTokenIsRenewedBeforeItExpiresAndTheRefreshTokenRotates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeOpenAI(t)
	store := NewStore(t.TempDir())
	if err := store.Save(&Credential{ClientID: "oaiapp_123", AccessToken: "old", RefreshToken: "rt-1", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	src := NewSource(f.endpoints(), store, f.srv.Client())
	tok, err := src.Token(context.Background())
	if err != nil || tok != "at-rt-1" {
		t.Fatalf("renewed token: %q %v", tok, err)
	}
	if len(f.refreshes) != 1 || f.refreshes[0].Get("client_id") != "oaiapp_123" || f.refreshes[0].Get("refresh_token") != "rt-1" {
		t.Fatalf("refresh request: %v", f.refreshes)
	}
	kept, _ := store.Load()
	if kept.RefreshToken != "rt-next" || kept.AccessToken != "at-rt-1" || time.Until(kept.ExpiresAt) < 50*time.Minute {
		t.Fatalf("kept after refresh: %+v", kept)
	}
	// Still fresh: no second refresh.
	if tok, err := src.Token(context.Background()); err != nil || tok != "at-rt-1" || len(f.refreshes) != 1 {
		t.Fatalf("a fresh token is not renewed again: %q %v %d", tok, err, len(f.refreshes))
	}
}

func TestAnExpiredSignInIsForgottenAndSaysSo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeOpenAI(t)
	store := NewStore(t.TempDir())
	_ = store.Save(&Credential{ClientID: "oaiapp_123", AccessToken: "old", RefreshToken: "rt-1", ExpiresAt: time.Now().Add(-time.Minute)})
	f.refuse = "refresh_token_expired"
	src := NewSource(f.endpoints(), store, f.srv.Client())
	if _, err := src.Token(context.Background()); err != ErrNotSignedIn {
		t.Fatalf("want ErrNotSignedIn, got %v", err)
	}
	if c, _ := store.Load(); c != nil {
		t.Fatal("the dead sign-in is forgotten")
	}
	if id, _ := store.HostID(); id == "" {
		t.Fatal("the host id outlives the sign-in")
	}
	if _, err := NewSource(f.endpoints(), NewStore(t.TempDir()), nil).Token(context.Background()); err != ErrNotSignedIn {
		t.Fatalf("nobody signed in: %v", err)
	}
}

func TestAPastedLinkMustBelongToThisSignIn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeOpenAI(t)
	l, err := BeginLogin(context.Background(), f.endpoints(), NewStore(t.TempDir()), f.srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Deliver("http://127.0.0.1:1/callback?code=x&state=other"); err == nil {
		t.Fatal("another sign-in's state is refused")
	}
	if err := l.Deliver("not a link"); err == nil {
		t.Fatal("a bare word is refused")
	}
}

func TestAnIDTokenSignedByAnotherKeyIsRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeOpenAI(t)
	other := newFakeOpenAI(t) // its own key, its own JWKS
	ep := f.endpoints()
	tok := other.idToken(t, "oaiapp_123", "n1")
	if _, err := verifyIDToken(context.Background(), f.srv.Client(), ep, tok, "oaiapp_123", "n1"); err == nil {
		t.Fatal("a token signed by another key verifies against this JWKS")
	}
	good := f.idToken(t, "oaiapp_123", "n1")
	if _, err := verifyIDToken(context.Background(), f.srv.Client(), ep, good, "oaiapp_123", "wrong-nonce"); err == nil {
		t.Fatal("a wrong nonce is refused")
	}
	if _, err := verifyIDToken(context.Background(), f.srv.Client(), ep, good, "someone-else", "n1"); err == nil {
		t.Fatal("another audience is refused")
	}
	if c, err := verifyIDToken(context.Background(), f.srv.Client(), ep, good, "oaiapp_123", "n1"); err != nil || c.Email != "dev@example.com" {
		t.Fatalf("the right token verifies: %+v %v", c, err)
	}
}
