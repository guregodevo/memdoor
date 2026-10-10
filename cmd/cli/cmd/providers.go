package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"memdoor/cmd/tui/ui"
)

// memdoor providers / memdoor connect — the provider registry from the
// shell (gateway/providers/registry.go, gateway/providers_connect.go). The
// window's /connect and /model are the same two calls.

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Where models come from: each provider, connected or not, and which one answers",
	Long: `Every provider this gateway knows — the company AI gateway, Anthropic,
OpenAI, Gemini, OpenRouter, and the ones you connected — with whether it has
a credential, where that credential came from, and which one is answering.

  memdoor providers              the list
  memdoor connect                add one: pick the kind, paste the URL and token; it is probed first
  memdoor connect --remove corp  forget one you added
  memdoor model search <name>    the models of every connected provider`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if refresh, _ := cmd.Flags().GetBool("refresh"); refresh {
			var res map[string]any
			if err := NewClient().PostJSON("/api/providers?refresh=1", map[string]any{}, &res); err != nil {
				return err
			}
			fmt.Println("✓ every provider's list and the reference catalogue will be read again on the next use")
			return nil
		}
		if audit, _ := cmd.Flags().GetBool("audit"); audit {
			// Where every connected provider's windows come from: the
			// maintenance number (Greg, 2026-10-02: "very important to
			// maintain it … let's add top 100 models"). A guess is a model
			// nobody states; those are listed so they can be declared.
			var res struct {
				Providers []struct {
					ID     string `json:"id"`
					Models []struct {
						ID      string `json:"id"`
						Context int    `json:"context"`
						MaxOut  int    `json:"max_output"`
						Source  string `json:"context_source"`
					} `json:"models"`
				} `json:"providers"`
			}
			if err := NewClient().GetJSON("/api/models?limit=0", &res); err != nil {
				return err
			}
			total, guessed := 0, 0
			for _, p := range res.Providers {
				if len(p.Models) == 0 {
					continue
				}
				n := map[string]int{}
				noOut := 0
				var guesses []string
				for _, m := range p.Models {
					n[m.Source]++
					if m.MaxOut == 0 {
						noOut++
					}
					if m.Source == "default" {
						guesses = append(guesses, m.ID)
					}
				}
				total += len(p.Models)
				guessed += len(guesses)
				fmt.Printf("%-12s %4d models · window from provider %d · catalogue %d · reference %d · declared %d · guessed %d · output cap unknown %d\n",
					p.ID, len(p.Models), n["provider"], n["catalogue"], n["reference"], n["declared"], n["default"], noOut)
				for i, g := range guesses {
					if i == 8 {
						fmt.Printf("             … and %d more\n", len(guesses)-8)
						break
					}
					fmt.Printf("             ~ %s\n", g)
				}
			}
			fmt.Printf("%d models, %d guessed (%.0f%% known)\n", total, guessed, 100*float64(total-guessed)/float64(max(total, 1)))
			return nil
		}
		if id, _ := cmd.Flags().GetString("models"); id != "" {
			// One provider's own list, as its endpoint names them.
			var res struct {
				Providers []struct {
					ID     string `json:"id"`
					Error  string `json:"error"`
					Models []struct {
						ID       string   `json:"id"`
						Name     string   `json:"name"`
						Context  int      `json:"context"`
						Source   string   `json:"context_source"`
						MaxOut   int      `json:"max_output"`
						Inputs   []string `json:"inputs"`
						Thinking bool     `json:"thinking"`
						InPerM   float64  `json:"in_per_m"`
						OutPerM  float64  `json:"out_per_m"`
					} `json:"models"`
				} `json:"providers"`
			}
			if err := NewClient().GetJSON("/api/models?limit=0", &res); err != nil {
				return err
			}
			for _, p := range res.Providers {
				if p.ID != id {
					continue
				}
				if p.Error != "" {
					return fmt.Errorf("%s: %s", id, p.Error)
				}
				fmt.Printf("%s — %d models (id · context · max out · $ per million in / out):\n", id, len(p.Models))
				for _, m := range p.Models {
					out := "    ?"
					if m.MaxOut > 0 {
						out = fmt.Sprintf("%4dk", m.MaxOut/1000)
					}
					mark := " "
					if m.Source == "default" {
						mark = "~" // a guess: neither the provider nor the reference states it
					}
					extra := ""
					if len(m.Inputs) > 1 {
						extra += " · " + strings.Join(m.Inputs, "+")
					}
					if m.Thinking {
						extra += " · thinking"
					}
					fmt.Printf("  %-44s %5dk%s %s  $%.2f / $%.2f  %s%s\n", m.ID, m.Context/1000, mark, out, m.InPerM, m.OutPerM, m.Name, extra)
				}
				fmt.Println("  (~ after a context = a guess: set it in ~/.memdoor/providers.json or MEMDOOR_MODEL_CONTEXT)")
				return nil
			}
			return fmt.Errorf("no provider %q — memdoor providers lists them", id)
		}
		var res struct {
			Providers []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				API       string `json:"api"`
				Base      string `json:"base"`
				KeySource string `json:"key_source"`
				Connected bool   `json:"connected"`
				Active    bool   `json:"active"`
				BuiltIn   bool   `json:"built_in"`
			} `json:"providers"`
		}
		if err := NewClient().GetJSON("/api/providers", &res); err != nil {
			return err
		}
		for _, p := range res.Providers {
			mark, state := " ", "not connected · memdoor connect "+p.ID
			if p.Connected {
				state = "connected (" + p.KeySource + ")"
			}
			if p.Active {
				mark = "●"
			}
			fmt.Printf("%s %-12s %-20s %-10s %s\n    %s\n", mark, p.ID, p.Name, p.API, state, p.Base)
		}
		return nil
	},
}

// connectKindList names every kind `connect` takes, from the table the
// window's /connect picker uses — the help listed five of them when there
// were nine (2026-10-03).
func connectKindList() string {
	ids := make([]string, 0, len(ui.ConnectKinds))
	for _, k := range ui.ConnectKinds {
		ids = append(ids, k.ID)
	}
	return strings.Join(ids, ", ")
}

// providerKeySource is where a provider's key already comes from on the
// gateway ("env ANTHROPIC_API_KEY", "providers.json"), or "" — the prefill.
func providerKeySource(id string) string {
	for _, p := range listProviders() {
		if p.ID == id && p.Connected {
			return p.KeySource
		}
	}
	return ""
}

// providerRow is one line of GET /api/providers.
type providerRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	API       string `json:"api"`
	KeySource string `json:"key_source"`
	Connected bool   `json:"connected"`
	Active    bool   `json:"active"`
}

// listProviders is every provider the gateway knows, or nothing when it
// cannot be asked.
func listProviders() []providerRow {
	var res struct {
		Providers []providerRow `json:"providers"`
	}
	if err := NewClient().GetJSON("/api/providers", &res); err != nil {
		return nil
	}
	return res.Providers
}

// tuiConnect is the window's /connect: one request, probed by the gateway.
func tuiConnect(req ui.ConnectRequest) (ui.ConnectResult, error) {
	var res ui.ConnectResult
	err := NewClient().PostJSON("/api/providers/connect", req, &res)
	return res, err
}

var connectCmd = &cobra.Command{
	Use:   "connect [kind]",
	Short: "Connect a provider: pick the kind, paste the URL and token, it is probed, then kept",
	Long: `Adds a provider to this gateway. The kind is one of: ` + connectKindList() + `.
The token is read without
echo; it can be the value, the NAME of an environment variable that holds it,
or !command that prints it (Keychain, 1Password, Vault). Before anything is
kept, the gateway reads the provider's model list and makes one tiny call;
a wrong URL or token fails here, in words. Kept in ~/.memdoor/providers.json
(0600). --probe tests without keeping.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rm, _ := cmd.Flags().GetString("remove"); rm != "" {
			var res map[string]any
			if rm == "chatgpt" {
				if err := NewClient().PostJSON("/api/providers/chatgpt", map[string]any{"action": "logout"}, &res); err != nil {
					return err
				}
				fmt.Println("✓ signed out of ChatGPT")
				return nil
			}
			if err := NewClient().DeleteJSON("/api/providers/connect?id="+rm, &res); err != nil {
				return err
			}
			fmt.Printf("✓ %s forgotten\n", rm)
			return nil
		}
		in := bufio.NewReader(os.Stdin)
		ask := func(prompt, def string) string {
			if def != "" {
				fmt.Printf("%s [%s]: ", prompt, def)
			} else {
				fmt.Printf("%s: ", prompt)
			}
			line, _ := in.ReadString('\n')
			if line = strings.TrimSpace(line); line == "" {
				return def
			}
			return line
		}
		kind := ""
		if len(args) > 0 {
			kind = strings.ToLower(args[0])
		}
		var pick *ui.ConnectKind
		for i := range ui.ConnectKinds {
			if ui.ConnectKinds[i].ID == kind {
				pick = &ui.ConnectKinds[i]
			}
		}
		if pick == nil {
			fmt.Println("Which kind of provider?")
			for i, k := range ui.ConnectKinds {
				fmt.Printf("  %d. %-32s %s\n", i+1, k.Name, k.Hint)
			}
			n := ask("Number", "1")
			for i := range ui.ConnectKinds {
				if fmt.Sprint(i+1) == n {
					pick = &ui.ConnectKinds[i]
				}
			}
			if pick == nil {
				return fmt.Errorf("pick a number from the list")
			}
		}
		if pick.API == "chatgpt" {
			return chatgptSignIn(os.Stdout, os.Stdin)
		}
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			id = pick.ID
			if id == "custom" {
				id = ask("A short id for it (letters, digits, dashes)", "")
			}
		}
		base, _ := cmd.Flags().GetString("base")
		if base == "" {
			base = ask("Base URL", pick.Base)
		}
		key, _ := cmd.Flags().GetString("key")
		if key == "" {
			// The kind's variable NAME is the default (none for a company
			// gateway, whose name is its own): Enter keeps it, and when the
			// gateway already holds it the prompt says so.
			keyVar, keySet := pick.KeyVar, false
			if src, ok := strings.CutPrefix(providerKeySource(id), "env "); ok && src != "" {
				keyVar, keySet = src, true
			}
			prompt := "Token or key (not shown; a value, an env var NAME, or !command)"
			switch {
			case keySet:
				prompt = fmt.Sprintf("Token or key [%s, set on the gateway — enter keeps it]", keyVar)
			case keyVar != "":
				prompt = fmt.Sprintf("Token or key [%s; or the value, not shown]", keyVar)
			}
			if term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Print(prompt + ": ")
				raw, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Println()
				if err != nil {
					return err
				}
				key = strings.TrimSpace(string(raw))
			} else {
				key = ask("Token or key", "")
			}
			if key == "" {
				key = keyVar
			}
		}
		model, _ := cmd.Flags().GetString("model")
		if model == "" && pick.AsksModel && term.IsTerminal(int(os.Stdin.Fd())) {
			model = ask("A model to test with (blank: the first the provider lists)", "")
		}
		probe, _ := cmd.Flags().GetBool("probe")
		fmt.Println("Probing …")
		var res struct {
			OK     bool     `json:"ok"`
			ID     string   `json:"id"`
			API    string   `json:"api"`
			Models int      `json:"models"`
			Sample []string `json:"sample"`
			Tested string   `json:"tested"`
			Answer string   `json:"answer"`
			Error  string   `json:"error"`
			Advice string   `json:"advice"`
			Saved  bool     `json:"saved"`
		}
		err := NewClient().PostJSON("/api/providers/connect", map[string]any{
			"id": id, "name": pick.Name, "api": pick.API, "base": base, "key": strings.TrimSpace(key), "model": model, "probe": probe,
		}, &res)
		if err != nil && res.Error == "" {
			return err
		}
		if !res.OK {
			fmt.Printf("✗ not connected: %s\n", res.Error)
			if res.Advice != "" {
				fmt.Printf("  %s\n", res.Advice)
			}
			os.Exit(1)
		}
		kept := "kept in ~/.memdoor/providers.json"
		if !res.Saved {
			kept = "probe only, nothing kept"
		}
		fmt.Printf("✓ %s connected · %s API · %d models · tested %s → %q · %s\n", res.ID, res.API, res.Models, res.Tested, res.Answer, kept)
		if len(res.Sample) > 0 {
			fmt.Printf("  e.g. %s\n", strings.Join(res.Sample, ", "))
		}
		fmt.Println("  /model <id> in the window pins one of its models; memdoor model search <name> finds them.")
		return nil
	},
}

func init() {
	providersCmd.Flags().String("models", "", "one provider's own model list, by id")
	providersCmd.Flags().Bool("audit", false, "where every connected provider's windows come from, and which are guesses")
	providersCmd.Flags().Bool("refresh", false, "read every provider's list and the reference catalogue again (a vendor shipped)")
	connectCmd.Flags().Bool("probe", false, "test the URL and token, keep nothing")
	connectCmd.Flags().String("id", "", "the provider's id (custom kind)")
	connectCmd.Flags().String("base", "", "the base URL (asked when absent)")
	connectCmd.Flags().String("key", "", "the token: a value, an env var NAME, or !command (asked, unseen, when absent)")
	connectCmd.Flags().String("model", "", "a model to test with (a gateway that lists none)")
	connectCmd.Flags().String("remove", "", "forget a provider you added, by id")
	rootCmd.AddCommand(providersCmd, connectCmd)
}

// ---- Sign in with ChatGPT (2026-10-10) --------------------------------------
//
// The browser step runs on the gateway (gateway/providers_chatgpt.go); the
// shell opens the link and polls, as the MCP sign-in does (mcp.go).

type chatgptLoginReply struct {
	ID      string           `json:"id"`
	AuthURL string           `json:"auth_url"`
	Done    bool             `json:"done"`
	Pending bool             `json:"pending"`
	Error   string           `json:"error"`
	Email   string           `json:"email"`
	Result  ui.ConnectResult `json:"-"`
}

func chatgptPost(body map[string]any, out any) error {
	return NewClient().PostJSON("/api/providers/chatgpt", body, out)
}

// chatgptLoginStart begins a sign-in: its id and the link.
func chatgptLoginStart() (id, authURL string, err error) {
	var out chatgptLoginReply
	err = chatgptPost(map[string]any{"action": "login"}, &out)
	return out.ID, out.AuthURL, err
}

// errSignInAgain is the gateway's "registered; approve once more": the
// caller starts a fresh sign-in, now under the issued client id.
var errSignInAgain = fmt.Errorf("sign in again")

// chatgptLoginWait waits up to 25 s: done with the probe's result, or not yet.
func chatgptLoginWait(id string) (done bool, res ui.ConnectResult, err error) {
	var out struct {
		ui.ConnectResult
		Done    bool   `json:"done"`
		Pending bool   `json:"pending"`
		Err     string `json:"error"`
		Again   bool   `json:"again"`
	}
	if err := chatgptPost(map[string]any{"action": "login/wait", "id": id}, &out); err != nil {
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

func chatgptLoginPaste(id, value string) error {
	return chatgptPost(map[string]any{"action": "login/paste", "id": id, "value": value}, nil)
}

func chatgptLoginCancel(id string) {
	_ = chatgptPost(map[string]any{"action": "login/cancel", "id": id}, nil)
}

// tuiConnectLoginStart is the window's start of a browser sign-in.
func tuiConnectLoginStart(kind string) (id, u string, opened, copied bool, err error) {
	if kind != "chatgpt" {
		return "", "", false, false, fmt.Errorf("no browser sign-in for %s", kind)
	}
	id, u, err = chatgptLoginStart()
	if err != nil {
		return "", "", false, false, err
	}
	opened = os.Getenv("MEMDOOR_NO_BROWSER") == "" && openBrowser(u) == nil
	copied = clipboard.WriteAll(u) == nil
	return id, u, opened, copied, nil
}

// chatgptSignIn runs the sign-in from the shell: the browser opens, the link
// is printed and copied, a pasted redirect URL is accepted, Ctrl+C cancels.
func chatgptSignIn(out io.Writer, in io.Reader) error {
	err := chatgptSignInOnce(out, in, "Sign in with ChatGPT")
	if errors.Is(err, errSignInAgain) {
		// A first registration: the host now has its client id; the
		// second approval, under it, completes (pkg/chatgpt ErrSignInAgain).
		return chatgptSignInOnce(out, in, "Memdoor is now registered with your ChatGPT — one more approval completes the sign-in")
	}
	return err
}

func chatgptSignInOnce(out io.Writer, in io.Reader, title string) error {
	id, authURL, err := chatgptLoginStart()
	if err != nil {
		return err
	}
	copied := clipboard.WriteAll(authURL) == nil
	fmt.Fprintln(out, "\n"+title)
	if os.Getenv("MEMDOOR_NO_BROWSER") == "" && openBrowser(authURL) == nil {
		fmt.Fprintln(out, "  Your browser is opening. If it doesn't, open this link:")
	} else {
		fmt.Fprintln(out, "  Open this link to approve:")
	}
	fmt.Fprintf(out, "  %s\n", authURL)
	if copied {
		fmt.Fprintln(out, "  (copied to the clipboard)")
	}
	fmt.Fprintln(out, "  Waiting for you to approve (5 minutes). If the browser is on another machine, paste the page's address here. Ctrl+C cancels.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)
	if in != nil {
		go func() {
			sc := bufio.NewScanner(in)
			for sc.Scan() {
				if v := strings.TrimSpace(sc.Text()); v != "" {
					if err := chatgptLoginPaste(id, v); err != nil {
						fmt.Fprintln(out, "  ✗ "+err.Error())
					}
				}
			}
		}()
	}
	type result struct {
		res ui.ConnectResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		deadline := time.Now().Add(6 * time.Minute)
		for time.Now().Before(deadline) {
			ok, res, err := chatgptLoginWait(id)
			if err != nil || ok {
				done <- result{res, err}
				return
			}
		}
		done <- result{err: fmt.Errorf("no sign-in within 6 minutes")}
	}()
	select {
	case <-stop:
		chatgptLoginCancel(id)
		return fmt.Errorf("sign-in cancelled")
	case r := <-done:
		if r.err != nil {
			return r.err
		}
		if !r.res.OK {
			fmt.Fprintf(out, "✗ signed in, but the plan could not be used: %s\n", r.res.Error)
			return fmt.Errorf("%s", r.res.Error)
		}
		fmt.Fprintf(out, "✓ ChatGPT plan connected · %d models · tested %s → %q\n", r.res.Models, r.res.Tested, r.res.Answer)
		if len(r.res.Sample) > 0 {
			fmt.Fprintf(out, "  e.g. %s\n", strings.Join(r.res.Sample, ", "))
		}
		fmt.Fprintln(out, "  Your plan's allowance answers Memdoor's turns, no API key. ChatGPT → Settings → Usage shows the weekly cap per app.")
		fmt.Fprintln(out, "  memdoor tui — /model chatgpt:<id> pins one of its models (the chatgpt: prefix: an OpenAI key lists the same ids); memdoor connect --remove chatgpt signs out.")
		return nil
	}
}

// alsoListedBy says which OTHER connected providers list a bare model id the
// person just pinned, and how to pin it there. A bare id goes to the active
// provider first (registry.go FindModel): live 2026-10-10, `/model
// gpt-5.6-sol` after Sign in with ChatGPT went to the OpenAI API key in the
// environment, whose chat endpoint refuses that model with tools, when the
// plan lists the same id for nothing. "" when the id is unambiguous, or
// when it was pinned with a provider prefix already.
func alsoListedBy(pinned, served string) string {
	if strings.Contains(pinned, ":") || served == "" {
		return ""
	}
	var res struct {
		Providers []struct {
			ID        string `json:"id"`
			Connected bool   `json:"connected"`
			Models    []struct {
				ID string `json:"id"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := NewClient().GetJSON("/api/models?limit=0", &res); err != nil {
		return ""
	}
	var others []string
	for _, p := range res.Providers {
		if p.ID == served || !p.Connected {
			continue
		}
		for _, m := range p.Models {
			if m.ID == pinned {
				others = append(others, fmt.Sprintf("/model %s:%s", p.ID, pinned))
				break
			}
		}
	}
	if len(others) == 0 {
		return ""
	}
	return fmt.Sprintf("Served by %s. Also listed elsewhere: %s.", served, strings.Join(others, " · "))
}
