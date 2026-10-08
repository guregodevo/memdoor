package gateway

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type relayUsers map[string]string // token -> user id

func (u relayUsers) ValidateToken(token string) (string, error) {
	if id, ok := u[token]; ok {
		return id, nil
	}
	return "", errors.New("bad token")
}

const (
	relayTestID    = "AAAAAAAAAAAAAAAA" // 12 bytes base64url
	relayTestProof = "the-browser-proof"
)

func relayTestVerifier() string {
	sum := sha256.Sum256([]byte(relayTestProof))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func newRelayTestServer(t *testing.T) (*remoteRelayHub, *httptest.Server) {
	t.Helper()
	hub := NewRemoteRelayHub()
	mux := http.NewServeMux()
	mux.Handle("/api/relay", hub.HandleRelayWS(relayUsers{"tok-a": "alice", "tok-b": "bob"}))
	mux.HandleFunc("/api/relay/browser", hub.HandleRelayBrowser)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close(); hub.Shutdown() })
	return hub, srv
}

// dialRelay dials as the clients do: the account token in the Authorization
// header (the TUI's way, never the URL), everything else in the query.
func dialRelay(t *testing.T, srv *httptest.Server, path string, q url.Values) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	var h http.Header
	if tok := q.Get("token"); tok != "" {
		h = http.Header{"Authorization": {"Bearer " + tok}}
		q = cloneWithout(q, "token")
	}
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + path + "?" + q.Encode()
	return websocket.DefaultDialer.Dial(u, h)
}

func cloneWithout(q url.Values, key string) url.Values {
	out := url.Values{}
	for k, v := range q {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func dialTerminal(t *testing.T, srv *httptest.Server, token string) (*websocket.Conn, int) {
	t.Helper()
	c, resp, err := dialRelay(t, srv, "/api/relay", url.Values{"token": {token}, "id": {relayTestID}, "verifier": {relayTestVerifier()}})
	if err != nil {
		return nil, resp.StatusCode
	}
	t.Cleanup(func() { c.Close() })
	return c, http.StatusSwitchingProtocols
}

func dialBrowser(t *testing.T, srv *httptest.Server, proof string) (*websocket.Conn, int) {
	t.Helper()
	c, resp, err := dialRelay(t, srv, "/api/relay/browser", url.Values{"id": {relayTestID}, "auth": {proof}})
	if err != nil {
		return nil, resp.StatusCode
	}
	t.Cleanup(func() { c.Close() })
	return c, http.StatusSwitchingProtocols
}

func readFrame(t *testing.T, c *websocket.Conn) string {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, b, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

// A second terminal of the same user takes over; the NEW one stays attached
// and receives the browser's turns, the old one is closed.
func TestRelayTerminalTakeoverKeepsTheNewTerminal(t *testing.T) {
	_, srv := newRelayTestServer(t)
	old, _ := dialTerminal(t, srv, "tok-a")
	fresh, _ := dialTerminal(t, srv, "tok-a")
	browser, code := dialBrowser(t, srv, relayTestProof)
	if browser == nil {
		t.Fatalf("browser refused: %d", code)
	}
	_ = old.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := old.ReadMessage(); err == nil {
		t.Fatal("the replaced terminal must be closed")
	}
	time.Sleep(50 * time.Millisecond) // the old pump's cleanup runs
	turn := `{"kind":"turn","payload":"x"}`
	if err := browser.WriteMessage(websocket.TextMessage, []byte(turn)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, fresh); got != turn {
		t.Fatalf("new terminal got %q", got)
	}
}

func TestRelayRefusesAnotherUsersTerminal(t *testing.T) {
	_, srv := newRelayTestServer(t)
	dialTerminal(t, srv, "tok-a")
	if c, code := dialTerminal(t, srv, "tok-b"); c != nil || code != http.StatusForbidden {
		t.Fatalf("bob attached to alice's session: %d", code)
	}
	if c, code := dialTerminal(t, srv, "nope"); c != nil || code != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", code)
	}
}

func TestRelayBrowserNeedsAnExistingSessionAndTheProof(t *testing.T) {
	_, srv := newRelayTestServer(t)
	if c, code := dialBrowser(t, srv, relayTestProof); c != nil || code != http.StatusNotFound {
		t.Fatalf("browser created a session: %d", code)
	}
	dialTerminal(t, srv, "tok-a")
	if c, code := dialBrowser(t, srv, "wrong"); c != nil || code != http.StatusNotFound {
		t.Fatalf("wrong proof admitted: %d", code)
	}
	if c, _ := dialBrowser(t, srv, relayTestProof); c == nil {
		t.Fatal("right proof refused")
	}
}

// Several pages share a session — a phone, a laptop, view-only links — up to
// remoteRelayMaxBrowsers. Every page gets what the terminal sends, and the
// terminal gets what any page sends.
func TestRelaySeveralPagesShareASession(t *testing.T) {
	_, srv := newRelayTestServer(t)
	terminal, _ := dialTerminal(t, srv, "tok-a")
	pages := make([]*websocket.Conn, 0, remoteRelayMaxBrowsers)
	for i := 0; i < remoteRelayMaxBrowsers; i++ {
		p, code := dialBrowser(t, srv, relayTestProof)
		if p == nil {
			t.Fatalf("page %d refused: %d", i+1, code)
		}
		pages = append(pages, p)
	}
	extra, _ := dialBrowser(t, srv, relayTestProof)
	if extra == nil {
		t.Fatal("one page too many: expected an upgrade then a close")
	}
	_ = extra.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := extra.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseTryAgainLater) {
		t.Fatalf("page %d must be refused: %v", remoteRelayMaxBrowsers+1, err)
	}

	screen := `{"kind":"event","payload":"screen"}`
	if err := terminal.WriteMessage(websocket.TextMessage, []byte(screen)); err != nil {
		t.Fatal(err)
	}
	for i, p := range pages {
		if got := readFrame(t, p); got != screen {
			t.Fatalf("page %d got %q", i+1, got)
		}
	}
	turn := `{"kind":"turn","payload":"from the last page"}`
	if err := pages[len(pages)-1].WriteMessage(websocket.TextMessage, []byte(turn)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, terminal); got != turn {
		t.Fatalf("terminal got %q", got)
	}

	// A page that leaves frees its place.
	pages[0].Close()
	time.Sleep(100 * time.Millisecond)
	if p, code := dialBrowser(t, srv, relayTestProof); p == nil {
		t.Fatalf("a freed place must admit a page: %d", code)
	}
}

// Delivering while the peer drops must never send on a closed channel
// (that panic would take the gateway down). Run with -race.
func TestRelayDeliverRacesDropWithoutPanic(t *testing.T) {
	for i := 0; i < 200; i++ {
		c := &remoteRelayConn{send: make(chan []byte, 1)}
		s := &remoteRelaySession{browsers: map[*remoteRelayConn]bool{c: true}}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); s.deliver(false, []byte("x")) }()
		go func() { defer wg.Done(); s.dropConn(false, c) }()
		wg.Wait()
	}
}

func TestRelaySweepKeepsConnectedSessions(t *testing.T) {
	hub := &remoteRelayHub{sessions: map[string]*remoteRelaySession{}}
	long := time.Now().Add(-2 * remoteRelaySessionTTL)
	hub.sessions["live"] = &remoteRelaySession{lastSeen: long, terminal: &remoteRelayConn{send: make(chan []byte)}}
	hub.sessions["idle"] = &remoteRelaySession{lastSeen: long}
	hub.sweep(time.Now())
	if hub.sessions["live"] == nil || hub.sessions["idle"] != nil {
		t.Fatalf("sweep: %v", hub.sessions)
	}
}

// One frame has a size limit: a large honest frame (a conversation's
// history) crosses, one past the cap closes the socket that sent it instead
// of filling the relay's memory, and the other end never sees it.
func TestRelayCapsFrameSize(t *testing.T) {
	_, srv := newRelayTestServer(t)
	terminal, _ := dialTerminal(t, srv, "tok-a")
	browser, code := dialBrowser(t, srv, relayTestProof)
	if terminal == nil || browser == nil {
		t.Fatalf("setup: browser %d", code)
	}
	frame := func(n int) []byte {
		return []byte(`{"kind":"event","payload":"` + strings.Repeat("A", n) + `"}`)
	}

	big := frame(1 << 20)
	if err := terminal.WriteMessage(websocket.TextMessage, big); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, browser); len(got) != len(big) {
		t.Fatalf("a 1 MiB frame must cross whole: got %d bytes", len(got))
	}

	if err := terminal.WriteMessage(websocket.TextMessage, frame(remoteRelayMaxFrame+1)); err != nil {
		t.Fatal(err)
	}
	_ = terminal.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := terminal.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
		t.Fatalf("the sender of an oversized frame must be closed with 1009, got %v", err)
	}
	_ = browser.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, b, err := browser.ReadMessage(); err == nil {
		t.Fatalf("the oversized frame was forwarded: %d bytes", len(b))
	}
}

// A revoked link ends at the relay: the terminal's end frame removes the
// session and closes every page with the code the page shows as "turned off",
// and the link no longer joins — instead of joining an empty session for the
// 12 h a session waits for a sleeping laptop (live 2026-09-29).
func TestRelayEndClosesTheSession(t *testing.T) {
	_, srv := newRelayTestServer(t)
	terminal, _ := dialTerminal(t, srv, "tok-a")
	page, _ := dialBrowser(t, srv, relayTestProof)
	if page == nil {
		t.Fatal("setup: page refused")
	}
	if err := terminal.WriteMessage(websocket.TextMessage, []byte(`{"kind":"end"}`)); err != nil {
		t.Fatal(err)
	}
	_ = page.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := page.ReadMessage(); !websocket.IsCloseError(err, remoteRelayRevokedCode) {
		t.Fatalf("the page must be closed as revoked: %v", err)
	}
	if c, code := dialBrowser(t, srv, relayTestProof); c != nil || code != http.StatusNotFound {
		t.Fatalf("a revoked link must not join: %d", code)
	}
}

// Only the terminal can end a session: a page sending the end frame is
// ignored.
func TestRelayPageCannotEndTheSession(t *testing.T) {
	_, srv := newRelayTestServer(t)
	dialTerminal(t, srv, "tok-a")
	page, _ := dialBrowser(t, srv, relayTestProof)
	if err := page.WriteMessage(websocket.TextMessage, []byte(`{"kind":"end"}`)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if c, _ := dialBrowser(t, srv, relayTestProof); c == nil {
		t.Fatal("a page ended the session")
	}
}

// A page whose handshake finished after the terminal ended the session is
// refused as revoked, not attached to a session nothing will ever speak on
// (the public CI page waited 2 s for a close that had already been sent).
func TestAPageJoiningAnEndedSessionIsRevoked(t *testing.T) {
	h := NewRemoteRelayHub()
	defer h.Shutdown()
	s := &remoteRelaySession{key: "k", browsers: map[*remoteRelayConn]bool{}}
	h.mu.Lock()
	h.sessions["k"] = s
	h.mu.Unlock()
	h.end(s)
	if ok, ended := s.attachBrowser(&remoteRelayConn{send: make(chan []byte, 1)}); ok || !ended {
		t.Fatalf("a page joining after the end must be told it was revoked: ok=%v ended=%v", ok, ended)
	}
}
