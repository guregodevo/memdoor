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

// A gateway on another machine is its own relay: the link is on that
// gateway, the terminal dials it with the gateway's token, and no account or
// plan is asked (Greg, 2026-10-23: "a phone can connect to daemon", "no pro
// story"). A gateway on this machine still goes through the hosted relay.
func TestRemoteOnAnotherMachineIsOnThatGateway(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MEMDOOR_REMOTE_URL", "")
	t.Setenv("MEMDOOR_BILLING_TOKEN", "")
	box := remoteSession{Gateway: "http://box.lan:18799/", Token: "gw-token", ChannelID: "c-box"}
	if !box.direct() || box.relayBase() != "http://box.lan:18799" || box.relayToken() != "gw-token" {
		t.Fatalf("a gateway on a box is the relay: direct=%v base=%q token=%q", box.direct(), box.relayBase(), box.relayToken())
	}
	out, err := tuiRemote(box)("")
	defer stopRemote("c-box")
	if err != nil || !strings.Contains(out, "http://box.lan:18799/r/") || !strings.Contains(out, "on your gateway at box.lan:18799") {
		t.Fatalf("the link is on the box, with no account: %q %v", out, err)
	}
	if strings.Contains(out, "relay only carries") {
		t.Fatalf("the sentence names the gateway, not a relay: %q", out)
	}
	laptop := remoteSession{Gateway: "http://127.0.0.1:18789", Token: "gw-token", Account: "acct", ChannelID: "c-lap"}
	if laptop.direct() || laptop.relayBase() != "https://memdoor.ai" || laptop.relayToken() != "acct" {
		t.Fatalf("a gateway on this machine goes through the hosted relay: base=%q token=%q", laptop.relayBase(), laptop.relayToken())
	}
	if ws, _ := remoteWSURL(box.relayBase()); ws != "ws://box.lan:18799/api/relay" {
		t.Fatalf("the terminal dials the box's relay: %q", ws)
	}
}

// The link lives on the gateway whenever a phone could reach it: a gateway
// reached through loopback that listens on the network puts the link on
// the address it names (the box's name through the ssh door); one on
// localhost alone goes through memdoor.ai and says why (Greg, 2026-10-23:
// "/remote should not go through memdoor.ai anymore unless it's needed").
func TestRemoteLinkIsOnTheGatewayWhenAPhoneCouldReachIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MEMDOOR_REMOTE_URL", "")
	listen := map[string]any{"host": "0.0.0.0", "port": 18799, "addresses": []string{"192.168.9.9", "100.100.1.2"}}
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "listen": listen})
	}))
	defer gw.Close()

	s := remoteSession{Gateway: gw.URL, Token: "gw-token", Account: "acct", ChannelID: "c-lan"}.resolved()
	if s.Relay != "http://192.168.9.9:18799" || s.RelayToken != "gw-token" || s.hosted() {
		t.Fatalf("a gateway on the network is the relay at its first address: %q %q", s.Relay, s.RelayToken)
	}
	out, err := tuiRemote(remoteSession{Gateway: gw.URL, Token: "gw-token", ChannelID: "c-lan"})("")
	defer stopRemote("c-lan")
	if err != nil || !strings.Contains(out, "http://192.168.9.9:18799/r/") || strings.Contains(out, "Through memdoor.ai") {
		t.Fatalf("the window prints the link on the gateway, with no account: %q %v", out, err)
	}

	sshDoorUp = &sshDoor{name: "ssh://me@box.lan:2222"}
	t.Cleanup(func() { sshDoorUp = nil })
	if s := (remoteSession{Gateway: gw.URL, Token: "gw-token"}).resolved(); s.Relay != "http://box.lan:18799" {
		t.Fatalf("through the ssh door the link is on the box's name: %q", s.Relay)
	}
	sshDoorUp = nil

	listen = map[string]any{"host": "localhost", "port": 18789}
	s = remoteSession{Gateway: gw.URL, Token: "gw-token", Account: "acct"}.resolved()
	if !s.hosted() || s.RelayToken != "acct" || !strings.Contains(s.RelayWhy, "localhost alone") {
		t.Fatalf("a gateway on localhost alone goes through memdoor.ai, and says why: %q %q", s.Relay, s.RelayWhy)
	}
	sshDoorUp = &sshDoor{name: "ssh://me@box.lan"}
	if s := (remoteSession{Gateway: gw.URL, Token: "gw-token", Account: "acct"}).resolved(); !strings.Contains(s.RelayWhy, "gateway on box.lan listens on localhost alone") {
		t.Fatalf("through the ssh door the reason names the box: %q", s.RelayWhy)
	}
	sshDoorUp = nil
}
