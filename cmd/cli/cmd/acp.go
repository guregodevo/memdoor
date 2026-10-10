package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/spf13/cobra"

	"memdoor/gateway/client"
	"memdoor/gateway/protocol"
)

// memdoor acp: the Agent Client Protocol agent over stdio (acp_agent.go),
// driving the local gateway the way the window does: a conversation per
// editor session, each prompt a turn posted to /api/messages with the
// editor's folder as the working directory, the turn's events read off the
// websocket, cancel and answers sent back on it. stdout is the protocol;
// everything else goes to stderr.

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Serve the coder to an editor over the Agent Client Protocol (Zed, JetBrains, VS Code's ACP client)",
	Long: `Runs Memdoor as an Agent Client Protocol agent on stdin/stdout, for an
editor to spawn: Zed and JetBrains host ACP agents natively; VS Code through an
ACP client extension (settings: "acp.agents": {"Memdoor": {"command": "memdoor",
"args": ["acp"]}}). Each editor session is a conversation on this machine's
gateway; a prompt is a turn of the coder in the editor's workspace folder, and
its text and tool calls stream back as the editor's own. The first run sets the
machine up like memdoor tui does.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := firstRunReady(); err != nil {
			return err
		}
		c := NewClient()
		handShellKeyToGateway(c)
		if workspaceSlug == "" {
			if slug, _, ok := resolveWorkspaceSlug(); ok {
				workspaceSlug = slug
			} else if only, single := soleWorkspace(); single {
				workspaceSlug = only
			}
		}
		runner := &gatewayRunner{c: c, workspace: workspaceSlug, feeds: map[string]*client.Client{}}
		agent := newACPAgent(runner)
		conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
		agent.SetAgentConnection(conn)
		fmt.Fprintf(os.Stderr, "memdoor acp: serving %s on the gateway at %s\n", workspaceSlug, gatewayAddr)
		<-conn.Done()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(acpCmd)
}

// gatewayRunner drives turns on the local gateway for the ACP agent.
type gatewayRunner struct {
	c         *Client
	workspace string
	mu        sync.Mutex
	feeds     map[string]*client.Client // the websocket of a running turn, by conversation
}

const acpAgentName = "coder"

func (r *gatewayRunner) NewConversation(ctx context.Context, cwd string) (string, error) {
	if b := readBrain(r.c); b.State == "none" {
		return "", errors.New(brainGate("none"))
	}
	id, err := r.c.createChannel("acp-" + time.Now().Format("0102-150405"))
	if err != nil {
		if gen, gerr := r.c.resolveChannelID("general"); gerr == nil {
			return gen, nil
		}
		return "", fmt.Errorf("could not open a conversation on the gateway: %w", err)
	}
	return id, nil
}

// runEnd is how a turn ended on the wire.
type runEnd struct {
	model string
	err   error
}

func (r *gatewayRunner) Run(ctx context.Context, conversation, cwd, text string, sink turnSink) error {
	opts := client.DefaultOptions()
	opts.EnableLogging = false
	opts.Workspace, opts.Channel, opts.Token = r.workspace, conversation, r.c.token
	feed := client.New(gatewayAddr, "", opts)
	done := make(chan runEnd, 1)
	end := func(e runEnd) {
		select {
		case done <- e:
		default:
		}
	}
	var streamed int // text streamed as deltas since the last whole-text frame
	feed.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
		stream, _ := msg.Data["stream"].(string)
		data, _ := msg.Data["data"].(map[string]any)
		if data == nil {
			return nil
		}
		str := func(k string) string { v, _ := data[k].(string); return v }
		switch stream {
		case "assistant":
			switch str("event") {
			case "text_delta":
				streamed += len(str("delta"))
				sink.Text(str("delta"))
			case "text":
				// The whole round's text follows its deltas: skip what was
				// streamed; an appended tail (the receipt line) is new.
				if append, _ := data["append"].(bool); append {
					sink.Text("\n" + str("text"))
				} else if streamed == 0 {
					sink.Text(str("text"))
				}
				streamed = 0
			}
		case "tool":
			switch str("event") {
			case "start":
				sink.ToolStart(str("id"), str("tool"), json.RawMessage(str("input")))
			case "complete":
				sink.ToolDone(str("id"), str("output"), str("error") != "")
			}
		case "question":
			var options []string
			if raw, ok := data["options"].([]any); ok {
				for _, o := range raw {
					if s, ok := o.(string); ok {
						options = append(options, s)
					}
				}
			}
			qid := str("question_id")
			go func() {
				answer := sink.Ask(str("question"), options, str("approval"))
				if answer == "" {
					answer = str("default")
					if str("approval") != "" {
						answer = "No"
					}
				}
				_ = feed.SendAnswer(qid, answer)
			}()
		case "lifecycle":
			switch str("event") {
			case "complete":
				end(runEnd{model: str("model")})
			case "error":
				end(runEnd{err: errors.New(str("error"))})
			}
		case "error":
			end(runEnd{err: errors.New(str("error"))})
		}
		return nil
	})
	feed.OnMessage("execution.failed", func(msg protocol.Message) error {
		if inner, ok := msg.Data["data"].(map[string]any); ok {
			if e, _ := inner["error"].(string); e != "" {
				end(runEnd{err: errors.New(e)})
			}
		}
		return nil
	})
	if err := feed.Connect(); err != nil {
		return fmt.Errorf("the gateway's event stream: %w", err)
	}
	feed.Listen()
	defer feed.Close()
	r.mu.Lock()
	r.feeds[conversation] = feed
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.feeds, conversation)
		r.mu.Unlock()
	}()
	recordSession(sessionRecord{ChannelID: conversation, ChannelName: "acp", Title: firstLine(text), Workspace: r.workspace, Dir: cwd})
	if err := r.c.PostExpectOK("/api/messages", map[string]interface{}{
		"channel_id":      conversation,
		"text":            "@" + acpAgentName + " " + text,
		"workspace":       r.workspace,
		"permission_mode": "acceptEdits",
		"workdir":         cwd,
	}); err != nil {
		return err
	}
	select {
	case e := <-done:
		return e.err
	case <-ctx.Done():
		_ = feed.SendMessage(protocol.Message{Type: protocol.MessageTypeCancel})
		return ctx.Err()
	}
}

func (r *gatewayRunner) Interrupt(ctx context.Context, conversation string) error {
	r.mu.Lock()
	feed := r.feeds[conversation]
	r.mu.Unlock()
	if feed == nil {
		return nil
	}
	return feed.SendMessage(protocol.Message{Type: protocol.MessageTypeCancel})
}

// acpStdoutIsTheProtocol: nothing of the CLI's own may print to stdout while
// acp serves; the next-step suggestion is the one thing that would.
var _ = strings.TrimSpace
