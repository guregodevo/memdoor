package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// A lapsed session is renewed from the login this Mac generated, without a
// word; without that file it is "sign in again" — never "ready" on a token
// that has run out (the mutation check: count any token as a session and
// the first case reports ready).
func TestLapsedLocalSessionRenewsOrAsksToSignIn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	logins := 0
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/login" {
			http.NotFound(w, r)
			return
		}
		logins++
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "fresh", "user": map[string]string{"email": "coach@example.com"}})
	}))
	defer gw.Close()
	gatewayAddr = gw.URL

	lapsed := func() {
		t.Helper()
		if err := saveCredentials(&credentials{Token: "old", Email: "coach@example.com", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	if ready, again, note := localSessionReady(); ready || !again || !strings.Contains(note, "sign in again") {
		t.Fatalf("no session at all: want 'sign in again', got ready=%v again=%v note=%q", ready, again, note)
	}
	lapsed()
	if ready, again, note := localSessionReady(); ready || !again || !strings.Contains(note, "run out") {
		t.Fatalf("lapsed session, nothing to renew it with: want the sign-in page, got ready=%v again=%v note=%q", ready, again, note)
	}
	if logins != 0 {
		t.Fatalf("nothing on disk to log in with, yet %d login(s) were tried", logins)
	}

	rec, _ := json.Marshal(localLogin{Email: "coach@example.com", Password: "generated"})
	if err := writeHomeFile(localLoginFile, rec, 0o600); err != nil {
		t.Fatal(err)
	}
	if ready, _, note := localSessionReady(); !ready {
		t.Fatalf("lapsed session with the generated login on disk: want renewed, got %q", note)
	}
	if logins != 1 {
		t.Fatalf("want one renewal login, got %d", logins)
	}
	creds, err := loadCredentials()
	if err != nil || creds.Token != "fresh" || !creds.ExpiresAt.After(time.Now()) {
		t.Fatalf("renewal must save the new session: %+v %v", creds, err)
	}
	if ready, _, _ := localSessionReady(); !ready || logins != 1 {
		t.Fatalf("a live session is ready without logging in again (logins=%d)", logins)
	}
}

// The sign-in makes the engine's user the account's: the user the LAPSED
// session belonged to is moved to the account's email and given a new
// password, the generated login is written for it, and a session is
// opened on it. The mutation check: count a lapsed token as a session and
// nothing is adopted; drop the rename and the login goes to the old name.
func TestSignInAdoptsTheEnginesOwnUser(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved, savedAdopt := gatewayAddr, adoptLocalUser
	t.Cleanup(func() { gatewayAddr, adoptLocalUser = saved, savedAdopt })
	var fromEmail, toEmail, setPassword string
	adopts := 0
	adoptLocalUser = func(_ context.Context, _, engineEmail, accountEmail, password string) error {
		adopts++
		fromEmail, toEmail, setPassword = engineEmail, accountEmail, password
		return nil
	}
	var loginEmail, loginPassword string
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		loginEmail, loginPassword = in["email"], in["password"]
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "adopted", "user": map[string]string{"email": in["email"]}})
	}))
	defer gw.Close()
	gatewayAddr = gw.URL
	if err := saveCredentials(&credentials{Token: "old", Email: "greg@test.local", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}

	// Through the sign-in's own decision: a lapsed token is not a session.
	if err := localSessionFor("greg@gmail.com"); err != nil {
		t.Fatal(err)
	}
	if adopts != 1 || fromEmail != "greg@test.local" || toEmail != "greg@gmail.com" {
		t.Fatalf("want the lapsed session's user moved to the account: adopts=%d %q → %q", adopts, fromEmail, toEmail)
	}
	if len(setPassword) < 8 || loginPassword != setPassword || loginEmail != "greg@gmail.com" {
		t.Fatalf("the session must open on the account's name with the password just set: set %q, login %q/%q", setPassword, loginEmail, loginPassword)
	}
	var ll localLogin
	if b, err := os.ReadFile(homePath(localLoginFile)); err != nil || json.Unmarshal(b, &ll) != nil || ll.Password != setPassword || ll.Email != "greg@gmail.com" {
		t.Fatalf("the generated login must be kept for the next renewal, under the account's name: %v %+v", err, ll)
	}
	if creds, err := loadCredentials(); err != nil || creds.Token != "adopted" || creds.Email != "greg@gmail.com" || !creds.ExpiresAt.After(time.Now()) {
		t.Fatalf("a live session for the account must be saved: %+v %v", creds, err)
	}
	if err := localSessionFor("greg@gmail.com"); err != nil || adopts != 1 {
		t.Fatalf("a live session for the account is left alone (adopts=%d, %v)", adopts, err)
	}
	// Someone else signs in on this Mac: the engine's user follows.
	if err := localSessionFor("coach@example.com"); err != nil || adopts != 2 || fromEmail != "greg@gmail.com" || toEmail != "coach@example.com" {
		t.Fatalf("a live session for another account is adopted: adopts=%d %q → %q (%v)", adopts, fromEmail, toEmail, err)
	}
}

// The gate names the state and the next step: a starting brain is a wait,
// nothing configured is a key or a subscription, and a brain that answers (or a
// developer gateway with none) gates nothing.
//
// THE FREE PATH IS THE PERSON'S OWN KEY (2026-09-27). This test used to require
// the words "get a seat", from the months when a subscription was the only way
// to a model at all. A coding agent with no model is one export away from
// working, so the key is the step (a seat never supplies one, 2026-10-04).
func TestTheBrainGateNamesTheNextStep(t *testing.T) {
	g := brainGate("none")
	// One way out: a key of your own. A seat never supplies one (Greg,
	// 2026-10-04: "Pro seat never offer key"), so it is not offered here.
	if !strings.Contains(g, "OPEN_ROUTER_API_KEY") || !strings.Contains(g, "/connect") || !strings.Contains(g, "memdoor tui again") || strings.Contains(g, "subscribe") {
		t.Fatalf("nothing configured names the key, never a seat: %q", g)
	}
	if strings.Contains(g, "shorts") {
		t.Fatalf("the video era is over: %q", g)
	}
	// An unknown state must not invent a step.
	for _, st := range []string{"ready", "starting", "stopped", ""} {
		if g := brainGate(st); g != "" {
			t.Fatalf("%q gates nothing, got %q", st, g)
		}
	}
}

// THE SIGN-IN RUNS WHERE A NEW PERSON STANDS. `memdoor account login` is the
// first command after paying, typed in whatever directory they are in, and it
// used to refuse with "no workspace resolved for \"account\"" and four ways to
// fix a problem they do not have — the workspace is named after the account the
// command is about to create (paid onboarding walk, 2026-09-27: Greg paid, then
// could not sign in).
func TestSignInNeedsNoWorkspace(t *testing.T) {
	for _, name := range []string{"account", "login", "auth"} {
		if !noWorkspaceTopLevels[name] {
			t.Errorf("%q must run before a workspace exists", name)
		}
	}
	// The exemption is an allowlist, not a hole: a workspace-scoped command
	// still has to be given one.
	for _, name := range []string{"agent", "conversations", "channels"} {
		if noWorkspaceTopLevels[name] {
			t.Errorf("%q talks to a workspace-scoped API and must still require one", name)
		}
	}
}

// WHAT A STRANGER MEETS FIRST, asserted so it cannot rot again. Each of these
// was wrong on 2026-09-27, found by walking the paid onboarding as a customer:
// the sign-in demanded a workspace it was about to create, `account subscribe`
// sent people to top up credits that no longer exist, and
// `model check` — the command for deciding which model to pin, typed by someone
// who has set nothing up — demanded a workspace too.
func TestFirstContactCommandsRunBeforeAnythingIsSetUp(t *testing.T) {
	if !noWorkspaceTopLevels["account"] || !noWorkspaceTopLevels["login"] {
		t.Fatal("the sign-in must not require a workspace")
	}
	// The stale next step: a subscription has nothing to do with prepaid
	// credits.
	err := brokerCall("GET", "/v1/me", nil, nil)
	if err == nil {
		t.Skip("a token exists on this machine; the not-signed-in message is not exercised")
	}
	msg := err.Error()
	if !strings.Contains(msg, "memdoor login") {
		t.Errorf("not signed in must name the command that signs you in: %q", msg)
	}
	for _, gone := range []string{"credits topup"} {
		if strings.Contains(msg, gone) {
			t.Errorf("prepaid credits are gone, yet the message still says %q: %s", gone, msg)
		}
	}
}

// `memdoor login` is `memdoor account login` under its short name (Greg,
// 2026-09-29: "shortcut memdoor login"): the same flags, the same sign-in.
// Before, it was the browser flow only and ignored an email given to it.
func TestLoginIsTheShortNameOfAccountLogin(t *testing.T) {
	for _, flag := range []string{"code", "send-only", "json"} {
		if loginCmd.Flags().Lookup(flag) == nil {
			t.Errorf("memdoor login lacks --%s, which memdoor account login has", flag)
		}
		if accountLoginCmd.Flags().Lookup(flag) == nil {
			t.Errorf("memdoor account login lost --%s", flag)
		}
	}
	if loginCmd.Flags().Lookup("browser") != nil {
		t.Error("the browser sign-in was removed 2026-10-05 (it proved nothing about the email): memdoor login --browser must not exist")
	}
	if err := loginCmd.Args(loginCmd, []string{"you@example.com"}); err != nil {
		t.Errorf("memdoor login must take an email: %v", err)
	}
	// No terminal and no email (a test has neither): both names refuse the
	// same way, naming the short form.
	for _, c := range []*cobra.Command{loginCmd, accountLoginCmd} {
		err := c.RunE(c, nil)
		if err == nil || !strings.Contains(err.Error(), "an email is required: memdoor login") {
			t.Errorf("%s with no email: %v", c.CommandPath(), err)
		}
	}
}

// Signing out of memdoor.ai forgets the seat, never the engine's session: a
// free user's agent runs on it and nothing would renew it (2026-10-03).
func TestLogoutForgetsTheSeatNotTheEngine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveCredentials(&credentials{Token: "engine", Email: "dev@example.com", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{billingTokenFile, accountRecordFile} {
		if err := writeHomeFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := logoutCmd.RunE(logoutCmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{billingTokenFile, accountRecordFile} {
		if _, err := os.Stat(homePath(f)); err == nil {
			t.Errorf("%s survived memdoor logout", f)
		}
	}
	if c, err := loadCredentials(); err != nil || c.Email != "dev@example.com" {
		t.Fatalf("the engine's session must stay: %v %+v", err, c)
	}
	st := currentAccountStatus()
	if st.SignedIn || st.Engine != "dev@example.com" {
		t.Fatalf("status after logout: %+v", st)
	}
	if !noWorkspaceTopLevels["logout"] {
		t.Error("memdoor logout must run in any directory")
	}
}

// One sign-in family a person sees; auth is the engine's own, typed by
// scripts, and its broken browser flow is gone.
func TestAuthIsHiddenAndHasNoBrowserLogin(t *testing.T) {
	hidden := false
	for _, n := range notCodingCommands {
		if n == "auth" {
			hidden = true
		}
	}
	if !hidden {
		t.Error("auth must be left out of the top-level listing")
	}
	for _, c := range authCmd.Commands() {
		if c.Name() == "login" {
			t.Error("auth login's callback had no listener; it must stay gone")
		}
	}
	for _, keep := range []string{"token", "login-direct", "whoami"} {
		found := false
		for _, c := range authCmd.Commands() {
			found = found || c.Name() == keep
		}
		if !found {
			t.Errorf("scripts use auth %s; it must still run", keep)
		}
	}
	if engineLine("") == engineLine("a@b.c") || !strings.Contains(engineLine("a@b.c"), "a@b.c") {
		t.Error("status names the engine user, or says there is none")
	}
}
