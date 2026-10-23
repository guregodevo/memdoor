package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"memdoor/gateway"
)

// THE WINDOW HANDS ITS LINK TO THE GATEWAY (2026-10-23): PUT /api/remote/<ch>
// with the spec; a gateway that hosts answers 200 and the window runs no
// relay of its own; one that cannot (404) leaves the relay to the window.
// GET finds the link the gateway already holds, and keeps it in this
// machine's links file so `memdoor remote` prints the same one.
func TestTheWindowHandsTheLinkToTheGateway(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var got gateway.RemoteHostSpec
	hosted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/remote/ch1" || r.Header.Get("Authorization") != "Bearer tok" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&got)
			hosted = true
			_ = json.NewEncoder(w).Encode(map[string]any{"hosted": true, "id": got.ID})
		case http.MethodGet:
			if !hosted {
				_ = json.NewEncoder(w).Encode(map[string]any{"hosted": false})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"hosted": true, "id": got.ID, "key": got.Key, "relay": got.Relay})
		case http.MethodDelete:
			hosted = false
			_ = json.NewEncoder(w).Encode(map[string]any{"hosted": false, "was": true})
		}
	}))
	defer srv.Close()
	s := remoteSession{Gateway: srv.URL, Token: "tok", Workspace: "w", ChannelID: "ch1", ChannelName: "c", Dir: func() string { return "/srv/proj" },
		Relay: srv.URL, RelayToken: "tok"}
	if _, ok := s.hostedLink(); ok {
		t.Fatal("nothing hosted yet")
	}
	l := remoteLink{ChannelID: "ch1", ID: "abc", Key: "k1"}
	if !hostRemoteOnGateway(s, l) {
		t.Fatal("the gateway took the link")
	}
	if got.ID != "abc" || got.Key != "k1" || got.Dir != "/srv/proj" || got.Workspace != "w" || got.Token != "tok" || got.Relay != srv.URL || got.ChannelName != "c" {
		t.Fatalf("the spec the gateway got: %+v", got)
	}
	back, ok := s.hostedLink()
	if !ok || back.ID != "abc" || back.Key != "k1" {
		t.Fatalf("the hosted link is read back: %+v %v", back, ok)
	}
	if kept := loadRemoteLinks()["ch1"]; kept.ID != "abc" {
		t.Fatalf("and kept in this machine's links file: %+v", kept)
	}
	if fi, err := os.Stat(filepath.Join(home, ".memdoor", "remote.json")); err != nil || fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("the links file is the owner's alone: %v %v", err, fi)
	}
	unhostRemoteOnGateway(s)
	if _, ok := s.hostedLink(); ok {
		t.Fatal("revoked on the gateway")
	}

	// An older gateway, or one that is not its own process: 404, the
	// window hosts.
	s.ChannelID = "other"
	if hostRemoteOnGateway(s, l) {
		t.Fatal("a gateway without a host leaves the relay to the window")
	}
}

// In the gateway's own process the phone's /remote turns nothing on, and the
// host never hands a link to itself.
func TestTheHostNeverHandsALinkToItself(t *testing.T) {
	saved := remoteHostedHere
	remoteHostedHere = true
	t.Cleanup(func() { remoteHostedHere = saved })
	s := remoteSession{Gateway: "http://127.0.0.1:1", Token: "tok", ChannelID: "ch1"}
	if hostRemoteOnGateway(s, remoteLink{ID: "a", Key: "k"}) {
		t.Fatal("no PUT from inside the gateway")
	}
	text, err := tuiRemote(s)("")
	if err != nil || text == "" {
		t.Fatalf("/remote on the phone's own terminal answers, turns nothing on: %q %v", text, err)
	}
}
