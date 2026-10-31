package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"memdoor/cmd/tui/ui"
)

// fakeChatGPTGateway is a gateway elsewhere running Sign in with ChatGPT: its
// link sends the browser back to 127.0.0.1:<port> (the gateway's loopback,
// not this machine's), it takes a pasted address only with its state, and its
// wait answers done once one is taken.
type fakeChatGPTGateway struct {
	mu        sync.Mutex
	redirect  string
	pasted    []string
	cancelled bool
}

func newFakeChatGPTGateway(t *testing.T) *fakeChatGPTGateway {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MEMDOOR_NO_BROWSER", "1")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")
	t.Setenv("DISPLAY", ":0")
	f := &fakeChatGPTGateway{redirect: "http://127.0.0.1:" + freeLocalPort(t) + "/callback"}
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/providers/chatgpt" {
			http.NotFound(w, r)
			return
		}
		var req struct{ Action, ID, Value string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		defer f.mu.Unlock()
		switch req.Action {
		case "login":
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "g1", "auth_url": "https://auth.openai.example/authorize?" + url.Values{"redirect_uri": {f.redirect}, "state": {"S"}}.Encode()})
		case "login/paste":
			u, err := url.Parse(req.Value)
			if err != nil || u.Query().Get("state") != "S" {
				http.Error(w, `{"error": "that link belongs to another sign-in"}`, http.StatusBadRequest)
				return
			}
			f.pasted = append(f.pasted, req.Value)
			_ = json.NewEncoder(w).Encode(map[string]bool{"delivered": true})
		case "login/wait":
			if len(f.pasted) == 0 {
				_ = json.NewEncoder(w).Encode(map[string]bool{"pending": true})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"done": true, "ok": true, "id": "chatgpt", "models": 3, "tested": "gpt-x", "answer": "ok"})
		case "login/cancel":
			f.cancelled = true
			_ = json.NewEncoder(w).Encode(map[string]bool{"cancelled": true})
		}
	}))
	t.Cleanup(gw.Close)
	saved, savedHere, savedCopy := gatewayAddr, gatewayIsHere, copyLink
	t.Cleanup(func() { gatewayAddr, gatewayIsHere, copyLink = saved, savedHere, savedCopy })
	gatewayAddr = gw.URL
	gatewayIsHere = func() bool { return false }
	copyLink = func(string) error { return nil }
	return f
}

func (f *fakeChatGPTGateway) port() string {
	u, _ := url.Parse(f.redirect)
	return u.Port()
}

func freeLocalPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	return port
}

// portIsFree says nothing here listens on the port.
func portIsFree(port string) bool {
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// With the gateway on another machine, the browser's return to this
// machine's loopback reaches the gateway, the sign-in completes, and the
// port is given back (the mutation check: relayBack returning false without
// a relay and the browser's return is refused, nothing is pasted).
func TestChatGPTSignInWithAGatewayElsewhereCatchesTheReturnHere(t *testing.T) {
	f := newFakeChatGPTGateway(t)
	id, _, headless, err := signInStart(ui.KindChatGPT)
	if err != nil || headless {
		t.Fatalf("start: headless=%v err=%v", headless, err)
	}
	resp, err := http.Get(f.redirect + "?code=C&state=S")
	if err != nil {
		t.Fatalf("the browser's return must reach this machine: %v", err)
	}
	resp.Body.Close()
	if len(f.pasted) != 1 || f.pasted[0] != f.redirect+"?code=C&state=S" {
		t.Fatalf("the gateway got %q", f.pasted)
	}
	done, res, err := signInWait(ui.KindChatGPT, id)
	if err != nil || !done || !res.OK || res.Models != 3 {
		t.Fatalf("wait: %v %+v %v", done, res, err)
	}
	if !portIsFree(f.port()) {
		t.Fatal("the relay must stop once signed in")
	}
}

// The negative paths: a cancel gives the port back; a port taken here, an
// ssh session into this machine and a gateway on this machine start no relay
// (the first two ask for the address instead).
func TestChatGPTSignInRelayNegativePaths(t *testing.T) {
	f := newFakeChatGPTGateway(t)
	id, _, _, _ := signInStart(ui.KindChatGPT)
	if portIsFree(f.port()) {
		t.Fatal("the relay listens while the sign-in runs")
	}
	signInCancel(ui.KindChatGPT, id)
	if !portIsFree(f.port()) || !f.cancelled {
		t.Fatalf("cancel stops the relay and the gateway's sign-in: free=%v cancelled=%v", portIsFree(f.port()), f.cancelled)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:"+f.port())
	if err != nil {
		t.Fatal(err)
	}
	id, _, headless, err := signInStart(ui.KindChatGPT)
	ln.Close()
	if err != nil || !headless {
		t.Fatalf("a port taken here asks for the address: headless=%v err=%v", headless, err)
	}
	signInCancel(ui.KindChatGPT, id)

	t.Setenv("SSH_CONNECTION", "10.0.0.1 22 10.0.0.2 22")
	if id, _, headless, _ = signInStart(ui.KindChatGPT); !headless || !portIsFree(f.port()) {
		t.Fatalf("an ssh session: no relay, the address is pasted (headless=%v)", headless)
	}
	signInCancel(ui.KindChatGPT, id)
	t.Setenv("SSH_CONNECTION", "")

	gatewayIsHere = func() bool { return true }
	if id, _, headless, _ = signInStart(ui.KindChatGPT); headless || !portIsFree(f.port()) {
		t.Fatalf("a gateway here catches its own return: no relay (headless=%v)", headless)
	}
	signInCancel(ui.KindChatGPT, id)
}

// In an ssh session the shell asks for the address the browser landed on:
// an empty line and another sign-in's address ask again, this one's signs in.
func TestChatGPTSignInWithNoBrowserHereTakesThePastedAddress(t *testing.T) {
	f := newFakeChatGPTGateway(t)
	t.Setenv("SSH_TTY", "/dev/ttys001")
	in := bufio.NewReader(strings.NewReader("\nhttp://127.0.0.1:1/callback?code=C&state=other\n" + f.redirect + "?code=C&state=S\n"))
	var out bytes.Buffer
	if err := providerSignIn(ui.KindChatGPT, &out, in, false); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	s := out.String()
	for _, want := range []string{"copy that page's whole address", "Address: ", "another sign-in", "✓ ChatGPT plan connected · 3 models"} {
		if !strings.Contains(s, want) {
			t.Fatalf("lacks %q:\n%s", want, s)
		}
	}
	if strings.Count(s, "Address: ") != 3 || strings.Contains(s, "Code: ") {
		t.Fatalf("three prompts for an address, none for a code:\n%s", s)
	}
}
