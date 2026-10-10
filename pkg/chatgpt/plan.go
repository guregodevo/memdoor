// Package chatgpt is "Sign in with ChatGPT" for an open-source, locally run
// app: a ChatGPT Plus or Pro subscriber spends their plan's allowance in
// Memdoor instead of paying for an API key (OpenAI, DevDay 2026-09-29; the
// flow at developers.openai.com/siwc/token-sharing-open-source). It is the
// one way a subscriber tries a BYOK agent for nothing, and OpenCode, Pi,
// Kilo and Amp all ship it (2026-10-10). Anthropic forbids the same for a
// Claude subscription; this package never touches one.
//
// The flow: a loopback listener on 127.0.0.1, the system browser opened on
// OpenAI's authorize page with PKCE, dynamic client registration on the
// first sign-in (the issued client id is kept), the code exchanged for an
// access token (an hour), a refresh token (30 days, rotated on use) and an
// ID token (verified against OpenAI's JWKS). Inference then goes to the
// Responses API with the bearer token, store:false, stream:true, and none
// of the fields the preview refuses. Nothing of this involves a client
// secret or an API key.
package chatgpt

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"memdoor/pkg/secrets"
)

// Endpoints is where the sign-in talks to; Default is OpenAI's, a test's is
// its own server.
type Endpoints struct {
	Issuer    string
	Authorize string
	Token     string
	JWKS      string
	Resource  string // the audience the access token is for: the API base
}

// Default is OpenAI's production sign-in.
func Default() Endpoints {
	const issuer = "https://auth.openai.com"
	return Endpoints{
		Issuer:    issuer,
		Authorize: issuer + "/api/accounts/authorize",
		Token:     issuer + "/api/accounts/oauth/token",
		JWKS:      issuer + "/.well-known/jwks.json",
		Resource:  APIBase,
	}
}

const (
	// APIBase is where a signed-in plan's requests go (the Responses API).
	APIBase = "https://api.openai.com/v1"
	// dynamicClient is the registration entrypoint: the first sign-in on a
	// host sends it and receives an issued client id to keep.
	dynamicClient = "dynamic_agent_client"
	// AgentName is how Memdoor is named on the consent page.
	AgentName = "Memdoor"
	// Scopes is the complete set plan usage needs; a sign-in missing one is
	// refused later with chatpass_v2_scope_not_authorized.
	Scopes = "openid profile email offline_access resource.invoke chatgpt.tokens.use.direct"

	credentialFile = "chatgpt-login.json"
	refreshBefore  = 5 * time.Minute
	loginTimeout   = 5 * time.Minute
)

// ErrNotSignedIn is the token source's answer when nobody signed in, or the
// sign-in expired: `memdoor connect chatgpt` again.
var ErrNotSignedIn = errors.New("not signed in with ChatGPT: run memdoor connect chatgpt (or /connect chatgpt in the window)")

// ErrSignInAgain is a registration that ended as OpenAI's docs say it can:
// the host was issued its client id and the code exchange answered
// invalid_grant. The id is kept; the next sign-in, under it, completes
// (developers.openai.com/siwc/token-sharing-open-source/sign-in: "discard
// that code and start a fresh authorization with the issued client ID").
// Live 2026-10-10: two registrations in a row ended exactly so.
var ErrSignInAgain = errors.New("Memdoor is now registered with your ChatGPT; one more approval completes the sign-in")

// Credential is one host's sign-in, as OpenAI's docs lay it out.
type Credential struct {
	Email        string    `json:"email"`
	Subject      string    `json:"subject"`
	ClientID     string    `json:"client_id"`
	HostID       string    `json:"ext_agent_host_id"`
	IDToken      string    `json:"id_token"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       []string  `json:"scopes"`
	SavedAt      time.Time `json:"saved_at"`
}

// Store keeps the credential on disk, encrypted with the master key
// (pkg/secrets), 0600, next to the host id — which stays after a sign-out,
// so the same host re-signs in under the client id it was issued.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore is the store under a memdoor directory (~/.memdoor).
func NewStore(memdoorDir string) *Store {
	return &Store{path: filepath.Join(memdoorDir, credentialFile)}
}

type storeFile struct {
	HostID     string `json:"ext_agent_host_id"`
	ClientID   string `json:"client_id,omitempty"`
	Credential string `json:"credential,omitempty"` // the Credential, encrypted
}

func (s *Store) read() (storeFile, error) {
	var f storeFile
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	return f, json.Unmarshal(b, &f)
}

func (s *Store) write(f storeFile) error {
	b, _ := json.MarshalIndent(f, "", "  ")
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Load is the credential, or nil when nobody is signed in.
func (s *Store) Load() (*Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil || f.Credential == "" {
		return nil, err
	}
	plain, err := secrets.Decrypt(f.Credential)
	if err != nil {
		return nil, err
	}
	var c Credential
	if err := json.Unmarshal([]byte(plain), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save keeps a credential (and its client id, which outlives it).
func (s *Store) Save(c *Credential) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	enc, err := secrets.Encrypt(string(b))
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	f.Credential, f.ClientID, f.HostID = enc, c.ClientID, c.HostID
	return s.write(f)
}

// Delete forgets the sign-in; the host id and the issued client id stay.
func (s *Store) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	if f.Credential == "" && f.HostID == "" {
		return nil
	}
	f.Credential = ""
	return s.write(f)
}

// HostID is this installation's ext_agent_host_id, made once and kept.
func (s *Store) HostID() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return "", err
	}
	if f.HostID != "" {
		return f.HostID, nil
	}
	f.HostID = "urn:uuid:" + uuid4()
	return f.HostID, s.write(f)
}

// savedClientID is the client id a past sign-in on this host was issued.
func (s *Store) savedClientID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, _ := s.read()
	return f.ClientID
}

// saveClientID keeps the id a registration issued, before any exchange: it
// is the host's from now on, whatever the first exchange answers.
func (s *Store) saveClientID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	if f.ClientID == id {
		return nil
	}
	f.ClientID = id
	return s.write(f)
}

// Login is one sign-in in progress: open AuthURL in a browser; the code
// arrives at the local listener, or through Deliver; Wait finishes it.
type Login struct {
	AuthURL string

	ep       Endpoints
	store    *Store
	http     *http.Client
	clientID string // dynamic_agent_client, or the one this host was issued
	hostID   string
	redirect string
	verifier string
	state    string
	nonce    string
	codes    chan callbackResult
	ln       net.Listener
	srv      *http.Server
}

type callbackResult struct {
	code, clientID, err string
	params              string // the callback's parameter names, for a refusal's diagnosis
}

// BeginLogin starts a sign-in: the local listener, PKCE, and the URL to
// open. A host signed in before reuses its issued client id.
func BeginLogin(ctx context.Context, ep Endpoints, store *Store, hc *http.Client) (*Login, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	hostID, err := store.HostID()
	if err != nil {
		return nil, fmt.Errorf("the host id could not be kept: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("no local port for the sign-in callback: %w", err)
	}
	l := &Login{ep: ep, store: store, http: hc, hostID: hostID, ln: ln, codes: make(chan callbackResult, 1),
		redirect: fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port),
		verifier: randomString(64), state: randomString(32), nonce: randomString(32)}
	l.clientID = store.savedClientID()
	q := url.Values{
		"response_type":         {"code"},
		"redirect_uri":          {l.redirect},
		"scope":                 {Scopes},
		"resource":              {ep.Resource},
		"state":                 {l.state},
		"nonce":                 {l.nonce},
		"code_challenge_method": {"S256"},
		"code_challenge":        {pkceChallenge(l.verifier)},
		"ext_agent_host_id":     {hostID},
	}
	if l.clientID == "" {
		l.clientID = dynamicClient
		q.Set("client_id", dynamicClient)
		q.Set("agent_name_hint", AgentName)
	} else {
		q.Set("client_id", l.clientID)
	}
	l.AuthURL = ep.Authorize + "?" + q.Encode()
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", l.callback)
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go l.srv.Serve(ln)
	return l, nil
}

const callbackPage = `<!doctype html><meta charset="utf-8"><title>Memdoor</title>
<body style="font-family:system-ui;max-width:32rem;margin:15vh auto;text-align:center">
<h2>%s</h2><p>%s</p></body>`

func (l *Login) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if e := q.Get("error"); e != "" {
		fmt.Fprintf(w, callbackPage, "Sign-in refused", "OpenAI said: "+htmlEscape(strings.TrimSpace(e+" "+q.Get("error_description")))+". You can close this tab.")
		l.deliver(callbackResult{err: strings.TrimSpace(e + " " + q.Get("error_description"))})
		return
	}
	if q.Get("state") != l.state || q.Get("code") == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, callbackPage, "This link is not for this sign-in", "Start again from Memdoor.")
		return
	}
	fmt.Fprintf(w, callbackPage, "Signed in to Memdoor with ChatGPT", "You can close this tab and go back to the terminal.")
	l.deliver(callbackResult{code: q.Get("code"), clientID: q.Get("client_id"), params: paramNames(q)})
}

// paramNames lists a query's parameter names (never their values), so a
// refused exchange can say what the callback carried.
func paramNames(q url.Values) string {
	names := make([]string, 0, len(q))
	for k := range q {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func (l *Login) deliver(r callbackResult) {
	select {
	case l.codes <- r:
	default:
	}
}

// Deliver hands the sign-in a pasted redirect URL, for a browser that could
// not reach the local listener.
func (l *Login) Deliver(pasted string) error {
	u, err := url.Parse(strings.TrimSpace(pasted))
	if err != nil || u.RawQuery == "" {
		return errors.New("paste the whole address of the page the browser landed on (it starts with http://127.0.0.1)")
	}
	q := u.Query()
	if q.Get("state") != l.state {
		return errors.New("that link belongs to another sign-in")
	}
	if q.Get("code") == "" {
		return errors.New("no code in that link")
	}
	l.deliver(callbackResult{code: q.Get("code"), clientID: q.Get("client_id")})
	return nil
}

// Wait waits for the code, exchanges it, verifies the ID token and keeps
// the credential.
func (l *Login) Wait(ctx context.Context) (*Credential, error) {
	defer l.Close()
	var r callbackResult
	select {
	case r = <-l.codes:
	case <-time.After(loginTimeout):
		return nil, fmt.Errorf("no sign-in within %s", loginTimeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if r.err != "" {
		return nil, fmt.Errorf("OpenAI refused the sign-in: %s", r.err)
	}
	clientID := l.clientID
	registered := false
	if r.clientID != "" && r.clientID != l.clientID {
		clientID, registered = r.clientID, true // the id this host was just issued
		if err := l.store.saveClientID(clientID); err != nil {
			return nil, fmt.Errorf("the issued client id could not be kept: %w", err)
		}
	}
	if clientID == dynamicClient {
		return nil, errors.New("the sign-in returned no client id for this host")
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {r.code},
		"code_verifier": {l.verifier},
		"redirect_uri":  {l.redirect},
		"resource":      {l.ep.Resource},
	}
	tr, err := tokenRequest(ctx, l.http, l.ep.Token, form)
	if err != nil {
		var te *TokenError
		if registered && errors.As(err, &te) && te.Code == "invalid_grant" {
			return nil, ErrSignInAgain
		}
		return nil, fmt.Errorf("%w (callback carried %s; client %s; redirect %s)", err, r.params, abbreviate(clientID), l.redirect)
	}
	claims, err := verifyIDToken(ctx, l.http, l.ep, tr.IDToken, clientID, l.nonce)
	if err != nil {
		return nil, fmt.Errorf("the ID token did not verify: %w", err)
	}
	c := &Credential{Email: claims.Email, Subject: claims.Subject, ClientID: clientID, HostID: l.hostID, IDToken: tr.IDToken,
		AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken, Scopes: strings.Fields(tr.Scope), SavedAt: time.Now().UTC()}
	if tr.ExpiresIn > 0 {
		c.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	if !hasScope(c.Scopes, "chatgpt.tokens.use.direct") {
		return nil, errors.New("the sign-in did not grant plan usage (chatgpt.tokens.use.direct): sign in again and allow it")
	}
	return c, l.store.Save(c)
}

// Close stops the listener.
func (l *Login) Close() {
	if l.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = l.srv.Shutdown(ctx)
		cancel()
	}
}

type tokenReply struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

func tokenRequest(ctx context.Context, hc *http.Client, endpoint string, form url.Values) (*tokenReply, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr tokenReply
	_ = json.Unmarshal(b, &tr)
	if resp.StatusCode >= 300 || tr.AccessToken == "" {
		why := strings.TrimSpace(tr.Error + " " + tr.Description)
		if tr.Description == "" {
			why = strings.TrimSpace(why + " " + truncate(string(b), 300))
		}
		return nil, &TokenError{Status: resp.StatusCode, Code: tr.Error, Message: why}
	}
	return &tr, nil
}

// TokenError is the token endpoint's refusal, with its code
// (token_expired, refresh_token_expired, invalid_grant …).
type TokenError struct {
	Status  int
	Code    string
	Message string
}

func (e *TokenError) Error() string {
	return fmt.Sprintf("token request refused (HTTP %d): %s", e.Status, e.Message)
}

// Source hands out the access token, refreshed before it expires and kept;
// the refresh token rotates on every use. A sign-in the server no longer
// accepts is forgotten, and the answer is ErrNotSignedIn.
type Source struct {
	ep    Endpoints
	store *Store
	http  *http.Client
	mu    sync.Mutex
}

// NewSource is the token source over a store.
func NewSource(ep Endpoints, store *Store, hc *http.Client) *Source {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Source{ep: ep, store: store, http: hc}
}

// Token is a valid access token, or ErrNotSignedIn.
func (s *Source) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.store.Load()
	if err != nil {
		return "", err
	}
	if c == nil || c.RefreshToken == "" {
		return "", ErrNotSignedIn
	}
	if c.ExpiresAt.IsZero() || time.Until(c.ExpiresAt) > refreshBefore {
		return c.AccessToken, nil
	}
	fresh, err := s.refresh(ctx, c)
	if err != nil {
		var te *TokenError
		if errors.As(err, &te) && (te.Code == "token_expired" || te.Code == "refresh_token_expired" || te.Code == "invalid_grant") {
			_ = s.store.Delete()
			return "", ErrNotSignedIn
		}
		if time.Now().Before(c.ExpiresAt) {
			return c.AccessToken, nil // still valid; refresh later
		}
		return "", fmt.Errorf("the ChatGPT sign-in could not be renewed: %w", err)
	}
	return fresh.AccessToken, nil
}

func (s *Source) refresh(ctx context.Context, c *Credential) (*Credential, error) {
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {c.RefreshToken}, "client_id": {c.ClientID}}
	tr, err := tokenRequest(ctx, s.http, s.ep.Token, form)
	if err != nil {
		return nil, err
	}
	fresh := *c
	fresh.AccessToken, fresh.SavedAt = tr.AccessToken, time.Now().UTC()
	if tr.RefreshToken != "" {
		fresh.RefreshToken = tr.RefreshToken
	}
	if tr.IDToken != "" {
		fresh.IDToken = tr.IDToken
	}
	if tr.ExpiresIn > 0 {
		fresh.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return &fresh, s.store.Save(&fresh)
}

// ---- the ID token ---------------------------------------------------------

type idClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience any    `json:"aud"`
	Expires  int64  `json:"exp"`
	Nonce    string `json:"nonce"`
	Email    string `json:"email"`
}

// verifyIDToken checks the ID token's signature against the issuer's JWKS,
// then its issuer, audience (the issued client id), expiry and the nonce
// of this attempt, as OpenAI's docs require.
func verifyIDToken(ctx context.Context, hc *http.Client, ep Endpoints, token, clientID, nonce string) (*idClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("not a JWT")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(hb, &header) != nil {
		return nil, errors.New("unreadable header")
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("unreadable claims")
	}
	var claims idClaims
	if err := json.Unmarshal(pb, &claims); err != nil {
		return nil, errors.New("unreadable claims")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("unreadable signature")
	}
	keys, err := fetchJWKS(ctx, hc, ep.JWKS)
	if err != nil {
		return nil, err
	}
	signed := []byte(parts[0] + "." + parts[1])
	sum := sha256.Sum256(signed)
	verified := false
	for _, k := range keys {
		if header.Kid != "" && k.Kid != "" && k.Kid != header.Kid {
			continue
		}
		switch {
		case header.Alg == "RS256" && k.Kty == "RSA":
			if pub, err := k.rsa(); err == nil && rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig) == nil {
				verified = true
			}
		case header.Alg == "ES256" && k.Kty == "EC":
			if pub, err := k.ecdsa(); err == nil && len(sig) == 64 {
				r := new(big.Int).SetBytes(sig[:32])
				s := new(big.Int).SetBytes(sig[32:])
				if ecdsa.Verify(pub, sum[:], r, s) {
					verified = true
				}
			}
		}
		if verified {
			break
		}
	}
	if !verified {
		return nil, fmt.Errorf("signature (%s) matched no key of %s", header.Alg, ep.JWKS)
	}
	if claims.Issuer != ep.Issuer {
		return nil, fmt.Errorf("issuer %q, want %q", claims.Issuer, ep.Issuer)
	}
	if !audienceHas(claims.Audience, clientID) {
		return nil, fmt.Errorf("audience %v is not this client", claims.Audience)
	}
	if claims.Expires > 0 && time.Now().Unix() >= claims.Expires {
		return nil, errors.New("expired")
	}
	if claims.Nonce != nonce {
		return nil, errors.New("nonce mismatch")
	}
	return &claims, nil
}

func audienceHas(aud any, clientID string) bool {
	switch a := aud.(type) {
	case string:
		return a == clientID
	case []any:
		for _, v := range a {
			if s, ok := v.(string); ok && s == clientID {
				return true
			}
		}
	}
	return false
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k jwk) rsa() (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
}

func (k jwk) ecdsa() (*ecdsa.PublicKey, error) {
	if k.Crv != "P-256" {
		return nil, fmt.Errorf("curve %s", k.Crv)
	}
	x, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, err
	}
	y, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, err
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
}

var jwksCache = struct {
	sync.Mutex
	at   map[string]time.Time
	keys map[string][]jwk
}{at: map[string]time.Time{}, keys: map[string][]jwk{}}

func fetchJWKS(ctx context.Context, hc *http.Client, u string) ([]jwk, error) {
	jwksCache.Lock()
	if keys, ok := jwksCache.keys[u]; ok && time.Since(jwksCache.at[u]) < time.Hour {
		jwksCache.Unlock()
		return keys, nil
	}
	jwksCache.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the signing keys could not be read: %w", err)
	}
	defer resp.Body.Close()
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("the signing keys could not be read: %w", err)
	}
	jwksCache.Lock()
	jwksCache.at[u], jwksCache.keys[u] = time.Now(), doc.Keys
	jwksCache.Unlock()
	return doc.Keys, nil
}

// ---- small helpers --------------------------------------------------------

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func uuid4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// abbreviate is a client id's shape for a message: its prefix and length.
func abbreviate(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8] + "…(" + fmt.Sprint(len(id)) + ")"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
