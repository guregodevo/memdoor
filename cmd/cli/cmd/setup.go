package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"memdoor/gateway/config"
	"memdoor/pkg/shared"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	setupWorkspace      string
	setupSkipBootstrap  bool
	setupAdminEmail     string
	setupAdminPassword  string
	setupWorkspaceName  string
	setupNonInteractive bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Onboard: workspace + first admin user",
	Long: `Onboard a fresh Memdoor install: create the workspace and the
first admin user in one go.

Phase 1 (always, no gateway required): create the on-disk workspace
directory tree, default config, bootstrap files
(AGENTS.md / SOUL.md / TOOLS.md), and 'git init' the workspace dir.

Phase 2 (needs a gateway — setup auto-starts one if it isn't already
running): register the first admin user via the public /api/setup
endpoint and auto-login. Credentials are saved to
~/.memdoor/credentials.json and stay valid for 30 days across gateway
restarts.

Modes:
  - Interactive (default on a TTY): prompts for workspace name,
    admin email, and admin password (input hidden, with confirm).
  - Non-interactive: pass '--admin-email', '--admin-password' (and
    optionally '--workspace-name'). Suitable for scripts/CI.
  - '--non-interactive' forces flag-only mode and never prompts.

Examples:
  # Interactive onboarding (recommended for first-time install).
  # Setup auto-starts the gateway — no 'memdoor gateway &' step needed.
  memdoor setup

  # Scripted: dirs + admin registration in one shot.
  memdoor setup \
    --workspace-name 'my-project' \
    --admin-email you@work.com \
    --admin-password 'sekret-not-shared-1234'

  # Phase 1 only (gateway not yet running): use --non-interactive.
  memdoor setup --non-interactive
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if setupWorkspace == "" {
			setupWorkspace = shared.MemdoorHome("workspace")
		}

		// Auto-start the gateway if it's not already running.
		// Removes the "memdoor gateway & then memdoor setup" two-step
		// from the install path. Best-effort — failure surfaces as a
		// "gateway not reachable" warning later in the flow, same as
		// before this autostart existed.
		ensureGatewayRunning()

		configPath, err := setupLocalDirs(setupWorkspace)
		if err != nil {
			return err
		}

		// Phase 2: register the first admin user.
		//
		// Three paths:
		//   1. Both --admin-email and --admin-password supplied: scripted, run directly.
		//   2. Exactly one of them supplied: error — both required together.
		//   3. Neither supplied: interactive prompts (TTY) OR print next-steps (no TTY / --non-interactive).
		flagsProvided := setupAdminEmail != "" || setupAdminPassword != ""
		if flagsProvided {
			if setupAdminEmail == "" || setupAdminPassword == "" {
				return fmt.Errorf("--admin-email and --admin-password must both be provided")
			}
			wsSlug, err := registerAdminViaSetup(setupAdminEmail, setupAdminPassword, setupWorkspaceName)
			if err != nil {
				return err
			}
			persistWorkspaceSlug(configPath, wsSlug)
			offerLocalModel(wsSlug)
			printPostSetupNextSteps(wsSlug)
			return nil
		}

		// Interactive onboarding: only when stdin is a real terminal AND
		// the user hasn't asked for non-interactive mode AND the gateway
		// is reachable. Otherwise fall through to next-steps.
		if !setupNonInteractive && term.IsTerminal(int(os.Stdin.Fd())) {
			if gatewayReachable() {
				email, password, wsName, err := promptOnboarding(setupWorkspaceName)
				if err != nil {
					return err
				}
				wsSlug, err := registerAdminViaSetup(email, password, wsName)
				if err != nil {
					return err
				}
				persistWorkspaceSlug(configPath, wsSlug)
				offerLocalModel(wsSlug)
				printPostSetupNextSteps(wsSlug)
				return nil
			}
			fmt.Println("\n  Gateway not reachable — skipping admin registration.")
			fmt.Println("  Start it with 'memdoor gateway &', then re-run 'memdoor setup'.")
		}

		fmt.Println("\nNext steps:")
		fmt.Println("  1. Start the gateway:  memdoor gateway &")
		fmt.Println("  2. Re-run onboarding:  memdoor setup")
		fmt.Println("")
		fmt.Println("  (Scripted alternative: memdoor setup --admin-email <email>")
		fmt.Println("   --admin-password <pass> --workspace-name <name>)")
		fmt.Println("")
		// No `memdoor workspace use` hint here — the gateway didn't
		// run, so the workspace_slug the server picks is still unknown.
		// `memdoor workspace which` after the next setup run prints
		// what to pin.

		return nil
	},
}

// setupLocalDirs is phase 1 of setup: ~/.memdoor, its config.json, the
// bootstrap workspace directory. Idempotent, so `memdoor login`
// runs it silently on a Mac that has never seen Memdoor, and `memdoor setup`
// runs it in the open. Returns the config path phase 2 back-fills.
func setupLocalDirs(setupWorkspace string) (string, error) {
	baseDir := filepath.Dir(setupWorkspace)
	fmt.Println("Initializing Memdoor workspace...")

	// Create directory structure
	dirs := []string{
		baseDir,
		filepath.Join(baseDir, "data"),
		filepath.Join(baseDir, "logs"),
		filepath.Join(baseDir, "cron"),
		filepath.Join(baseDir, "emails"),
		setupWorkspace,
		filepath.Join(setupWorkspace, "memory"),
		filepath.Join(setupWorkspace, "skills"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	fmt.Println("  Created directory structure")

	// Create default config if not exists
	configPath := filepath.Join(baseDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		if cfg.Agents == nil {
			cfg.Agents = &config.AgentsConfig{}
		}
		if cfg.Agents.Defaults == nil {
			cfg.Agents.Defaults = &config.AgentDefaultsConfig{}
		}
		cfg.Agents.Defaults.Workspace = setupWorkspace
		// The top-level WorkspaceID holds the workspace SLUG (not the dir
		// path) — it's what resolveWorkspaceSlug()'s config fallback returns,
		// so after setup the user's single workspace auto-resolves and -w is
		// never required. The authoritative
		// slug comes from /api/setup (phase 2), so it can't be known here at
		// phase-1 config creation; persistWorkspaceSlug() back-fills it after
		// registration. (Writing setupWorkspace — a filesystem PATH — here was
		// a bug: every post-setup command that resolved the workspace from
		// config then built malformed API URLs and 404'd.)

		data, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return "", fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Printf("  Created config: %s\n", configPath)
	} else {
		fmt.Printf("  Config exists: %s\n", configPath)
	}

	// Create bootstrap files
	if !setupSkipBootstrap {
		bootstrapFiles := map[string]string{
			"AGENTS.md": "# Agents\n\nList of configured agents for this workspace.\n",
			"SOUL.md":   "# Soul\n\nPersonality and behavior guidelines for agents.\n",
			"TOOLS.md":  "# Tools\n\nAvailable tools and their descriptions.\n",
		}
		for name, content := range bootstrapFiles {
			path := filepath.Join(setupWorkspace, name)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					return "", fmt.Errorf("failed to create %s: %w", name, err)
				}
			}
		}
		fmt.Println("  Created bootstrap files")
	}

	// Initialize git repo in workspace
	if _, err := os.Stat(filepath.Join(setupWorkspace, ".git")); os.IsNotExist(err) {
		gitCmd := exec.Command("git", "init", setupWorkspace)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		_ = gitCmd.Run()
		fmt.Println("  Initialized git repository")
	}

	fmt.Println("\n  Workspace ready at: " + setupWorkspace)
	return configPath, nil
}

// promptOnboarding asks for a workspace name, the admin's email and password
// (twice, hidden). workspaceDefault is offered when --workspace-name was passed.
func promptOnboarding(workspaceDefault string) (email, password, workspaceName string, err error) {
	fmt.Println("")
	fmt.Println("Memdoor onboarding")
	fmt.Println("──────────────────")
	fmt.Println("This sets up your workspace and the first admin user.")
	fmt.Println("")

	reader := bufio.NewReader(os.Stdin)

	defaultName := workspaceDefault
	if defaultName == "" {
		defaultName = "Memdoor"
	}
	fmt.Printf("Workspace name [%s]: ", defaultName)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", "", "", fmt.Errorf("read workspace name: %w", err)
	}
	workspaceName = strings.TrimSpace(line)
	if workspaceName == "" {
		workspaceName = defaultName
	}

	for {
		fmt.Print("Admin email: ")
		line, err = reader.ReadString('\n')
		if err != nil {
			return "", "", "", fmt.Errorf("read email: %w", err)
		}
		email = strings.TrimSpace(line)
		if email == "" {
			fmt.Println("  Email is required.")
			continue
		}
		if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
			fmt.Println("  That doesn't look like an email address.")
			continue
		}
		break
	}

	for {
		fmt.Print("Admin password (min 8 chars, hidden): ")
		pw, perr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println("")
		if perr != nil {
			return "", "", "", fmt.Errorf("read password: %w", perr)
		}
		if len(pw) < 8 {
			fmt.Println("  Password must be at least 8 characters.")
			continue
		}
		fmt.Print("Confirm password: ")
		confirm, cerr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println("")
		if cerr != nil {
			return "", "", "", fmt.Errorf("read confirm password: %w", cerr)
		}
		if string(pw) != string(confirm) {
			fmt.Println("  Passwords don't match — try again.")
			continue
		}
		password = string(pw)
		break
	}

	fmt.Println("")
	return email, password, workspaceName, nil
}

// ensureGatewayRunning checks if the gateway is reachable, and if
// not, starts it in the background using the same binary that's
// running this command. Removes the "memdoor gateway & then memdoor
// setup" two-step from the install path — single command does both.
//
// Best-effort. If the spawn fails (binary path lookup error, port
// already bound, etc.) the rest of setup continues and the gateway-
// reachable check inside the interactive flow surfaces the failure
// with the "make start, then re-run setup" hint that already exists.
func ensureGatewayRunning() {
	if gatewayReachable() {
		return
	}
	binary, err := os.Executable()
	if err != nil {
		return
	}
	logDir := shared.MemdoorHome()
	_ = os.MkdirAll(logDir, 0o755)
	logF, err := os.Create(filepath.Join(logDir, "gateway.log"))
	if err != nil {
		return
	}
	port, local := localGatewayPort()
	if !local {
		// A gateway elsewhere (MEMDOOR_GATEWAY, --gateway) is not this
		// machine's to start; starting one on 18789 instead answered a
		// different address than the one the CLI was told to use.
		_ = logF.Close()
		return
	}
	cmd := exec.Command(binary, "gateway", "--port", port)
	cmd.Stdout = logF
	cmd.Stderr = logF
	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		return
	}
	// Don't wait on the daemon — let it run detached. cmd.Process
	// keeps the OS handle but Go's exec doesn't auto-reap; calling
	// .Release() lets the gateway outlive setup cleanly.
	if cmd.Process != nil {
		// The pid marks this gateway as one Memdoor started, the only kind
		// the TUI may restart (to hand it a key exported after it started).
		_ = os.WriteFile(shared.MemdoorHome(gatewayPidFile), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600)
		_ = cmd.Process.Release()
	}
	fmt.Fprintln(os.Stderr, "  → starting gateway on :"+port+" (background)")
	// Give it a moment to bind the port and run migrations before
	// the rest of setup hits /api/setup/status.
	for i := 0; i < 30; i++ {
		if gatewayReachable() {
			fmt.Fprintln(os.Stderr, "  ✓ gateway ready")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "  ⚠ gateway didn't come up in 15s — see ~/.memdoor/gateway.log")
}

// gatewayReachable probes /api/setup/status to see if the gateway is up.
// Used to decide whether interactive onboarding can proceed.
func gatewayReachable() bool {
	c := NewClient()
	var status struct {
		Initialized bool `json:"initialized"`
	}
	return c.GetJSON("/api/setup/status", &status) == nil
}

func printPostSetupNextSteps(workspaceSlug string) {
	fmt.Println("\n✓ Setup complete.")
	fmt.Print(nextStepsText(workspaceSlug))
}

// nextStepsText is the last thing setup says.
//
// It used to point at prepaid credits and commands from a product that no
// longer exists: a stranger who had just installed a coding agent was sent
// there (onboarding walk, 2026-09-27). Five lines now, and every one of them
// is something they will actually type.
func nextStepsText(slug string) string {
	if slug == "" {
		slug = "<slug>"
	}
	// In the order they will be typed, and nothing else.
	return fmt.Sprintf(`
Next steps:
  export OPEN_ROUTER_API_KEY=sk-or-...   your key, your bill, at list price (or any provider's key; or memdoor connect)
  cd <your-project> && memdoor workspace use %s
  memdoor tui                            it reads, patches and tests in there

  /model what answers · /usage what it saved · /remote this on your phone.
  The decision model judges on the same key. Workflows run free here, schedules too: memdoor cron add --workflow <name>.
`, slug)
}

// registerAdminViaSetup calls the public /api/setup endpoint to provision
// the first admin user, then saves the returned auth token so subsequent
// CLI commands run authenticated. /api/setup is one-time and refuses if
// the workspace already has any users — when that happens, we surface a
// hint to use `memdoor users create` instead.
//
// Returns the canonical workspace_slug the server picked. Slugification
// happens server-side (one source of truth) and the slug is what every
// downstream `-w` flag and discovery file should carry.
func registerAdminViaSetup(email, password, workspaceName string) (string, error) {
	c := NewClient()

	var status struct {
		Initialized   bool   `json:"initialized"`
		WorkspaceSlug string `json:"workspace_slug"`
	}
	if err := c.GetJSON("/api/setup/status", &status); err != nil {
		return "", fmt.Errorf("gateway not reachable — start it with 'memdoor gateway &' then re-run with --admin-email/--admin-password (%w)", err)
	}
	if status.Initialized {
		fmt.Println("\n  Workspace already initialized — skipping admin registration.")
		fmt.Println("  To add another admin account, log in first then run:")
		fmt.Println("    memdoor users create --email <email> --password <pass> --admin")
		// Return whatever workspace slug status reported so the caller's
		// downstream LLM-step still targets the right workspace.
		return status.WorkspaceSlug, nil
	}

	if workspaceName == "" {
		workspaceName = "Memdoor"
	}

	body := map[string]interface{}{
		"workspace_name": workspaceName,
		"email":          email,
		"password":       password,
	}

	var result struct {
		Token         string `json:"token"`
		WorkspaceSlug string `json:"workspace_slug"`
		User          struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"user"`
	}
	if err := c.PostJSON("/api/setup", body, &result); err != nil {
		return "", fmt.Errorf("admin registration failed: %w", err)
	}

	creds := &credentials{
		Token: result.Token,
		Email: result.User.Email,
	}
	if err := saveCredentials(creds); err != nil {
		return result.WorkspaceSlug, fmt.Errorf("registered but failed to save credentials: %w", err)
	}

	fmt.Printf("\n  Registered admin: %s\n", email)
	fmt.Println("  Logged in — credentials saved.")
	return result.WorkspaceSlug, nil
}

// persistWorkspaceSlug back-fills config.json's workspace_id with the
// authoritative workspace SLUG from /api/setup, so post-setup commands
// auto-resolve the workspace without -w. Phase-1 config creation can't set it
// (the slug isn't known until registration), so this runs after. Best-effort:
// on any failure the user simply passes -w until they pin a workspace.
func persistWorkspaceSlug(configPath, slug string) {
	if slug == "" {
		return
	}
	cfg, err := config.LoadConfigFromFile(configPath)
	if err != nil {
		return
	}
	cfg.WorkspaceID = slug
	_ = config.SaveConfigToFile(cfg, configPath)
}

func init() {
	setupCmd.Flags().StringVar(&setupWorkspace, "workspace", "", "Workspace directory path")
	setupCmd.Flags().BoolVar(&setupSkipBootstrap, "skip-bootstrap", false, "Skip creating bootstrap files")
	setupCmd.Flags().StringVar(&setupAdminEmail, "admin-email", "", "Email for the first admin user (skips interactive prompt)")
	setupCmd.Flags().StringVar(&setupAdminPassword, "admin-password", "", "Password for the first admin user (min 8 chars)")
	setupCmd.Flags().StringVar(&setupWorkspaceName, "workspace-name", "", "Display name for the workspace (defaults to 'Memdoor')")
	setupCmd.Flags().BoolVar(&setupNonInteractive, "non-interactive", false, "Never prompt — Phase 1 only unless --admin-email/--admin-password are provided")
	rootCmd.AddCommand(setupCmd)
}

// offerLocalModel says how to get a model when setup finds no key: a
// provider key, or memdoor connect. Memdoor is API-only (2026-10-03).
func offerLocalModel(workspaceSlug string) {
	if strings.TrimSpace(os.Getenv("OPEN_ROUTER_API_KEY")) != "" {
		fmt.Println("  Running on your own key.")
		return
	}
	fmt.Println("")
	fmt.Println("  No provider key here yet. Either:")
	fmt.Println("    export OPEN_ROUTER_API_KEY=sk-or-...   your key, your bill, at list price (or ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, …)")
	fmt.Println("    memdoor connect                        pick a provider, paste its key; it is probed, then kept")
}
