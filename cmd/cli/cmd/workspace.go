package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"memdoor/gateway/config"
	"memdoor/pkg/shared"

	"github.com/spf13/cobra"
)

// `memdoor workspace` — manage which workspace the CLI talks to.
//
// Both subcommands are exempt from requireWorkspaceSlug (see
// noWorkspaceTopLevels in root.go) because:
//   - `which` is the command you run when you want to *find out* what
//     would resolve, before running anything that needs a workspace.
//   - `use` is the command that *sets* the workspace; it can't itself
//     require one.

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage which workspace the CLI targets",
	Long: `Workspace resolution chain (highest precedence first):

  1. -w/--workspace flag
  2. $MEMDOOR_WORKSPACE environment variable
  3. .memdoor/workspace file (single line, the slug) discovered by
     walking up from the current directory to $HOME
  4. workspace_id in ~/.memdoor/config.json

Same pattern as git's repo discovery, kubectl --context, direnv, and
.nvmrc / .python-version style dotfiles.`,
}

var workspaceWhichCmd = &cobra.Command{
	Use:   "which",
	Short: "Print the resolved workspace and where it came from",
	Long: `Resolves the workspace slug using the standard chain (flag → env →
.memdoor/workspace walk-up → config) and prints what was selected and
which step produced it. Exits non-zero when nothing resolves.

Designed for three callers:
  • Humans — "wait, which workspace am I about to hit?"
  • Agents — confirm context before any destructive action.
  • Shell prompts — pass --bare to print just the slug (no source,
    no error, exit 0) for safe use in PS1 / RPROMPT.

PS1 example (zsh):
  PROMPT='%~ $(memdoor which --bare 2>/dev/null) %# '

PS1 example (bash):
  PS1='\w $(memdoor which --bare 2>/dev/null) \$ '`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bare, _ := cmd.Flags().GetBool("bare")
		slug, source, ok := resolveWorkspaceSlug()
		if !ok {
			if bare {
				// PS1-safe: silent, exit 0, empty output.
				return nil
			}
			fmt.Fprintln(os.Stderr, "no workspace resolved")
			fmt.Fprintln(os.Stderr, "  fix: memdoor workspace use <slug>")
			os.Exit(1)
		}
		if bare {
			fmt.Println(slug)
			return nil
		}
		fmt.Printf("%s\t(from %s)\n", slug, source)
		return nil
	},
}

var workspaceUseCmd = &cobra.Command{
	Use:               "use <slug>",
	Short:             "Pin the current directory to a workspace (writes ./.memdoor/workspace)",
	ValidArgsFunction: completeWorkspaceSlugs,
	Long: `Writes <slug> into ./.memdoor/workspace so every memdoor invocation
from this directory (or any subdirectory below it, up to $HOME)
auto-resolves the workspace without needing -w.

Idempotent — re-running with the same slug just overwrites the file
with the same contents. Safe to commit the .memdoor/ directory to the
repo so collaborators inherit the pinned workspace.

Also appends to ~/.memdoor/recent_workspaces (most-recent-first,
deduped, capped at 20) so 'memdoor workspace recent' can show your
recent context switches.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := strings.TrimSpace(args[0])
		if slug == "" {
			return fmt.Errorf("workspace slug cannot be empty")
		}
		path, err := writePinFile(discoveryFileName, slug)
		if err != nil {
			return err
		}
		// Best-effort append to recent-workspaces history; never fails the
		// command if the home dir is weird (read-only, missing, etc.).
		_ = appendRecentWorkspace(slug)
		fmt.Printf("✓ Pinned workspace %q in %s\n", slug, path)
		fmt.Println("  (any memdoor command run from this directory or below will use it; -w still overrides)")
		return nil
	},
}

const recentWorkspacesFile = "recent_workspaces"
const recentWorkspacesCap = 20

// appendRecentWorkspace prepends slug to ~/.memdoor/recent_workspaces,
// dedupes, and caps the file at recentWorkspacesCap entries. Best-effort:
// returns an error but callers ignore it (history is a nicety, not a
// correctness requirement).
func appendRecentWorkspace(slug string) error {
	dir := shared.MemdoorHome()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, recentWorkspacesFile)
	existing, _ := os.ReadFile(path) // missing file is fine, treated as empty
	lines := []string{slug}
	for _, line := range strings.Split(string(existing), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || s == slug {
			continue
		}
		lines = append(lines, s)
		if len(lines) >= recentWorkspacesCap {
			break
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// readRecentWorkspaces returns the deduped, ordered list of slugs in
// ~/.memdoor/recent_workspaces. Returns (nil, nil) when the file is
// missing — first-run / never-used-yet is a normal state, not an error.
// Returns the underlying error only when the file exists but can't be
// read; even then most callers should soft-fail.
func readRecentWorkspaces() ([]string, error) {
	path := shared.MemdoorHome(recentWorkspacesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, nil
}

// listAllWorkspaceSlugs returns every workspace the user has touched: the
// configured default, plus the recent-history slugs. Sorted, deduped.
//
// Best-effort: returns whatever it can. A nil repo, missing recent
// file, or unreadable DB all degrade gracefully — the function returns
// the slugs it could find rather than erroring. Tab completion and
// `workspace list` both rely on never erroring out under normal
// "fresh install" / "no DB yet" conditions.
func listAllWorkspaceSlugs() []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(slug string) {
		slug = strings.TrimSpace(slug)
		if slug == "" || seen[slug] {
			return
		}
		seen[slug] = true
		out = append(out, slug)
	}
	if recents, err := readRecentWorkspaces(); err == nil {
		for _, s := range recents {
			add(s)
		}
	}
	// THE ONE SETUP MADE COUNTS (battle test, 2026-09-27). The list was the
	// recents file alone, so a machine straight out of
	// `memdoor setup` answered "no workspaces yet" about the workspace it had
	// just created, and the default-workspace fallback had nothing to fall back
	// to. The configured default is a workspace whether or not anyone has
	// pinned a directory to it yet.
	if cfg, err := config.LoadConfig(); err == nil {
		add(cfg.WorkspaceID)
	}
	sort.Strings(out)
	return out
}

// completeWorkspaceSlugs is the cobra ValidArgsFunction for any
// command that takes a workspace slug as an argument (`use <slug>`,
// `workspace use <slug>`, etc.). Cobra also calls this for flag
// completion when registered via RegisterFlagCompletionFunc.
//
// Soft-fails to an empty list — tab completion that errors makes the
// shell beep, which is much worse than the user just typing the slug
// manually.
func completeWorkspaceSlugs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return listAllWorkspaceSlugs(), cobra.ShellCompDirectiveNoFileComp
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List every workspace you have",
	Long: `Lists every workspace this install has touched, from
~/.memdoor/recent_workspaces (the slugs you have pinned with
'memdoor use'). The active workspace is marked with a leading '*'.

The output is one slug per line, sorted alphabetically. Pipe into
fzf/grep/awk for shell-script use.

The top-level 'memdoor workspaces' command is a shorthand for this.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		slugs := listAllWorkspaceSlugs()
		if len(slugs) == 0 {
			fmt.Fprintln(os.Stderr, "no workspaces yet — run 'memdoor use <slug>' or 'memdoor setup' first")
			os.Exit(1)
		}
		active, _, _ := resolveWorkspaceSlug()
		for _, s := range slugs {
			if s == active {
				fmt.Printf("* %s\n", s)
			} else {
				fmt.Printf("  %s\n", s)
			}
		}
		return nil
	},
}

// workspacesCmd is the top-level shortcut: `memdoor workspaces`
// instead of `memdoor workspace list`. Same handler. Mirrors the
// `use` / `which` shortcut pattern.
var workspacesCmd = &cobra.Command{
	Use:   "workspaces",
	Short: "List every workspace you have (shortcut for 'workspace list')",
	Long:  workspaceListCmd.Long,
	RunE:  workspaceListCmd.RunE,
}

var workspaceRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "List recently-used workspaces (most recent first)",
	Long: `Reads ~/.memdoor/recent_workspaces — the history written by every
'memdoor use <slug>' invocation, deduped and capped at the 20 most
recent. Exits non-zero with an empty file or no history yet.

Useful for shell completion and "what was that workspace I was just
in?" recall after switching projects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := shared.MemdoorHome(recentWorkspacesFile)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(os.Stderr, "no recent workspaces yet — run 'memdoor use <slug>' first")
				os.Exit(1)
			}
			return fmt.Errorf("read %s: %w", path, err)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			fmt.Fprintln(os.Stderr, "no recent workspaces yet")
			os.Exit(1)
		}
		for _, line := range lines {
			if s := strings.TrimSpace(line); s != "" {
				fmt.Println(s)
			}
		}
		return nil
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <slug>",
	Short: "Permanently delete a workspace (admin only, destructive)",
	Long: `Cascade-deletes a workspace and every row that references it across
the SQL schema (channels, messages, sessions, buddies, cron_*,
agent_*, …), then removes the filesystem dir at
~/.memdoor/workspaces/<slug>/ and the RAG store at
~/.memdoor/rag/<workspace_id>.db.

Irreversible. The CLI prompts for explicit confirmation unless --yes
is set.

Requires admin authentication. Run 'memdoor auth login-direct' first
if your session is unauthenticated.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := strings.TrimSpace(args[0])
		if slug == "" {
			return fmt.Errorf("workspace slug cannot be empty")
		}
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Printf("⚠ This will permanently delete workspace %q and all its data:\n", slug)
			fmt.Println("   • SQL rows: channels, messages, sessions, buddies, cron_*, agent_*")
			fmt.Println("   • Filesystem: ~/.memdoor/workspaces/" + slug + "/")
			fmt.Println("   • RAG store: ~/.memdoor/rag/<workspace_id>.db")
			fmt.Print("\nType the workspace slug to confirm: ")
			r := bufio.NewReader(os.Stdin)
			line, _ := r.ReadString('\n')
			if strings.TrimSpace(line) != slug {
				return fmt.Errorf("confirmation did not match — aborted")
			}
		}

		client := NewClient()
		var resp map[string]interface{}
		if err := client.DeleteJSON("/api/workspaces/"+slug, &resp); err != nil {
			return err
		}
		fmt.Printf("✓ Deleted workspace %q (id=%v)\n", resp["slug"], resp["workspace_id"])
		if rows, ok := resp["rows_deleted"].(map[string]interface{}); ok && len(rows) > 0 {
			fmt.Println("  Rows deleted:")
			for table, n := range rows {
				fmt.Printf("    %-22s %v\n", table, n)
			}
		}
		if warns, ok := resp["warnings"].([]interface{}); ok && len(warns) > 0 {
			fmt.Println("  ⚠ Warnings (SQL committed; filesystem cleanup partial):")
			for _, w := range warns {
				fmt.Printf("    - %v\n", w)
			}
		}
		return nil
	},
}

// useCmd / whichCmd are top-level shortcuts: `memdoor use <slug>` and
// `memdoor which` instead of the longer `memdoor workspace use ...` /
// `memdoor workspace which`. Same handlers, half the keystrokes. The
// MUST roadmap entry "Workspace Ergonomics + Cross-Workspace Query"
// motivates this — fast switching + PS1-friendly status make per-task
// workspaces ergonomic enough to actually use.
var useCmd = &cobra.Command{
	Use:               "use <slug>",
	Short:             "Pin the current directory to a workspace (shortcut for 'workspace use')",
	Long:              workspaceUseCmd.Long,
	Args:              cobra.ExactArgs(1),
	RunE:              workspaceUseCmd.RunE,
	ValidArgsFunction: completeWorkspaceSlugs,
}

var whichCmd = &cobra.Command{
	Use:   "which",
	Short: "Print the resolved workspace (shortcut for 'workspace which')",
	Long:  workspaceWhichCmd.Long,
	RunE:  workspaceWhichCmd.RunE,
}

func init() {
	workspaceDeleteCmd.Flags().Bool("yes", false, "Skip confirmation prompt (for scripted use)")
	workspaceWhichCmd.Flags().Bool("bare", false, "Print just the slug (or empty); exit 0 even on no resolve. Use in PS1.")
	whichCmd.Flags().Bool("bare", false, "Print just the slug (or empty); exit 0 even on no resolve. Use in PS1.")
	workspaceCmd.AddCommand(workspaceWhichCmd)
	workspaceCmd.AddCommand(workspaceUseCmd)
	workspaceCmd.AddCommand(workspaceDeleteCmd)
	workspaceCmd.AddCommand(workspaceRecentCmd)
	workspaceCmd.AddCommand(workspaceListCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(whichCmd)
	rootCmd.AddCommand(workspacesCmd)
}
