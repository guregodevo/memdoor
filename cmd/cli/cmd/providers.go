package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"memdoor/cmd/tui/ui"
	"memdoor/gateway/providers"
	"memdoor/pkg/modelkey"
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
			// maintenance number (the maintainer, 2026-10-02: "very important to
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
With no kind, a key already on this machine comes first: one in this shell
under any variable name, or in the project's .env, .env.local or .envrc, found
by its shape (sk-or-, sk-ant-, AIza, AQ., …) or, for a key with no shape, under
its provider's own name (BASETEN_API_KEY); Enter connects it. The keys the
gateway already reads from its environment are named, not offered again. A
shell variable is kept by its NAME when the gateway can read it, else by value.
With none, it asks before looking in the keys other coding tools saved (Codex,
Claude Code, Hermes, Goose, …). Then the sign-ins, no key needed:
openrouter-login signs in with OpenRouter in the browser (free account;
OpenRouter makes a key for Memdoor), chatgpt with a plan; then the kinds that
take a key.
The token is read without
echo; it can be the value, the NAME of an environment variable that holds it,
or !command that prints it (Keychain, 1Password, Vault). A local runtime
(memdoor connect local: Ollama, llama.cpp, janis, LM Studio on this machine or
the box) has no token and is asked for none. Before anything is
kept, the gateway reads the provider's model list and makes one tiny call;
a wrong URL or token fails here, in words. Kept in ~/.memdoor/providers.json
(0600). --probe tests without keeping. The gateway starts if none is running.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// A fresh machine (a CI runner) connects before anything else ran.
		if err := firstRunReady(); err != nil {
			return err
		}
		if rm, _ := cmd.Flags().GetString("remove"); rm != "" {
			var res map[string]any
			if rm == "chatgpt" {
				if err := signInPost(ui.KindChatGPT, map[string]any{"action": "logout"}, &res); err != nil {
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
			// A key already on this machine comes first: Enter connects it.
			var found []modelkey.Found
			if probeOnly, _ := cmd.Flags().GetBool("probe"); !probeOnly {
				found = foundKeysOrAsk(in, os.Stdout)
			}
			found, inUse := splitInUse(found)
			menu := connectMenu(found, inUse, os.Stdout)
			n := ask("Number", "1")
			i, err := strconv.Atoi(n)
			if err != nil || i < 1 || i > len(menu) {
				return fmt.Errorf("pick a number from the list")
			}
			if entry := menu[i-1]; entry.found != nil {
				fmt.Println("Probing …")
				res, err := connectFound(*entry.found)
				if err != nil {
					return err
				}
				if !res.OK {
					fmt.Printf("✗ not connected: %s\n", res.Error)
					if res.Advice != "" {
						fmt.Printf("  %s\n", res.Advice)
					}
					os.Exit(1)
				}
				printConnected(os.Stdout, res)
				return nil
			}
			pick = menu[i-1].kind
		}
		if signsIn(pick.ID) {
			return providerSignIn(pick.ID, os.Stdout, in, true)
		}
		// `memdoor connect openrouter` with no key anywhere: a newcomer has
		// none, so signing in comes first (the maintainer, 2026-10-31).
		if flagKey, _ := cmd.Flags().GetString("key"); pick.ID == providers.OpenRouterID && flagKey == "" &&
			term.IsTerminal(int(os.Stdin.Fd())) && providerKeySource(pick.ID) == "" {
			fmt.Println("  1. Sign in with OpenRouter in the browser (no key to copy; sign up is free)")
			fmt.Println("  2. Paste a key")
			if ask("Number", "1") == "1" {
				return providerSignIn(ui.KindOpenRouterLogin, os.Stdout, in, true)
			}
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
			// `echo $KEY | memdoor connect anthropic`: the piped line is the
			// key, and the kind's base stands (asked for, the key was read
			// as the base URL and refused, 2026-10-21).
			if !term.IsTerminal(int(os.Stdin.Fd())) && pick.Base != "" {
				base = pick.Base
			} else {
				if pick.Keyless && term.IsTerminal(int(os.Stdin.Fd())) {
					var err error
					if base, err = chooseLocalRuntime(ask, func(t string) { fmt.Print(t) }); err != nil {
						return err
					}
				} else {
					prompt := "Base URL"
					if pick.Keyless {
						prompt = "Base URL (blank: find a runtime on the gateway's machine — janis :8090, Ollama :11434, llama.cpp :8080, LM Studio :1234, vLLM :8000)"
					}
					base = ask(prompt, pick.Base)
				}
			}
		}
		key, _ := cmd.Flags().GetString("key")
		if key == "" && !pick.Keyless {
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
			OK        bool     `json:"ok"`
			ID        string   `json:"id"`
			API       string   `json:"api"`
			Models    int      `json:"models"`
			Sample    []string `json:"sample"`
			Tested    string   `json:"tested"`
			Answer    string   `json:"answer"`
			Error     string   `json:"error"`
			Advice    string   `json:"advice"`
			Saved     bool     `json:"saved"`
			Decisions bool     `json:"decisions"`
			Base      string   `json:"base"`
			FreeModel string   `json:"free_model"`
		}
		err := NewClient().PostJSON("/api/providers/connect", map[string]any{
			"id": id, "name": pick.Name, "api": pick.API, "base": base, "key": strings.TrimSpace(key), "model": model, "probe": probe, "keyless": pick.Keyless,
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
		if res.Base != "" {
			fmt.Printf("  found at %s\n", res.Base)
		}
		fmt.Printf("✓ %s connected · %s API · %d models · tested %s → %q · %s\n", res.ID, res.API, res.Models, res.Tested, res.Answer, kept)
		if len(res.Sample) > 0 {
			fmt.Printf("  e.g. %s\n", strings.Join(res.Sample, ", "))
		}
		if res.FreeModel != "" {
			fmt.Println("  " + ui.FreeModelNote(res.FreeModel))
		}
		fmt.Println("  /model <id> in the window pins one of its models; memdoor model search <name> finds them.")
		if res.Decisions {
			fmt.Printf("  it judges too: kept as %s-decisions, the decision model runs here with no key.\n", res.ID)
		}
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
