package client

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// A gateway that closes the socket as policy (1008) is refusing, not dropping:
// the client says why once and does not dial again. Until 2026-10-30 it
// reconnected every few seconds, and the window showed "connection lost /
// reconnected" forever without the reason.
func TestClient_RefusalIsFinal(t *testing.T) {
	var dials atomic.Int32
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dials.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		reason := "not your workspace (yours is mine)\x1b[2J"
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, reason), time.Now().Add(time.Second))
		conn.Close()
	}))
	defer srv.Close()

	opts := DefaultOptions()
	opts.EnableLogging = false
	c := New(srv.URL, "s", opts)
	events := make(chan Event, 8)
	c.OnEvent(func(ev Event) { events <- ev })
	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}
	c.Listen()

	var refused *Event
	deadline := time.After(3 * time.Second)
	for refused == nil {
		select {
		case ev := <-events:
			if ev.Type == EventDisconnected {
				t.Fatal("a refusal was reported as a drop")
			}
			if ev.Type == EventRefused {
				refused = &ev
			}
		case <-deadline:
			t.Fatal("no EventRefused")
		}
	}
	if got := refused.Error.Error(); got != "not your workspace (yours is mine)[2J" {
		t.Fatalf("reason = %q, want the gateway's text without control characters", got)
	}
	time.Sleep(2500 * time.Millisecond) // past the reconnect loop's first 1s backoff
	if n := dials.Load(); n != 1 {
		t.Fatalf("dialed %d times after a refusal, want 1", n)
	}
}
