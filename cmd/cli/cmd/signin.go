package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"

	"memdoor/cmd/tui/ui"
	"memdoor/gateway/providers"
	"memdoor/pkg/oidc"
	"memdoor/pkg/openrouter"
)

// ---- Browser sign-ins: ChatGPT (2026-10-10), OpenRouter (2026-10-31) --------
//
// Sign in with ChatGPT runs on the gateway (gateway/provider_signin.go): its
// credential lives there and refreshes there. Its link sends the browser back
// to the gateway's loopback, so with a gateway elsewhere (the ssh door, a box)
// this machine catches that return on the same port and passes it on
// (relayBack). Sign in with OpenRouter runs HERE, where the person's terminal
// and browser are, and hands the key it gets to the gateway the way a pasted
// key goes (`/api/providers/connect`). Both work the same with a gateway on
// this machine, over the ssh door or on a box, with no port to forward and no
// device code. Where no browser can come back to this machine (an ssh session
// into it) the person pastes what the page ends on: OpenRouter's code, or the
// address ChatGPT's browser landed on. The steps (start, wait, paste, cancel)
// are the same for both; the window drives them through ui.ConnectOps.

// signInPaths is the gateway route of each sign-in that runs there.
var signInPaths = map[string]string{
	ui.KindChatGPT: "/api/providers/chatgpt",
}

// signsIn says whether a connect kind is a browser sign-in.
func signsIn(kind string) bool {
	_, onGateway := signInPaths[kind]
	return onGateway || kind == ui.KindOpenRouterLogin
}

func signInPost(kind string, body map[string]any, out any) error {
	path, ok := signInPaths[kind]
	if !ok {
		return fmt.Errorf("no gateway sign-in for %s", kind)
	}
	return NewClient().PostJSON(path, body, out)
}

// signInStart begins a sign-in: its id, the link, and whether the page will
// show a code to paste (no browser can reach back here).
func signInStart(kind string) (id, authURL string, headless bool, err error) {
	if kind == ui.KindOpenRouterLogin {
		return localSignInStart()
	}
	var out struct {
		ID      string `json:"id"`
		AuthURL string `json:"auth_url"`
	}
	if err = signInPost(kind, map[string]any{"action": "login"}, &out); err != nil {
		return "", "", false, err
	}
	return out.ID, out.AuthURL, relayBack(kind, out.ID, out.AuthURL), nil
}

// relays are the browser returns this machine catches for a sign-in on a
// gateway elsewhere, by sign-in id.
var relays = struct {
	sync.Mutex
	m map[string]*oidc.Relay
}{m: map[string]*oidc.Relay{}}

// relayBack makes a gateway sign-in's browser return work from here. The
// link sends the browser back to the GATEWAY's loopback: on this machine
// that is the gateway itself, and elsewhere (the ssh door, a box) this
// machine catches it on the same port and passes it on like a pasted
// address. It says headless when no browser here can come back at all (an
// ssh session into this machine), or the port is taken here: the address
// the browser lands on is then pasted.
func relayBack(kind, id, authURL string) (headless bool) {
	if noBrowserHere() {
		return true
	}
	if gatewayIsHere() {
		return false
	}
	r, err := oidc.StartRelay(authURL, func(address string) error { return signInPaste(kind, id, address) })
	if err != nil {
		return true
	}
	relays.Lock()
	relays.m[id] = r
	relays.Unlock()
	return false
}

// copyLink puts a sign-in link on the clipboard; a test keeps the person's.
var copyLink = clipboard.WriteAll

// gatewayIsHere says the gateway is on this machine; a test says otherwise.
var gatewayIsHere = func() bool { _, local := localGatewayPort(); return local }

func stopRelay(id string) {
	relays.Lock()
	r := relays.m[id]
	delete(relays.m, id)
	relays.Unlock()
	r.Stop()
}

// errSignInAgain is the gateway's "registered; approve once more" (ChatGPT's
// first registration): the caller starts a fresh sign-in.
var errSignInAgain = fmt.Errorf("sign in again")

// signInWait waits up to 25 s: done with the connect's result, or not yet.
func signInWait(kind, id string) (done bool, res ui.ConnectResult, err error) {
	if kind == ui.KindOpenRouterLogin {
		return localSignInWait(id)
	}
	var out struct {
		ui.ConnectResult
		Done  bool   `json:"done"`
		Err   string `json:"error"`
		Again bool   `json:"again"`
	}
	err = signInPost(kind, map[string]any{"action": "login/wait", "id": id}, &out)
	if err != nil || out.Done || out.Err != "" {
		stopRelay(id)
	}
	if err != nil {
		return false, res, err
	}
	if out.Again {
		return false, res, fmt.Errorf("%w: %s", errSignInAgain, out.Err)
	}
	if out.Err != "" && !out.Done {
		return false, res, fmt.Errorf("%s", out.Err)
	}
	res = out.ConnectResult
	if out.Err != "" {
		res.Error = out.Err
	}
	return out.Done, res, nil
}

func signInPaste(kind, id, value string) error {
	if kind == ui.KindOpenRouterLogin {
		return localSignInPaste(id, value)
	}
	return signInPost(kind, map[string]any{"action": "login/paste", "id": id, "value": value}, nil)
}

func signInCancel(kind, id string) {
	if kind == ui.KindOpenRouterLogin {
		localSignInCancel(id)
		return
	}
	stopRelay(id)
	_ = signInPost(kind, map[string]any{"action": "login/cancel", "id": id}, nil)
}

// ---- Sign in with OpenRouter, here ------------------------------------------

// openRouterEndpoints is where the sign-in goes; a test points it at a fake.
var openRouterEndpoints = openrouter.Default()

// noBrowserHere says no browser can reach back to this machine's loopback:
// an ssh session, or a Linux with no display. OpenRouter's page then shows a
// code to paste instead of redirecting here.
func noBrowserHere() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return true
	}
	return runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

type localSignIn struct {
	login  *openrouter.Login
	cancel context.CancelFunc
	done   chan localSignInResult
}

type localSignInResult struct {
	key string
	err error
}

var localSignIns = struct {
	sync.Mutex
	m map[string]*localSignIn
}{m: map[string]*localSignIn{}}

// localSignInWaitFor is how long one wait holds before answering "not yet".
var localSignInWaitFor = 25 * time.Second

func localSignInStart() (id, authURL string, headless bool, err error) {
	var l *openrouter.Login
	if headless = noBrowserHere(); headless {
		l = openrouter.BeginHeadlessLogin(openRouterEndpoints, nil)
	} else if l, err = openrouter.BeginLogin(openRouterEndpoints, nil); err != nil {
		return "", "", false, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &localSignIn{login: l, cancel: cancel, done: make(chan localSignInResult, 1)}
	id = fmt.Sprintf("openrouter-%d", time.Now().UnixNano())
	localSignIns.Lock()
	localSignIns.m[id] = s
	localSignIns.Unlock()
	go func() { key, err := l.Wait(ctx); s.done <- localSignInResult{key, err} }()
	return id, l.AuthURL, headless, nil
}

func localSignInOf(id string) *localSignIn {
	localSignIns.Lock()
	defer localSignIns.Unlock()
	return localSignIns.m[id]
}

// localSignInWait waits for the key, then connects it on the gateway like a
// pasted one: probed, kept, and the engine chosen again (its ladder, or a
// free model on an account with no credit).
func localSignInWait(id string) (bool, ui.ConnectResult, error) {
	s := localSignInOf(id)
	if s == nil {
		return false, ui.ConnectResult{}, fmt.Errorf("no sign-in %q in progress", id)
	}
	select {
	case r := <-s.done:
		localSignIns.Lock()
		delete(localSignIns.m, id)
		localSignIns.Unlock()
		s.cancel()
		if r.err != nil {
			return false, ui.ConnectResult{}, r.err
		}
		res, err := connectOpenRouterKey(r.key)
		if err != nil {
			return false, res, err
		}
		return true, res, nil
	case <-time.After(localSignInWaitFor):
		return false, ui.ConnectResult{}, nil
	}
}

// connectOpenRouterKey hands the key the sign-in got to the gateway. A
// gateway that does not answer at once is asked again: the key exists on the
// person's account now, and losing it would mean signing in again.
func connectOpenRouterKey(key string) (ui.ConnectResult, error) {
	req := ui.ConnectRequest{ID: providers.OpenRouterID, Name: "OpenRouter", API: providers.APIOpenRouter, Base: providers.OpenRouterBase, Key: key}
	var res ui.ConnectResult
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if res, err = tuiConnect(req); err == nil {
			return res, nil
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return res, fmt.Errorf("signed in, but the gateway did not take the key: %w (the key is on your OpenRouter account as \"Memdoor\": sign in again, or paste it with memdoor connect openrouter)", err)
}

func localSignInPaste(id, value string) error {
	s := localSignInOf(id)
	if s == nil {
		return fmt.Errorf("no sign-in %q in progress", id)
	}
	return s.login.Deliver(value)
}

func localSignInCancel(id string) {
	if s := localSignInOf(id); s != nil {
		s.cancel()
	}
}

// ---- The window and the shell ----------------------------------------------

// tuiConnectLoginStart is the window's start of a browser sign-in.
func tuiConnectLoginStart(kind string) (id, u string, opened, copied bool, err error) {
	id, u, headless, err := signInStart(kind)
	if err != nil {
		return "", "", false, false, err
	}
	opened = !headless && os.Getenv("MEMDOOR_NO_BROWSER") == "" && openBrowser(u) == nil
	copied = copyLink(u) == nil
	return id, u, opened, copied, nil
}

// signInWords are what the shell says for one provider's sign-in.
type signInWords struct {
	title     string
	again     string   // ChatGPT's second approval; "" when there is none
	failed    string   // before the error, when signed in but not usable
	elsewhere string   // with no browser here: what to do with the link
	prompt    string   // and what to paste
	success   string   // the ✓ line's subject
	after     []string // what to know once connected
}

var signInText = map[string]signInWords{
	ui.KindChatGPT: {
		title:     "Sign in with ChatGPT",
		again:     "Memdoor is now registered with your ChatGPT — one more approval completes the sign-in",
		failed:    "✗ signed in, but the plan could not be used",
		elsewhere: "Open this link in a browser on any machine and approve. The browser then lands on a page that does not load (http://127.0.0.1…): copy that page's whole address and paste it here.",
		prompt:    "Address",
		success:   "ChatGPT plan connected",
		after: []string{
			"Your plan's allowance answers Memdoor's turns, no API key. ChatGPT → Settings → Usage shows the weekly cap per app.",
			"memdoor tui — /model chatgpt:<id> pins one of its models (the chatgpt: prefix: an OpenAI key lists the same ids); memdoor connect --remove chatgpt signs out.",
		},
	},
	ui.KindOpenRouterLogin: {
		title:     "Sign in with OpenRouter",
		failed:    "✗ signed in, but the key OpenRouter made could not be used",
		elsewhere: "Open this link in a browser on any machine, sign in (or sign up, free), then paste the code it shows:",
		prompt:    "Code",
		success:   "OpenRouter connected",
		after: []string{
			"OpenRouter made a key named Memdoor on your account (openrouter.ai/settings/keys); turns are billed to your credits at list price.",
		},
	},
}

// providerSignIn runs a sign-in from the shell: the browser opens (or, with
// no browser here, the page shows a code to paste), the link is printed and
// copied, Ctrl+C cancels. With pasteAnytime a line typed at any moment is
// taken as a pasted address (a browser on another machine); a first run
// passes false, since the window opens next and a reader left on stdin would
// take its keys: the code of a headless sign-in is then read once, here.
func providerSignIn(kind string, out io.Writer, in *bufio.Reader, pasteAnytime bool) error {
	words := signInText[kind]
	err := signInOnce(kind, words, words.title, out, in, pasteAnytime)
	if errors.Is(err, errSignInAgain) && words.again != "" {
		return signInOnce(kind, words, words.again, out, in, pasteAnytime)
	}
	return err
}

func signInOnce(kind string, words signInWords, title string, out io.Writer, in *bufio.Reader, pasteAnytime bool) error {
	id, authURL, headless, err := signInStart(kind)
	if err != nil {
		return err
	}
	copied := copyLink(authURL) == nil
	fmt.Fprintln(out, "\n"+title)
	switch {
	case headless:
		fmt.Fprintln(out, "  "+words.elsewhere)
	case os.Getenv("MEMDOOR_NO_BROWSER") == "" && openBrowser(authURL) == nil:
		fmt.Fprintln(out, "  Your browser is opening. If it doesn't, open this link:")
	default:
		fmt.Fprintln(out, "  Open this link to approve:")
	}
	fmt.Fprintf(out, "  %s\n", authURL)
	if copied {
		fmt.Fprintln(out, "  (copied to the clipboard)")
	}

	if headless {
		// Read here, before Ctrl+C is caught: Ctrl+C at the prompt ends the
		// program as anywhere else, and no reader is left on stdin.
		if in == nil {
			signInCancel(kind, id)
			return fmt.Errorf("no terminal to paste into: run memdoor connect %s in one", kind)
		}
		if err := readCode(kind, id, words.prompt, out, in); err != nil {
			signInCancel(kind, id)
			return err
		}
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)

	if !headless {
		fmt.Fprint(out, "  Waiting for you to approve.")
		if pasteAnytime && in != nil {
			fmt.Fprint(out, " If the browser is on another machine, paste the page's address here.")
			go func() {
				for {
					line, err := in.ReadString('\n')
					if v := strings.TrimSpace(line); v != "" {
						if perr := signInPaste(kind, id, v); perr != nil {
							fmt.Fprintln(out, "  ✗ "+perr.Error())
						}
					}
					if err != nil {
						return
					}
				}
			}()
		}
		fmt.Fprintln(out, " Ctrl+C cancels.")
	}

	type result struct {
		res ui.ConnectResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		deadline := time.Now().Add(11 * time.Minute)
		for time.Now().Before(deadline) {
			ok, res, err := signInWait(kind, id)
			if err != nil || ok {
				done <- result{res, err}
				return
			}
		}
		done <- result{err: fmt.Errorf("no sign-in within 11 minutes")}
	}()
	select {
	case <-stop:
		signInCancel(kind, id)
		return fmt.Errorf("sign-in cancelled")
	case r := <-done:
		if r.err != nil {
			return r.err
		}
		if !r.res.OK {
			fmt.Fprintf(out, "%s: %s\n", words.failed, r.res.Error)
			return fmt.Errorf("%s", r.res.Error)
		}
		fmt.Fprintf(out, "✓ %s · %d models · tested %s → %q\n", words.success, r.res.Models, r.res.Tested, r.res.Answer)
		if r.res.FreeModel != "" {
			fmt.Fprintln(out, "  "+ui.FreeModelNote(r.res.FreeModel))
		}
		for _, n := range ui.PlanNotes(r.res) {
			fmt.Fprintln(out, "  "+n)
		}
		for _, line := range words.after {
			fmt.Fprintln(out, "  "+line)
		}
		return nil
	}
}

// readCode reads what a headless sign-in hands back (OpenRouter's code, the
// address ChatGPT's browser landed on) until one is taken: an empty line or a
// refused one asks again; the input's end stops.
func readCode(kind, id, prompt string, out io.Writer, in *bufio.Reader) error {
	for {
		fmt.Fprint(out, "  "+prompt+": ")
		line, err := in.ReadString('\n')
		if v := strings.TrimSpace(line); v != "" {
			perr := signInPaste(kind, id, v)
			if perr == nil {
				return nil
			}
			fmt.Fprintln(out, "  ✗ "+perr.Error())
		}
		if err != nil {
			fmt.Fprintln(out)
			return fmt.Errorf("the input ended before a code was pasted: run it in a terminal, open the link, then paste the code")
		}
	}
}
