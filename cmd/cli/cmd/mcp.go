package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"memdoor/cmd/tui/ui"
)

// memdoor mcp: the person's MCP servers, from the shell. The gateway runs
// them and holds the sign-ins (/api/mcp); the TUI's /mcp uses the same
// calls (mcpStatusLines, mcpAddServers, mcpSignIn).

// mcpServerStatus is one server as /api/mcp reports it.
type mcpServerStatus struct {
	Name      string   `json:"name"`
	Scope     string   `json:"scope"`
	Transport string   `json:"transport"`
	Target    string   `json:"target"`
	State     string   `json:"state"`
	Error     string   `json:"error"`
	Tools     []string `json:"tools"`
	Server    string   `json:"server"`
	SignedIn  bool     `json:"signed_in"`
	Saved     bool     `json:"saved"`
}

type mcpPanel struct {
	Servers []mcpServerStatus `json:"servers"`
	Trusted bool              `json:"trusted"`
}

// mcpDir is the project the command is about: where it runs.
func mcpDir() (string, error) {
	return os.Getwd()
}

func mcpList(dir string) (mcpPanel, error) {
	var p mcpPanel
	err := NewClient().GetJSON("/api/mcp?dir="+url.QueryEscape(dir), &p)
	return p, err
}

func mcpPost(action string, body map[string]interface{}, v interface{}) error {
	return NewClient().PostJSON("/api/mcp/"+action, body, v)
}

// mcpStateGlyph is a state at a glance.
func mcpStateGlyph(state string) string {
	switch state {
	case "connected":
		return "●"
	case "needs-sign-in":
		return "◐"
	case "failed":
		return "✗"
	case "off":
		return "○"
	case "not-trusted":
		return "?"
	case "connecting":
		return "◌"
	}
	return "·"
}

// mcpStateWords is a state in plain words.
func mcpStateWords(s mcpServerStatus) string {
	switch s.State {
	case "connected":
		n := len(s.Tools)
		return fmt.Sprintf("%d tool%s", n, map[bool]string{true: "", false: "s"}[n == 1])
	case "needs-sign-in":
		return "needs sign-in"
	case "not-trusted":
		return "waiting for your yes (this project's file)"
	case "idle":
		return "starts on first use"
	}
	return s.State
}

// mcpStatusLines is the panel as text: one line per server, the reason
// under the ones that need something.
// prefix is how commands are typed where the lines are shown: "/mcp" in
// the TUI, "memdoor mcp" in a shell.
func mcpStatusLines(p mcpPanel, prefix string) []string {
	if len(p.Servers) == 0 {
		return []string{"No MCP servers yet. Add one: " + prefix + " add <URL, command, or .mcp.json snippet>"}
	}
	w := 0
	for _, s := range p.Servers {
		if len(s.Name) > w {
			w = len(s.Name)
		}
	}
	var out []string
	for _, s := range p.Servers {
		out = append(out, fmt.Sprintf("%s %-*s  %-5s  %-7s  %s", mcpStateGlyph(s.State), w, s.Name, s.Transport, s.Scope, mcpStateWords(s)))
		switch s.State {
		case "failed":
			out = append(out, "    "+mcpFixHint(s))
		case "needs-sign-in":
			out = append(out, "    sign in: "+prefix+" login "+s.Name)
		}
	}
	if !p.Trusted {
		out = append(out, "", "This project's .mcp.json servers start after your yes: "+prefix+" trust")
	}
	return out
}

// mcpFixHint is the error with what to do about it.
func mcpFixHint(s mcpServerStatus) string {
	e := s.Error
	if s.State == "failed" && !s.Saved {
		// Not in any file: there is nothing to edit, only something to paste.
		return e + " — not saved. Fix it and add it again."
	}
	switch {
	case strings.Contains(e, "command not found"):
		return e + " — install it, or fix the command in " + mcpFileHint(s.Scope)
	case strings.Contains(e, "needs") && strings.Contains(e, "set in the environment"):
		return e + " — set it where the gateway runs, then /mcp test " + s.Name
	case strings.Contains(e, "connection refused"):
		return e + " — is the server running?"
	case strings.Contains(e, "HTTP 404"):
		return e + " — check the URL (MCP endpoints usually end in /mcp)"
	}
	return e
}

func mcpFileHint(scope string) string {
	if scope == "user" {
		return "~/.memdoor/mcp.json"
	}
	return ".mcp.json"
}

// mcpAddServers adds what the person gave and tests it; it returns the
// servers' states (a needs-sign-in one is signed in next).
func mcpAddServers(dir, input, name string, everywhere bool) ([]mcpServerStatus, error) {
	scope := "project"
	if everywhere {
		scope = "user"
	}
	var out struct {
		Servers []mcpServerStatus `json:"servers"`
	}
	err := mcpPost("add", map[string]interface{}{"dir": dir, "input": input, "name": name, "scope": scope}, &out)
	return out.Servers, err
}

// mcpLoginStart begins a sign-in: its id and the URL to open.
func mcpLoginStart(dir, name string) (id, authURL string, err error) {
	var out struct {
		ID      string `json:"id"`
		AuthURL string `json:"auth_url"`
	}
	err = mcpPost("login", map[string]interface{}{"dir": dir, "name": name}, &out)
	return out.ID, out.AuthURL, err
}

// mcpLoginWait waits up to 25 s for the sign-in: done with the server's
// state, still pending, or failed.
func mcpLoginWait(id string) (done bool, st mcpServerStatus, err error) {
	var out struct {
		Done    bool            `json:"done"`
		Pending bool            `json:"pending"`
		Error   string          `json:"error"`
		Server  mcpServerStatus `json:"server"`
	}
	if err := mcpPost("login/wait", map[string]interface{}{"id": id}, &out); err != nil {
		return false, st, err
	}
	if out.Error != "" {
		return false, st, fmt.Errorf("%s", out.Error)
	}
	return out.Done, out.Server, nil
}

// mcpSignIn runs a sign-in from the shell: the browser opens, the link is
// printed and copied, a pasted redirect URL is accepted, Ctrl+C cancels.
func mcpSignIn(dir, name string, out io.Writer, in io.Reader) (mcpServerStatus, error) {
	id, authURL, err := mcpLoginStart(dir, name)
	if err != nil {
		return mcpServerStatus{}, err
	}
	copied := clipboard.WriteAll(authURL) == nil
	fmt.Fprintf(out, "\nSign in to %s\n", name)
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
					if err := mcpPost("login/paste", map[string]interface{}{"id": id, "value": v}, nil); err != nil {
						fmt.Fprintln(out, "  ✗ "+err.Error())
					}
				}
			}
		}()
	}
	type result struct {
		st  mcpServerStatus
		err error
	}
	res := make(chan result, 1)
	go func() {
		deadline := time.Now().Add(6 * time.Minute)
		for time.Now().Before(deadline) {
			done, st, err := mcpLoginWait(id)
			if err != nil || done {
				res <- result{st, err}
				return
			}
		}
		res <- result{err: fmt.Errorf("no sign-in in time")}
	}()
	select {
	case r := <-res:
		return r.st, r.err
	case <-stop:
		_ = mcpPost("login/cancel", map[string]interface{}{"id": id}, nil)
		return mcpServerStatus{}, fmt.Errorf("sign-in cancelled")
	}
}

// mcpRegistryResult is a server found in the official MCP registry.
type mcpRegistryResult struct {
	Name        string `json:"name"`
	Suggested   string `json:"suggested"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Via         string `json:"via"`
	Needs       []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"needs"`
	Entry json.RawMessage `json:"entry"`
}

// snippet is the result as /mcp add takes it: {"<name>": <entry>}.
func (r mcpRegistryResult) snippet() string {
	b, _ := json.Marshal(map[string]json.RawMessage{r.Suggested: r.Entry})
	return string(b)
}

func (r mcpRegistryResult) needs() []string {
	var n []string
	for _, v := range r.Needs {
		n = append(n, v.Name)
	}
	return n
}

func mcpSearch(dir, q string) ([]mcpRegistryResult, error) {
	var out struct {
		Servers []mcpRegistryResult `json:"servers"`
	}
	err := NewClient().GetJSON("/api/mcp/search?dir="+url.QueryEscape(dir)+"&q="+url.QueryEscape(q), &out)
	return out.Servers, err
}

// mcpResultLine is a server's state after an add, a test or a sign-in.
func mcpResultLine(s mcpServerStatus) string {
	switch s.State {
	case "connected":
		tools := strings.Join(s.Tools, ", ")
		if len(s.Tools) > 6 {
			tools = strings.Join(s.Tools[:6], ", ") + fmt.Sprintf(", … %d more", len(s.Tools)-6)
		}
		srv := ""
		if s.Server != "" {
			srv = " (" + s.Server + ")"
		}
		return fmt.Sprintf("✓ %s connected%s: %d tools — %s", s.Name, srv, len(s.Tools), tools)
	case "needs-sign-in":
		return fmt.Sprintf("◐ %s needs sign-in", s.Name)
	case "failed":
		return fmt.Sprintf("✗ %s: %s", s.Name, mcpFixHint(s))
	}
	return fmt.Sprintf("%s %s: %s", mcpStateGlyph(s.State), s.Name, mcpStateWords(s))
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP servers: add one, sign in, see their state",
	Long: `The MCP servers the coder can use, for the project in this directory.

  memdoor mcp                       every server and its state
  memdoor mcp add <anything>        a URL, a command line, or a .mcp.json snippet;
                                    tested at once, and signed in to if it asks
      --name <name>                 instead of the suggested one
      --everywhere                  ~/.memdoor/mcp.json (every project), not .mcp.json
      --registry <name>             a server from the MCP registry, by its name
  memdoor mcp search <words>        find servers in the official MCP registry
  memdoor mcp login <name>          sign in (OAuth, in your browser)
  memdoor mcp logout <name>         forget the sign-in
  memdoor mcp test <name>           connect now and list its tools
  memdoor mcp on|off <name>         turn a server on or off
  memdoor mcp remove <name>         delete it from its file
  memdoor mcp trust                 let this project's .mcp.json servers start

In the TUI, /mcp does the same.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := mcpDir()
		if err != nil {
			return err
		}
		p, err := mcpList(dir)
		if err != nil {
			return err
		}
		for _, l := range mcpStatusLines(p, "memdoor mcp") {
			fmt.Println(l)
		}
		return nil
	},
}

var (
	mcpAddName       string
	mcpAddEverywhere bool
	mcpAddRegistry   string
)

var mcpSearchCmd = &cobra.Command{
	Use:   "search <words>",
	Short: "Find servers in the official MCP registry",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := mcpDir()
		if err != nil {
			return err
		}
		found, err := mcpSearch(dir, strings.Join(args, " "))
		if err != nil {
			return err
		}
		if len(found) == 0 {
			fmt.Println("Nothing in the MCP registry matches.")
			return nil
		}
		for i, r := range found {
			fmt.Printf("%2d. %s  (%s %s, %s)\n", i+1, r.Suggested, r.Name, r.Version, r.Via)
			if r.Description != "" {
				fmt.Printf("    %s\n", r.Description)
			}
			if n := r.needs(); len(n) > 0 {
				fmt.Printf("    needs %s set where the gateway runs\n", strings.Join(n, ", "))
			}
			fmt.Printf("    memdoor mcp add --registry %s\n", r.Name)
		}
		return nil
	},
}

var mcpAddCmd = &cobra.Command{
	Use:   "add <URL | command | .mcp.json snippet>",
	Short: "Add a server; it is tested at once, and signed in to if it asks",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && mcpAddRegistry == "" {
			return fmt.Errorf("give a URL, a command, a .mcp.json snippet, or --registry <name>")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := mcpDir()
		if err != nil {
			return err
		}
		name, everywhere := mcpAddName, mcpAddEverywhere
		if mcpAddRegistry != "" {
			found, err := mcpSearch(dir, mcpAddRegistry)
			if err != nil {
				return err
			}
			for _, r := range found {
				if r.Name == mcpAddRegistry {
					args = []string{r.snippet()}
					if name != "" {
						args = []string{string(must(json.Marshal(map[string]json.RawMessage{name: r.Entry})))}
						name = ""
					}
					break
				}
			}
			if len(args) == 0 {
				return fmt.Errorf("no server named %q in the MCP registry (memdoor mcp search <words>)", mcpAddRegistry)
			}
		}
		// A URL takes no arguments, so after one our own flags may follow
		// it (`add https://… --name dw`); after a command they are the
		// command's (`docker run --name x`).
		if strings.HasPrefix(args[0], "http://") || strings.HasPrefix(args[0], "https://") {
			rest := args[1:]
			args = args[:1]
			for i := 0; i < len(rest); i++ {
				switch a := rest[i]; {
				case a == "--everywhere":
					everywhere = true
				case a == "--name" && i+1 < len(rest):
					name, i = rest[i+1], i+1
				case strings.HasPrefix(a, "--name="):
					name = strings.TrimPrefix(a, "--name=")
				default:
					return fmt.Errorf("after a URL only --name and --everywhere: %q", a)
				}
			}
		}
		input := strings.Join(args, " ")
		if len(args) > 1 {
			input = shellJoin(args)
		}
		fmt.Println("Connecting…")
		sts, err := mcpAddServers(dir, input, name, everywhere)
		if err != nil {
			return err
		}
		for _, s := range sts {
			fmt.Println(mcpResultLine(s))
			if s.State == "needs-sign-in" {
				st, err := mcpSignIn(dir, s.Name, os.Stdout, os.Stdin)
				if err != nil {
					fmt.Println("✗ " + err.Error())
					fmt.Println("  Saved. Sign in later: memdoor mcp login " + s.Name)
					continue
				}
				fmt.Println(mcpResultLine(st))
			}
		}
		return nil
	},
}

// shellJoin quotes args that a shell split, so the command line survives.
func shellJoin(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t'\"\\") {
			q[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		} else {
			q[i] = a
		}
	}
	return strings.Join(q, " ")
}

func mcpNamed(use, short string, run func(dir, name string) error) *cobra.Command {
	return &cobra.Command{Use: use + " <name>", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := mcpDir()
			if err != nil {
				return err
			}
			return run(dir, args[0])
		}}
}

func init() {
	// Flags end at the input: `memdoor mcp add npx -y pkg` passes -y to npx.
	mcpAddCmd.Flags().SetInterspersed(false)
	mcpAddCmd.Flags().StringVar(&mcpAddName, "name", "", "the server's name, instead of the suggested one")
	mcpAddCmd.Flags().BoolVar(&mcpAddEverywhere, "everywhere", false, "add it to ~/.memdoor/mcp.json, for every project")
	mcpAddCmd.Flags().StringVar(&mcpAddRegistry, "registry", "", "a server's name in the MCP registry (from memdoor mcp search)")
	mcpCmd.AddCommand(mcpAddCmd, mcpSearchCmd,
		mcpNamed("login", "Sign in to a server (OAuth, in your browser)", func(dir, name string) error {
			st, err := mcpSignIn(dir, name, os.Stdout, os.Stdin)
			if err != nil {
				return err
			}
			fmt.Println(mcpResultLine(st))
			return nil
		}),
		mcpNamed("logout", "Forget a server's sign-in", func(dir, name string) error {
			if err := mcpPost("logout", map[string]interface{}{"dir": dir, "name": name}, nil); err != nil {
				return err
			}
			fmt.Println("Signed out of " + name)
			return nil
		}),
		mcpNamed("test", "Connect a server now and list its tools", func(dir, name string) error {
			var st mcpServerStatus
			if err := mcpPost("test", map[string]interface{}{"dir": dir, "name": name}, &st); err != nil {
				return err
			}
			fmt.Println(mcpResultLine(st))
			return nil
		}),
		mcpNamed("on", "Turn a server on", func(dir, name string) error {
			return mcpPost("enable", map[string]interface{}{"dir": dir, "name": name, "on": true}, nil)
		}),
		mcpNamed("off", "Turn a server off", func(dir, name string) error {
			return mcpPost("enable", map[string]interface{}{"dir": dir, "name": name, "on": false}, nil)
		}),
		mcpNamed("remove", "Delete a server from its file", func(dir, name string) error {
			var out struct {
				From string `json:"from"`
			}
			if err := mcpPost("remove", map[string]interface{}{"dir": dir, "name": name}, &out); err != nil {
				return err
			}
			fmt.Printf("Removed %s from %s\n", name, out.From)
			return nil
		}),
		&cobra.Command{Use: "trust", Short: "Let this project's .mcp.json servers start", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				dir, err := mcpDir()
				if err != nil {
					return err
				}
				p, err := mcpList(dir)
				if err != nil {
					return err
				}
				fmt.Println("This project's .mcp.json starts:")
				for _, s := range p.Servers {
					if s.Scope == "project" {
						fmt.Printf("  %s  %s\n", s.Name, s.Target)
					}
				}
				if err := mcpPost("trust", map[string]interface{}{"dir": dir}, nil); err != nil {
					return err
				}
				fmt.Println("Trusted. They start on the next turn; a change to them asks again.")
				return nil
			}},
	)
	rootCmd.AddCommand(mcpCmd)
}

// tuiMCPOps are the TUI's /mcp calls, for the directory the TUI works in.
// The sign-in opens the browser and copies the link here, on the person's
// machine; the gateway waits for the code.
func tuiMCPOps(dir func() string) ui.MCPOps {
	toUI := func(s mcpServerStatus) ui.MCPServer {
		return ui.MCPServer{Name: s.Name, Scope: s.Scope, Transport: s.Transport, Target: s.Target,
			State: s.State, Error: mcpFixHint(s), Tools: s.Tools, Server: s.Server, Saved: s.Saved}
	}
	return ui.MCPOps{
		List: func() (ui.MCPPanel, error) {
			p, err := mcpList(dir())
			out := ui.MCPPanel{Trusted: p.Trusted}
			for _, s := range p.Servers {
				out.Servers = append(out.Servers, toUI(s))
			}
			return out, err
		},
		Add: func(input string, everywhere bool) ([]ui.MCPServer, error) {
			sts, err := mcpAddServers(dir(), input, "", everywhere)
			var out []ui.MCPServer
			for _, s := range sts {
				out = append(out, toUI(s))
			}
			return out, err
		},
		Action: func(action, name string) (ui.MCPServer, error) {
			body := map[string]interface{}{"dir": dir(), "name": name}
			switch action {
			case "test":
				var st mcpServerStatus
				err := mcpPost("test", body, &st)
				return toUI(st), err
			case "on", "off":
				body["on"] = action == "on"
				return ui.MCPServer{Name: name}, mcpPost("enable", body, nil)
			}
			return ui.MCPServer{Name: name}, mcpPost(action, body, nil)
		},
		LoginStart: func(name string) (string, string, bool, bool, error) {
			id, u, err := mcpLoginStart(dir(), name)
			if err != nil {
				return "", "", false, false, err
			}
			opened := os.Getenv("MEMDOOR_NO_BROWSER") == "" && openBrowser(u) == nil
			copied := clipboard.WriteAll(u) == nil
			return id, u, opened, copied, nil
		},
		LoginWait: func(id string) (bool, ui.MCPServer, error) {
			done, st, err := mcpLoginWait(id)
			return done, toUI(st), err
		},
		LoginPaste: func(id, value string) error {
			return mcpPost("login/paste", map[string]interface{}{"id": id, "value": value}, nil)
		},
		LoginCancel: func(id string) {
			_ = mcpPost("login/cancel", map[string]interface{}{"id": id}, nil)
		},
		Prompts: func() ([]ui.MCPPrompt, error) {
			var out struct {
				Prompts []struct {
					Server      string `json:"server"`
					Name        string `json:"name"`
					Description string `json:"description"`
					Arguments   []struct {
						Name        string `json:"name"`
						Description string `json:"description"`
						Required    bool   `json:"required"`
					} `json:"arguments"`
				} `json:"prompts"`
			}
			if err := NewClient().GetJSON("/api/mcp/prompts?dir="+url.QueryEscape(dir()), &out); err != nil {
				return nil, err
			}
			var list []ui.MCPPrompt
			for _, p := range out.Prompts {
				mp := ui.MCPPrompt{Server: p.Server, Name: p.Name, Description: p.Description}
				for _, a := range p.Arguments {
					mp.Args = append(mp.Args, ui.MCPPromptArg{Name: a.Name, Description: a.Description, Required: a.Required})
				}
				list = append(list, mp)
			}
			return list, nil
		},
		Search: func(q string) ([]ui.MCPRegistryServer, error) {
			found, err := mcpSearch(dir(), q)
			var out []ui.MCPRegistryServer
			for _, r := range found {
				out = append(out, ui.MCPRegistryServer{Name: r.Name, Suggested: r.Suggested, Description: r.Description,
					Version: r.Version, Via: r.Via, Needs: r.needs(), Snippet: r.snippet()})
			}
			return out, err
		},
		GetPrompt: func(server, name string, args map[string]string) (string, error) {
			b, _ := json.Marshal(map[string]interface{}{"server": server, "args": args})
			var out struct {
				Text string `json:"text"`
			}
			err := mcpPost("prompt", map[string]interface{}{"dir": dir(), "name": name, "value": string(b)}, &out)
			return out.Text, err
		},
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
