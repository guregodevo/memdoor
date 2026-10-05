package billingsvc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/email"
)

// fakeMailer keeps what would have been sent, so the test can read the
// code off the "inbox" the way a person does.
type fakeMailer struct{ sent []email.Email }

func (f *fakeMailer) Send(e email.Email) error { f.sent = append(f.sent, e); return nil }

func newEmailAuthServer(t *testing.T) (*Server, *fakeMailer) {
	t.Helper()
	m := &fakeMailer{}
	s := &Server{auth: newAuthStore(filepath.Join(t.TempDir(), "auth.json")), mail: m}
	return s, m
}

func postJSON(t *testing.T, h http.HandlerFunc, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.RemoteAddr = "203.0.113.7:4242"
	rec := httptest.NewRecorder()
	h(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

var codeRe = regexp.MustCompile(`\b(\d{6})\b`)

func TestEmailCodeSignIn_CreatesAccountAndToken(t *testing.T) {
	authEmailLimiter.Reset("203.0.113.7")
	s, m := newEmailAuthServer(t)

	rec, out := postJSON(t, s.handleAuthEmail, map[string]string{"email": "Sara.Pop@Gmail.com"})
	if rec.Code != http.StatusOK || out["sent"] != true {
		t.Fatalf("request code: %d %s", rec.Code, rec.Body.String())
	}
	if len(m.sent) != 1 || m.sent[0].To != "sara.pop@gmail.com" {
		t.Fatalf("code should be mailed to the normalized address, got %+v", m.sent)
	}
	code := codeRe.FindString(m.sent[0].Body)
	if code == "" {
		t.Fatalf("no six-digit code in the mail body:\n%s", m.sent[0].Body)
	}

	// A wrong code is refused and counted.
	rec, _ = postJSON(t, s.handleAuthCode, map[string]string{"email": "sara.pop@gmail.com", "code": "000000"})
	if rec.Code != http.StatusUnauthorized && code != "000000" {
		t.Fatalf("wrong code should be 401, got %d", rec.Code)
	}

	rec, out = postJSON(t, s.handleAuthCode, map[string]string{"email": "sara.pop@gmail.com", "code": code})
	if rec.Code != http.StatusOK {
		t.Fatalf("right code: %d %s", rec.Code, rec.Body.String())
	}
	token, _ := out["token"].(string)
	if token == "" || out["workspace"] != "sara-pop" || out["plan"] != "free" {
		t.Fatalf("unexpected sign-in answer: %v", out)
	}

	// The code is single-use.
	rec, _ = postJSON(t, s.handleAuthCode, map[string]string{"email": "sara.pop@gmail.com", "code": code})
	if rec.Code == http.StatusOK {
		t.Fatalf("a used code must not sign in again")
	}

	// The token answers /v1/me with the same account.
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.handleMe(rec, req)
	var me map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if rec.Code != http.StatusOK || me["email"] != "sara.pop@gmail.com" || me["workspace"] != "sara-pop" {
		t.Fatalf("/v1/me: %d %v", rec.Code, me)
	}
	if _, leaked := me["token"]; leaked {
		t.Fatalf("/v1/me must not mint or echo a token")
	}

	// Signing in again lands on the SAME workspace, and a different person
	// with the same local part gets a different one.
	s.auth.mu.Lock()
	ws1, _, _ := s.auth.ensureAccountLocked("sara.pop@gmail.com")
	ws2, _, _ := s.auth.ensureAccountLocked("sara.pop@icloud.com")
	s.auth.mu.Unlock()
	if ws1 != "sara-pop" || ws2 == ws1 || len(ws2) <= len("sara-pop") {
		t.Fatalf("workspaces: %q %q", ws1, ws2)
	}
}

func TestEmailCode_ExpiresAndLocksAfterAttempts(t *testing.T) {
	s, _ := newEmailAuthServer(t)
	s.auth.Codes["x@example.com"] = &emailCode{Code: "123456", CreatedAt: time.Now().Add(-emailCodeTTL - time.Second)}
	rec, _ := postJSON(t, s.handleAuthCode, map[string]string{"email": "x@example.com", "code": "123456"})
	if rec.Code != http.StatusGone {
		t.Fatalf("expired code should be 410, got %d", rec.Code)
	}

	s.auth.Codes["y@example.com"] = &emailCode{Code: "654321", CreatedAt: time.Now()}
	for i := 0; i < emailCodeAttempts; i++ {
		postJSON(t, s.handleAuthCode, map[string]string{"email": "y@example.com", "code": "000000"})
	}
	rec, _ = postJSON(t, s.handleAuthCode, map[string]string{"email": "y@example.com", "code": "654321"})
	if rec.Code == http.StatusOK {
		t.Fatalf("after %d wrong tries the right code must not work", emailCodeAttempts)
	}
}

// A WRONG CODE HAS TO SAY WHAT IS LEFT (onboarding battle test, 2026-09-27).
// "that code is not right" taught nobody anything: not that it is six digits
// from the newest email, not that it dies in ten minutes, not that the last
// wrong try throws the code away. Greg pasted the example from the docs and
// learned only that he was wrong.
func TestWrongCodeSaysWhatIsLeft(t *testing.T) {
	s, _ := newEmailAuthServer(t)
	s.auth.Codes["z@example.com"] = &emailCode{Code: "654321", CreatedAt: time.Now()}
	rec, out := postJSON(t, s.handleAuthCode, map[string]string{"email": "z@example.com", "code": "000000"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong code is 401, got %d", rec.Code)
	}
	body, _ := out["error"].(string)
	for _, want := range []string{"six digits", "10 minutes", "4 tries left"} {
		if !strings.Contains(body, want) {
			t.Errorf("the message should mention %q: %s", want, body)
		}
	}
	// The last try says so, rather than counting down to zero.
	for i := 0; i < emailCodeAttempts-2; i++ {
		postJSON(t, s.handleAuthCode, map[string]string{"email": "z@example.com", "code": "000000"})
	}
	_, out = postJSON(t, s.handleAuthCode, map[string]string{"email": "z@example.com", "code": "000000"})
	body, _ = out["error"].(string)
	if !strings.Contains(body, "last try") || !strings.Contains(body, "ask for a new one") {
		t.Errorf("the final wrong try should say the code is gone: %s", body)
	}
}

func TestEmailCode_WithoutMailerSaysSo(t *testing.T) {
	authEmailLimiter.Reset("203.0.113.7")
	s := &Server{auth: newAuthStore(filepath.Join(t.TempDir(), "auth.json"))}
	rec, _ := postJSON(t, s.handleAuthEmail, map[string]string{"email": "x@example.com"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no mailer should be 503, got %d", rec.Code)
	}
}

func TestWorkspaceSlugFor(t *testing.T) {
	cases := map[string]string{
		"sara.pop@gmail.com":                    "sara-pop",
		"Coach_Mike+yt@example.com":             "coach-mike-yt",
		"@nothing.com":                          "creator",
		"averyveryverylongnamethatgoeson@x.com": "averyveryverylongnametha",
	}
	for in, want := range cases {
		if got := workspaceSlugFor(in); got != want {
			t.Errorf("%s → %q, want %q", in, got, want)
		}
	}
}
