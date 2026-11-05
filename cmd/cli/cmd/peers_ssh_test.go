package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// fakePeerTunnel swaps the tunnel seam so no test dials a real ssh: the
// tunnel's local end is the box itself, its close says so.
func fakePeerTunnel(t *testing.T) (closings <-chan string) {
	t.Helper()
	saved := peerTunnelOpen
	closing := make(chan string, 16)
	peerTunnelOpen = func(ctx context.Context, gw string) (*sshDoor, error) {
		u, err := url.Parse(gw)
		port := u.Query().Get("port")
		if err != nil || port == "" || u.Hostname() == "" {
			return nil, fmt.Errorf("%q is not an ssh address", gw)
		}
		box := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/status":
				_ = json.NewEncoder(w).Encode(map[string]any{"version": "test", "box": u.Hostname()})
			default:
				http.NotFound(w, r)
			}
		})}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		go func() { _ = box.Serve(ln) }()
		t.Cleanup(func() { _ = box.Close() })
		closer := closedPipeCloser{onClose: func() { closing <- gw; _ = box.Close() }}
		return &sshDoor{name: gw, local: "http://" + ln.Addr().String(), stdin: closer}, nil
	}
	t.Cleanup(func() { peerTunnelOpen = saved })
	return closing
}

// An ssh peer is named by the ssh address as typed, never normalized into
// http://box:18789: the address is the credentials' key and the name in
// the resume line; the tunnel's local end is what is dialed.
func TestAnSSHPeerIsKeptUnderTheAddressAsTyped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	closings := fakePeerTunnel(t)
	savedRenew := renewSessionForPeer
	t.Cleanup(func() { renewSessionForPeer = savedRenew })

	gw := "ssh://me@box?port=18789"
	// No session yet: the peer signs in over its own ssh.
	sshRenew := false
	renewSessionForPeer = func(ctx context.Context, d *sshDoor) error {
		sshRenew = true
		return saveCredentialsFor(gw, &credentials{Token: "tok", Email: "me@example.test", ExpiresAt: time.Now().Add(time.Hour)})
	}
	c, closeTunnel, err := sshPeerClient(context.Background(), gw)
	if err != nil {
		t.Fatalf("ssh peer: %v", err)
	}
	if !sshRenew {
		t.Error("a peer with no session signs in over its own ssh")
	}
	if !strings.HasPrefix(c.BaseURL, "http://127.0.0.1:") {
		t.Fatalf("the client dials the tunnel, got %q", c.BaseURL)
	}
	// The session is saved under the ssh address, for the next call.
	st, err := loadCredentialStore()
	if err != nil || st.Gateways[normalizeGateway(gw)].Token != "tok" {
		t.Fatalf("the session is under the ssh address: %v %v", st.Gateways, err)
	}
	// With a session there, a second client needs no sign-in.
	sshRenew = false
	c2, close2, err := sshPeerClient(context.Background(), gw)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	close2()
	if sshRenew {
		t.Error("an existing session is not renewed")
	}
	_ = c2
	closeTunnel()
	close2()
	// Both tunnels closed with their clients.
	for i := 0; i < 2; i++ {
		select {
		case <-closings:
		case <-time.After(time.Second):
			t.Fatalf("tunnel %d outlived its client", i)
		}
	}
}

// Two ssh peers are two tunnels: each answers for its own box only, and
// closing one leaves the other open.
func TestTwoSSHPeersAreTwoTunnels(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = fakePeerTunnel(t)
	savedRenew := renewSessionForPeer
	t.Cleanup(func() { renewSessionForPeer = savedRenew })
	renewSessionForPeer = func(ctx context.Context, d *sshDoor) error { return nil }

	clients := make([]*Client, 2)
	closers := make([]func(), 2)
	for i := 0; i < 2; i++ {
		gw := fmt.Sprintf("ssh://me@box%d?port=18789", i)
		_ = saveCredentialsFor(gw, &credentials{Token: "tok"})
		c, closeTunnel, err := sshPeerClient(context.Background(), gw)
		if err != nil {
			t.Fatalf("peer %d: %v", i, err)
		}
		clients[i], closers[i] = c, closeTunnel
	}
	if clients[0].BaseURL == clients[1].BaseURL {
		t.Fatal("the two peers got one tunnel")
	}
	var v0, v1 struct {
		Box string `json:"box"`
	}
	if err := clients[0].GetJSON("/api/status", &v0); err != nil || v0.Box != "box0" {
		t.Fatalf("peer 0 through its own tunnel: %v %v", v0, err)
	}
	if err := clients[1].GetJSON("/api/status", &v1); err != nil || v1.Box != "box1" {
		t.Fatalf("peer 1 through its own tunnel: %v %v", v1, err)
	}
	closers[0]()
	if _, err := clients[0].Get("/api/status"); err == nil {
		t.Fatal("the tunnel outlived its closer")
	}
	if err := clients[1].GetJSON("/api/status", &v1); err != nil || v1.Box != "box1" {
		t.Fatalf("closing one tunnel closed the other: %v %v", v1, err)
	}
	closers[1]()
}

// The process's ssh door is not touched by a peer's tunnel: the door's
// state and the globals that name the gateway stay what they were.
func TestAPeerTunnelLeavesTheDoorAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	closings := fakePeerTunnel(t)
	savedRenew := renewSessionForPeer
	t.Cleanup(func() { renewSessionForPeer = savedRenew })
	renewSessionForPeer = func(ctx context.Context, d *sshDoor) error {
		return saveCredentialsFor(d.name, &credentials{Token: "tok"})
	}
	savedAddr, savedDoor := gatewayAddr, sshDoorUp
	t.Cleanup(func() { gatewayAddr, sshDoorUp = savedAddr, savedDoor })
	c, closeTunnel, err := sshPeerClient(context.Background(), "ssh://me@box?port=18789")
	if err != nil {
		t.Fatal(err)
	}
	closeTunnel()
	if sshDoorUp != nil {
		t.Error("the peer's tunnel became the door")
	}
	if gatewayAddr != savedAddr {
		t.Errorf("the peer's tunnel moved the gateway address: %q -> %q", savedAddr, gatewayAddr)
	}
	if strings.HasPrefix(c.BaseURL, "ssh://") {
		t.Errorf("the client dials an ssh address: %q", c.BaseURL)
	}
	select {
	case <-closings:
	case <-time.After(time.Second):
		t.Fatal("the tunnel outlived its client")
	}
}

// A peer tunnel ends when the ask does, not when the process ends: peerAsk
// with an ssh peer whose turn cannot open closes the tunnel on the way out.
func TestAPeerAskClosesItsTunnel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := peerTunnelOpen
	t.Cleanup(func() { peerTunnelOpen = saved })
	closed := make(chan struct{})
	peerTunnelOpen = func(_ context.Context, _ string) (*sshDoor, error) {
		return &sshDoor{name: "ssh://me@box", local: "http://127.0.0.1:1",
			stdin: closedPipeCloser{onClose: func() { close(closed) }}}, nil
	}
	if _, err := peerAsk(context.Background(), "ssh://me@box", "coder", "", "hi", nil); err == nil {
		t.Fatal("a dead tunnel answers no turn")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("the tunnel outlived the ask")
	}
}

// closedPipeCloser is an ssh stdin pipe that says when it is closed.
type closedPipeCloser struct{ onClose func() }

func (c closedPipeCloser) Write(p []byte) (int, error) { return len(p), nil }
func (c closedPipeCloser) Close() error                { c.onClose(); return nil }
