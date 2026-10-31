package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/term"

	"memdoor/cmd/tui/ui"
	"memdoor/gateway/providers"
	"memdoor/pkg/modelkey"
)

// FIND THE KEY BEFORE ASKING FOR ONE (Greg, 2026-10-31: "when they connect we
// should scan keys … easy frictionless"; "they dont know the key"). A key in
// this shell under any name, or in the project's .env, is offered first:
// Enter connects it. Nothing leaves but the probe `memdoor connect` makes to
// the key's own provider.

// foundKeys is the model keys on this machine that a connect kind takes.
func foundKeys() []modelkey.Found {
	dir, _ := os.Getwd()
	return connectable(modelkey.Find(os.Environ(), dir))
}

func connectable(keys []modelkey.Found) []modelkey.Found {
	var out []modelkey.Found
	for _, k := range keys {
		if connectKindOf(k.Providers[0]) != nil {
			out = append(out, k)
		}
	}
	return out
}

// foundKeysOrAsk is foundKeys and, when there are none and a person is at the
// terminal, the keys other coding tools saved (Codex, OpenCode, llm, aider):
// read only after a yes, being another program's secrets.
func foundKeysOrAsk(in *bufio.Reader, out io.Writer) []modelkey.Found {
	if found := foundKeys(); len(found) > 0 {
		return found
	}
	return askToolStores(in, out)
}

// askToolStores is the keys other coding tools saved, after a yes; nil with
// nobody at the terminal, no store, or a no.
func askToolStores(in *bufio.Reader, out io.Writer) []modelkey.Found {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	home, _ := os.UserHomeDir()
	stores := modelkey.ToolStores(home)
	if len(stores) == 0 {
		return nil
	}
	var tools []string
	for _, s := range stores {
		if len(tools) == 0 || tools[len(tools)-1] != s.Tool {
			tools = append(tools, s.Tool)
		}
	}
	fmt.Fprintf(out, "No key in this shell or this project. %s saved keys on this machine: look in them? [Y/n] ", strings.Join(tools, ", "))
	line, _ := in.ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(line)); a != "" && a != "y" && a != "yes" {
		return nil
	}
	found := connectable(modelkey.FindInTools(stores, home))
	if len(found) == 0 {
		fmt.Fprintln(out, "  no model key in them.")
	}
	return found
}

func connectKindOf(p modelkey.Provider) *ui.ConnectKind {
	for i := range ui.ConnectKinds {
		if ui.ConnectKinds[i].ID == string(p) {
			return &ui.ConnectKinds[i]
		}
	}
	return nil
}

// foundLabel is a found key as a line of a list: the provider, then where.
// A key the variable's name decided is that provider's alone; a tie the name
// does not break reads "DeepSeek or OpenAI" (the probe decides).
func foundLabel(k modelkey.Found) string {
	var names []string
	for _, p := range k.Providers {
		if kind := connectKindOf(p); kind != nil {
			names = append(names, kind.Name)
		}
		if k.Named {
			break
		}
	}
	return fmt.Sprintf("%-20s %s", strings.Join(names, " or "), k.Where())
}

// gatewayKeySources is each connected provider's key source on the gateway
// ("env OPENROUTER_API_KEY"), by id; nil when it cannot be read.
var gatewayKeySources = func() map[string]string {
	var res struct {
		Providers []struct {
			ID        string `json:"id"`
			Connected bool   `json:"connected"`
			KeySource string `json:"key_source"`
		} `json:"providers"`
	}
	if err := NewClient().GetJSON("/api/providers", &res); err != nil {
		return nil
	}
	out := map[string]string{}
	for _, p := range res.Providers {
		if p.Connected {
			out[p.ID] = p.KeySource
		}
	}
	return out
}

// splitInUse sets aside the found keys the gateway already answers on: its
// provider connected from the gateway's environment under the same variable
// (it was started from this shell or this .envrc). Offering those again
// changes nothing (live 2026-10-31: five keys from .envrc, all connected,
// were listed before the sign-ins). Only for a gateway on this machine, where
// a name means the same variable.
func splitInUse(found []modelkey.Found) (offer, inUse []modelkey.Found) {
	if _, local := localGatewayPort(); !local || len(found) == 0 {
		return found, nil
	}
	sources := gatewayKeySources()
	for _, k := range found {
		if src, ok := sources[string(k.Providers[0])]; ok && sameVariable(strings.TrimPrefix(src, "env "), k.Name) && strings.HasPrefix(src, "env ") {
			inUse = append(inUse, k)
			continue
		}
		offer = append(offer, k)
	}
	return offer, inUse
}

// connectFound connects a found key, trying each provider its shape fits.
// A variable of this shell goes by its NAME when the gateway is local, so
// nothing is copied and a rotated key keeps working; when the gateway cannot
// read it (started from another shell, a service, a box) the value goes,
// kept in ~/.memdoor/providers.json (0600) like a pasted key.
func connectFound(k modelkey.Found) (ui.ConnectResult, error) {
	_, local := localGatewayPort()
	var res ui.ConnectResult
	for _, p := range k.Providers {
		kind := connectKindOf(p)
		if kind == nil {
			continue
		}
		req := ui.ConnectRequest{ID: kind.ID, Name: kind.Name, API: kind.API, Base: kind.Base, Key: k.Value}
		if local && k.InEnvironment() {
			req.Key = k.Name
		}
		var err error
		res, err = tuiConnect(req)
		if err == nil && !res.OK && req.Key == k.Name && strings.Contains(res.Error, providers.KeyNameNotSet) {
			req.Key = k.Value
			res, err = tuiConnect(req)
		}
		if err != nil {
			return res, err
		}
		if res.OK {
			return res, nil
		}
	}
	return res, nil
}

// The window's /connect lists the same keys (ui.ConnectOps Found,
// ConnectFound): the list it showed is kept until the next /connect, so the
// number picked is the key shown.
var tuiFound struct {
	sync.Mutex
	keys []modelkey.Found
}

func tuiFoundKeys() []string {
	keys, _ := splitInUse(foundKeys())
	tuiFound.Lock()
	tuiFound.keys = keys
	tuiFound.Unlock()
	labels := make([]string, len(keys))
	for i, k := range keys {
		labels[i] = foundLabel(k)
	}
	return labels
}

func tuiConnectFound(i int) (ui.ConnectResult, error) {
	tuiFound.Lock()
	keys := tuiFound.keys
	tuiFound.Unlock()
	if i < 0 || i >= len(keys) {
		return ui.ConnectResult{}, fmt.Errorf("that key is no longer in the list: /connect again")
	}
	return connectFound(keys[i])
}

// printConnected is the ✓ line `memdoor connect` prints.
func printConnected(out io.Writer, res ui.ConnectResult) {
	fmt.Fprintf(out, "✓ %s connected · %d models · tested %s → %q\n", res.ID, res.Models, res.Tested, res.Answer)
	if res.FreeModel != "" {
		fmt.Fprintln(out, "  "+ui.FreeModelNote(res.FreeModel))
	}
}

// offerFoundKey gets a gateway with no model one, with as few questions as
// can be (Greg, 2026-10-31: "less friction"), when a person is at the
// terminal: a key in this shell is connected without a word asked (the
// person's own session, as aider does); a key in the project's files is
// asked about once (it may be the project's production key, someone else's
// bill); another tool's saved keys are looked in after a yes and the first
// that works is connected; with none, a sign-in is offered. On a gateway
// with a model, or with nobody to answer, it does nothing.
func offerFoundKey(c *Client) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || readBrain(c).State != "none" {
		return
	}
	in := bufio.NewReader(os.Stdin)
	var shell, files []modelkey.Found
	for _, k := range foundKeys() {
		if k.InEnvironment() {
			shell = append(shell, k)
		} else {
			files = append(files, k)
		}
	}
	if connectFirstThatWorks(shell, os.Stderr) {
		return
	}
	if len(files) > 0 {
		offerFileKeys(files, in)
		return
	}
	if connectFirstThatWorks(askToolStores(in, os.Stderr), os.Stderr) {
		return
	}
	offerSignIn(in)
}

// connectFirstThatWorks connects the keys in order until one is taken, and
// says which, and how to undo it; false when none was.
func connectFirstThatWorks(keys []modelkey.Found, out io.Writer) bool {
	for _, k := range keys {
		fmt.Fprintf(out, "No model connected yet: trying the %s key in %s …\n", connectKindOf(k.Providers[0]).Name, k.Where())
		res, err := connectFound(k)
		switch {
		case err != nil:
			fmt.Fprintf(out, "  ✗ %v\n", err)
		case !res.OK:
			fmt.Fprintf(out, "  ✗ %s\n", res.Error)
		default:
			printConnected(out, res)
			fmt.Fprintf(out, "  memdoor connect --remove %s forgets it.\n", res.ID)
			return true
		}
	}
	return false
}

// offerFileKeys asks once before using a key from the project's files.
func offerFileKeys(found []modelkey.Found, in *bufio.Reader) {
	fmt.Fprintln(os.Stderr, "No model connected yet. Found in this project:")
	for i, k := range found {
		fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, foundLabel(k))
	}
	fmt.Fprint(os.Stderr, "Connect 1? [Enter: yes · a number · n: not now] ")
	line, _ := in.ReadString('\n')
	pick, ok := pickFound(strings.TrimSpace(line), len(found))
	if !ok {
		return
	}
	connectFirstThatWorks(found[pick:pick+1], os.Stderr)
}

// offerSignIn is the first run with no key anywhere: a choice instead of a
// sentence (Greg, 2026-10-31: "they dont know the key"). Enter signs in with
// OpenRouter, free; 2 with a ChatGPT plan; n leaves it.
func offerSignIn(in *bufio.Reader) {
	fmt.Fprintln(os.Stderr, "No model connected yet, and no key here that works.")
	fmt.Fprintln(os.Stderr, "  1. Sign in with OpenRouter (free account, in the browser, nothing to copy)")
	fmt.Fprintln(os.Stderr, "  2. Sign in with ChatGPT (a Plus or Pro plan)")
	fmt.Fprint(os.Stderr, "Which? [Enter: 1 · 2 · n: not now; memdoor connect <provider> takes a key] ")
	line, _ := in.ReadString('\n')
	kind := ""
	switch a := strings.ToLower(strings.TrimSpace(line)); {
	case a == "" || a == "1" || a == "y" || a == "yes":
		kind = ui.KindOpenRouterLogin
	case a == "2":
		kind = ui.KindChatGPT
	default:
		return
	}
	// No paste-anytime reader: the window opens next, and a reader left on
	// stdin would take its keys. A headless sign-in reads its code once.
	if err := providerSignIn(kind, os.Stderr, in, false); err != nil {
		fmt.Fprintf(os.Stderr, "✗ not signed in: %v\n", err)
	}
}

// pickFound reads the answer to the offer: "" is the first, a number from
// the list that one, anything else none.
func pickFound(answer string, n int) (int, bool) {
	if answer == "" || strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes") {
		return 0, true
	}
	i, err := strconv.Atoi(answer)
	if err != nil || i < 1 || i > n {
		return 0, false
	}
	return i - 1, true
}

// connectEntry is one numbered line of `memdoor connect`'s list: a found
// key, or a kind.
type connectEntry struct {
	found *modelkey.Found
	kind  *ui.ConnectKind
}

// connectMenu prints the list and returns its entries in number order: the
// keys found here that would change something, then the sign-ins (no key
// needed; never buried under found keys), then every kind that takes a key.
// The keys the gateway already uses are named, not numbered. Enter (1) is the
// first found key, else Sign in with OpenRouter.
func connectMenu(found, inUse []modelkey.Found, out io.Writer) []connectEntry {
	var menu []connectEntry
	var signIns, keyed []*ui.ConnectKind
	for i := range ui.ConnectKinds {
		if k := &ui.ConnectKinds[i]; signsIn(k.ID) {
			signIns = append(signIns, k)
		} else {
			keyed = append(keyed, k)
		}
	}
	total := len(found) + len(signIns) + len(keyed)
	width := len(strconv.Itoa(total))
	line := func(text string) {
		fmt.Fprintf(out, "  %*d. %s\n", width, len(menu), text)
	}
	if len(inUse) > 0 {
		var names []string
		for _, k := range inUse {
			if kind := connectKindOf(k.Providers[0]); kind != nil {
				names = append(names, kind.Name)
			}
		}
		fmt.Fprintf(out, "Already connected from your environment: %s.\n", strings.Join(names, ", "))
	}
	if len(found) > 0 {
		fmt.Fprintln(out, "Found on this machine:")
		for i := range found {
			menu = append(menu, connectEntry{found: &found[i]})
			line(foundLabel(found[i]))
		}
	}
	fmt.Fprintln(out, "Sign in, no key:")
	for _, k := range signIns {
		menu = append(menu, connectEntry{kind: k})
		line(kindLabel(k, width))
	}
	fmt.Fprintln(out, "Or a provider's key:")
	for _, k := range keyed {
		menu = append(menu, connectEntry{kind: k})
		line(kindLabel(k, width))
	}
	return menu
}

// kindLabel is a kind's name and hint; a name too long for the column puts
// its hint on the next line, under the names.
func kindLabel(k *ui.ConnectKind, width int) string {
	const col = 32
	if len(k.Name) <= col {
		return fmt.Sprintf("%-*s %s", col, k.Name, k.Hint)
	}
	return k.Name + "\n" + strings.Repeat(" ", 2+width+2+col+1) + k.Hint
}

// sameVariable says two variable names hold the same key for the gateway:
// the same name, or OpenRouter's two (the gateway reads either; ByokKeyVar).
func sameVariable(a, b string) bool {
	if a == b {
		return true
	}
	var hasA, hasB bool
	for _, name := range providers.OpenRouterKeyVars {
		hasA, hasB = hasA || name == a, hasB || name == b
	}
	return hasA && hasB
}
