package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// OAuth signs in to HTTP MCP servers the way the MCP authorization spec
// lays out: the server's 401 names its protected-resource metadata (RFC
// 9728), that names the authorization server (RFC 8414 / OpenID), the
// client registers itself (RFC 7591) as a public client, and the person
// approves in a browser with PKCE (S256); the code comes back to a listener
// on 127.0.0.1, or is pasted. Tokens go to a TokenStore and are refreshed
// before they expire.
type OAuth struct {
	Tokens TokenStore
	HTTP   *http.Client
	// ClientName is how the app is named on the server's consent page.
	ClientName string
	// LoginTimeout bounds the wait for the person to approve.
	LoginTimeout time.Duration
}

// NewOAuth is OAuth over tokens.
func NewOAuth(tokens TokenStore) *OAuth {
	return &OAuth{Tokens: tokens, HTTP: &http.Client{Timeout: 30 * time.Second}, ClientName: "Memdoor", LoginTimeout: 5 * time.Minute}
}

// refreshBefore is how early a token is refreshed before it expires.
const refreshBefore = 5 * time.Minute

// TokenSource is the bearer token for serverURL, refreshed when it is
// about to expire; "" when there is no sign-in (the server then refuses,
// and the server is reported as needing sign-in).
func (o *OAuth) TokenSource(serverURL string) func(ctx context.Context) (string, error) {
	var mu sync.Mutex
	return func(ctx context.Context) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		t, err := o.Tokens.Get(serverURL)
		if err != nil || t == nil {
			return "", err
		}
		if !t.ExpiresAt.IsZero() && time.Until(t.ExpiresAt) < refreshBefore && t.RefreshToken != "" {
			fresh, err := o.refresh(ctx, t)
			if err == nil {
				if err := o.Tokens.Put(serverURL, fresh); err != nil {
					return "", err
				}
				return fresh.AccessToken, nil
			}
			if !t.ExpiresAt.Before(time.Now()) {
				return t.AccessToken, nil // still valid; refresh later
			}
			return "", nil // expired and not refreshable: sign in again
		}
		return t.AccessToken, nil
	}
}

// SignOut forgets the sign-in for serverURL.
func (o *OAuth) SignOut(serverURL string) error { return o.Tokens.Delete(serverURL) }

// SignedIn reports whether there is a sign-in for serverURL.
func (o *OAuth) SignedIn(serverURL string) bool {
	t, err := o.Tokens.Get(serverURL)
	return err == nil && t != nil
}

// endpoints is what discovery finds.
type endpoints struct {
	authorize, token, register string
	scopes                     []string
	resource                   string
}

var (
	resourceMetadataRe = regexp.MustCompile(`resource_metadata="([^"]+)"`)
	scopeRe            = regexp.MustCompile(`scope="([^"]+)"`)
)

// discover finds where to sign in for serverURL, from its challenge.
func (o *OAuth) discover(ctx context.Context, serverURL, challenge string) (*endpoints, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	origin := u.Scheme + "://" + u.Host
	ep := &endpoints{resource: canonicalResource(u)}

	// RFC 9728: protected-resource metadata, named by the challenge or at
	// the well-known path (with the resource's path, then without).
	var prmURLs []string
	if m := resourceMetadataRe.FindStringSubmatch(challenge); m != nil {
		prmURLs = append(prmURLs, m[1])
	}
	if p := strings.TrimSuffix(u.Path, "/"); p != "" {
		prmURLs = append(prmURLs, origin+"/.well-known/oauth-protected-resource"+p)
	}
	prmURLs = append(prmURLs, origin+"/.well-known/oauth-protected-resource")
	issuer := ""
	for _, pu := range prmURLs {
		var prm struct {
			Resource             string   `json:"resource"`
			AuthorizationServers []string `json:"authorization_servers"`
			ScopesSupported      []string `json:"scopes_supported"`
		}
		if o.getJSON(ctx, pu, &prm) == nil && len(prm.AuthorizationServers) > 0 {
			issuer = prm.AuthorizationServers[0]
			ep.scopes = prm.ScopesSupported
			if prm.Resource != "" {
				ep.resource = prm.Resource
			}
			break
		}
	}
	if m := scopeRe.FindStringSubmatch(challenge); m != nil {
		ep.scopes = strings.Fields(m[1])
	}
	if issuer == "" {
		issuer = origin // servers from before RFC 9728: the server is its own authorization server
	}

	// RFC 8414 / OpenID: the authorization server's metadata.
	iu, err := url.Parse(issuer)
	if err != nil {
		return nil, fmt.Errorf("authorization server %q: %w", issuer, err)
	}
	issuerOrigin := iu.Scheme + "://" + iu.Host
	ip := strings.TrimSuffix(iu.Path, "/")
	var metaURLs []string
	if ip != "" {
		metaURLs = append(metaURLs, issuerOrigin+"/.well-known/oauth-authorization-server"+ip, issuerOrigin+"/.well-known/openid-configuration"+ip, issuerOrigin+ip+"/.well-known/openid-configuration")
	} else {
		metaURLs = append(metaURLs, issuerOrigin+"/.well-known/oauth-authorization-server", issuerOrigin+"/.well-known/openid-configuration")
	}
	for _, mu := range metaURLs {
		var meta struct {
			AuthorizationEndpoint string   `json:"authorization_endpoint"`
			TokenEndpoint         string   `json:"token_endpoint"`
			RegistrationEndpoint  string   `json:"registration_endpoint"`
			ScopesSupported       []string `json:"scopes_supported"`
		}
		if o.getJSON(ctx, mu, &meta) == nil && meta.AuthorizationEndpoint != "" && meta.TokenEndpoint != "" {
			ep.authorize, ep.token, ep.register = meta.AuthorizationEndpoint, meta.TokenEndpoint, meta.RegistrationEndpoint
			if len(ep.scopes) == 0 {
				ep.scopes = meta.ScopesSupported
			}
			return ep, nil
		}
	}
	// No metadata at all: the spec's default endpoints on the issuer.
	ep.authorize, ep.token, ep.register = issuerOrigin+"/authorize", issuerOrigin+"/token", issuerOrigin+"/register"
	return ep, nil
}

// canonicalResource is the server URL as RFC 8707 names a resource:
// scheme and host lowercase, no fragment, no trailing slash.
func canonicalResource(u *url.URL) string {
	c := *u
	c.Scheme, c.Host, c.Fragment, c.RawQuery = strings.ToLower(c.Scheme), strings.ToLower(c.Host), "", ""
	return strings.TrimSuffix(c.String(), "/")
}

func (o *OAuth) getJSON(ctx context.Context, u string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", u, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}

// Login is one sign-in in progress: open AuthURL in a browser; the code
// arrives at the local listener, or through Deliver.
type Login struct {
	AuthURL string

	o        *OAuth
	server   string
	ep       *endpoints
	clientID string
	secret   string
	redirect string
	verifier string
	state    string
	codes    chan string
	ln       net.Listener
	srv      *http.Server
}

// BeginLogin starts a sign-in to serverURL: discovery, client
// registration, the local listener, and the URL to open. challenge is the
// server's WWW-Authenticate header, when known.
func (o *OAuth) BeginLogin(ctx context.Context, serverURL, challenge string) (*Login, error) {
	if challenge == "" {
		challenge = o.probe(ctx, serverURL)
	}
	ep, err := o.discover(ctx, serverURL, challenge)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("no local port for the sign-in callback: %w", err)
	}
	l := &Login{o: o, server: serverURL, ep: ep, ln: ln, codes: make(chan string, 1),
		redirect: fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)}
	if err := l.register(ctx); err != nil {
		ln.Close()
		return nil, err
	}
	l.verifier = randomString(48)
	l.state = randomString(24)
	sum := sha256.Sum256([]byte(l.verifier))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {l.clientID},
		"redirect_uri":          {l.redirect},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"state":                 {l.state},
		"resource":              {ep.resource},
	}
	if len(ep.scopes) > 0 {
		q.Set("scope", strings.Join(ep.scopes, " "))
	}
	sep := "?"
	if strings.Contains(ep.authorize, "?") {
		sep = "&"
	}
	l.AuthURL = ep.authorize + sep + q.Encode()
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", l.callback)
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go l.srv.Serve(ln)
	return l, nil
}

// probe asks the server for its challenge (an initialize without a token).
func (o *OAuth) probe(ctx context.Context, serverURL string) string {
	body, _ := encodeRequest(1, "initialize", map[string]interface{}{"protocolVersion": ProtocolVersion,
		"capabilities": map[string]interface{}{}, "clientInfo": map[string]string{"name": "memdoor", "version": "1"}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL, bytes.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	return resp.Header.Get("WWW-Authenticate")
}

// register makes Memdoor a client of the authorization server (RFC 7591),
// unless a sign-in kept from before already holds a client id for it.
func (l *Login) register(ctx context.Context) error {
	if t, _ := l.o.Tokens.Get(l.server); t != nil && t.ClientID != "" && t.TokenEndpoint == l.ep.token {
		l.clientID, l.secret = t.ClientID, t.ClientSecret
		return nil
	}
	if l.ep.register == "" {
		return errors.New("the server's authorization server does not let apps register themselves; it needs a client id configured by hand")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"client_name":                l.o.ClientName,
		"redirect_uris":              []string{l.redirect},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.ep.register, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := l.o.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("client registration: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("client registration refused (HTTP %d): %s", resp.StatusCode, truncate(string(b), 200))
	}
	var reg struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(b, &reg); err != nil || reg.ClientID == "" {
		return fmt.Errorf("client registration: no client id in the answer")
	}
	l.clientID, l.secret = reg.ClientID, reg.ClientSecret
	return nil
}

const callbackPage = `<!doctype html><meta charset="utf-8"><title>Memdoor</title>
<body style="font-family:system-ui;max-width:32rem;margin:15vh auto;text-align:center">
<h2>%s</h2><p>%s</p></body>`

func (l *Login) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if e := q.Get("error"); e != "" {
		fmt.Fprintf(w, callbackPage, "Sign-in refused", "The server said: "+htmlEscape(e+" "+q.Get("error_description"))+". You can close this tab.")
		l.deliver("error:" + e + " " + q.Get("error_description"))
		return
	}
	if q.Get("state") != l.state || q.Get("code") == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, callbackPage, "This link is not for this sign-in", "Start again from Memdoor.")
		return
	}
	fmt.Fprintf(w, callbackPage, "Signed in to Memdoor", "You can close this tab and go back to the terminal.")
	l.deliver(q.Get("code"))
}

func (l *Login) deliver(code string) {
	select {
	case l.codes <- code:
	default:
	}
}

// Deliver hands the sign-in a pasted redirect URL (or a bare code), for a
// browser that could not reach the local listener.
func (l *Login) Deliver(pasted string) error {
	pasted = strings.TrimSpace(pasted)
	if u, err := url.Parse(pasted); err == nil && u.RawQuery != "" {
		q := u.Query()
		if q.Get("state") != "" && q.Get("state") != l.state {
			return errors.New("that link belongs to another sign-in")
		}
		if q.Get("code") != "" {
			pasted = q.Get("code")
		}
	}
	if pasted == "" {
		return errors.New("no code in what was pasted")
	}
	l.deliver(pasted)
	return nil
}

// Wait waits for the code, exchanges it, and stores the token.
func (l *Login) Wait(ctx context.Context) error {
	defer l.Close()
	timeout := l.o.LoginTimeout
	var code string
	select {
	case code = <-l.codes:
	case <-time.After(timeout):
		return fmt.Errorf("no sign-in within %s", timeout)
	case <-ctx.Done():
		return ctx.Err()
	}
	if e, ok := strings.CutPrefix(code, "error:"); ok {
		return fmt.Errorf("the server refused the sign-in: %s", strings.TrimSpace(e))
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {l.redirect},
		"client_id":     {l.clientID},
		"code_verifier": {l.verifier},
		"resource":      {l.ep.resource},
	}
	if l.secret != "" {
		form.Set("client_secret", l.secret)
	}
	t, err := l.o.tokenRequest(ctx, l.ep.token, form)
	if err != nil {
		return err
	}
	t.TokenEndpoint, t.ClientID, t.ClientSecret, t.Resource = l.ep.token, l.clientID, l.secret, l.ep.resource
	return l.o.Tokens.Put(l.server, t)
}

// Close stops the listener.
func (l *Login) Close() {
	if l.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = l.srv.Shutdown(ctx)
		cancel()
	}
}

func (o *OAuth) refresh(ctx context.Context, t *Token) (*Token, error) {
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {t.RefreshToken}, "client_id": {t.ClientID}}
	if t.Resource != "" {
		form.Set("resource", t.Resource)
	}
	if t.ClientSecret != "" {
		form.Set("client_secret", t.ClientSecret)
	}
	fresh, err := o.tokenRequest(ctx, t.TokenEndpoint, form)
	if err != nil {
		return nil, err
	}
	fresh.TokenEndpoint, fresh.ClientID, fresh.ClientSecret, fresh.Resource = t.TokenEndpoint, t.ClientID, t.ClientSecret, t.Resource
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = t.RefreshToken // the server may keep the old one valid
	}
	return fresh, nil
}

func (o *OAuth) tokenRequest(ctx context.Context, endpoint string, form url.Values) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}
	_ = json.Unmarshal(b, &tr)
	if resp.StatusCode >= 300 || tr.AccessToken == "" {
		why := strings.TrimSpace(tr.Error + " " + tr.Description)
		if why == "" {
			why = truncate(string(b), 200)
		}
		return nil, fmt.Errorf("token request refused (HTTP %d): %s", resp.StatusCode, why)
	}
	t := &Token{AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken, Scope: tr.Scope}
	if tr.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return t, nil
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
