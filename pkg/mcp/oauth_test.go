package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAuth is a resource server (/mcp) and its authorization server (/as):
// the 401 challenge, RFC 9728 and RFC 8414 metadata, registration, an
// authorize endpoint that redirects with a code, and a token endpoint that
// checks PKCE and refreshes.
type fakeAuth struct {
	srv       *httptest.Server
	mu        sync.Mutex
	challenge string // the code_challenge of the last authorize
	refreshed int
}

func newFakeAuth(t *testing.T) *fakeAuth {
	f := &fakeAuth{}
	mux := http.NewServeMux()
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	base := func() string { return f.srv.URL }
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if a := r.Header.Get("Authorization"); a != "Bearer access1" && a != "Bearer access2" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource/mcp"`, base()))
			w.WriteHeader(401)
			return
		}
		var m message
		json.NewDecoder(r.Body).Decode(&m)
		if m.Method == "initialize" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":%q,"serverInfo":{"name":"linear"}}}`, m.ID, ProtocolVersion)
			return
		}
		w.WriteHeader(202)
	})
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"resource": base() + "/mcp", "authorization_servers": []string{base() + "/as"}, "scopes_supported": []string{"read"}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server/as", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"authorization_endpoint": base() + "/as/authorize",
			"token_endpoint": base() + "/as/token", "registration_endpoint": base() + "/as/register"})
	})
	mux.HandleFunc("/as/register", func(w http.ResponseWriter, r *http.Request) {
		var reg map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reg)
		if reg["token_endpoint_auth_method"] != "none" {
			http.Error(w, "public clients only", 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"client_id": "c1"})
	})
	mux.HandleFunc("/as/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") != "c1" || q.Get("code_challenge_method") != "S256" || q.Get("resource") != base()+"/mcp" || q.Get("scope") != "read" {
			http.Error(w, "bad authorize request: "+r.URL.RawQuery, 400)
			return
		}
		f.mu.Lock()
		f.challenge = q.Get("code_challenge")
		f.mu.Unlock()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code=code1&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("/as/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			f.mu.Lock()
			ok := base64.RawURLEncoding.EncodeToString(sum[:]) == f.challenge
			f.mu.Unlock()
			if !ok || r.Form.Get("code") != "code1" {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "access1", "refresh_token": "r1", "expires_in": 60})
		case "refresh_token":
			f.mu.Lock()
			f.refreshed++
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "access2", "expires_in": 3600})
		}
	})
	return f
}

func testOAuth(t *testing.T) *OAuth {
	t.Setenv("HOME", t.TempDir())
	return NewOAuth(NewFileTokenStore(t.TempDir()))
}

// The whole sign-in: the 401's challenge leads to the metadata, Memdoor
// registers, the browser approves, the code comes back to the local
// listener, PKCE holds, the token is stored (encrypted) and refreshed when
// it is about to expire; the server then answers with it.
func TestOAuthSignInEndToEnd(t *testing.T) {
	f := newFakeAuth(t)
	o := testOAuth(t)
	server := f.srv.URL + "/mcp"
	ctx := context.Background()

	c := NewHTTP(Spec{Name: "linear", URL: server, Token: o.TokenSource(server)}, nil)
	if err := c.Start(ctx); !IsAuthRequired(err) {
		t.Fatalf("before sign-in: want needs-sign-in, got %v", err)
	}

	login, err := o.BeginLogin(ctx, server, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(login.AuthURL, f.srv.URL+"/as/authorize?") {
		t.Fatalf("auth url %s", login.AuthURL)
	}
	go func() { // the browser
		resp, err := http.Get(login.AuthURL)
		if err == nil {
			resp.Body.Close()
		}
	}()
	if err := login.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if !o.SignedIn(server) {
		t.Fatal("not stored")
	}

	// The token expires in 60 s: the source refreshes it first.
	c = NewHTTP(Spec{Name: "linear", URL: server, Token: o.TokenSource(server)}, nil)
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if f.refreshed != 1 {
		t.Fatalf("refreshed %d times", f.refreshed)
	}
	if tok, _ := o.Tokens.Get(server); tok.AccessToken != "access2" || tok.RefreshToken != "r1" || time.Until(tok.ExpiresAt) < 50*time.Minute {
		t.Fatalf("stored after refresh: %+v", tok)
	}
	if err := o.SignOut(server); err != nil || o.SignedIn(server) {
		t.Fatalf("sign out: %v", err)
	}
}

// A browser that cannot reach the listener: the redirect URL is pasted.
func TestOAuthPastedRedirect(t *testing.T) {
	f := newFakeAuth(t)
	o := testOAuth(t)
	server := f.srv.URL + "/mcp"
	login, err := o.BeginLogin(context.Background(), server, "")
	if err != nil {
		t.Fatal(err)
	}
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noFollow.Get(login.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if err := login.Deliver("http://elsewhere/callback?code=code1&state=wrong"); err == nil {
		t.Fatal("a link from another sign-in must be refused")
	}
	if err := login.Deliver(resp.Header.Get("Location")); err != nil {
		t.Fatal(err)
	}
	if err := login.Wait(context.Background()); err != nil || !o.SignedIn(server) {
		t.Fatalf("wait: %v", err)
	}
}
