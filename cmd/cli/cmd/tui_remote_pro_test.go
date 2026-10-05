package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// /remote on the hosted relay tells a free account it is Pro, in the window,
// and hands out no link; a Pro account gets its link (review, 2026-10-04:
// the relay's refusal never reached the window).
func TestRemoteSaysProBeforeHandingOutALink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MEMDOOR_REMOTE_URL", "")
	plan := "free"
	billing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"email": "a@b.c", "workspace": "ws", "plan": plan})
	}))
	defer billing.Close()
	t.Setenv("MEMDOOR_BILLING_URL", billing.URL)
	t.Setenv("MEMDOOR_BILLING_TOKEN", "acct")
	remote := tuiRemote(remoteSession{ChannelID: "c1", Account: "acct"})

	if _, err := remote(""); !errors.Is(err, errRemoteIsPro) {
		t.Fatalf("a free account is told remote is Pro: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".memdoor", "remote.json")); err == nil {
		t.Fatal("no link is made for a free account")
	}
	plan = "pro"
	out, err := remote("")
	defer stopRemote("c1")
	if err != nil || !strings.Contains(out, "/r/") {
		t.Fatalf("a Pro account gets its link: %q %v", out, err)
	}
}

// A relay that refuses for the plan is not dialled again every five seconds.
func TestTheRelayStopsOnAPlanRefusal(t *testing.T) {
	var dials int
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dials++
		http.Error(w, "remote control is a Pro feature", http.StatusPaymentRequired)
	}))
	defer relay.Close()
	t.Setenv("MEMDOOR_REMOTE_URL", relay.URL)
	gw := httptest.NewServer(http.NotFoundHandler()) // the conversation feed; never dialled once the relay refuses
	defer gw.Close()
	l := remoteLink{ID: "id1", Key: strings.Repeat("k", 43)}
	r := newRemoteRelay(remoteSession{Gateway: gw.URL, Account: "acct"}, l)
	err := r.attach(nil)
	if !errors.Is(err, errRemoteIsPro) {
		t.Fatalf("a 402 is the plan refusal: %v", err)
	}
}
