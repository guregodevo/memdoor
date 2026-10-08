package gateway

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/infra"

	"github.com/gorilla/websocket"
)

// REMOTE CONTROL, STEP 2 (Greg, 2026-09-28): the encrypted relay behind
// https://memdoor.ai/r/<id>#k=<key>. The TUI connects to /api/relay as an
// authenticated terminal; the page at /r/<id> (the SPA) connects to
// /api/relay/browser with a proof derived from the key. The
// gateway only forwards opaque frames between the two — every payload is
// AES-GCM ciphertext the gateway has no key for, so memdoor.ai carries
// the session without being able to read it.
//
// The pairing key is <id>; the key after "#" in the link never reaches this
// server, only sha256 of a proof derived from it. One terminal and up to
// remoteRelayMaxBrowsers pages per id: a second terminal of the same user
// takes the session over; every page gets what the terminal sends, and the
// terminal gets what any page sends. Which page may DRIVE is not decided
// here: a view-only link has no write key, and the terminal drops a turn
// that is not sealed with it (cmd/cli/cmd/tui_remote.go).

const (
	// remoteRelayKeyBytes is the length of a pairing id before base64
	// (12 bytes -> 16 url-safe chars, the same shape /remote generates).
	remoteRelayKeyBytes = 12
	// remoteRelaySessionTTL is how long a session survives without ANY
	// connection. A terminal's laptop sleeps; the link must still work when
	// it wakes. Swept by remoteRelaySweeper.
	remoteRelaySessionTTL = 12 * time.Hour
	// remoteRelaySweepEvery is the sweep interval. This is a janitor, not a
	// timer: a session is removed the moment both ends go, the sweep only
	// collects what never said goodbye.
	remoteRelaySweepEvery = time.Hour
	// remoteRelayWriteWait bounds one control write (a close, not data — data
	// frames carry no deadline: the TUI writes while the browser is away).
	remoteRelayWriteWait = 10 * time.Second
	// remoteRelayMaxFrame is the largest frame either end may send. The
	// largest honest ones are a conversation's history (20 turns of 8,000
	// characters) and a full repaint of a big terminal, both under 1 MiB
	// once sealed; past this the socket is closed (1009), not the relay's
	// memory filled.
	remoteRelayMaxFrame = 4 << 20
	// remoteRelayEndKind is the one frame the relay reads rather than
	// forwards: the terminal saying the link was revoked (/remote off). The
	// session goes, and every page is closed with remoteRelayRevokedCode —
	// otherwise a revoked link would still join for the 12 h a session waits
	// for a sleeping laptop, and show an empty, "open" screen.
	remoteRelayEndKind     = "end"
	remoteRelayRevokedCode = 4001
	// remoteRelayMaxBrowsers bounds the pages one session holds: the full
	// link on a phone and a laptop, and view-only links (/remote view)
	// handed to whoever should watch.
	remoteRelayMaxBrowsers = 8
	// remoteRelayMaxSessionsPerUser bounds how many sessions one account
	// can hold open on the relay (one per conversation with /remote on).
	remoteRelayMaxSessionsPerUser = 32
)

var (
	errRelayNotOwner        = errors.New("session belongs to another user")
	errRelayTooManySessions = errors.New("too many remote sessions for this account")
)

// relayWire is one frame as it crosses the relay. Payload is opaque to
// this server; it is never decrypted here.
type relayWire struct {
	Kind    string          `json:"kind"`    // "event" (terminal -> browser) | "turn" (browser -> terminal)
	Payload json.RawMessage `json:"payload"` // the AEAD ciphertext, base64url
}

// remoteRelayHub holds the sessions by their pairing key.
type remoteRelayHub struct {
	mu       sync.RWMutex
	sessions map[string]*remoteRelaySession
	stop     chan struct{}
	stopped  sync.Once
}

// remoteRelayConn is one connected end of a session.
type remoteRelayConn struct {
	mu   sync.Mutex
	conn *websocket.Conn
	send chan []byte
}

// remoteRelaySession pairs one authenticated terminal with one browser.
// owner is the user whose terminal created it: only that user's terminal
// may attach again. verifier is sha256 of the browser proof the terminal
// derives from the link key (remoteBrowserProof): a browser proves it holds
// the key without sending it. lastSeen is the last time ANY end was
// connected or a frame crossed; the sweeper retires only idle, empty sessions.
// Every field is guarded by mu; closing a conn's send channel happens under
// the write lock, delivering to it under the read lock, so a send never
// meets a closed channel.
type remoteRelaySession struct {
	mu       sync.RWMutex
	key      string
	owner    string
	verifier [sha256.Size]byte
	terminal *remoteRelayConn
	browsers map[*remoteRelayConn]bool
	lastSeen time.Time
	// ended is set when the terminal's end frame removed the session. A page
	// whose handshake finished in between attaches to nothing that will ever
	// speak: it is closed as revoked instead (public CI, 2026-10-08: the
	// page waited out its deadline on a session already gone).
	ended bool
}

// NewRemoteRelayHub creates the relay and starts its session sweeper; call
// Shutdown to stop it.
func NewRemoteRelayHub() *remoteRelayHub {
	h := &remoteRelayHub{
		sessions: make(map[string]*remoteRelaySession),
		stop:     make(chan struct{}),
	}
	go h.sweeper()
	return h
}

// Shutdown stops the sweeper and drops every session.
func (h *remoteRelayHub) Shutdown() {
	h.stopped.Do(func() { close(h.stop) })
	h.mu.Lock()
	sessions := h.sessions
	h.sessions = make(map[string]*remoteRelaySession)
	h.mu.Unlock()
	for _, s := range sessions {
		s.dropConn(true, nil)
		s.dropConn(false, nil)
	}
}

// sweeper drops sessions with no end connected for remoteRelaySessionTTL.
func (h *remoteRelayHub) sweeper() {
	t := time.NewTicker(remoteRelaySweepEvery)
	defer t.Stop()
	for {
		select {
		case <-h.stop:
			return
		case now := <-t.C:
			h.sweep(now)
		}
	}
}

func (h *remoteRelayHub) sweep(now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, s := range h.sessions {
		if s.idleSince(now) > remoteRelaySessionTTL {
			delete(h.sessions, key)
		}
	}
}

// idleSince is how long the session has had no end connected; zero while
// either end is.
func (s *remoteRelaySession) idleSince(now time.Time) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.terminal != nil || len(s.browsers) > 0 {
		return 0
	}
	return now.Sub(s.lastSeen)
}

// remoteRelayUpgrader upgrades HTTP to WebSocket for the relay. A browser's
// request must come from this host; the TUI's carries no Origin (it is a
// CLI client) and authenticates by bearer token instead.
var remoteRelayUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		if i := strings.Index(origin, "://"); i > 0 {
			return origin[i+3:] == r.Host
		}
		return false
	},
}

// HandleRelayWS is the TUI's side:
// GET /api/relay?id=<pairing>&verifier=<base64url sha256(proof)>&token=<bearer>.
// The first terminal on an id creates the session and owns it; a later
// terminal must be the same user, and replaces the one attached.
func (h *remoteRelayHub) HandleRelayWS(auth interface {
	ValidateToken(token string) (string, error)
}) http.HandlerFunc {
	if auth == nil {
		panic("remote relay: HandleRelayWS requires a TokenValidator")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// The account token travels in the Authorization header only (the TUI
		// dials with it, tui_remote.go). Never the query string: a URL is
		// written to the access log, and this token lives for months.
		var token string
		if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
			token = strings.TrimPrefix(ah, "Bearer ")
		}
		userID, err := auth.ValidateToken(token)
		if errors.Is(err, errRemoteIsPro) {
			// Said in words, not as a failed handshake: what the plan lacks and
			// the command that adds it (the TUI prints this body).
			http.Error(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		if err != nil || userID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		key := r.URL.Query().Get("id")
		if !validRelayKey(key) {
			http.Error(w, "missing or malformed id", http.StatusBadRequest)
			return
		}
		verifier, ok := parseRelayVerifier(r.URL.Query().Get("verifier"))
		if !ok {
			http.Error(w, "missing or malformed verifier", http.StatusBadRequest)
			return
		}
		if err := h.claim(key, userID); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		conn, err := remoteRelayUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.SetReadLimit(remoteRelayMaxFrame)
		terminal := &remoteRelayConn{conn: conn, send: make(chan []byte, 64)}
		s := h.attachTerminal(key, userID, verifier, terminal)
		if s == nil {
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "session belongs to another user"),
				time.Now().Add(remoteRelayWriteWait))
			_ = conn.Close()
			return
		}
		go h.pumpRelay(s, terminal, true)
	}
}

// HandleRelayBrowser is the browser's side:
// GET /api/relay/browser?id=<pairing>&auth=<proof> upgraded to WebSocket.
// The page at /r/<id> is the SPA; it derives the proof from the key in the
// link fragment and opens this socket. Only a session a terminal created can
// be joined, only with the proof, and by at most remoteRelayMaxBrowsers pages.
func (h *remoteRelayHub) HandleRelayBrowser(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("id")
	if !validRelayKey(key) {
		http.Error(w, "missing or malformed id", http.StatusBadRequest)
		return
	}
	s := h.session(key)
	if s == nil || !s.admits(r.URL.Query().Get("auth")) {
		http.Error(w, "no such session", http.StatusNotFound)
		return
	}
	conn, err := remoteRelayUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(remoteRelayMaxFrame)
	browser := &remoteRelayConn{conn: conn, send: make(chan []byte, 64)}
	if ok, ended := s.attachBrowser(browser); !ok {
		msg := websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "this session has as many pages open as it takes")
		if ended {
			msg = websocket.FormatCloseMessage(remoteRelayRevokedCode, "this link was turned off")
		}
		_ = conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(remoteRelayWriteWait))
		_ = conn.Close()
		return
	}
	go h.pumpRelay(s, browser, false)
}

// validRelayKey accepts the pairing id /remote generates: 12 random bytes,
// base64url without padding.
func validRelayKey(key string) bool {
	b, err := base64.RawURLEncoding.DecodeString(key)
	return err == nil && len(b) == remoteRelayKeyBytes
}

func parseRelayVerifier(v string) ([sha256.Size]byte, bool) {
	var out [sha256.Size]byte
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil || len(b) != sha256.Size {
		return out, false
	}
	copy(out[:], b)
	return out, true
}

func (h *remoteRelayHub) session(key string) *remoteRelaySession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[key]
}

// claim checks, before the upgrade, that userID may attach a terminal to key.
func (h *remoteRelayHub) claim(key, userID string) error {
	h.mu.RLock()
	s, ok := h.sessions[key]
	owned := 0
	if !ok {
		for _, other := range h.sessions {
			if other.owner == userID {
				owned++
			}
		}
	}
	h.mu.RUnlock()
	if ok && s.owner != userID {
		return errRelayNotOwner
	}
	if !ok && owned >= remoteRelayMaxSessionsPerUser {
		return errRelayTooManySessions
	}
	return nil
}

// attachTerminal puts c on session key, creating it for userID if new. An
// attached terminal is replaced (a laptop waking up beats a zombie). nil
// when the session belongs to another user.
func (h *remoteRelayHub) attachTerminal(key, userID string, verifier [sha256.Size]byte, c *remoteRelayConn) *remoteRelaySession {
	h.mu.Lock()
	s, ok := h.sessions[key]
	if !ok {
		s = &remoteRelaySession{key: key, owner: userID}
		h.sessions[key] = s
	}
	h.mu.Unlock()
	if s.owner != userID {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifier = verifier
	s.lastSeen = time.Now()
	if old := s.terminal; old != nil {
		close(old.send)
	}
	s.terminal = c
	return s
}

// attachBrowser puts c on the session unless a browser is already there.
func (s *remoteRelaySession) attachBrowser(c *remoteRelayConn) (ok, ended bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return false, true
	}
	if len(s.browsers) >= remoteRelayMaxBrowsers {
		return false, false
	}
	if s.browsers == nil {
		s.browsers = make(map[*remoteRelayConn]bool)
	}
	s.browsers[c] = true
	s.lastSeen = time.Now()
	return true, false
}

// admits reports whether proof hashes to the verifier the terminal set.
func (s *remoteRelaySession) admits(proof string) bool {
	if proof == "" {
		return false
	}
	sum := sha256.Sum256([]byte(proof))
	s.mu.RLock()
	defer s.mu.RUnlock()
	return subtle.ConstantTimeCompare(sum[:], s.verifier[:]) == 1
}

// pumpRelay reads frames from one end and delivers them to the other.
func (h *remoteRelayHub) pumpRelay(s *remoteRelaySession, c *remoteRelayConn, terminal bool) {
	go c.writePump()
	defer func() {
		s.dropConn(terminal, c)
		_ = c.conn.Close()
	}()
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var frame relayWire
		if jerr := json.Unmarshal(raw, &frame); jerr != nil {
			continue
		}
		if terminal && frame.Kind == remoteRelayEndKind {
			h.end(s)
			return
		}
		if frame.Payload == nil {
			continue // not a frame we understand; the AEAD will reject garbage anyway
		}
		s.deliver(!terminal, raw)
	}
}

// end removes a revoked session and closes its pages with a reason they show.
func (h *remoteRelayHub) end(s *remoteRelaySession) {
	h.mu.Lock()
	if h.sessions[s.key] == s {
		delete(h.sessions, s.key)
	}
	h.mu.Unlock()
	s.mu.Lock()
	s.ended = true
	for b := range s.browsers {
		_ = b.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(remoteRelayRevokedCode, "this link was turned off"),
			time.Now().Add(remoteRelayWriteWait))
	}
	s.mu.Unlock()
	s.dropConn(false, nil)
}

// deliver hands raw to the terminal (toTerminal) or the browser. The read
// lock keeps dropConn from closing the channel mid-send; the send never
// blocks — a peer that is not reading loses the frame rather than stalling
// the other end's pump.
func (s *remoteRelaySession) deliver(toTerminal bool, raw []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if toTerminal {
		if s.terminal != nil {
			offer(s.terminal, raw)
		}
		return
	}
	for b := range s.browsers {
		offer(b, raw)
	}
}

// offer queues raw for c without waiting: a peer that is not reading loses
// the frame rather than stalling the sender's pump.
func offer(c *remoteRelayConn, raw []byte) {
	select {
	case c.send <- raw:
	default:
	}
}

// writePump drains the send channel to the socket until it closes, then
// closes the socket so its reader ends too.
func (c *remoteRelayConn) writePump() {
	for frame := range c.send {
		if err := c.write(frame); err != nil {
			_ = c.conn.Close()
			return
		}
	}
	_ = c.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(remoteRelayWriteWait))
	_ = c.conn.Close()
}

// write serializes writes to one socket (gorilla allows one writer).
func (c *remoteRelayConn) write(frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, frame)
}

// dropConn detaches one end and closes its send channel. With only set, it
// detaches that end only if it is still the attached one — a replaced
// terminal's late cleanup must not remove its successor. The session stays
// in the map: a terminal that disconnected a minute ago must still be
// reachable by a browser opening the link now (the sweeper retires it).
func (s *remoteRelaySession) dropConn(terminal bool, only *remoteRelayConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !terminal {
		// A page drops itself (only); a shutdown drops every page (nil).
		for b := range s.browsers {
			if only == nil || b == only {
				delete(s.browsers, b)
				close(b.send)
			}
		}
		s.lastSeen = time.Now()
		return
	}
	c := s.terminal
	if c == nil || (only != nil && c != only) {
		return
	}
	s.terminal = nil
	s.lastSeen = time.Now()
	close(c.send)
}

// remoteRelayEventToRelay adapts ONE agent event into a relay frame's
// payload shape (the encrypted part is the caller's job). Deliberately
// minimal: the browser renders text, tool and lifecycle events; anything
// else is noise a phone does not need.
type remoteRelayFrameData struct {
	RunID     string                 `json:"run_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Seq       int                    `json:"seq,omitempty"`
	Stream    string                 `json:"stream"`
	Event     string                 `json:"event,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
}

// RemoteRelayEventName is the relay frame's "kind" for agent events.
const RemoteRelayEventName = "event"

// relayEventFromAgentEvent shapes an infra.AgentEvent for the wire.
func relayEventFromAgentEvent(ev infra.AgentEvent) (relayWire, error) {
	event, _ := ev.Data["event"].(string)
	payload, err := json.Marshal(remoteRelayFrameData{
		RunID:     ev.RunID,
		SessionID: ev.SessionID,
		Seq:       ev.Seq,
		Stream:    string(ev.Stream),
		Event:     event,
		Data:      ev.Data,
		Timestamp: ev.Timestamp,
	})
	if err != nil {
		return relayWire{}, err
	}
	return relayWire{Kind: RemoteRelayEventName, Payload: payload}, nil
}

// RelayEvents is the TUI's subscription: it receives every agent event the
// gateway broadcasts on the conversation's session and turns them into
// relay frames. Unused today — the TUI-side relay client (cmd/cli/cmd)
// listens directly through its own websocket; this is the documented shape
// for anything that wants to do the same in-process.
func (h *remoteRelayHub) RelayEvents(key string, evs <-chan infra.AgentEvent) {
	go func() {
		for ev := range evs {
			frame, err := relayEventFromAgentEvent(ev)
			if err != nil {
				continue
			}
			s := h.session(key)
			if s == nil {
				continue
			}
			raw, err := json.Marshal(frame)
			if err != nil {
				continue
			}
			s.deliver(false, raw)
		}
	}()
}
