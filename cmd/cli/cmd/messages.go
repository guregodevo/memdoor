package cmd

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var (
	messagesChannel        string
	messagesLimit          int
	messagesBefore         string
	messagesIncludeThreads bool
)

var messagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "View messages in a channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		if messagesChannel == "" {
			return fmt.Errorf("--channel is required")
		}

		c := NewClient()
		channelID, err := c.resolveChannelID(messagesChannel)
		if err != nil {
			return err
		}

		params := url.Values{}
		params.Set("limit", strconv.Itoa(messagesLimit))
		if messagesBefore != "" {
			params.Set("before", messagesBefore)
		}

		var result struct {
			Messages []map[string]interface{} `json:"messages"`
		}
		if err := c.GetJSON("/api/channels/"+channelID+"/messages?"+params.Encode(), &result); err != nil {
			return err
		}
		messages := result.Messages

		fmt.Printf("Messages in #%s (showing %d)\n\n", messagesChannel, len(messages))

		for _, msg := range messages {
			isReply := false
			if pid, ok := msg["parent_id"]; ok && pid != nil {
				if pidFloat, ok := pid.(float64); ok && pidFloat > 0 {
					isReply = true
				}
			}

			// Skip thread replies unless --include-threads is set
			if isReply && !messagesIncludeThreads {
				continue
			}

			ts := ""
			if t, ok := msg["created_at"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339, t); err == nil {
					ts = parsed.Format("2006-01-02 15:04:05")
				}
			}

			authorID := ""
			if a, ok := msg["author_id"].(string); ok {
				authorID = a
			}
			authorName := ""
			if a, ok := msg["author_name"].(string); ok {
				authorName = a
			}
			displayAuthor := authorName
			if displayAuthor == "" {
				displayAuthor = authorID
			}

			content := ""
			if c, ok := msg["content"].(map[string]interface{}); ok {
				if t, ok := c["text"].(string); ok {
					content = t
				}
			} else if c, ok := msg["content"].(string); ok {
				content = c
			}

			prefix := ""
			if isReply {
				prefix = "  -> "
			}

			fmt.Printf("%s[%s] %s: %s\n", prefix, ts, displayAuthor, content)
			fmt.Println()
		}

		if len(messages) == messagesLimit {
			lastID := ""
			if id, ok := messages[len(messages)-1]["id"]; ok {
				lastID = fmt.Sprintf("%v", id)
			}
			fmt.Printf("  Tip: Use --before %s --limit %d to fetch older messages\n", lastID, messagesLimit)
		}
		if !messagesIncludeThreads {
			fmt.Println("  Tip: Use --include-threads to see thread replies")
		}

		return nil
	},
}

func init() {
	messagesCmd.Flags().StringVarP(&messagesChannel, "channel", "c", "", "Channel name (required)")
	messagesCmd.Flags().IntVarP(&messagesLimit, "limit", "l", 20, "Number of messages to fetch")
	messagesCmd.Flags().StringVarP(&messagesBefore, "before", "b", "", "Message ID for pagination")
	messagesCmd.Flags().BoolVar(&messagesIncludeThreads, "include-threads", false, "Include thread replies")
	rootCmd.AddCommand(messagesCmd)
}
