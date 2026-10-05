package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage sessions",
}

// shortSessionKey makes a session key readable. The raw form
// ("workspace:hackernews:channel:8488c86b-…") is a lookup key, not something a
// person reads; the workspace and the first block of the channel id are enough
// to recognise one.
func shortSessionKey(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) == 4 && parts[0] == "workspace" && parts[2] == "channel" {
		id := parts[3]
		if len(id) > 8 {
			id = id[:8]
		}
		return parts[1] + " · channel " + id
	}
	return key
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the gateway's live sessions (for conversations you can resume, see `memdoor conversations`)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var result struct {
			Count    int `json:"count"`
			Sessions []struct {
				SessionKey string `json:"session_key"`
				Kind       string `json:"kind"`
				CreatedAt  int64  `json:"created_at"`
				UpdatedAt  int64  `json:"updated_at"`
			} `json:"sessions"`
		}
		if err := c.GetJSON("/sessions", &result); err != nil {
			return err
		}
		if len(result.Sessions) == 0 {
			fmt.Println("No live sessions.")
			return nil
		}

		// Newest first: the session you were just in is the one you are looking
		// for.
		sort.Slice(result.Sessions, func(i, j int) bool {
			return result.Sessions[i].UpdatedAt > result.Sessions[j].UpdatedAt
		})

		fmt.Printf("%-34s %-8s %-16s %s\n", "SESSION", "KIND", "CREATED", "UPDATED")
		fmt.Printf("%-34s %-8s %-16s %s\n", strings.Repeat("-", 7), "----", "-------", "-------")
		for _, s := range result.Sessions {
			created, updated := "", ""
			if s.CreatedAt > 0 {
				created = time.Unix(s.CreatedAt, 0).Local().Format("Jan 2 15:04")
			}
			if s.UpdatedAt > 0 {
				updated = humanSince(time.Unix(s.UpdatedAt, 0))
			}
			fmt.Printf("%-34s %-8s %-16s %s\n", shortSessionKey(s.SessionKey), s.Kind, created, updated)
		}
		fmt.Printf("\nTotal: %d sessions\n", result.Count)
		return nil
	},
}

var sessionsGetCmd = &cobra.Command{
	Use:   "get [session-key]",
	Short: "Get session details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		var session map[string]interface{}
		if err := c.GetJSON("/sessions/detail?id="+args[0], &session); err != nil {
			return err
		}

		for k, v := range session {
			fmt.Printf("%-20s %v\n", k+":", v)
		}
		return nil
	},
}

// sessionsClearCmd wipes the conversation history for a single session
// without removing the session itself. Use this when an agent's prior
// turns are poisoning its replies — a model that pattern-matches its own
// earlier "nothing found" response can stop calling tools entirely.
//
// Two ways to address the session:
//
//	memdoor sessions clear --session-key workspace:<ws-uuid>:channel:<ch-uuid>
//	memdoor sessions clear --channel general
//
// The --channel form looks up the workspace UUID + channel UUID itself
// and assembles the canonical "workspace:<wsID>:channel:<chID>" key
// used by the chat router.
var sessionsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear conversation history for a session (chat memory wipe)",
	Long: `Clear an agent's conversation history for a session. The session
itself is preserved — just its message log is reset, so the next turn
starts with the system prompt only and no prior context to
pattern-match against.

Use --channel to clear the chat session for a workspace+channel pair
(the common case for testing a background agent). The CLI
resolves the canonical session key for you.

Examples:
  memdoor -w cyberlaw sessions clear --channel general
  memdoor sessions clear --session-key agent:coder:cron:<job-id>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionKey, _ := cmd.Flags().GetString("session-key")
		channel, _ := cmd.Flags().GetString("channel")
		if sessionKey == "" && channel == "" {
			return fmt.Errorf("either --session-key or --channel is required")
		}

		c := NewClient()

		// Channel-form: build the canonical session key from
		// workspace UUID + channel UUID. Mirrors the format the chat
		// dispatcher uses (gateway/agent_handlers.go: "workspace:%s:channel:%s").
		if sessionKey == "" {
			channelID, err := c.resolveChannelID(channel)
			if err != nil {
				return fmt.Errorf("resolve channel %q: %w", channel, err)
			}
			workspaceID, err := resolveWorkspaceUUID(c, workspaceSlug)
			if err != nil {
				return fmt.Errorf("resolve workspace UUID for slug %q: %w", workspaceSlug, err)
			}
			sessionKey = fmt.Sprintf("workspace:%s:channel:%s", workspaceID, channelID)
		}

		body := map[string]interface{}{"session_id": sessionKey}
		var resp map[string]interface{}
		if err := c.PostJSON("/sessions/clear", body, &resp); err != nil {
			return fmt.Errorf("clear session: %w", err)
		}
		fmt.Printf("✓ Cleared session %s\n", sessionKey)
		return nil
	},
}

// resolveWorkspaceUUID looks up the workspace ID for a slug. For the
// id-equals-slug workspaces (cyberlaw, hackernews, the more recent
// pattern) the slug IS the ID. For UUID-keyed workspaces (the older
// installs) /api/setup/status returns the canonical
// workspace_id; we try that first and fall back to the slug.
func resolveWorkspaceUUID(c *Client, slug string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("workspace slug required (use -w)")
	}
	var status struct {
		WorkspaceID   string `json:"workspace_id"`
		WorkspaceSlug string `json:"workspace_slug"`
	}
	// /api/setup/status returns the bootstrap workspace's ID/slug; if
	// the slug we asked for matches, the returned workspace_id is the
	// answer. Otherwise fall through to the slug-as-id assumption.
	if err := c.GetJSON("/api/setup/status", &status); err == nil {
		if status.WorkspaceSlug == slug && status.WorkspaceID != "" {
			return status.WorkspaceID, nil
		}
	}
	// id-equals-slug workspace pattern (newer cyberlaw / hackernews
	// installs use the slug directly as the row id).
	return slug, nil
}

// sessionsExportCmd walks the local session store and writes one JSONL record
// per agent session — real tool-calling trajectories (message + timestamp
// entries, exactly as persisted) wrapped in workspace/channel/agent metadata.
// STRICTLY manual and local: nothing is uploaded; the output file is yours.
// The intended consumers are model/prompt evaluation and fine-tuning datasets
// (the pi-share-hf pattern), where real sessions beat synthetic ones.
var sessionsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export local agent sessions as JSONL (for evals / fine-tuning datasets)",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := shared.MemdoorHome("workspaces")
		wsFilter, _ := cmd.Flags().GetString("workspace")
		agFilter, _ := cmd.Flags().GetString("agent")
		outPath, _ := cmd.Flags().GetString("out")
		if outPath == "" {
			outPath = fmt.Sprintf("memdoor-sessions-%s.jsonl", time.Now().Format("2006-01-02"))
		}

		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()
		enc := json.NewEncoder(out)

		type record struct {
			Workspace  string            `json:"workspace"`
			Channel    string            `json:"channel"`
			Agent      string            `json:"agent"`
			Entries    []json.RawMessage `json:"entries"`
			ExportedAt string            `json:"exported_at"`
		}

		var files, entries int
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() != "sessions.json" {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			parts := strings.Split(rel, string(filepath.Separator))
			// <ws>/channels/<ch>/sessions.json or <ws>/channels/<ch>/agents/<agent>/sessions.json
			if len(parts) < 4 || parts[1] != "channels" {
				return nil
			}
			ws, ch, agent := parts[0], parts[2], "channel"
			if len(parts) >= 6 && parts[3] == "agents" {
				agent = parts[4]
			}
			if wsFilter != "" && ws != wsFilter {
				return nil
			}
			if agFilter != "" && agent != agFilter {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			rec := record{Workspace: ws, Channel: ch, Agent: agent, ExportedAt: time.Now().UTC().Format(time.RFC3339)}
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" || !json.Valid([]byte(line)) {
					continue
				}
				rec.Entries = append(rec.Entries, json.RawMessage(line))
			}
			if len(rec.Entries) == 0 {
				return nil
			}
			if err := enc.Encode(rec); err != nil {
				return err
			}
			files++
			entries += len(rec.Entries)
			return nil
		})
		if err != nil {
			return err
		}
		fmt.Printf("Exported %d sessions (%d entries) to %s\n", files, entries, outPath)
		fmt.Println("NOTE: sessions contain your prompts, code, and tool outputs verbatim — review before sharing anywhere.")
		return nil
	},
}

// sessionsRewindCmd undoes the last N turns of a session instead of wiping it
// — the recovery lever for a derailed turn (bad patch loop, poisoned context):
// drop the poison, keep the session.
var sessionsRewindCmd = &cobra.Command{
	Use:   "rewind",
	Short: "Undo the last N turns of a session (keep the rest)",
	Long: `Rewind a session by dropping its last N turns — a turn is a user
prompt plus everything the agent did in response. Unlike clear, the earlier
conversation survives. The next turn continues from the rewound state.

Examples:
  memdoor -w hackernews sessions rewind --channel general
  memdoor sessions rewind --channel dev --turns 2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionKey, _ := cmd.Flags().GetString("session-key")
		channel, _ := cmd.Flags().GetString("channel")
		turns, _ := cmd.Flags().GetInt("turns")
		if sessionKey == "" && channel == "" {
			return fmt.Errorf("either --session-key or --channel is required")
		}
		c := NewClient()
		if sessionKey == "" {
			channelID, err := c.resolveChannelID(channel)
			if err != nil {
				return fmt.Errorf("resolve channel %q: %w", channel, err)
			}
			workspaceID, err := resolveWorkspaceUUID(c, workspaceSlug)
			if err != nil {
				return fmt.Errorf("resolve workspace UUID for slug %q: %w", workspaceSlug, err)
			}
			sessionKey = fmt.Sprintf("workspace:%s:channel:%s", workspaceID, channelID)
		}
		var resp map[string]interface{}
		if err := c.PostJSON("/sessions/rewind", map[string]interface{}{"session_id": sessionKey, "turns": turns}, &resp); err != nil {
			return err
		}
		fmt.Printf("Rewound %v turn(s): %v entries dropped.\n", resp["turns"], resp["entries_dropped"])
		return nil
	},
}

func init() {
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsGetCmd)
	sessionsClearCmd.Flags().String("session-key", "", "Exact session key to clear (e.g. 'agent:coder:cron:<job-id>')")
	sessionsClearCmd.Flags().String("channel", "", "Channel name — CLI resolves to the workspace+channel session key")
	sessionsCmd.AddCommand(sessionsClearCmd)
	sessionsExportCmd.Flags().String("workspace", "", "Only this workspace slug")
	sessionsExportCmd.Flags().String("agent", "", "Only this agent (e.g. coder)")
	sessionsExportCmd.Flags().String("out", "", "Output file (default memdoor-sessions-<date>.jsonl)")
	sessionsCmd.AddCommand(sessionsExportCmd)
	sessionsRewindCmd.Flags().String("session-key", "", "Exact session key to rewind")
	sessionsRewindCmd.Flags().String("channel", "", "Channel name — CLI resolves the session key")
	sessionsRewindCmd.Flags().Int("turns", 1, "How many trailing turns to drop")
	sessionsCmd.AddCommand(sessionsRewindCmd)
	rootCmd.AddCommand(sessionsCmd)
}
