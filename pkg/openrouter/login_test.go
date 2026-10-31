package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"memdoor/pkg/oidc"
)

// fakeOpenRouter answers the key exchange: the key for code "c1" when the
// verifier matches the challenge the sign-in page was opened with.
func fakeOpenRouter(t *testing.T, challenge *string) Endpoints {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.Method != http.MethodPost || body["code"] != "c1" || body["code_challenge_method"] != "S256" || oidc.PKCEChallenge(body["code_verifier"]) != *challenge {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "Invalid code"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"key": "sk-or-v1-the-key"})
	}))
	t.Cleanup(srv.Close)
	return Endpoints{Auth: "https://openrouter.test/auth", Keys: srv.URL}
}

func begin(t *testing.T, ep Endpoints, challenge *string) (*Login, url.Values) {
	t.Helper()
	l, err := BeginLogin(ep, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(l.Close)
	u, err := url.Parse(l.AuthURL)
	if err != nil || !strings.HasPrefix(l.AuthURL, ep.Auth+"?") {
		t.Fatalf("AuthURL %q", l.AuthURL)
	}
	q := u.Query()
	*challenge = q.Get("code_challenge")
	if q.Get("code_challenge_method") != "S256" || q.Get("key_label") != KeyLabel || q.Get("state") == "" ||
		!strings.HasPrefix(q.Get("callback_url"), "http://localhost:") {
		t.Fatalf("the sign-in page is opened with %v", q)
	}
	return l, q
}

func wait(t *testing.T, l *Login) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return l.Wait(ctx)
}

// The browser lands on the local callback: the code is exchanged, with the
// verifier, for the person's key.
func TestSignInThroughTheCallback(t *testing.T) {
	var challenge string
	ep := fakeOpenRouter(t, &challenge)
	l, q := begin(t, ep, &challenge)
	cb := strings.Replace(q.Get("callback_url"), "localhost", "127.0.0.1", 1)
	resp, err := http.Get(cb + "?code=c1&state=" + url.QueryEscape(q.Get("state")))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("callback: %v %v", err, resp)
	}
	resp.Body.Close()
	key, err := wait(t, l)
	if err != nil || key != "sk-or-v1-the-key" {
		t.Fatalf("key %q, err %v", key, err)
	}
}

// A browser on another machine: the code alone, or the address it landed
// on, is pasted. A link from another sign-in is refused.
func TestSignInByPaste(t *testing.T) {
	var challenge string
	ep := fakeOpenRouter(t, &challenge)
	l, _ := begin(t, ep, &challenge)
	if err := l.Deliver("http://localhost:1/callback?code=c1&state=someone-else"); err == nil {
		t.Fatal("a link from another sign-in was taken")
	}
	if err := l.Deliver(""); err == nil {
		t.Fatal("nothing pasted was taken")
	}
	if err := l.Deliver("c1"); err != nil {
		t.Fatal(err)
	}
	if key, err := wait(t, l); err != nil || key != "sk-or-v1-the-key" {
		t.Fatalf("key %q, err %v", key, err)
	}

	l2, q2 := begin(t, ep, &challenge)
	if err := l2.Deliver("http://localhost:9/callback?code=c1&state=" + url.QueryEscape(q2.Get("state"))); err != nil {
		t.Fatal(err)
	}
	if key, err := wait(t, l2); err != nil || key != "sk-or-v1-the-key" {
		t.Fatalf("pasted address: key %q, err %v", key, err)
	}
}

// Headless: no callback and no listener; the code OpenRouter shows is pasted.
func TestHeadlessSignIn(t *testing.T) {
	var challenge string
	ep := fakeOpenRouter(t, &challenge)
	l := BeginHeadlessLogin(ep, nil)
	u, _ := url.Parse(l.AuthURL)
	q := u.Query()
	challenge = q.Get("code_challenge")
	if q.Get("callback_url") != "" || q.Get("code_challenge_method") != "S256" || q.Get("key_label") != KeyLabel || len(l.srvs) != 0 {
		t.Fatalf("headless: no callback, no listener: %v %d", q, len(l.srvs))
	}
	if err := l.Deliver("c1"); err != nil {
		t.Fatal(err)
	}
	if key, err := wait(t, l); err != nil || key != "sk-or-v1-the-key" {
		t.Fatalf("key %q, err %v", key, err)
	}
}

// A refused exchange is said in OpenRouter's words; a refused sign-in too.
func TestSignInRefused(t *testing.T) {
	var challenge string
	ep := fakeOpenRouter(t, &challenge)
	l, _ := begin(t, ep, &challenge)
	_ = l.Deliver("wrong-code")
	if _, err := wait(t, l); err == nil || !strings.Contains(err.Error(), "Invalid code") {
		t.Fatalf("want OpenRouter's refusal, got %v", err)
	}
	l2, q2 := begin(t, ep, &challenge)
	cb := strings.Replace(q2.Get("callback_url"), "localhost", "127.0.0.1", 1)
	resp, err := http.Get(cb + "?error=access_denied")
	if err == nil {
		resp.Body.Close()
	}
	if _, err := wait(t, l2); err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Fatalf("want the refusal, got %v", err)
	}
}
