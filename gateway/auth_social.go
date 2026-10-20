package gateway

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/oidc"
)

// Sign in with Google and Sign in with GitHub on the web sign-in page (Greg,
// 2026-10-20: "Is it hard to get google account sign in?", "Github too").
// The browser is sent to the provider, comes back to
// /api/auth/<provider>/callback with a code, the gateway turns the code into
// a verified email address and signs that address in through
// auth.Service.LoginVerifiedEmail (an account on first sight), then sends
// the browser to /login with the session token in the URL fragment, which
// never reaches a server or a log. Configured by GOOGLE_CLIENT_ID and
// GOOGLE_CLIENT_SECRET, GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET; a provider
// without both is not offered.

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleIssuer   = "https://accounts.google.com"
	googleJWKS     = "https://www.googleapis.com/oauth2/v3/certs"

	githubAuthURL   = "https://github.com/login/oauth/authorize"
	githubTokenURL  = "https://github.com/login/oauth/access_token"
	githubUserURL   = "https://api.github.com/user"
	githubEmailsURL = "https://api.github.com/user/emails"

	socialAttemptTTL = 10 * time.Minute
)

// socialIdentity is what a provider says about the person who signed in.
type socialIdentity struct {
	Email    string
	Name     string
	Verified bool
}

// identityProvider is one button on the sign-in page.
type identityProvider interface {
	name() string
	authURL(state, nonce, redirect string) string
	identity(ctx context.Context, code, nonce, redirect string) (socialIdentity, error)
}

type socialAttempt struct {
	nonce    string
	callback string
	at       time.Time
}

// socialSignIn holds the configured providers and the sign-ins under way.
type socialSignIn struct {
	baseURL   string
	providers map[string]identityProvider
	mu        sync.Mutex
	pending   map[string]socialAttempt
}

// newSocialSignIn reads the providers from the environment; nil when none
// is configured, and the page then shows no button.
func newSocialSignIn(baseURL string, getenv func(string) string) *socialSignIn {
	hc := &http.Client{Timeout: 15 * time.Second}
	s := &socialSignIn{baseURL: strings.TrimRight(baseURL, "/"), providers: map[string]identityProvider{}, pending: map[string]socialAttempt{}}
	if id, secret := getenv("GOOGLE_CLIENT_ID"), getenv("GOOGLE_CLIENT_SECRET"); id != "" && secret != "" {
		s.providers["google"] = &googleProvider{clientID: id, clientSecret: secret, hc: hc}
	}
	if id, secret := getenv("GITHUB_CLIENT_ID"), getenv("GITHUB_CLIENT_SECRET"); id != "" && secret != "" {
		s.providers["github"] = &githubProvider{clientID: id, clientSecret: secret, hc: hc}
	}
	if len(s.providers) == 0 {
		return nil
	}
	return s
}

// Names lists the providers offered, for the sign-in page.
func (s *socialSignIn) Names() []string {
	if s == nil {
		return nil
	}
	var out []string
	for _, n := range []string{"google", "github"} {
		if _, ok := s.providers[n]; ok {
			out = append(out, n)
		}
	}
	return out
}

func (s *socialSignIn) redirectFor(provider string) string {
	return s.baseURL + "/api/auth/" + provider + "/callback"
}

func (s *socialSignIn) begin(callback string) (state, nonce string) {
	state, nonce = randomToken(), randomToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, a := range s.pending {
		if time.Since(a.at) > socialAttemptTTL {
			delete(s.pending, k)
		}
	}
	s.pending[state] = socialAttempt{nonce: nonce, callback: callback, at: time.Now()}
	return state, nonce
}

// take spends a state: one callback per attempt.
func (s *socialSignIn) take(state string) (socialAttempt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.pending[state]
	if ok {
		delete(s.pending, state)
	}
	if ok && time.Since(a.at) > socialAttemptTTL {
		return socialAttempt{}, false
	}
	return a, ok
}

// SetSocial gives the adapter its providers.
func (a *AuthGatewayAdapter) SetSocial(s *socialSignIn) { a.social = s }

// SocialProviders is what the sign-in page may offer.
func (a *AuthGatewayAdapter) SocialProviders() []string {
	if a == nil {
		return nil
	}
	return a.social.Names()
}

// HandleSocialStart is GET /api/auth/<provider>: it sends the browser to
// the provider. ?callback= (a same-site path) is kept for the return trip,
// as the form keeps it: the TUI's browser sign-in lands back where it asked.
func (a *AuthGatewayAdapter) HandleSocialStart(w http.ResponseWriter, r *http.Request) {
	p, ok := a.provider(r, "")
	if !ok {
		http.NotFound(w, r)
		return
	}
	callback := r.URL.Query().Get("callback")
	if !strings.HasPrefix(callback, "/") || strings.HasPrefix(callback, "//") {
		callback = ""
	}
	state, nonce := a.social.begin(callback)
	http.Redirect(w, r, p.authURL(state, nonce, a.social.redirectFor(p.name())), http.StatusFound)
}

// HandleSocialCallback is GET /api/auth/<provider>/callback: the code
// becomes a verified address, the address a session, the session a URL
// fragment on /login.
func (a *AuthGatewayAdapter) HandleSocialCallback(w http.ResponseWriter, r *http.Request) {
	p, ok := a.provider(r, "/callback")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if ip := guestClientIP(r); !loginRateLimiter.Allow(ip) {
		http.Error(w, `{"error":"too many attempts, slow down"}`, http.StatusTooManyRequests)
		return
	}
	q := r.URL.Query()
	if q.Get("error") != "" {
		http.Redirect(w, r, "/login?error="+p.name()+"_denied", http.StatusFound)
		return
	}
	attempt, ok := a.social.take(q.Get("state"))
	if !ok || q.Get("code") == "" {
		http.Error(w, `{"error":"this sign-in link is not one we started, or it expired — start again from the sign-in page"}`, http.StatusBadRequest)
		return
	}
	id, err := p.identity(r.Context(), q.Get("code"), attempt.nonce, a.social.redirectFor(p.name()))
	if err != nil {
		logs.New("Auth").Warn("social sign-in failed", "provider", p.name(), "error", err.Error())
		http.Redirect(w, r, "/login?error="+p.name()+"_failed", http.StatusFound)
		return
	}
	if !id.Verified || id.Email == "" {
		http.Redirect(w, r, "/login?error="+p.name()+"_unverified", http.StatusFound)
		return
	}
	// A first sign-in gets a workspace of its own, named after the person.
	wsName := ""
	workspaceFor := func(ctx context.Context, username string) (string, error) {
		wsName = personalWorkspaceName(id.Name, username)
		return a.workspaceForSignUp(ctx, wsName, username)
	}
	user, token, created, err := a.service.LoginVerifiedEmail(r.Context(), id.Email, id.Name, workspaceFor)
	if err != nil {
		logs.New("Auth").Warn("social sign-in refused", "provider", p.name(), "error", err.Error())
		http.Redirect(w, r, "/login?error="+p.name()+"_failed", http.StatusFound)
		return
	}
	if created {
		a.seedIfOwn(r.Context(), user.WorkspaceID, wsName, user.ID)
	}
	logs.New("Auth").Info("signed in with "+p.name(), "user", user.ID, "created", created)
	target := "/login"
	if attempt.callback != "" {
		target += "?callback=" + url.QueryEscape(attempt.callback)
	}
	http.Redirect(w, r, target+"#token="+url.QueryEscape(token), http.StatusFound)
}

// provider is the one the path names: /api/auth/<name><suffix>.
func (a *AuthGatewayAdapter) provider(r *http.Request, suffix string) (identityProvider, bool) {
	if a.social == nil {
		return nil, false
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/auth/")
	name = strings.TrimSuffix(name, suffix)
	p, ok := a.social.providers[name]
	return p, ok && r.Method == http.MethodGet
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ---- Google: OpenID Connect, the email comes signed in the ID token ----

type googleProvider struct {
	clientID, clientSecret string
	hc                     *http.Client
	tokenURL, issuer, jwks string // "" = Google's; a test points them at its own server
}

func (g *googleProvider) name() string { return "google" }

func (g *googleProvider) authURL(state, nonce, redirect string) string {
	v := url.Values{
		"client_id":     {g.clientID},
		"redirect_uri":  {redirect},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"nonce":         {nonce},
		"prompt":        {"select_account"},
	}
	return googleAuthURL + "?" + v.Encode()
}

func (g *googleProvider) identity(ctx context.Context, code, nonce, redirect string) (socialIdentity, error) {
	tokenURL, issuer, jwks := g.tokenURL, g.issuer, g.jwks
	if tokenURL == "" {
		tokenURL, issuer, jwks = googleTokenURL, googleIssuer, googleJWKS
	}
	form := url.Values{
		"code":          {code},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"redirect_uri":  {redirect},
		"grant_type":    {"authorization_code"},
	}
	var tr struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
	}
	if err := postForm(ctx, g.hc, tokenURL, form, &tr); err != nil {
		return socialIdentity{}, err
	}
	if tr.Error != "" || tr.IDToken == "" {
		return socialIdentity{}, fmt.Errorf("google refused the code: %s", tr.Error)
	}
	claims, err := oidc.VerifyIDToken(ctx, g.hc, issuer, jwks, tr.IDToken, g.clientID, nonce)
	if err != nil {
		return socialIdentity{}, fmt.Errorf("google's id token: %w", err)
	}
	return socialIdentity{Email: claims.Email, Name: claims.Name, Verified: claims.EmailVerified}, nil
}

// ---- GitHub: OAuth, the email comes from the API, flagged verified by GitHub ----

type githubProvider struct {
	clientID, clientSecret string
	hc                     *http.Client
	tokenURL, apiURL       string // "" = GitHub's; a test points them at its own server
}

func (g *githubProvider) name() string { return "github" }

func (g *githubProvider) authURL(state, _ string, redirect string) string {
	v := url.Values{
		"client_id":    {g.clientID},
		"redirect_uri": {redirect},
		"scope":        {"read:user user:email"},
		"state":        {state},
	}
	return githubAuthURL + "?" + v.Encode()
}

func (g *githubProvider) identity(ctx context.Context, code, _ string, redirect string) (socialIdentity, error) {
	tokenURL, userURL, emailsURL := g.tokenURL, githubUserURL, githubEmailsURL
	if tokenURL == "" {
		tokenURL = githubTokenURL
	}
	if g.apiURL != "" {
		userURL, emailsURL = g.apiURL+"/user", g.apiURL+"/user/emails"
	}
	form := url.Values{
		"code":          {code},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"redirect_uri":  {redirect},
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := postForm(ctx, g.hc, tokenURL, form, &tr); err != nil {
		return socialIdentity{}, err
	}
	if tr.Error != "" || tr.AccessToken == "" {
		return socialIdentity{}, fmt.Errorf("github refused the code: %s", tr.Error)
	}
	var user struct {
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := getJSON(ctx, g.hc, userURL, tr.AccessToken, &user); err != nil {
		return socialIdentity{}, err
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := getJSON(ctx, g.hc, emailsURL, tr.AccessToken, &emails); err != nil {
		return socialIdentity{}, err
	}
	name := user.Name
	if name == "" {
		name = user.Login
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return socialIdentity{Email: e.Email, Name: name, Verified: true}, nil
		}
	}
	for _, e := range emails {
		if e.Verified {
			return socialIdentity{Email: e.Email, Name: name, Verified: true}, nil
		}
	}
	return socialIdentity{Email: user.Email, Name: name, Verified: false}, nil
}

func postForm(ctx context.Context, hc *http.Client, u string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("the provider did not answer: %w", err)
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

func getJSON(ctx context.Context, hc *http.Client, u, bearer string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "memdoor")
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("the provider did not answer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the provider answered %d for %s", resp.StatusCode, u)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
