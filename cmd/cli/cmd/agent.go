package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	agentMessage  string
	agentChannel  string
	agentID       string
	agentThinking string
	agentMode     string
)

var agentCmd = &cobra.Command{
	Use:     "agent",
	Aliases: []string{"agents"},
	Short:   "Manage agents and send messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		if agentMessage == "" || agentChannel == "" || agentID == "" {
			return cmd.Help()
		}

		// Piped content (`cat log | memdoor agent …`) is attached to the message.
		stdinContent := pipedStdin(os.Stdin, stdinGrace)

		fullMessage := agentMessage
		if stdinContent != "" {
			fullMessage = agentMessage + "\n\n```\n" + stdinContent + "\n```"
		}

		c := NewClient()
		channelID, err := c.resolveChannelID(agentChannel)
		if err != nil {
			return err
		}

		// The run is scoped to the workspace: that is what the gateway keys the
		// session on.
		body := map[string]interface{}{
			"channel_id": channelID,
			"text":       "@" + agentID + " " + fullMessage,
			"workspace":  workspaceSlug,
		}
		// Claude-style interaction mode: plan (read-only research + exit_plan_mode),
		// acceptEdits, or default. Empty = default.
		if agentMode != "" {
			body["permission_mode"] = agentMode
		}

		if err := c.PostExpectOK("/api/messages", body); err != nil {
			return err
		}

		fmt.Printf("Message posted to #%s mentioning @%s\n", agentChannel, agentID)
		fmt.Printf("\n  View responses: memdoor messages --channel %s\n", agentChannel)
		fmt.Printf("  Or visit Web UI: %s\n", gatewayAddr)
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var result struct {
			Agents []map[string]interface{} `json:"agents"`
		}
		if err := c.GetJSON("/api/agents", &result); err != nil {
			return err
		}
		agents := result.Agents

		// LLM provider/model are locked at install time (managed tier).
		// Per-agent rows just carry name + emoji.
		fmt.Printf("%-32s %s\n", "AGENT ID", "NAME")
		fmt.Printf("%-32s %s\n", "--------", "----")
		for _, a := range agents {
			id := fmt.Sprintf("%v", a["id"])
			name := fmt.Sprintf("%v", a["name"])
			fmt.Printf("%-32s %s\n", id, name)
		}

		fmt.Printf("\nTotal: %d agents\n", len(agents))
		return nil
	},
}

var agentShowCmd = &cobra.Command{
	Use:   "show [agent-id]",
	Short: "Show agent details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var result struct {
			Agents []map[string]interface{} `json:"agents"`
		}
		if err := c.GetJSON("/api/agents", &result); err != nil {
			return err
		}

		for _, a := range result.Agents {
			if fmt.Sprintf("%v", a["id"]) == args[0] || fmt.Sprintf("%v", a["name"]) == args[0] {
				fmt.Printf("Agent: %v\n", a["name"])
				fmt.Printf("  ID:          %v\n", a["id"])
				if desc, ok := a["description"]; ok && desc != nil {
					fmt.Printf("  Description: %v\n", desc)
				}
				if tools, ok := a["tools"]; ok && tools != nil {
					fmt.Printf("  Tools:       %v\n", tools)
				}
				return nil
			}
		}
		return fmt.Errorf("agent not found: %s", args[0])
	},
}

var agentAddCmd = &cobra.Command{
	Use:   "add [agent-id]",
	Short: "Add a new agent",
	Long: `Add a new buddy to the workspace.

Agents run on the gateway's models: each agent's ladder, on the provider
key the gateway holds ('memdoor providers', 'memdoor model'). Each agent
carries a name, a prompt, tools, a sandbox scope, etc.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		body := map[string]interface{}{
			"name": args[0],
		}
		if v, _ := cmd.Flags().GetString("description"); v != "" {
			body["description"] = v
		}
		if v, _ := cmd.Flags().GetString("system-prompt"); v != "" {
			body["system_prompt"] = v
		}
		if v, _ := cmd.Flags().GetString("personality"); v != "" {
			body["personality"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("tools"); len(v) > 0 {
			body["tools"] = v
		}

		if err := c.PostExpectOK("/api/agents", body); err != nil {
			return err
		}
		fmt.Printf("Agent %q created successfully\n", args[0])
		return nil
	},
}

var agentUpdateCmd = &cobra.Command{
	Use:   "update [agent-name]",
	Short: "Update an agent's configuration",
	Long: `Update an agent's prompt, tools, sandbox scope, etc.

The model is not set here: every agent runs on the gateway's models, on the
provider key it holds ('memdoor model' shows each agent's ladder).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		body := map[string]interface{}{
			"name": args[0],
		}
		if v, _ := cmd.Flags().GetString("description"); v != "" {
			body["description"] = v
		}
		if v, _ := cmd.Flags().GetString("system-prompt"); v != "" {
			body["system_prompt"] = v
		}
		if v, _ := cmd.Flags().GetString("personality"); v != "" {
			body["personality"] = v
		}
		if v, _ := cmd.Flags().GetStringSlice("tools"); len(v) > 0 {
			body["tools"] = v
		}
		if v, _ := cmd.Flags().GetString("endpoint"); v != "" {
			body["endpoint"] = v
		}
		if v, _ := cmd.Flags().GetString("execution-type"); v != "" {
			body["execution_type"] = v
		}
		if cmd.Flags().Changed("admin-only") {
			v, _ := cmd.Flags().GetBool("admin-only")
			body["admin_only"] = v
		}
		if cmd.Flags().Changed("learning-enabled") {
			v, _ := cmd.Flags().GetBool("learning-enabled")
			body["learning_enabled"] = v
		}

		if err := c.PutExpectOK("/api/agents/"+args[0], body); err != nil {
			return err
		}
		fmt.Printf("Agent %q updated successfully\n", args[0])
		return nil
	},
}

var agentDeleteCmd = &cobra.Command{
	Use:   "delete [agent-name]",
	Short: "Delete an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("Delete agent %q? (y/N): ", args[0])
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(answer) != "y" {
				fmt.Println("Cancelled")
				return nil
			}
		}

		c := NewClient()
		if err := c.DeleteExpectOK("/api/agents/" + args[0]); err != nil {
			return err
		}
		fmt.Printf("Agent %q deleted\n", args[0])
		return nil
	},
}

var agentSessionsCmd = &cobra.Command{
	Use:   "sessions [agent-id]",
	Short: "List agent sessions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var result struct {
			Sessions []map[string]interface{} `json:"sessions"`
		}
		if err := c.GetJSON("/sessions", &result); err != nil {
			return err
		}

		agentFilter := args[0]
		fmt.Printf("Sessions for agent: %s\n\n", agentFilter)
		count := 0
		for _, s := range result.Sessions {
			key := fmt.Sprintf("%v", s["key"])
			if fmt.Sprintf("%v", s["agent_id"]) == agentFilter || strings.Contains(key, agentFilter) {
				messages := 0
				if m, ok := s["message_count"].(float64); ok {
					messages = int(m)
				}
				fmt.Printf("  %-40s  %d messages\n", key, messages)
				count++
			}
		}
		if count == 0 {
			fmt.Println("  No sessions found")
		}
		return nil
	},
}

func init() {
	agentCmd.Flags().StringVarP(&agentMessage, "message", "m", "", "Message to send")
	agentCmd.Flags().StringVarP(&agentChannel, "channel", "c", "", "Channel name")
	agentCmd.Flags().StringVarP(&agentID, "agent-id", "a", "", "Agent ID to mention")
	agentCmd.Flags().StringVarP(&agentThinking, "thinking", "t", "auto", "Thinking mode: enabled|disabled|auto")
	agentCmd.Flags().StringVar(&agentMode, "mode", "", "Interaction mode: plan (read-only) | acceptEdits | default")

	// LLM provider/model are locked at install time (managed tier) — no
	// --model or --provider flag here, and no runtime switch elsewhere.
	agentAddCmd.Flags().String("description", "", "Agent description")
	agentAddCmd.Flags().String("system-prompt", "", "System prompt")
	agentAddCmd.Flags().String("personality", "", "Personality")
	agentAddCmd.Flags().StringSlice("tools", nil, "Agent tools (comma-separated)")

	agentUpdateCmd.Flags().String("description", "", "Agent description")
	agentUpdateCmd.Flags().String("system-prompt", "", "System prompt")
	agentUpdateCmd.Flags().String("personality", "", "Personality")
	agentUpdateCmd.Flags().StringSlice("tools", nil, "Agent tools (comma-separated)")
	agentUpdateCmd.Flags().String("endpoint", "", "Remote endpoint URL (OpenAI-compatible, e.g. http://127.0.0.1:8081/v1/chat/completions)")
	agentUpdateCmd.Flags().String("execution-type", "", "Execution type: local or remote")
	agentUpdateCmd.Flags().Bool("admin-only", false, "Restrict agent to admin users only")
	agentUpdateCmd.Flags().Bool("learning-enabled", true, "Enable memory retrieval and reflection learning")

	agentDeleteCmd.Flags().Bool("force", false, "Skip confirmation")

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentShowCmd)
	agentCmd.AddCommand(agentAddCmd)
	agentCmd.AddCommand(agentUpdateCmd)
	agentCmd.AddCommand(agentDeleteCmd)
	agentCmd.AddCommand(agentSessionsCmd)
	rootCmd.AddCommand(agentCmd)
}

// stdinGrace is how long `memdoor agent` waits for piped input to start.
const stdinGrace = 2 * time.Second

// pipedStdin returns what is piped into stdin, or "" when stdin is a terminal
// or nothing arrives within grace. Once input starts it is read to the end.
//
// Waiting for the end unconditionally hung a script for 20 minutes
// (2026-09-25): run from a harness whose stdin was a pipe nobody wrote to or
// closed, the command sat reading it before posting anything. A caller that
// pipes content writes it at once; one that merely inherits an open pipe
// should not block.
func pipedStdin(f *os.File, grace time.Duration) string {
	stat, err := f.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice != 0 {
		return ""
	}
	br := bufio.NewReader(f)
	started := make(chan error, 1)
	go func() { _, err := br.Peek(1); started <- err }()
	select {
	case err := <-started:
		if err != nil { // empty input (EOF) or unreadable
			return ""
		}
	case <-time.After(grace):
		fmt.Fprintln(os.Stderr, "note: stdin is open but empty; sending the message without it")
		return ""
	}
	b, _ := io.ReadAll(br)
	return strings.TrimRight(string(b), "\n")
}
