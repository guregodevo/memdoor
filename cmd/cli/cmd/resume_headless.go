package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"memdoor/gateway/client"
	"memdoor/gateway/protocol"
)

// memdoor resume <id> --headless prints the conversation as text instead
// of opening a window (Greg, 2026-10-11: "memdoor resume xxxx --headless |
// head"): a pipe reads it, --follow keeps printing what a running turn does
// until it ends. Your lines start with "> ", the agent's are plain, a tool
// call is one ⏺ line.

// transcriptLine is one printed message.
type transcriptLine struct {
	at   string
	mine bool
	text string
}

// transcriptLines orders the channel's messages oldest first and marks
// which are the person's (the agent's author id starts with "agent").
func transcriptLines(msgs []map[string]any) []transcriptLine {
	var out []transcriptLine
	for _, m := range msgs {
		text := ""
		if c, ok := m["content"].(map[string]any); ok {
			text, _ = c["text"].(string)
		} else if c, ok := m["content"].(string); ok {
			text = c
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		author, _ := m["author_id"].(string)
		kind, _ := m["author_type"].(string)
		mine := kind != "agent" && !strings.HasPrefix(author, "agent")
		at, _ := m["created_at"].(string)
		text = strings.TrimSpace(text)
		if mine && strings.HasPrefix(text, "@") {
			// The window addressed the agent; the person did not type that.
			if _, rest, ok := strings.Cut(text, " "); ok {
				text = rest
			}
		}
		out = append(out, transcriptLine{at: at, mine: mine, text: text})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at < out[j].at })
	return out
}

func writeTranscript(w io.Writer, lines []transcriptLine) {
	for i, l := range lines {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if l.mine {
			fmt.Fprintln(w, "> "+strings.ReplaceAll(l.text, "\n", "\n  "))
			continue
		}
		fmt.Fprintln(w, l.text)
	}
}

// resumeHeadless prints the conversation; with follow it then streams the
// running turn, if any, until it ends or the pipe closes.
func resumeHeadless(r sessionRecord, limit int, follow bool) error {
	if err := firstRunReady(); err != nil {
		return err
	}
	c := NewClient()
	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	var result struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := c.GetJSON("/api/channels/"+r.ChannelID+"/messages?"+params.Encode(), &result); err != nil {
		return err
	}
	writeTranscript(os.Stdout, transcriptLines(result.Messages))
	if !follow {
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opts := client.DefaultOptions()
	opts.EnableLogging = false
	opts.Workspace, opts.Channel, opts.Token = r.Workspace, r.ChannelID, c.token
	feed := client.New(gatewayAddr, "", opts)
	done := make(chan struct{}, 1)
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
				fmt.Print(str("delta"))
			case "text":
				if a, _ := data["append"].(bool); a {
					fmt.Println("\n" + str("text"))
				}
			}
		case "tool":
			if str("event") == "start" {
				var in map[string]any
				_ = json.Unmarshal([]byte(str("input")), &in)
				arg := ""
				for _, k := range []string{"command", "file_path", "path", "pattern", "query", "task"} {
					if v, ok := in[k].(string); ok && v != "" {
						arg = v
						break
					}
				}
				fmt.Printf("\n⏺ %s(%s)\n", str("tool"), firstLineOf(arg, 100))
			}
		case "lifecycle":
			if e := str("event"); e == "complete" || e == "error" {
				fmt.Println()
				select {
				case done <- struct{}{}:
				default:
				}
			}
		}
		return nil
	})
	if err := feed.Connect(); err != nil {
		return fmt.Errorf("the gateway's event stream: %w", err)
	}
	feed.Listen()
	defer feed.Close()
	fmt.Fprintln(os.Stderr, "following · ctrl+c stops")
	select {
	case <-done:
	case <-ctx.Done():
	}
	return nil
}
