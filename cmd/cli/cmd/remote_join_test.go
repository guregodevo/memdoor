package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"memdoor/gateway"
	"memdoor/gateway/protocol"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

// screenBuf is a guest's terminal: what the host's screen wrote to it.
type screenBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *screenBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *screenBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A host with /remote on: a relay host, a gateway feed, and a TUI program
// (the probe) behind the link.
func remoteJoinHost(t *testing.T) remoteLink {
	t.Helper()
	hub := gateway.NewRemoteRelayHub()
	t.Cleanup(hub.Shutdown)
	relayMux := http.NewServeMux()
	relayMux.Handle("/api/relay", hub.HandleRelayWS(remoteTestUsers{"acct": "alice"}))
	relayMux.HandleFunc("/api/relay/browser", hub.HandleRelayBrowser)
	relay := httptest.NewServer(relayMux)
	t.Cleanup(relay.Close)
	t.Setenv("MEMDOOR_REMOTE_URL", relay.URL)

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		for {
			var m protocol.Message
			if c.ReadJSON(&m) != nil {
				return
			}
		}
	}))
	t.Cleanup(gw.Close)

	link := remoteLink{ChannelID: "join-1", ID: "BBBBBBBBBBBBBBBB", Key: "join-link-key"}
	startRemote(remoteSession{
		Gateway: gw.URL, Token: "tok", Account: "acct", Workspace: "w", ChannelID: "join-1",
		TTY: func() tea.Model { return ttyProbe{} },
	}, link)
	t.Cleanup(func() { stopRemote("join-1") })
	return link
}

func startGuest(t *testing.T, link string) (typing *io.PipeWriter, screen *screenBuf, done chan error) {
	t.Helper()
	pr, pw := io.Pipe()
	screen = &screenBuf{}
	done = make(chan error, 1)
	go func() { done <- joinRemote(link, pr, screen, func() (int, int) { return 60, 20 }, nil) }()
	t.Cleanup(func() { pw.Close() })
	return pw, screen, done
}

// From a terminal, the full link drives the host's TUI and the watch-only
// link watches it: the viewer is replayed the screen and its keys go
// nowhere. Ctrl+] leaves; /remote off ends the watcher's session as revoked.
func TestJoinFromATerminal(t *testing.T) {
	link := remoteJoinHost(t)
	time.Sleep(300 * time.Millisecond) // the host's terminal attaches to the relay

	driverKeys, driverScreen, driverDone := startGuest(t, link.url())
	waitFor(t, "the host's screen on the driver", func() bool { return strings.Contains(driverScreen.String(), "cols=60") })
	if _, err := driverKeys.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the driver's keys to reach the host", func() bool { return strings.Contains(driverScreen.String(), "typed=[hi]") })

	view, err := link.viewURL()
	if err != nil {
		t.Fatal(err)
	}
	viewerKeys, viewerScreen, viewerDone := startGuest(t, view)
	waitFor(t, "the watcher's replay", func() bool { return strings.Contains(viewerScreen.String(), "typed=[hi]") })
	if _, err := viewerKeys.Write([]byte("zz")); err != nil {
		t.Fatal(err)
	}
	if _, err := driverKeys.Write([]byte("!")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "live typing on both", func() bool {
		return strings.Contains(driverScreen.String(), "typed=[hi!]") && strings.Contains(viewerScreen.String(), "typed=[hi!]")
	})
	if strings.Contains(driverScreen.String(), "zz") {
		t.Fatal("the watcher's keys reached the host")
	}

	if _, err := driverKeys.Write([]byte{remoteJoinDetach}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-driverDone:
		if err != nil {
			t.Fatalf("Ctrl+] must leave cleanly: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl+] did not leave")
	}

	stopRemote("join-1")
	select {
	case err := <-viewerDone:
		if err != errRemoteRevoked {
			t.Fatalf("/remote off must end the watcher as revoked: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("/remote off did not reach the watcher")
	}
}

func TestParseRemoteLink(t *testing.T) {
	l := remoteLink{ID: "CCCCCCCCCCCCCCCC", Key: "k"}
	t.Setenv("MEMDOOR_REMOTE_URL", "https://memdoor.ai")
	full, err := parseRemoteLink(l.url())
	if err != nil || !full.canDrive() || full.id != l.ID || full.base != "https://memdoor.ai" {
		t.Fatalf("full link: %+v %v", full, err)
	}
	v, _ := l.viewURL()
	view, err := parseRemoteLink(v)
	if err != nil || view.canDrive() || view.keys.proof != full.keys.proof {
		t.Fatalf("watch-only link: drive=%v same proof=%v %v", view.canDrive(), view.keys.proof == full.keys.proof, err)
	}
	for _, bad := range []string{"memdoor.ai/r/x", "https://memdoor.ai/r/", "https://memdoor.ai/r/x", "https://memdoor.ai/r/x#v=short"} {
		if _, err := parseRemoteLink(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}
