package cmd

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"memdoor/cmd/tui/ui"
	"memdoor/gateway"
	"memdoor/gateway/protocol"

	"github.com/gorilla/websocket"
)

// The key is only ever in the link's fragment, the link is on the gateway the
// TUI talks to, the same conversation keeps its link, /remote off revokes it,
// and the file holding keys is owner-only.
func TestRemoteLink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// A relay host of its own — not the gateway the TUI talks to (…:1),
	// and not memdoor.ai (no network in a unit test). The link must point
	// at the relay host, never at the gateway.
	t.Setenv("MEMDOOR_REMOTE_URL", "http://127.0.0.1:2")
	s := remoteSession{Gateway: "http://127.0.0.1:1", ChannelID: "chan-1", Account: "acct-tok"}
	t.Cleanup(func() { stopRemote(s.ChannelID) })
	remote := tuiRemote(s)

	out, err := remote("")
	if err != nil {
		t.Fatal(err)
	}
	var link, view string
	for _, f := range strings.Fields(out) {
		if !strings.HasPrefix(f, "http://127.0.0.1:2/r/") {
			continue
		}
		if strings.Contains(f, "#k=") && link == "" {
			link = f
		}
		if strings.Contains(f, "#v=") {
			view = f
		}
	}
	// The watch-only link: same session, no link key in it, and nothing a
	// page could derive the write key from.
	vu, err := url.Parse(view)
	if err != nil || view == "" || !strings.HasPrefix(vu.Fragment, "v=") {
		t.Fatalf("no watch-only link in:\n%s", out)
	}
	full, _ := url.Parse(link)
	if key := strings.TrimPrefix(full.Fragment, "k="); strings.Contains(view, key) {
		t.Fatal("the watch-only link must not carry the link key")
	}
	if out, err := remote("view"); err != nil || !strings.Contains(out, view) || strings.Contains(out, "#k=") {
		t.Fatalf("/remote view prints the watch-only link and not the full one: %v\n%s", err, out)
	}
	u, err := url.Parse(link)
	if err != nil || link == "" {
		t.Fatalf("no link on the relay host in:\n%s", out)
	}
	if !strings.HasPrefix(u.Fragment, "k=") || len(u.Fragment) < 40 {
		t.Fatalf("the key must be in the fragment, got %q", u.Fragment)
	}
	if strings.Contains(u.Path, u.Fragment[2:]) || u.RawQuery != "" {
		t.Fatal("the key must never be in the path or query a server receives")
	}
	if !strings.Contains(out, "▀") && !strings.Contains(out, "▄") {
		t.Fatal("a QR code is drawn")
	}

	again, _ := remote("")
	if !strings.Contains(again, link) {
		t.Fatal("the same conversation keeps its link")
	}
	if info, err := os.Stat(remoteLinksPath()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the key file must be owner-only, got %v", info.Mode().Perm())
	}

	if off, _ := remote("off"); !strings.Contains(off, "off") {
		t.Fatalf("off: %s", off)
	}
	fresh, _ := remote("")
	if strings.Contains(fresh, link) {
		t.Fatal("after off, a new link and key")
	}
}

// A frame opens only for the direction it was sealed for: a reflected frame
// is rejected whatever the key or nonce (the old scheme set a bit on a
// derived byte, and for half of all keys both directions shared nonces).
func TestRemoteFramesAreDirectionBound(t *testing.T) {
	for _, key := range []string{"k1", "k2", "k3", "k4", "k5", "k6", "k7", "k8"} {
		aead, err := remoteAEAD(key)
		if err != nil {
			t.Fatal(err)
		}
		p, err := remoteSeal(aead, remoteToBrowser, []byte(`{"x":1}`))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := remoteOpen(aead, remoteToTerminal, p); err == nil {
			t.Fatalf("key %q: a terminal frame opened as a browser frame", key)
		}
		if plain, err := remoteOpen(aead, remoteToBrowser, p); err != nil || string(plain) != `{"x":1}` {
			t.Fatalf("key %q: round trip %q %v", key, plain, err)
		}
		q, _ := remoteSeal(aead, remoteToBrowser, []byte(`{"x":1}`))
		if p == q {
			t.Fatal("two frames must never share a nonce")
		}
	}
}

// The browser's crypto (web/src/remoteCrypto.ts, run in Node) and the
// terminal's agree byte for byte: the proof the relay checks, a terminal
// frame the page opens, a page frame the terminal opens, and a reflected
// frame both reject. This is the contract between the two languages.
func TestRemoteCryptoMatchesTheBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	esbuild, _ := filepath.Abs("../../../web/node_modules/.bin/esbuild")
	if _, err := os.Stat(esbuild); err != nil {
		t.Skip("web/node_modules not installed")
	}
	dir := t.TempDir()
	bundle := filepath.Join(dir, "remoteCrypto.mjs")
	src, _ := filepath.Abs("../../../web/src/remoteCrypto.ts")
	if out, err := exec.Command(esbuild, src, "--format=esm", "--outfile="+bundle).CombinedOutput(); err != nil {
		t.Fatalf("esbuild: %v\n%s", err, out)
	}

	const key, plain = "kat-link-key", `{"text":"hello"}`
	nonce, _ := hex.DecodeString("000102030405060708090a0b")
	aead, _ := remoteAEAD(key)
	proof, _ := remoteBrowserProof(key)
	fromTerminal := remoteSealWithNonce(aead, remoteToBrowser, nonce, []byte(plain))
	view, _ := remoteViewToken(key)
	write, _ := remoteWriteAEAD(key)

	driver := filepath.Join(dir, "check.mjs")
	_ = os.WriteFile(driver, []byte(`
import { deriveRemote, deriveView, open, seal, TO_BROWSER, TO_TERMINAL } from './remoteCrypto.mjs';
const [key, plain, fromTerminal, viewToken] = process.argv.slice(2);
const k = await deriveRemote(key);
const v = await deriveView(viewToken);
const nonce = Uint8Array.from([0,1,2,3,4,5,6,7,8,9,10,11]);
let reflected = 'rejected';
try { await open(k, TO_TERMINAL, fromTerminal); reflected = 'opened'; } catch {}
console.log(JSON.stringify({
  proof: k.proof,
  opened: await open(k, TO_BROWSER, fromTerminal),
  reflected,
  fromBrowser: await seal(k, TO_TERMINAL, plain, nonce),
  viewProof: v.proof,
  viewOpened: await open(v, TO_BROWSER, fromTerminal),
  viewCanWrite: v.write !== undefined,
  fromViewer: await seal(v, TO_TERMINAL, plain, nonce),
}));
`), 0o644)
	out, err := exec.Command(node, driver, key, plain, fromTerminal, view).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	var got struct {
		Proof, Opened, Reflected, FromBrowser string
		ViewProof, ViewOpened, FromViewer     string
		ViewCanWrite                          bool
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("node output: %s", out)
	}
	if got.Proof != proof {
		t.Errorf("proof: browser %s, terminal %s", got.Proof, proof)
	}
	if got.Opened != plain {
		t.Errorf("the page could not open a terminal frame: %q", got.Opened)
	}
	if got.Reflected != "rejected" {
		t.Error("the page opened a terminal frame as its own direction")
	}
	// A page on the full link seals what it sends with the write key: the
	// terminal opens it as a driving frame.
	if want := remoteSealWithNonce(write, remoteToTerminal, nonce, []byte(plain)); got.FromBrowser != want {
		t.Errorf("the page seals differently:\n page %s\n term %s", got.FromBrowser, want)
	}
	// A watch-only page joins with the same proof and reads the same frames,
	// has no write key, and what it sends opens only as a watching frame.
	if got.ViewProof != proof || got.ViewOpened != plain {
		t.Errorf("a watch-only page must join and read: proof %v, opened %q", got.ViewProof == proof, got.ViewOpened)
	}
	if got.ViewCanWrite {
		t.Error("a watch-only page must not have a write key")
	}
	if _, err := remoteOpen(write, remoteToTerminal, got.FromViewer); err == nil {
		t.Error("a watch-only page's frame opened with the write key")
	}
	if opened, err := remoteOpen(aead, remoteToTerminal, got.FromViewer); err != nil || string(opened) != plain {
		t.Errorf("a watch-only page's frame must open as a watching frame: %q %v", opened, err)
	}
	if opened, err := remoteOpen(write, remoteToTerminal, got.FromBrowser); err != nil || string(opened) != plain {
		t.Errorf("the terminal could not open a page frame: %q %v", opened, err)
	}
}

type remoteTestUsers map[string]string

func (u remoteTestUsers) ValidateToken(token string) (string, error) {
	if id, ok := u[token]; ok {
		return id, nil
	}
	return "", errors.New("bad token")
}

// End to end through the real relay: an agent event on the conversation
// reaches a browser holding the link's key, and the browser's sealed turn is
// posted as the next turn; a cancel reaches the gateway.
func TestRemoteRelayEndToEnd(t *testing.T) {
	hub := gateway.NewRemoteRelayHub()
	t.Cleanup(hub.Shutdown)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	cancels := make(chan struct{}, 1)

	// The RELAY host (memdoor.ai in production): the terminal attaches and
	// the browser joins here, and it admits the account token only.
	relayMux := http.NewServeMux()
	relayMux.Handle("/api/relay", hub.HandleRelayWS(remoteTestUsers{"acct": "alice"}))
	relayMux.HandleFunc("/api/relay/browser", hub.HandleRelayBrowser)
	relay := httptest.NewServer(relayMux)
	t.Cleanup(relay.Close)
	t.Setenv("MEMDOOR_REMOTE_URL", relay.URL)

	// The GATEWAY host: the conversation's event feed and the posted turns
	// stay here, and it admits the gateway token only. It broadcasts one
	// agent event a little after the subscriber connects, and records a
	// cancel.
	gatewayMux := http.NewServeMux()
	gatewayMux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "bad token", http.StatusUnauthorized)
			return
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go func() {
			for {
				var m protocol.Message
				if c.ReadJSON(&m) != nil {
					return
				}
				if m.Type == protocol.MessageTypeCancel {
					cancels <- struct{}{}
				}
			}
		}()
		for range 20 {
			time.Sleep(100 * time.Millisecond)
			_ = c.WriteJSON(protocol.Message{Type: protocol.MessageTypeAgentEvent, Data: map[string]interface{}{
				"run_id": "r1", "stream": "assistant", "data": map[string]interface{}{"event": "text_delta", "delta": "hi"},
			}})
		}
	})
	gw := httptest.NewServer(gatewayMux)
	t.Cleanup(gw.Close)
	// A token valid on one host must not open the other: the account token
	// must not reach the feed.
	if _, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gw.URL, "http")+"/ws", http.Header{"Authorization": {"Bearer acct"}}); err == nil {
		t.Fatal("the account token must not open the gateway's feed")
	}

	var mu sync.Mutex
	var posted []string
	link := remoteLink{ChannelID: "c1", ID: "AAAAAAAAAAAAAAAA", Key: "e2e-link-key"}
	// The relay socket carries the ACCOUNT token to the relay host; the
	// event feed and the posted turns, the GATEWAY token to the gateway.
	s := remoteSession{Gateway: gw.URL, Token: "tok", Account: "acct", Workspace: "w", ChannelID: "c1", Post: func(text string) error {
		mu.Lock()
		posted = append(posted, text)
		mu.Unlock()
		return nil
	}, History: func() []ui.Message {
		return []ui.Message{{Role: "user", Content: "fix the login bug"}, {Role: "assistant", Content: "Fixed: the token was read before it was set."}}
	}}
	startRemote(s, link)
	t.Cleanup(func() { stopRemote("c1") })

	proof, _ := remoteBrowserProof(link.Key)
	aead, _ := remoteAEAD(link.Key)
	// ws:// on a plain-http relay host, wss:// on an https one.
	relayURL := "ws" + strings.TrimPrefix(remoteURL(), "http") + "/api/relay/browser?" + url.Values{"id": {link.ID}, "auth": {proof}}.Encode()
	var browser *websocket.Conn
	for i := 0; i < 50 && browser == nil; i++ { // the terminal attaches first
		browser, _, _ = websocket.DefaultDialer.Dial(relayURL, nil)
		if browser == nil {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if browser == nil {
		t.Fatal("the browser never joined: the terminal did not attach")
	}
	defer browser.Close()

	_ = browser.SetReadDeadline(time.Now().Add(5 * time.Second))
	var wire remoteRelayWire
	if err := browser.ReadJSON(&wire); err != nil || wire.Kind != "event" {
		t.Fatalf("no event reached the browser: %v %+v", err, wire)
	}
	plain, err := remoteOpen(aead, remoteToBrowser, wire.Payload)
	if err != nil || !strings.Contains(string(plain), `"delta":"hi"`) {
		t.Fatalf("event frame: %s %v", plain, err)
	}

	// The page asks for the conversation so far as it opens; the terminal
	// answers among the live events.
	ask, _ := json.Marshal(remoteRelayTurn{History: true})
	askSealed, _ := remoteSeal(aead, remoteToTerminal, ask)
	if err := browser.WriteJSON(remoteRelayWire{Kind: "turn", Payload: askSealed}); err != nil {
		t.Fatal(err)
	}
	var history string
	for history == "" {
		_ = browser.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err := browser.ReadJSON(&wire); err != nil {
			t.Fatalf("no history reached the browser: %v", err)
		}
		if p, err := remoteOpen(aead, remoteToBrowser, wire.Payload); err == nil && strings.Contains(string(p), `"stream":"history"`) {
			history = string(p)
		}
	}
	if !strings.Contains(history, "fix the login bug") || !strings.Contains(history, "the token was read before it was set") {
		t.Fatalf("history frame: %s", history)
	}

	// A turn sealed with the session key alone — what a watch-only link
	// holds — is dropped at the terminal; the full link's write key drives.
	watcher, _ := json.Marshal(remoteRelayTurn{Text: "delete everything"})
	wp, _ := remoteSeal(aead, remoteToTerminal, watcher)
	if err := browser.WriteJSON(remoteRelayWire{Kind: "turn", Payload: wp}); err != nil {
		t.Fatal(err)
	}
	write, _ := remoteWriteAEAD(link.Key)
	for _, turn := range []remoteRelayTurn{{Text: "run the tests"}, {Cancel: true}} {
		b, _ := json.Marshal(turn)
		p, _ := remoteSeal(write, remoteToTerminal, b)
		if err := browser.WriteJSON(remoteRelayWire{Kind: "turn", Payload: p}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(posted)
		mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(posted) != 1 || posted[0] != "run the tests" {
		t.Fatalf("the browser's turn was not posted: %v", posted)
	}
	select {
	case <-cancels:
	case <-time.After(5 * time.Second):
		t.Fatal("the browser's cancel never reached the gateway")
	}

	// /remote off: the page on the old link is closed as revoked.
	mu.Unlock()
	stopRemote("c1")
	mu.Lock()
	_ = browser.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		if _, _, err := browser.ReadMessage(); err != nil {
			if !websocket.IsCloseError(err, 4001) {
				t.Fatalf("/remote off must close the page as revoked: %v", err)
			}
			break
		}
	}
}

// Without an account token on memdoor.ai, a link would be printed that no
// browser can open: the relay would not admit the terminal. /remote says so
// instead, and how to fix it.
func TestRemoteLinkNeedsAccount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MEMDOOR_BILLING_TOKEN", "")
	out, err := tuiRemote(remoteSession{Gateway: "http://127.0.0.1:1", ChannelID: "chan-noacct"})("")
	if err == nil || !strings.Contains(err.Error(), "memdoor login") {
		t.Fatalf("no token must be an error naming `memdoor login`, got %q, %v", out, err)
	}
	if strings.Contains(out, "/r/") {
		t.Fatal("no link without an account token")
	}
}

// The relay host is data: memdoor.ai by default, overridable for testing a
// local gateway.
func TestRemoteURLOverride(t *testing.T) {
	t.Setenv("MEMDOOR_REMOTE_URL", "http://127.0.0.1:9/")
	if got, want := remoteURL(), "http://127.0.0.1:9"; got != want {
		t.Fatalf("remoteURL() = %q, want %q", got, want)
	}
	l := remoteLink{ID: "i"}
	if got := l.url(); !strings.HasPrefix(got, "http://127.0.0.1:9/r/i#k=") {
		t.Fatalf("the link points at the override, got %q", got)
	}
}

// The history the terminal sends is what the page shows: the page's own
// transcript code (web/src/remoteTranscript.ts) runs on the terminal's real
// frame, so a renamed field on either side fails here instead of leaving
// the phone blank.
func TestRemoteHistoryRendersInThePage(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	esbuild, _ := filepath.Abs("../../../web/node_modules/.bin/esbuild")
	if _, err := os.Stat(esbuild); err != nil {
		t.Skip("web/node_modules not installed")
	}
	dir := t.TempDir()
	src, _ := filepath.Abs("../../../web/src/remoteTranscript.ts")
	if out, err := exec.Command(esbuild, src, "--format=esm", "--outfile="+filepath.Join(dir, "remoteTranscript.mjs")).CombinedOutput(); err != nil {
		t.Fatalf("esbuild: %v\n%s", err, out)
	}
	long := strings.Repeat("x", remoteHistoryTextMax+50)
	frame, err := remoteHistoryEvent([]ui.Message{
		{Role: "user", Content: "fix the login bug"},
		{Role: "assistant", Content: "Fixed it."},
		{Role: "assistant", Content: long},
	})
	if err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(dir, "check.mjs")
	_ = os.WriteFile(driver, []byte(`
import { apply } from './remoteTranscript.mjs';
const stale = [{ kind: 'reply', run: 'r0', text: 'shown before a reconnect' }];
console.log(JSON.stringify(apply(stale, JSON.parse(process.argv[2]))));
`), 0o644)
	out, err := exec.Command(node, driver, string(frame)).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	var items []struct{ Kind, Text string }
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatalf("page output: %s", out)
	}
	if len(items) != 3 || items[0].Kind != "you" || items[0].Text != "fix the login bug" ||
		items[1].Kind != "reply" || items[1].Text != "Fixed it." {
		t.Fatalf("the page must show exactly the history, replacing what it had: %+v", items)
	}
	if n := len([]rune(items[2].Text)); n != remoteHistoryTextMax+1 || !strings.HasSuffix(items[2].Text, "…") {
		t.Fatalf("a long reply must be cut to %d runes and marked: got %d", remoteHistoryTextMax, n)
	}
}
