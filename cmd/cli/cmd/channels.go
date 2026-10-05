package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Manage channels",
}

// Channels could be listed but never created from the CLI: every one in a
// workspace came from the TUI, so `--channel test` failed with "channel not
// found" and nothing could fix that without opening the TUI. Scripting an
// agent — which is what `memdoor agent --message` is for — needs somewhere to
// post.
var channelsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a channel to post into",
	Long: `Create a channel.

Useful for scripting an agent from the shell:

  memdoor channels create ci
  memdoor agent --message "review this diff" --channel ci --agent-id chief

The TUI makes its own channel per session; this is for everything else.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		// Reuses the same creation path the CLI flow uses, and trusts the id
		// the create RETURNED — looking it up again by name is how a paged,
		// name-ordered list silently hid new channels once before.
		id, err := createChannelGetID(NewClient(), name, "")
		if err != nil {
			return err
		}
		fmt.Printf("✓ #%s created (%s)\n", name, id)
		fmt.Printf("  memdoor agent --message \"…\" --channel %s --agent-id chief\n", name)
		return nil
	},
}

var channelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var result struct {
			Channels []map[string]interface{} `json:"channels"`
		}
		if err := c.GetJSON("/api/channels", &result); err != nil {
			return err
		}
		channels := result.Channels

		fmt.Printf("%-20s %-40s %s\n", "NAME", "DESCRIPTION", "TYPE")
		fmt.Println("-------------------------------------------------------------------")
		for _, ch := range channels {
			name := fmt.Sprintf("%v", ch["name"])
			desc := ""
			if d, ok := ch["description"]; ok && d != nil {
				desc = fmt.Sprintf("%v", d)
			}
			chType := "public"
			if t, ok := ch["type"]; ok && t != nil {
				chType = fmt.Sprintf("%v", t)
			}
			fmt.Printf("%-20s %-40s %s\n", name, desc, chType)
		}
		fmt.Printf("\nTotal: %d channels\n", len(channels))
		return nil
	},
}

var channelsMembersChannel string

var channelsMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "List members of a channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		if channelsMembersChannel == "" {
			return fmt.Errorf("--channel is required")
		}

		c := NewClient()
		channelID, err := c.resolveChannelID(channelsMembersChannel)
		if err != nil {
			return err
		}

		var result struct {
			Members []map[string]interface{} `json:"members"`
		}
		if err := c.GetJSON("/api/channels/"+channelID+"/members", &result); err != nil {
			return err
		}
		members := result.Members

		fmt.Printf("Members of #%s\n\n", channelsMembersChannel)

		humans := 0
		agents := 0

		fmt.Println("Humans:")
		for _, m := range members {
			mType := fmt.Sprintf("%v", m["type"])
			if mType == "human" || mType == "user" {
				humans++
				fmt.Printf("  %-30s %-20s %s\n", m["actor_id"], m["name"], m["role"])
			}
		}

		fmt.Println("\nAgents:")
		for _, m := range members {
			mType := fmt.Sprintf("%v", m["type"])
			if mType == "agent" || mType == "bot" {
				agents++
				fmt.Printf("  %-30s %-20s %s\n", m["actor_id"], m["name"], m["role"])
			}
		}

		fmt.Printf("\nTotal: %d members (%d humans, %d agents)\n", humans+agents, humans, agents)
		return nil
	},
}

var channelsAddMemberCmd = &cobra.Command{
	Use:   "add-member",
	Short: "Add a member to a channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		channelName, _ := cmd.Flags().GetString("channel")
		actorID, _ := cmd.Flags().GetString("actor")
		role, _ := cmd.Flags().GetString("role")

		c := NewClient()
		channelID, err := c.resolveChannelID(channelName)
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"channel_id": channelID,
			"actor_id":   actorID,
			"role":       role,
		}

		var result struct {
			ChannelID string `json:"channel_id"`
			ActorID   string `json:"actor_id"`
			Role      string `json:"role"`
			JoinedAt  string `json:"joined_at"`
		}
		if err := c.PostJSON("/api/channels/members", body, &result); err != nil {
			return err
		}

		fmt.Printf("Added member to channel\n")
		fmt.Printf("  Channel:   %s\n", result.ChannelID)
		fmt.Printf("  Actor:     %s\n", result.ActorID)
		fmt.Printf("  Role:      %s\n", result.Role)
		fmt.Printf("  Joined at: %s\n", result.JoinedAt)
		return nil
	},
}

func init() {
	channelsMembersCmd.Flags().StringVarP(&channelsMembersChannel, "channel", "c", "", "Channel name (required)")

	channelsAddMemberCmd.Flags().StringP("channel", "c", "", "Channel name (required)")
	channelsAddMemberCmd.Flags().StringP("actor", "a", "", "Actor ID to add (e.g., user:email or agent:name)")
	channelsAddMemberCmd.Flags().String("role", "member", "Role: member or admin")
	_ = channelsAddMemberCmd.MarkFlagRequired("channel")
	_ = channelsAddMemberCmd.MarkFlagRequired("actor")

	channelsCmd.AddCommand(channelsCreateCmd)
	channelsCmd.AddCommand(channelsListCmd)
	channelsCmd.AddCommand(channelsMembersCmd)
	channelsCmd.AddCommand(channelsAddMemberCmd)
	rootCmd.AddCommand(channelsCmd)
}

// createChannelGetID creates a channel and returns its UUID from the response.
// The gateway auto-adds the caller as an admin member: a channel created
// without owner_id stays invisible to its creator, because the list endpoint
// filters by membership. (Lived in a removed maintenance command until it
// left, 2026-09-27.)
func createChannelGetID(c *Client, name, ownerID string) (string, error) {
	body := map[string]interface{}{
		"name": name,
		"type": "public",
	}
	if ownerID != "" {
		body["owner_id"] = ownerID
	}
	resp, err := c.Post("/api/channels", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("create channel %q: HTTP %d", name, resp.StatusCode)
	}
	var ch struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ch); err != nil {
		return "", err
	}
	if ch.ID == "" {
		return "", fmt.Errorf("create channel %q: empty id in response", name)
	}
	return ch.ID, nil
}
