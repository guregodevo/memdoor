// Package openrouter is "Sign in with OpenRouter": a person with no key signs
// in, or signs up for free, in the browser, and OpenRouter hands Memdoor a key
// of their own (OAuth PKCE, openrouter.ai/docs/use-cases/oauth-pkce). The maintainer,
// 2026-10-31: "they dont know the key". The key is an ordinary user key,
// billed to their credits; nothing of it passes through memdoor.ai.
//
// The flow: a loopback listener (127.0.0.1 and ::1, one port), the browser
// opened on openrouter.ai/auth with the S256 challenge and a state, the code
// back at /callback (or pasted, for a browser on another machine), then one
// POST to /api/v1/auth/keys with the verifier, which answers the key.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"memdoor/pkg/oidc"
)

// Endpoints are OpenRouter's sign-in page and its key exchange.
type Endpoints struct {
	Auth string
	Keys string
}

// Default is openrouter.ai.
func Default() Endpoints {
	return Endpoints{Auth: "https://openrouter.ai/auth", Keys: "https://openrouter.ai/api/v1/auth/keys"}
}

// KeyLabel names the key on the person's OpenRouter keys page.
const KeyLabel = "Memdoor"

// loginTimeout is how long a sign-in waits for the browser: OpenRouter's
// code lives ten minutes.
const loginTimeout = 10 * time.Minute

// Login is one sign-in in progress: open AuthURL; the code arrives at the
// local listener or through Deliver; Wait exchanges it for the key.
type Login struct {
	AuthURL string

	ep       Endpoints
	http     *http.Client
	verifier string
	state    string
	codes    chan callback
	srvs     []*http.Server
}

type callback struct{ code, err string }

// BeginLogin starts a sign-in: the local listener and the URL to open.
func BeginLogin(ep Endpoints, hc *http.Client) (*Login, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("no local port for the sign-in callback: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	l := &Login{ep: ep, http: hc, verifier: oidc.RandomString(64), state: oidc.RandomString(32), codes: make(chan callback, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", l.callback)
	l.serve(ln, mux)
	// OpenRouter takes a "localhost" callback; a browser may try ::1 first.
	if ln6, err := net.Listen("tcp", net.JoinHostPort("::1", strconv.Itoa(port))); err == nil {
		l.serve(ln6, mux)
	}
	q := url.Values{
		"callback_url":          {fmt.Sprintf("http://localhost:%d/callback", port)},
		"code_challenge":        {oidc.PKCEChallenge(l.verifier)},
		"code_challenge_method": {"S256"},
		"state":                 {l.state},
		"key_label":             {KeyLabel},
	}
	l.AuthURL = ep.Auth + "?" + q.Encode()
	return l, nil
}

// BeginHeadlessLogin starts a sign-in for a machine no browser can reach back
// to (an ssh session on a box): no callback, so OpenRouter's page shows a
// code, and the person pastes it (Deliver).
func BeginHeadlessLogin(ep Endpoints, hc *http.Client) *Login {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	l := &Login{ep: ep, http: hc, verifier: oidc.RandomString(64), state: oidc.RandomString(32), codes: make(chan callback, 1)}
	q := url.Values{
		"code_challenge":        {oidc.PKCEChallenge(l.verifier)},
		"code_challenge_method": {"S256"},
		"key_label":             {KeyLabel},
	}
	l.AuthURL = ep.Auth + "?" + q.Encode()
	return l
}

func (l *Login) serve(ln net.Listener, h http.Handler) {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	l.srvs = append(l.srvs, srv)
	go srv.Serve(ln)
}

const callbackPage = `<!doctype html><meta charset="utf-8"><title>Memdoor</title>
<body style="font-family:system-ui;max-width:32rem;margin:15vh auto;text-align:center">
<h2>%s</h2><p>%s</p></body>`

func (l *Login) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if e := strings.TrimSpace(q.Get("error") + " " + q.Get("error_description")); e != "" {
		fmt.Fprintf(w, callbackPage, "Sign-in refused", "OpenRouter said: "+html.EscapeString(e)+". You can close this tab.")
		l.deliver(callback{err: e})
		return
	}
	if (q.Get("state") != "" && q.Get("state") != l.state) || q.Get("code") == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, callbackPage, "This link is not for this sign-in", "Start again from Memdoor.")
		return
	}
	fmt.Fprintf(w, callbackPage, "Signed in to Memdoor with OpenRouter", "You can close this tab and go back to the terminal.")
	l.deliver(callback{code: q.Get("code")})
}

func (l *Login) deliver(c callback) {
	select {
	case l.codes <- c:
	default:
	}
}

// Deliver hands the sign-in what the browser showed on another machine: the
// address it landed on, or the code alone.
func (l *Login) Deliver(pasted string) error {
	pasted = strings.TrimSpace(pasted)
	if pasted == "" {
		return errors.New("paste the address the browser landed on, or the code")
	}
	if !strings.Contains(pasted, "://") {
		l.deliver(callback{code: pasted})
		return nil
	}
	u, err := url.Parse(pasted)
	if err != nil {
		return errors.New("paste the whole address of the page the browser landed on (it starts with http://localhost)")
	}
	q := u.Query()
	if s := q.Get("state"); s != "" && s != l.state {
		return errors.New("that link belongs to another sign-in")
	}
	if q.Get("code") == "" {
		return errors.New("no code in that link")
	}
	l.deliver(callback{code: q.Get("code")})
	return nil
}

// Wait waits for the code and exchanges it for the person's key.
func (l *Login) Wait(ctx context.Context) (string, error) {
	defer l.Close()
	var c callback
	select {
	case c = <-l.codes:
	case <-time.After(loginTimeout):
		return "", fmt.Errorf("no sign-in within %s", loginTimeout)
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if c.err != "" {
		return "", fmt.Errorf("OpenRouter refused the sign-in: %s", c.err)
	}
	return l.exchange(ctx, c.code)
}

// exchange is the one call that turns the code into a key.
func (l *Login) exchange(ctx context.Context, code string) (string, error) {
	body, _ := json.Marshal(map[string]string{"code": code, "code_verifier": l.verifier, "code_challenge_method": "S256"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.ep.Keys, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := l.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenRouter did not answer the key exchange: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var out struct {
		Key   string `json:"key"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK || out.Key == "" {
		msg := out.Error.Message
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		return "", fmt.Errorf("OpenRouter refused the key exchange (HTTP %d): %s", resp.StatusCode, msg)
	}
	return out.Key, nil
}

// Close stops the listeners.
func (l *Login) Close() {
	for _, srv := range l.srvs {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = srv.Shutdown(ctx)
		cancel()
	}
}
