package cmd

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"memdoor/pkg/shared"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"memdoor/cmd/tui/ui"
	"memdoor/gateway/client"
	"memdoor/gateway/protocol"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gorilla/websocket"
	qrterminal "github.com/mdp/qrterminal/v3"
	"golang.org/x/crypto/hkdf"
)

// REMOTE CONTROL (Greg, 2026-09-28: "remote control"): /remote gives this
// conversation a link and a QR code to open it on another device, and the
// TUI runs the terminal side of the relay while the conversation is open.
//
// The link is <relay host>/r/<id>#k=<key>. The id names the conversation to
// the relay; the key encrypts everything that passes through it, and it sits
// after "#", the part of a link a browser never sends to a server — so the
// relay carries the session but cannot read it.
//
// TWO HOSTS. The link and the terminal's relay socket go through
// memdoor.ai — it is already the outbound tunnel, so a phone behind NAT
// reaches it without port forwarding — with the memdoor.ai account token;
// the conversation's event feed and the posted turns stay on the gateway
// this TUI talks to (remoteSession.Gateway and Token).

// remoteURL is where the link points and where the terminal's relay socket
// dials: the public relay host. Overridable so a local gateway can be
// tested end to end.
func remoteURL() string {
	// Only memdoor.ai or this machine (a self-hosted relay): the account
	// token and the session go there.
	if v := strings.TrimSpace(os.Getenv("MEMDOOR_REMOTE_URL")); v != "" && shared.AccountHostTrusted(v) {
		return strings.TrimRight(v, "/")
	}
	return "https://memdoor.ai"
}

// remoteRelayReconnectWait is the pause between reconnect attempts: the
// gateway being down must not spin the laptop awake.
const remoteRelayReconnectWait = 5 * time.Second

// remoteFrameMax is the largest frame the terminal reads from the relay: the
// relay's own cap (gateway/remote_relay.go, remoteRelayMaxFrame). A browser
// sends keys and short turns; nothing honest comes near it.
const remoteFrameMax = 4 << 20

// The two directions of the relay, bound into every frame as AES-GCM
// additional data: a frame sealed for one direction never opens in the other,
// so a reflected frame is rejected whatever its nonce.
const (
	remoteToBrowser  = "t2b"
	remoteToTerminal = "b2t"
)

// remoteHKDFSalt domain-separates this product's keys: the link key derives
// the AEAD key under info "session" and the browser's proof under
// "browser-auth". web/src/remoteCrypto.ts uses the same labels.
const remoteHKDFSalt = "memdoor/remote-control/v1"

// remoteLink is one conversation's remote identity, kept on this machine so
// the relay client can find it (~/.memdoor/remote.json).
type remoteLink struct {
	ChannelID string    `json:"channel_id"`
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Created   time.Time `json:"created"`
}

func remoteLinksPath() string {
	return shared.MemdoorHome("remote.json")
}

func loadRemoteLinks() map[string]remoteLink {
	links := map[string]remoteLink{}
	if b, err := os.ReadFile(remoteLinksPath()); err == nil {
		_ = json.Unmarshal(b, &links)
	}
	return links
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// remoteLinkFor returns the conversation's link, making one the first time.
// "off" forgets it, so the old link and key stop meaning anything.
func remoteLinkFor(channelID string, off bool) (remoteLink, bool, error) {
	links := loadRemoteLinks()
	if off {
		_, had := links[channelID]
		delete(links, channelID)
		return remoteLink{}, had, saveRemoteLinks(links)
	}
	if l, ok := links[channelID]; ok {
		return l, true, nil
	}
	id, err := randomToken(12)
	if err != nil {
		return remoteLink{}, false, err
	}
	key, err := randomToken(32) // 256-bit key
	if err != nil {
		return remoteLink{}, false, err
	}
	l := remoteLink{ChannelID: channelID, ID: id, Key: key, Created: time.Now()}
	links[channelID] = l
	return l, false, saveRemoteLinks(links)
}

func saveRemoteLinks(links map[string]remoteLink) error {
	b, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(remoteLinksPath()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(remoteLinksPath(), b, 0o600) // it holds keys: owner only
}

// url is the link on the relay host.
func (l remoteLink) url() string {
	return remoteURL() + "/r/" + l.ID + "#k=" + l.Key
}

// viewURL is the watch-only link (/remote view): after #v= it carries the
// session key and the relay proof, both derived from the link key, and not
// the link key itself. A page holding it reads every frame and joins the
// relay, and cannot derive the write key a turn must be sealed with.
func (l remoteLink) viewURL() (string, error) {
	tok, err := remoteViewToken(l.Key)
	if err != nil {
		return "", err
	}
	return remoteURL() + "/r/" + l.ID + "#v=" + tok, nil
}

// qrText draws a link as a QR code in half-block characters.
func qrText(link string) string {
	var buf bytes.Buffer
	qrterminal.GenerateHalfBlock(link, qrterminal.L, &buf)
	return strings.TrimRight(buf.String(), "\n")
}

// remoteSession is what a conversation's relay needs from the TUI: where the
// gateway is, the credential it holds, the conversation, and how a turn is
// posted (the page's own poster, so a phone's turn is an ordinary turn).
type remoteSession struct {
	Gateway   string
	Token     string
	Workspace string
	ChannelID string
	// Account is the memdoor.ai account token: the relay host's credential
	// for /api/relay, distinct from the local gateway's.
	Account string
	Post    func(text string) error
	// History reads the conversation's last turns from the gateway.
	History func() []ui.Message
	// TTY builds the TUI for this conversation, for the page's terminal view.
	TTY func() tea.Model
}

// tuiRemote answers /remote and /remote off for one conversation, and keeps
// the relay running while the link exists.
func tuiRemote(s remoteSession) func(arg string) (string, error) {
	return func(arg string) (string, error) {
		if strings.TrimSpace(arg) == "off" {
			stopRemote(s.ChannelID)
			_, had, err := remoteLinkFor(s.ChannelID, true)
			if err != nil {
				return "", err
			}
			if !had {
				return "Remote control was not on for this conversation.", nil
			}
			return "Remote control is off: the old link and its key no longer open this conversation.", nil
		}
		if s.Account == "" {
			return "", errors.New("remote control needs a memdoor.ai account: run `memdoor login you@example.com` first, then try /remote again")
		}
		// On the hosted relay remote is Pro: say so here, in the window,
		// instead of handing out a link the relay will refuse (the relay's own
		// check stays the authority). A self-hosted relay is never asked.
		if !shared.IsLoopbackURL(remoteURL()) && remoteNeedsPro() {
			return "", errRemoteIsPro
		}
		l, existing, err := remoteLinkFor(s.ChannelID, false)
		if err != nil {
			return "", err
		}
		startRemote(s, l)
		view, err := l.viewURL()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(arg) == "view" {
			return fmt.Sprintf("A watch-only link to this conversation — whoever opens it sees the terminal and cannot type:\n\n%s\n\n%s\n\n"+
				"/remote off revokes it, with the full link.", view, qrText(view)), nil
		}
		lead := "Remote control for this conversation — open the link or scan the code:"
		if existing {
			lead = "Remote control is on for this conversation — the same link as before:"
		}
		return fmt.Sprintf("%s\n\n%s\n\n%s\n\nWatch only (sees, cannot type): %s\n\n"+
			"The key after # never leaves your devices; the relay only carries what it cannot read. "+
			"/remote off revokes both links.", lead, l.url(), qrText(l.url()), view), nil
	}
}

// resumeRemote restarts the relay for a conversation that already has a
// link (a resumed conversation, a relaunched TUI): the link on the phone
// keeps working without typing /remote again.
func resumeRemote(s remoteSession) {
	if l, ok := loadRemoteLinks()[s.ChannelID]; ok {
		startRemote(s, l)
	}
}

// remoteRelayTurn is a browser's turn: text to post, an answer to a pending
// question, a cancel of the running turn, or a request for the conversation
// so far (the page sends it each time its socket opens).
type remoteRelayTurn struct {
	Text       string `json:"text,omitempty"`
	QuestionID string `json:"question_id,omitempty"`
	Answer     string `json:"answer,omitempty"`
	Cancel     bool   `json:"cancel,omitempty"`
	History    bool   `json:"history,omitempty"`
	// The terminal view (tui_remote_tty.go): start or resize the phone's
	// TUI, and the keys typed into it.
	TTY   *remoteTTYSize `json:"tty,omitempty"`
	TTYIn string         `json:"tty_in,omitempty"`
	// Watch asks for the screen so far, addressed to the page that asked
	// (its random id): a page that opens late sees what the others see.
	Watch string `json:"watch,omitempty"`
}

// A page that opens mid-conversation is sent its last turns, so it does not
// start blank. Long replies are cut: the phone needs the gist, not the file.
const (
	remoteHistoryLimit   = 20
	remoteHistoryTextMax = 8000
	remoteHistoryWait    = 2 * time.Second
)

// remoteHistoryStream is the event stream the page reads history from
// (web/src/remoteTranscript.ts).
const remoteHistoryStream = "history"

type remoteHistoryLine struct {
	Role string `json:"role"` // "user" | "assistant"
	Text string `json:"text"`
}

// remoteHistoryEvent is the conversation so far as one agent-event-shaped
// frame: {"stream":"history","data":{"messages":[{role,text}…]}}.
func remoteHistoryEvent(msgs []ui.Message) ([]byte, error) {
	lines := make([]remoteHistoryLine, 0, len(msgs))
	for _, m := range msgs {
		text := m.Content
		if r := []rune(text); len(r) > remoteHistoryTextMax {
			text = string(r[:remoteHistoryTextMax]) + "…"
		}
		lines = append(lines, remoteHistoryLine{Role: m.Role, Text: text})
	}
	return json.Marshal(map[string]any{
		"stream": remoteHistoryStream,
		"data":   map[string]any{"messages": lines},
	})
}

// remoteRelayEndKind tells the relay the link was revoked (gateway/remote_relay.go).
const remoteRelayEndKind = "end"

// remoteRelayWire is one frame as it crosses the relay socket; the gateway
// forwards it verbatim. Payload is base64url(nonce ‖ AES-GCM ciphertext).
type remoteRelayWire struct {
	Kind    string `json:"kind"` // "event" (terminal -> browser) | "turn" (browser -> terminal)
	Payload string `json:"payload"`
}

// remoteSessionKey derives the AES-256 key from the link key.
func remoteSessionKey(key string) ([]byte, error) {
	prk := hkdf.Extract(sha256.New, []byte(key), []byte(remoteHKDFSalt))
	k := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, []byte("session")), k); err != nil {
		return nil, err
	}
	return k, nil
}

// remoteBrowserProof is what a browser shows the relay to join the session:
// an HKDF subkey of the link key under its own label, so it proves the key
// without revealing it or the AEAD key. The terminal registers
// remoteBrowserVerifier (its sha256) when it attaches.
func remoteBrowserProof(key string) (string, error) {
	prk := hkdf.Extract(sha256.New, []byte(key), []byte(remoteHKDFSalt))
	proof := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, []byte("browser-auth")), proof); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(proof), nil
}

// remoteBrowserVerifier is the value the terminal sends as ?verifier= on
// /api/relay: base64url(sha256(proof)).
func remoteBrowserVerifier(key string) (string, error) {
	proof, err := remoteBrowserProof(key)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(proof))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// remoteWriteAEAD is the cipher a DRIVING frame from a page is sealed with:
// a turn, an answer, a stop, a key, a resize. Its key is an HKDF subkey of
// the link key under "write", which a view-only link (remoteViewToken) does
// not carry and cannot derive, so the terminal can tell a page that may
// drive from one that may only watch.
func remoteWriteAEAD(key string) (cipher.AEAD, error) {
	prk := hkdf.Extract(sha256.New, []byte(key), []byte(remoteHKDFSalt))
	k := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.Expand(sha256.New, prk, []byte("write")), k); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// remoteViewToken is what follows #v= in a view-only link:
// base64url(session key ‖ browser proof), 64 bytes. Enough to read and to
// join; not enough to write.
func remoteViewToken(key string) (string, error) {
	sk, err := remoteSessionKey(key)
	if err != nil {
		return "", err
	}
	proof, err := remoteBrowserProof(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.RawURLEncoding.DecodeString(proof)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(append(sk, raw...)), nil
}

// remoteAEAD is the session's AES-GCM cipher.
func remoteAEAD(key string) (cipher.AEAD, error) {
	k, err := remoteSessionKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// remoteSeal encrypts one payload for a direction under a fresh random
// nonce, carried in front of the ciphertext. A random 96-bit nonce per frame
// needs no state shared between the ends, survives dropped frames and
// reconnects, and never repeats in practice.
func remoteSeal(aead cipher.AEAD, dir string, plain []byte) (string, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return remoteSealWithNonce(aead, dir, nonce, plain), nil
}

func remoteSealWithNonce(aead cipher.AEAD, dir string, nonce, plain []byte) string {
	return base64.RawURLEncoding.EncodeToString(aead.Seal(append([]byte(nil), nonce...), nonce, plain, []byte(dir)))
}

// remoteOpen decrypts a payload sealed for dir.
func remoteOpen(aead cipher.AEAD, dir, payload string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	if len(raw) < aead.NonceSize() {
		return nil, errors.New("remote frame too short")
	}
	return aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte(dir))
}

// remoteRelays are the relays running in this process, one per conversation.
var remoteRelays = struct {
	sync.Mutex
	byChannel map[string]*remoteRelay
}{byChannel: map[string]*remoteRelay{}}

func startRemote(s remoteSession, l remoteLink) {
	remoteRelays.Lock()
	defer remoteRelays.Unlock()
	if r, ok := remoteRelays.byChannel[s.ChannelID]; ok {
		if r.link.Key == l.Key {
			return
		}
		r.Close()
	}
	r := newRemoteRelay(s, l)
	remoteRelays.byChannel[s.ChannelID] = r
	go r.run()
}

// stopRemote revokes a conversation's link (/remote off): the relay is told
// to end the session, so pages on the old link close and say so.
func stopRemote(channelID string) {
	remoteRelays.Lock()
	defer remoteRelays.Unlock()
	if r, ok := remoteRelays.byChannel[channelID]; ok {
		r.revoked.Store(true)
		r.Close()
		delete(remoteRelays.byChannel, channelID)
	}
}

// remoteRelay is the terminal side of one conversation's relay. It follows
// the conversation's agent events on its own gateway websocket (the one the
// TUI screen uses is the TUI's), seals each for the browser, and turns the
// browser's frames into posted turns, answers and cancels.
type remoteRelay struct {
	s      remoteSession
	link   remoteLink
	events chan []byte
	ttyOut chan []byte
	closed chan struct{}
	once   sync.Once

	ttyMu sync.Mutex
	tty   *remoteTTY

	// revoked: closed by /remote off, not by the TUI quitting. The relay is
	// told to end the session; a TUI that quits leaves it, so the link works
	// again when the conversation is reopened.
	revoked atomic.Bool
}

func newRemoteRelay(s remoteSession, l remoteLink) *remoteRelay {
	return &remoteRelay{s: s, link: l, events: make(chan []byte, 256), ttyOut: make(chan []byte, 16), closed: make(chan struct{})}
}

// Close stops the relay: no reconnect, no more frames either way.
func (r *remoteRelay) Close() {
	r.once.Do(func() {
		close(r.closed)
		r.stopTTY()
	})
}

func (r *remoteRelay) run() {
	feed, err := r.follow()
	if err != nil {
		return
	}
	defer feed.Close()
	for {
		err := r.attach(feed)
		if err == nil || errors.Is(err, errRemoteIsPro) {
			// A plan refusal does not change in five seconds: stop, rather
			// than ask billing every five seconds for nothing (review,
			// 2026-10-04).
			return
		}
		select {
		case <-r.closed:
			return
		case <-time.After(remoteRelayReconnectWait):
		}
	}
}

// follow subscribes to the conversation's agent events. The client
// reconnects on its own when the gateway restarts.
func (r *remoteRelay) follow() (*client.Client, error) {
	opts := client.DefaultOptions()
	opts.EnableLogging = false
	opts.Workspace = r.s.Workspace
	opts.Channel = r.s.ChannelID
	opts.Token = r.s.Token
	c := client.New(r.s.Gateway, "", opts)
	c.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
		b, err := json.Marshal(msg.Data)
		if err != nil {
			return nil
		}
		select {
		case r.events <- b:
		default: // no browser reading and the buffer is full: the phone misses a frame, the turn does not stall
		}
		return nil
	})
	for {
		if err := c.Connect(); err == nil {
			c.Listen()
			return c, nil
		}
		select {
		case <-r.closed:
			return nil, errors.New("closed")
		case <-time.After(remoteRelayReconnectWait):
		}
	}
}

// attach holds one relay socket until it breaks (error: reconnect) or the
// relay is closed (nil).
func (r *remoteRelay) attach(feed *client.Client) error {
	verifier, err := remoteBrowserVerifier(r.link.Key)
	if err != nil {
		return err
	}
	aead, err := remoteAEAD(r.link.Key)
	if err != nil {
		return err
	}
	wsURL, err := remoteWSURL()
	if err != nil {
		return err
	}
	wsURL += "?" + url.Values{"id": {r.link.ID}, "verifier": {verifier}}.Encode()
	// The token rides the header, as on /ws, so it stays out of access logs.
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": {"Bearer " + r.s.Account}})
	if err != nil {
		// The relay says why in its body (remote is Pro on the hosted relay);
		// pass that sentence on instead of "bad handshake".
		if resp != nil && resp.StatusCode == http.StatusPaymentRequired {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
			return errRemoteIsPro
		}
		return err
	}
	defer conn.Close()
	conn.SetReadLimit(remoteFrameMax)

	writeAEAD, err := remoteWriteAEAD(r.link.Key)
	if err != nil {
		return err
	}
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var wire remoteRelayWire
			if json.Unmarshal(raw, &wire) != nil || wire.Kind != "turn" {
				continue
			}
			// Sealed with the write key: the full link, which may drive.
			// Sealed with the session key only: a view-only link, which may
			// read. Neither: not this link's, dropped.
			drive := true
			plain, err := remoteOpen(writeAEAD, remoteToTerminal, wire.Payload)
			if err != nil {
				drive = false
				if plain, err = remoteOpen(aead, remoteToTerminal, wire.Payload); err != nil {
					continue
				}
			}
			var turn remoteRelayTurn
			if json.Unmarshal(plain, &turn) == nil {
				r.handle(feed, turn, drive)
			}
		}
	}()

	for {
		select {
		case <-r.closed:
			if r.revoked.Load() {
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_ = conn.WriteJSON(remoteRelayWire{Kind: remoteRelayEndKind})
			}
			return nil
		case <-readerDone:
			return errors.New("relay socket closed")
		case ev := <-r.events:
			if err := sendRemoteEvent(conn, aead, ev); err != nil {
				return err
			}
		case ev := <-r.ttyOut:
			if err := sendRemoteEvent(conn, aead, ev); err != nil {
				return err
			}
		}
	}
}

// sendRemoteEvent seals one event for the browser and writes it. Only a
// failed write is an error: the socket is gone.
func sendRemoteEvent(conn *websocket.Conn, aead cipher.AEAD, ev []byte) error {
	payload, err := remoteSeal(aead, remoteToBrowser, ev)
	if err != nil {
		return nil
	}
	raw, err := json.Marshal(remoteRelayWire{Kind: "event", Payload: payload})
	if err != nil {
		return nil
	}
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, raw)
}

// handle acts on one browser turn: a cancel interrupts the running turn, an
// answer unblocks a pending question, text is posted as the next turn.
func (r *remoteRelay) handle(feed *client.Client, t remoteRelayTurn, drive bool) {
	if !drive {
		// A view-only page may ask to see; everything else is driving.
		switch {
		case t.History:
			r.sendHistory()
		case t.Watch != "":
			r.ttyReplay(t.Watch)
		}
		return
	}
	switch {
	case t.Cancel:
		_ = feed.SendMessage(protocol.Message{Type: protocol.MessageTypeCancel})
	case t.QuestionID != "":
		_ = feed.SendAnswer(t.QuestionID, t.Answer)
	case t.History:
		r.sendHistory()
	case t.Watch != "":
		r.ttyReplay(t.Watch)
	case t.TTY != nil:
		r.ttyScreen(*t.TTY)
	case t.TTYIn != "":
		r.ttyType(t.TTYIn)
	case strings.TrimSpace(t.Text) != "" && r.s.Post != nil:
		_ = r.s.Post(t.Text)
	}
}

// sendHistory queues the conversation so far for the browser. It waits a
// little for room: unlike a live delta, a dropped history leaves the page blank.
func (r *remoteRelay) sendHistory() {
	if r.s.History == nil {
		return
	}
	b, err := remoteHistoryEvent(r.s.History())
	if err != nil {
		return
	}
	select {
	case r.events <- b:
	case <-r.closed:
	case <-time.After(remoteHistoryWait):
	}
}

// remoteWSURL is the relay host's /api/relay socket: wss on an https host,
// ws on a plain-http one (tests).
func remoteWSURL() (string, error) {
	base := remoteURL()
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("remote relay host %q is not a URL", base)
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "wss":
		u.Scheme = "wss"
	case "http", "ws":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("remote relay host %q has no http(s) scheme", base)
	}
	u.Path = "/api/relay"
	return u.String(), nil
}

// errRemoteIsPro is what a free account hears about remote control on the
// hosted relay — the same sentence the relay answers with (gateway
// remote_relay_auth.go).
var errRemoteIsPro = errors.New("remote control is a Pro feature on memdoor.ai: memdoor account subscribe, then /remote again")

// remoteNeedsPro reports whether the signed-in account is on the free plan.
// An unanswered question is not a refusal: the relay decides then.
func remoteNeedsPro() bool {
	var me meAnswer
	if err := brokerCall(http.MethodGet, "/v1/me", nil, &me); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(me.Plan), "free") || strings.TrimSpace(me.Plan) == ""
}
