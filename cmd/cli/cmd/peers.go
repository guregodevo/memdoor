package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"memdoor/gateway"
	"memdoor/gateway/client"
	"memdoor/gateway/logs"

	"github.com/spf13/cobra"
)

// PEERS, EXPERIMENTAL (2026-10-24, gateway/peers.go): other gateways this
// person has signed in to, by name. `@<peer>:<agent>` in any conversation
// here runs the turn on that gateway and brings the answer back. The ask
// is the door `memdoor --gateway <peer> run` already opens, with the
// sign-in it already has (credentials.json, per gateway): peerAsk below is
// what the gateway's own process calls (gateway.SetPeerCaller).

var peerAddCmd = &cobra.Command{
	Use:   "add <name> <gateway>",
	Short: "Name a gateway you are signed in to; @<name>:<agent> reaches it",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		if name == "" || strings.ContainsAny(name, ":@/ ") {
			return fmt.Errorf("a peer's name is a word: letters, digits, dots and dashes")
		}
		gw := canonicalGatewayAddr(args[1])
		c, err := peerClient(gw)
		if err != nil {
			return err
		}
		agents, version, err := peerProbe(c)
		if err != nil {
			return fmt.Errorf("%s does not answer as you: %v", gw, err)
		}
		peers, err := gateway.LoadPeers()
		if err != nil {
			return err
		}
		dir, _ := cmd.Flags().GetString("dir")
		peers[name] = gateway.PeerEntry{Gateway: gw, Dir: strings.TrimSpace(dir)}
		if err := gateway.SavePeers(peers); err != nil {
			return err
		}
		fmt.Printf("%s → %s (%s) · agents: %s\n", name, gw, version, strings.Join(agents, ", "))
		if len(agents) > 0 {
			fmt.Printf("  @%s:%s <what> in any conversation here asks it\n", name, agents[0])
		}
		return nil
	},
}

var peerRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Forget a peer (the sign-in stays)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		peers, err := gateway.LoadPeers()
		if err != nil {
			return err
		}
		name := strings.ToLower(strings.TrimSpace(args[0]))
		if _, ok := peers[name]; !ok {
			return fmt.Errorf("no peer named %s", name)
		}
		delete(peers, name)
		return gateway.SavePeers(peers)
	},
}

var peersCmd = &cobra.Command{
	Use:   "peers",
	Short: "Experimental: other gateways you can ask by name, @<peer>:<agent>",
	Long: `memdoor peers                                              each peer: answering or not, its agents
memdoor peers add <name> <gateway> [--dir <path there>]    name a gateway you are signed in to
memdoor peers remove <name>

Experimental. A peer is a gateway you have signed in to once (memdoor --gateway
<host> auth login-direct …, or the ssh door, which signs you in on the way). From
then on "@<name>:<agent> …" in a window, a run or a message here sends that
turn to the agent on the peer, in the directory --dir names there, and its
answer comes back as the reply; a name in that reply wakes an agent here. The
run on the peer is a conversation there: memdoor --gateway <host> resume <id>
opens it.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		peers, err := gateway.LoadPeers()
		if err != nil {
			return err
		}
		if len(peers) == 0 {
			fmt.Println("no peers — memdoor peers add <name> <gateway> names a gateway you are signed in to")
			return nil
		}
		names := make([]string, 0, len(peers))
		for n := range peers {
			names = append(names, n)
		}
		sort.Strings(names)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PEER\tGATEWAY\tSTATE\tAGENTS\tDIR")
		for _, n := range names {
			p := peers[n]
			state, agents := "not answering", ""
			if c, err := peerClient(p.Gateway); err != nil {
				state = "no session: " + err.Error()
			} else if names, version, err := peerProbe(c); err == nil {
				state, agents = "answering "+version, strings.Join(names, ", ")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", n, p.Gateway, state, agents, p.Dir)
		}
		return w.Flush()
	},
}

// peerClient is a client for a peer gateway with the session this person
// has for it, the one `memdoor --gateway <peer>` would use.
func peerClient(gw string) (*Client, error) {
	st, err := loadCredentialStore()
	if err != nil {
		return nil, fmt.Errorf("not signed in to %s: memdoor --gateway %s auth login-direct --email … --password …, or reach it once through ssh", gw, gw)
	}
	creds, ok := st.Gateways[normalizeGateway(gw)]
	if !ok || creds.Token == "" {
		return nil, fmt.Errorf("not signed in to %s: memdoor --gateway %s auth login-direct --email … --password …, or reach it once through ssh", gw, gw)
	}
	return &Client{BaseURL: gw, HTTPClient: &http.Client{Timeout: 30 * time.Second}, token: creds.Token}, nil
}

// peerProbe asks a peer who it is: the agents it has and its version.
func peerProbe(c *Client) (agents []string, version string, err error) {
	var st struct {
		Version string `json:"version"`
	}
	if err := c.GetJSON("/api/status", &st); err != nil {
		return nil, "", err
	}
	var who map[string]any
	if err := c.GetJSON("/api/auth/whoami", &who); err != nil {
		return nil, "", fmt.Errorf("the session for it has lapsed: %w", err)
	}
	var result struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := c.GetJSON("/api/agents", &result); err == nil {
		for _, a := range result.Agents {
			if n, _ := a["name"].(string); n != "" {
				agents = append(agents, n)
			}
		}
		sort.Strings(agents)
	}
	return agents, st.Version, nil
}

// peerWorkspace is the peer's one workspace, the one its runs belong to.
func peerWorkspace(c *Client) string {
	var st struct {
		Slug string `json:"workspace_slug"`
	}
	if err := c.GetJSON("/api/setup/status", &st); err != nil {
		return ""
	}
	return strings.TrimSpace(st.Slug)
}

// peerAsk is one turn of an agent on a peer, as the gateway's process asks
// it: a conversation there, the text posted to that agent, the answer
// collected to its end. A question the agent asks there has nobody to
// answer it: the first option is taken and said.
func peerAsk(ctx context.Context, gw, agent, dir, text string, onText func(string)) (string, error) {
	c, err := peerClient(gw)
	if err != nil {
		return "", err
	}
	c.workspaceSlug = peerWorkspace(c)
	if c.workspaceSlug == "" {
		return "", fmt.Errorf("%s names no workspace for a run", gw)
	}
	if strings.TrimSpace(agent) == "" {
		agent = acpAgentName
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	runner := &gatewayRunner{c: c, workspace: c.workspaceSlug, agent: agent, addr: gw, asPeer: true, feeds: map[string]*client.Client{}}
	log := logs.New("Peers")
	conv, err := runner.NewConversation(ctx, dir)
	if err != nil {
		log.Warn("Peer ask: no conversation", slog.String("gateway", gw), slog.String("error", err.Error()))
		return "", err
	}
	log.Info("Peer ask: conversation opened", slog.String("gateway", gw), slog.String("agent", agent), slog.String("conversation", conv))
	sink := &peerSink{onText: onText}
	err = runner.Run(ctx, conv, dir, text, sink)
	log.Info("Peer ask: turn ended", slog.String("gateway", gw), slog.String("conversation", conv), slog.Int("text_bytes", sink.text.Len()), slog.Any("error", err))
	answer := strings.TrimSpace(sink.text.String())
	if err != nil {
		if answer == "" {
			return "", err
		}
		answer += "\n\n(the turn ended with: " + err.Error() + ")"
	}
	if sink.asked > 0 {
		answer += fmt.Sprintf("\n\n(%d question%s asked there had nobody to answer; the first option was taken)", sink.asked, plural(sink.asked))
	}
	return answer + fmt.Sprintf("\n\n— %s on %s · memdoor --gateway %s resume %s", agent, gw, gw, conv), nil
}

// peerSink keeps a peer turn's text; tools pass, questions take the default.
type peerSink struct {
	mu     sync.Mutex
	text   strings.Builder
	asked  int
	onText func(string)
}

func (s *peerSink) Text(t string) {
	s.mu.Lock()
	s.text.WriteString(t)
	s.mu.Unlock()
	if s.onText != nil {
		s.onText(t)
	}
}
func (s *peerSink) ToolStart(string, string, json.RawMessage) {}
func (s *peerSink) ToolDone(string, string, bool)             {}
func (s *peerSink) Ask(question string, options []string, approval string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked++
	if len(options) > 0 {
		return options[0]
	}
	return ""
}

func init() {
	peerAddCmd.Flags().String("dir", "", "the project on the peer a turn runs in (its absolute path there)")
	peersCmd.AddCommand(peerAddCmd, peerRemoveCmd)
	rootCmd.AddCommand(peersCmd)
}
