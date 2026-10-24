package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A peer is a named gateway in the person's file; a mention reaches only
// one of those, through the caller the binary provides.
func TestAPeerIsANamedGatewayAndOnlyThat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := SavePeers(map[string]PeerEntry{"box": {Gateway: "http://box:18789", Dir: "/srv/app"}}); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(home, ".memdoor", "peers.json")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("the peers file is the person's alone: %v %v", err, fi)
	}
	ps := newPeerSet(nil)
	if !ps.IsPeer("box") || !ps.IsPeer("Box") || ps.IsPeer("vps") {
		t.Fatal("box is a peer, vps is not")
	}
	SetPeerCaller(nil)
	if _, err := ps.Ask(context.Background(), "box", "coder", "hi", "", ""); err == nil || !strings.Contains(err.Error(), "cannot ask") {
		t.Fatalf("no caller, no call: %v", err)
	}
	var got []string
	SetPeerCaller(func(_ context.Context, gateway, agent, dir, text string, onText func(string)) (string, error) {
		got = []string{gateway, agent, dir, text}
		onText("po")
		onText("ng")
		return "pong", nil
	})
	t.Cleanup(func() { SetPeerCaller(nil) })
	reply, err := ps.Ask(context.Background(), "box", "coder", "ping", "", "")
	if err != nil || reply != "pong" || strings.Join(got, "|") != "http://box:18789|coder|/srv/app|ping" {
		t.Fatalf("the caller got the peer's gateway, the agent, the dir and the text: %v %v %v", reply, err, got)
	}
	if _, err := ps.Ask(context.Background(), "vps", "coder", "ping", "", ""); err == nil {
		t.Fatal("not a peer, not asked")
	}
}
