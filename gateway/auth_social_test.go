package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"memdoor/pkg/auth"
)

// socialFake is the auth service behind Sign in with Google / GitHub here:
// it signs the vouched-for address in and remembers it.
type socialFake struct {
	auth.Service
	signedIn []string
}

func (f *socialFake) LoginVerifiedEmail(_ context.Context, email, name string) (*auth.User, string, error) {
	f.signedIn = append(f.signedIn, email)
	u := &auth.User{}
	u.ID, u.Email, u.Name = "human:"+email, email, name
	return u, "session-for-" + email, nil
}

// fakeProvider answers the identity a test wants.
type fakeProvider struct {
	id   socialIdentity
	err  error
	seen struct{ code, nonce, redirect string }
}

func (p *fakeProvider) name() string { return "google" }
func (p *fakeProvider) authURL(state, nonce, redirect string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + state + "&nonce=" + nonce + "&redirect_uri=" + url.QueryEscape(redirect)
}
func (p *fakeProvider) identity(_ context.Context, code, nonce, redirect string) (socialIdentity, error) {
	p.seen.code, p.seen.nonce, p.seen.redirect = code, nonce, redirect
	return p.id, p.err
}

func socialAdapter(t *testing.T, p identityProvider) (*AuthGatewayAdapter, *socialFake) {
	t.Helper()
	fake := &socialFake{}
	a := NewAuthGatewayAdapter(fake)
	a.SetSocial(&socialSignIn{baseURL: "https://memdoor.ai", providers: map[string]identityProvider{p.name(): p}, pending: map[string]socialAttempt{}})
	return a, fake
}

func get(a *AuthGatewayAdapter, h func(http.ResponseWriter, *http.Request), target, from string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.RemoteAddr = from
	h(rec, req)
	return rec
}

// The start sends the browser to the provider with a state and a nonce of
// its own; the callback with that state signs the vouched-for address in
// and hands the session to /login in the URL fragment.
func TestSignInWithAProviderEndsSignedInThroughTheFragment(t *testing.T) {
	p := &fakeProvider{id: socialIdentity{Email: "dev@example.com", Name: "Dev", Verified: true}}
	a, fake := socialAdapter(t, p)

	start := get(a, a.HandleSocialStart, "/api/auth/google?callback=/tui/login", "198.51.100.1:1")
	if start.Code != http.StatusFound {
		t.Fatalf("start: %d %s", start.Code, start.Body.String())
	}
	to, _ := url.Parse(start.Header().Get("Location"))
	if to.Host != "accounts.google.com" || to.Query().Get("state") == "" || to.Query().Get("nonce") == "" {
		t.Fatalf("start sends to the provider with state and nonce: %s", to)
	}
	if to.Query().Get("redirect_uri") != "https://memdoor.ai/api/auth/google/callback" {
		t.Fatalf("redirect: %s", to.Query().Get("redirect_uri"))
	}

	state := to.Query().Get("state")
	back := get(a, a.HandleSocialCallback, "/api/auth/google/callback?code=c-1&state="+state, "198.51.100.1:1")
	if back.Code != http.StatusFound {
		t.Fatalf("callback: %d %s", back.Code, back.Body.String())
	}
	loc := back.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?callback=%2Ftui%2Flogin#token=session-for-dev%40example.com") {
		t.Fatalf("the session rides the fragment, the callback the query: %s", loc)
	}
	if p.seen.code != "c-1" || p.seen.nonce != to.Query().Get("nonce") || p.seen.redirect != "https://memdoor.ai/api/auth/google/callback" {
		t.Fatalf("the provider was asked with the attempt's code, nonce and redirect: %+v", p.seen)
	}
	if len(fake.signedIn) != 1 || fake.signedIn[0] != "dev@example.com" {
		t.Fatalf("signed in: %v", fake.signedIn)
	}

	replay := get(a, a.HandleSocialCallback, "/api/auth/google/callback?code=c-1&state="+state, "198.51.100.1:1")
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("a state is spent by its first callback: %d", replay.Code)
	}
}

// Only a verified address signs in; a refusal by the provider or a cancel
// goes back to the sign-in page with a reason, never a session.
func TestSignInWithAProviderRefusesWhatItCannotTrust(t *testing.T) {
	p := &fakeProvider{id: socialIdentity{Email: "dev@example.com", Verified: false}}
	a, fake := socialAdapter(t, p)
	state, _ := a.social.begin("")
	back := get(a, a.HandleSocialCallback, "/api/auth/google/callback?code=c&state="+state, "198.51.100.2:1")
	if back.Code != http.StatusFound || back.Header().Get("Location") != "/login?error=google_unverified" {
		t.Fatalf("unverified: %d %s", back.Code, back.Header().Get("Location"))
	}
	denied := get(a, a.HandleSocialCallback, "/api/auth/google/callback?error=access_denied", "198.51.100.2:1")
	if denied.Header().Get("Location") != "/login?error=google_denied" {
		t.Fatalf("denied: %s", denied.Header().Get("Location"))
	}
	foreign := get(a, a.HandleSocialCallback, "/api/auth/google/callback?code=c&state=not-ours", "198.51.100.2:1")
	if foreign.Code != http.StatusBadRequest {
		t.Fatalf("a state we did not start: %d", foreign.Code)
	}
	if len(fake.signedIn) != 0 {
		t.Fatalf("nobody signs in: %v", fake.signedIn)
	}
	// A callback to another site is dropped at the start.
	start := get(a, a.HandleSocialStart, "/api/auth/google?callback=https://evil.example/x", "198.51.100.2:1")
	to, _ := url.Parse(start.Header().Get("Location"))
	att, _ := a.social.take(to.Query().Get("state"))
	if att.callback != "" {
		t.Fatalf("an off-site callback is not kept: %q", att.callback)
	}
	// An unknown provider is not a route.
	if rec := get(a, a.HandleSocialStart, "/api/auth/facebook", "198.51.100.2:1"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown provider: %d", rec.Code)
	}
}

// GitHub has no ID token: the email comes from its API, and only one GitHub
// marks verified counts.
func TestGitHubIdentityTakesTheVerifiedPrimaryEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("code") != "gh-code" || r.Form.Get("client_secret") != "s3" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad_verification_code"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "gho_x", "token_type": "bearer"})
		case "/user":
			if r.Header.Get("Authorization") != "Bearer gho_x" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "octo", "name": "Octo Cat", "email": nil})
		case "/user/emails":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"email": "old@example.com", "primary": false, "verified": true},
				{"email": "octo@example.com", "primary": true, "verified": true},
				{"email": "spoof@example.com", "primary": false, "verified": false},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	g := &githubProvider{clientID: "c1", clientSecret: "s3", hc: srv.Client(), tokenURL: srv.URL + "/token", apiURL: srv.URL}
	id, err := g.identity(context.Background(), "gh-code", "", "https://memdoor.ai/api/auth/github/callback")
	if err != nil || id.Email != "octo@example.com" || !id.Verified || id.Name != "Octo Cat" {
		t.Fatalf("identity: %+v %v", id, err)
	}
	if _, err := g.identity(context.Background(), "wrong", "", ""); err == nil {
		t.Fatal("a refused code is an error")
	}
	if u := g.authURL("st", "", "https://memdoor.ai/x"); !strings.Contains(u, "scope=read%3Auser+user%3Aemail") || !strings.Contains(u, "state=st") {
		t.Fatalf("github authorize url: %s", u)
	}
}

// Nothing configured, no button and no route.
func TestNoProviderMeansNoButton(t *testing.T) {
	if s := newSocialSignIn("https://memdoor.ai", func(string) string { return "" }); s != nil {
		t.Fatal("no client ids, no sign-in")
	}
	env := map[string]string{"GITHUB_CLIENT_ID": "a", "GITHUB_CLIENT_SECRET": "b", "GOOGLE_CLIENT_ID": "only-the-id"}
	s := newSocialSignIn("https://memdoor.ai/", func(k string) string { return env[k] })
	if got := strings.Join(s.Names(), ","); got != "github" {
		t.Fatalf("a provider needs both its id and its secret: %q", got)
	}
	if s.redirectFor("github") != "https://memdoor.ai/api/auth/github/callback" {
		t.Fatalf("redirect: %s", s.redirectFor("github"))
	}
	var none *AuthGatewayAdapter
	if none.SocialProviders() != nil {
		t.Fatal("a gateway without auth offers nothing")
	}
}
