package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"memdoor/gateway/broadcast"
	"memdoor/gateway/ratelimit"
)

type fakeValidator struct{ users map[string]string }

func (f fakeValidator) ValidateToken(token string) (string, error) {
	if u, ok := f.users[token]; ok {
		return u, nil
	}
	return "", errors.New("bad token")
}

func wsRequest(remote string, headers map[string]string, query string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/ws"+query, nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

// Who a /ws connection acts as. Until 2026-08-27 the answer was "whoever
// asked": memdoor.ai's nginx proxies /ws and the handler checked nothing.
func TestWSActor(t *testing.T) {
	v := fakeValidator{users: map[string]string{"good": "user-42"}}
	proxied := map[string]string{"X-Real-IP": "203.0.113.9", "X-Forwarded-For": "203.0.113.9"}

	cases := []struct {
		name string
		r    *http.Request
		want string
		err  bool
	}{
		{"local TUI, no token", wsRequest("127.0.0.1:5000", nil, ""), wsActorLocal, false},
		{"local IPv6, no token", wsRequest("[::1]:5000", nil, ""), wsActorLocal, false},
		{"through nginx, no token", wsRequest("127.0.0.1:5000", proxied, ""), "", false},
		{"remote, no token", wsRequest("203.0.113.9:5000", nil, ""), "", false},
		{"through nginx, header token", wsRequest("127.0.0.1:5000", map[string]string{"X-Real-IP": "203.0.113.9", "Authorization": "Bearer good"}, ""), "user-42", false},
		{"through nginx, query token", wsRequest("127.0.0.1:5000", proxied, "?token=good"), "user-42", false},
		{"through nginx, bad token", wsRequest("127.0.0.1:5000", proxied, "?token=forged"), "", true},
	}
	for _, c := range cases {
		got, err := wsActor(c.r, v)
		if (err != nil) != c.err {
			t.Errorf("%s: err = %v, want error=%v", c.name, err, c.err)
		}
		if got != c.want {
			t.Errorf("%s: actor = %q, want %q", c.name, got, c.want)
		}
	}
}

// The gate must hold at the MESSAGE, not just the handshake: an anonymous
// listener that sends "chat" must be refused before anything touches the
// queue. The server here has no queue at all — reaching it would panic.
func TestAnonymousClientCannotRunTheCoder(t *testing.T) {
	s := &Server{
		clients:     make(map[string]*Client),
		clientsMu:   sync.RWMutex{},
		rateLimiter: ratelimit.NewPerClientLimiter(100, time.Minute),
	}
	for _, typ := range []string{"chat", "agent", "agent.wait", "cancel", "answer"} {
		c := &Client{ID: "anon-" + typ, Actor: "", send: make(chan []byte, 4)}
		s.handleMessage(c, &Message{Type: typ, Data: map[string]interface{}{"text": "rm -rf /", "workdir": "/"}})
		select {
		case raw := <-c.send:
			var reply Message
			if err := json.Unmarshal(raw, &reply); err != nil {
				t.Fatal(err)
			}
			if reply.Type != "error" || !strings.Contains(reply.Error, "authentication required") {
				t.Fatalf("%s: anonymous client got %+v, want an authentication error", typ, reply)
			}
		default:
			t.Fatalf("%s: anonymous client was not answered", typ)
		}
	}
}

// ping keeps the connection alive for a page's event stream, which
// is the one thing an anonymous listener is for.
func TestAnonymousClientMayPing(t *testing.T) {
	s := &Server{clients: make(map[string]*Client), rateLimiter: ratelimit.NewPerClientLimiter(100, time.Minute)}
	c := &Client{ID: "anon", send: make(chan []byte, 4)}
	s.handleMessage(c, &Message{Type: "ping"})
	raw := <-c.send
	if !strings.Contains(string(raw), "pong") {
		t.Fatalf("ping from an anonymous client got %s", raw)
	}
}

// Through the real handshake. A connection that arrives the way memdoor.ai's
// nginx delivers it — loopback, with forwarding headers — is anonymous, and
// its "chat" is refused before it reaches anything. The same dial without
// the headers is the local TUI and is trusted.
func TestProxiedConnectionIsAnonymousAndLocalIsNot(t *testing.T) {
	s := &Server{
		clients:     make(map[string]*Client),
		sessions:    NewSessionManager(),
		broadcaster: broadcast.NewSubscriptionManager(false),
		rateLimiter: ratelimit.NewPerClientLimiter(100, time.Minute),
		upgrader:    websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
	srv := httptest.NewServer(http.HandlerFunc(s.handleWebSocket))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?session=session_main"

	readType := func(conn *websocket.Conn) Message {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var m Message
		if err := conn.ReadJSON(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}

	// As nginx delivers it.
	proxied, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"X-Real-IP": {"203.0.113.9"}, "X-Forwarded-For": {"203.0.113.9"}})
	if err != nil {
		t.Fatal(err)
	}
	defer proxied.Close()
	if w := readType(proxied); w.SessionID != wsAnonymousSession {
		t.Fatalf("a proxied caller was attached to %q — it asked for session_main and must get neither", w.SessionID)
	}
	if err := proxied.WriteJSON(Message{Type: "chat", Data: map[string]interface{}{"text": "hi", "workdir": "/etc"}}); err != nil {
		t.Fatal(err)
	}
	if reply := readType(proxied); reply.Type != "error" || !strings.Contains(reply.Error, "authentication required") {
		t.Fatalf("proxied chat got %+v, want an authentication error", reply)
	}

	// As the local TUI dials it.
	local, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	if w := readType(local); w.SessionID != "session_main" {
		t.Fatalf("the local TUI was attached to %q, want the session it asked for", w.SessionID)
	}
	s.clientsMu.RLock()
	actors := map[string]int{}
	for _, c := range s.clients {
		if c != nil {
			actors[c.Actor]++
		}
	}
	s.clientsMu.RUnlock()
	if actors[wsActorLocal] != 1 || actors[""] != 1 {
		t.Fatalf("actors = %v, want one local and one anonymous", actors)
	}
}
