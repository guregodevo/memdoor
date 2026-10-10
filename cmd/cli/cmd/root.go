package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	gatewayAddr   string
	verbose       bool
	workspaceSlug string
)

// noWorkspaceTopLevels is the allowlist of top-level subcommands that
// do NOT require -w/--workspace. Everything else does — that's the
// uniform contract enforced by the PersistentPreRunE below.
//
// Why these are exempt:
//
//   - auth: login/logout/whoami runs before a workspace is even known
//   - gateway: starts/stops the daemon process, gateway-wide
//   - doctor: system diagnostics, gateway-wide
//   - setup: bootstraps the install before any workspace exists
//   - mcp: the person's MCP servers; every call names its project directory
//   - version: prints the binary version, no API call
//   - logs: queries the gateway-wide event log, not workspace-scoped
//
// Every other top-level command (channels, messages, agent, sessions,
// cron, secrets, rag, users, …) talks to a
// workspace-scoped API and MUST receive an explicit workspace.
var noWorkspaceTopLevels = map[string]bool{
	"auth":       true,
	"gateway":    true,
	"doctor":     true,
	"setup":      true,
	"version":    true,
	"logs":       true,
	"billing":    true, // the billing service serves every workspace; it belongs to none
	"workspace":  true, // `workspace which` resolves before any -w; `workspace use` sets it
	"use":        true, // top-level shortcut for `workspace use <slug>`
	"which":      true, // top-level shortcut for `workspace which` (PS1-safe with --bare)
	"workspaces": true, // top-level shortcut for `workspace list`
	"ask":        true, // `ask --all` / `--workspaces <list>` runs without a single resolved workspace; the no-flag path validates inside the handler
	// savings reads ~/.memdoor/savings.jsonl, a machine-local ledger with no
	// workspace in it; requiring one failed outside a pinned directory
	// (onboarding walk, 2026-09-27).
	"savings":   true,
	"uninstall": true, // tears down the install; running pre-setup must work
	"upgrade":   true, // swaps the memdoor binary; orthogonal to any workspace
	"claude":    true, // `claude hook install` writes settings.json (no workspace); `claude hook capture` + `claude import` resolve opportunistically and validate in their own RunE
	// THE SIGN-IN CANNOT REQUIRE WHAT THE SIGN-IN CREATES (paid onboarding
	// walk, 2026-09-27). `memdoor account login` is the FIRST command a person
	// runs after paying, in whatever directory they happen to be in, and it
	// answered "no workspace resolved for \"account\"" with four ways to fix a
	// problem they do not have — the workspace is named after the account this
	// very command is about to create. Exempt for the same reason as `auth`.
	"account": true,
	"login":   true, // the same sign-in under its short name
	"logout":  true, // forgetting a sign-in needs no workspace either
	"join":    true, // a /remote link: the host's session, nothing local
}

var rootCmd = &cobra.Command{
	Use:   "memdoor",
	Short: "Memdoor — a coding agent that cuts your model bill",
	// WHAT A STRANGER READS FIRST (onboarding walk, 2026-09-27). This once
	// described a product that was deleted; every line below is something
	// they will actually type.
	Long: `Memdoor is a coding agent in your terminal, on YOUR key, with a decision model
in front of the chat model to cut what it reads and what it costs. Say the steps
and it builds the workflow.

Quick start:
  export OPEN_ROUTER_API_KEY=sk-or-...   # or ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, …; or memdoor connect
  memdoor setup                          # once on this machine
  cd <your-project> && memdoor workspace use <slug> && memdoor tui

Subscribed to Pro? memdoor login you@example.com — a six-digit code
arrives there, and remote control (/remote) is on.

In the window: /model chooses which model answers, /usage is what this month
used and saved, /remote opens the conversation on your phone.`,
	// SilenceUsage stops cobra from dumping the full --help block
	// after every RunE error. Subcommands inherit this — routine
	// errors ("not authenticated", "no LLM key") print just the
	// message instead of pages of usage noise.
	SilenceUsage: true,
	// SilenceErrors stops cobra from printing the "Error: …" line
	// itself; Execute() below prints the error exactly once with our
	// own format. Without this, missing-flag errors and similar were
	// printed twice.
	SilenceErrors: true,
}

func Execute() {
	// assignCommandGroups runs after every per-command init() has
	// registered against rootCmd, so it can safely walk Commands()
	// and tag each one with its GroupID for the --help layout.
	assignCommandGroups()
	hideNonCodingCommands()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// THIS IS A CODING AGENT (Greg, 2026-09-27: "this is coding agent only",
// "less is more"). The binary grew commands for every epoch it has lived
// through — a creator funnel, workspace administration, an operator's billing
// service — and a solo developer who installed a coding agent should not have
// to read past them to find `tui`.
//
// Nothing is removed: every command below still runs, exactly as before, and is
// still in its own `--help`. They are only left out of the top-level listing.
var notCodingCommands = []string{
	// The creator funnel and workspace administration.
	"users",
	// Multi-party plumbing the window drives for you.
	"channels", "messages", "sessions",
	// Operator and recovery surfaces.
	"billing",
	// The gateway's own sign-in, by hand: `memdoor login` sets it up
	// for a person; scripts keep `auth token` / `login-direct` / `whoami`
	// (2026-10-03: three sign-in families, two identities, one broken flow).
	"auth",
	// Capabilities the agent reaches through tools, not the person through a
	// command: a browser and a scheduler.
	"chrome", "cron",
}

func hideNonCodingCommands() {
	hide := make(map[string]bool, len(notCodingCommands))
	for _, n := range notCodingCommands {
		hide[n] = true
	}
	for _, c := range rootCmd.Commands() {
		if hide[c.Name()] {
			c.Hidden = true
		}
	}
}

// requireWorkspaceSlug runs before every subcommand. It runs the
// resolution chain (flag → env → .memdoor/workspace walk-up → config
// default) and writes the result back into the package-level
// workspaceSlug var so every existing command's RunE keeps working
// unchanged. Allowlisted top-level commands (see noWorkspaceTopLevels)
// are skipped entirely — they may run with no workspace at all.
func requireWorkspaceSlug(cmd *cobra.Command, _ []string) error {
	// Walk up to the top-level subcommand. cmd.Name() for
	// "workspace use" returns "use"; we want "workspace".
	top := cmd
	for top.Parent() != nil && top.Parent() != rootCmd {
		top = top.Parent()
	}
	if top == rootCmd {
		return nil
	}
	if noWorkspaceTopLevels[top.Name()] {
		// Even for exempt commands, run resolution so subcommands
		// like `workspace which` see the same answer the rest of the
		// CLI would. Don't fail when nothing resolves.
		if slug, source, ok := resolveWorkspaceSlug(); ok {
			workspaceSlug = slug
			logResolvedWorkspace(slug, source)
		}
		return nil
	}
	// `model check` asks a model whether it can run a coder turn and reads the
	// reports in ~/.memdoor/model-checks.json. No workspace is involved, and the
	// person most likely to run it — someone deciding which model to pin before
	// they have set anything up — has none (battle test, 2026-09-27). The rest
	// of `model` shows an agent's ladder, which IS workspace-scoped.
	if top.Name() == "model" && cmd.Name() == "check" {
		if slug, source, ok := resolveWorkspaceSlug(); ok {
			workspaceSlug = slug
			logResolvedWorkspace(slug, source)
		}
		return nil
	}
	slug, source, ok := resolveWorkspaceSlug()
	if !ok {
		// The only workspace on the machine is the default one: a question
		// with a single possible answer is not a question (workspace_discovery.go).
		if only, single := soleWorkspace(); single {
			slug, source, ok = only, workspaceSource{kind: "only workspace"}, true
		}
	}
	if !ok && (top.Name() == "tui" || top.Name() == "acp") && len(listAllWorkspaceSlugs()) == 0 {
		// A machine with no workspace at all is a first run: the TUI sets
		// it up itself (tui_firstrun.go) instead of sending a stranger to
		// `memdoor setup` (stranger walk, 2026-10-08).
		return nil
	}
	if !ok {
		return noWorkspaceErrorMessage(top.Name())
	}
	workspaceSlug = slug
	logResolvedWorkspace(slug, source)
	return nil
}

// logResolvedWorkspace emits one stderr line in verbose mode when the
// workspace came from anywhere other than the explicit -w flag. The
// flag case is silent because the user already typed the slug — they
// don't need to be told which workspace they targeted.
func logResolvedWorkspace(slug string, source workspaceSource) {
	if !verbose || source.kind == "flag" {
		return
	}
	fmt.Fprintf(os.Stderr, "memdoor: workspace %q (from %s)\n", slug, source)
}

func init() {
	// MEMDOOR_GATEWAY is documented as the address the CLI talks to and was
	// read by nothing: the flag's default won, so a gateway on another port was
	// unreachable (onboarding walk, 2026-09-27). The flag still wins when given.
	defaultGateway := "http://localhost:18789"
	if v := strings.TrimSpace(os.Getenv("MEMDOOR_GATEWAY")); v != "" {
		defaultGateway = v
	}
	rootCmd.PersistentFlags().StringVar(&gatewayAddr, "gateway", defaultGateway, "Gateway server address (or MEMDOOR_GATEWAY)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&workspaceSlug, "workspace", "w", "", "Workspace slug. Optional — overrides auto-discovery (env $MEMDOOR_WORKSPACE → .memdoor/workspace walked up from cwd → ~/.memdoor/config.json). Run 'memdoor workspace which' to see what would resolve.")
	// Wired here (not on the rootCmd literal) to avoid an init cycle:
	// the closure references rootCmd to detect the top-level subcommand.
	rootCmd.PersistentPreRunE = requireWorkspaceSlug
	// After a command succeeds, print a consistent "what to do next" line
	// (see suggest.go). Cobra runs PersistentPostRunE only on RunE success.
	rootCmd.PersistentPostRunE = printNextStep
}
