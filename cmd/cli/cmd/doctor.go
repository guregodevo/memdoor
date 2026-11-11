package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"memdoor/gateway/config"
	"memdoor/pkg/shared"

	"github.com/spf13/cobra"
)

var doctorWorkspace string

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Is the gateway up and healthy, and what this machine is missing",
	RunE: func(cmd *cobra.Command, args []string) error {
		// A FRESH MACHINE IS NOT A SICK ONE (battle test, 2026-09-27). Run on a
		// machine where nothing has been set up yet, doctor answered with a
		// column of X's — config file missing, workspace directory missing —
		// which reads as a broken install to the one person most likely to type
		// it: someone who just installed the binary and is checking whether it
		// worked. Nothing is wrong; nothing has happened yet. Say that, and the
		// one command that changes it.
		if nothingSetUpYet() {
			fmt.Println("Nothing is set up on this machine yet — which is what a fresh install looks like.")
			fmt.Println()
			fmt.Println("  export OPENROUTER_API_KEY=sk-or-...    (or ANTHROPIC_API_KEY, DEEPSEEK_API_KEY, …; or memdoor connect)")
			fmt.Println("  cd <your-project> && memdoor           the first run sets this machine up")
			fmt.Println()
			fmt.Println("Workflows run free here; schedule one with `memdoor cron add --workflow <name>`.")
			return nil
		}
		fmt.Println("Memdoor Doctor - Workspace Health Check")
		fmt.Println()

		// Lead with the binary's identity so bug reports include
		// the version without an extra `memdoor version` call. A
		// bare SHA here means the build ran from an untagged commit
		// — deploy.sh auto-tags HEAD now, so this should be a real
		// release string on anything pulled via `memdoor upgrade`.
		fmt.Println("Binary")
		fmt.Printf("  Version: %s\n", Version)
		if Commit != "" && Commit != Version {
			fmt.Printf("  Commit:  %s\n", Commit)
		}
		if BuildDate != "" {
			fmt.Printf("  Built:   %s\n", BuildDate)
		}
		fmt.Println()

		baseDir := shared.MemdoorHome()

		if doctorWorkspace == "" {
			doctorWorkspace = filepath.Join(baseDir, "workspace")
		}
		fmt.Printf("  Workspace: %s\n\n", doctorWorkspace)

		allGood := true

		// Check config
		fmt.Println("Configuration")
		configPath, err := config.ResolveConfigPath()
		if err != nil {
			fmt.Printf("  X Config path resolution failed: %v\n", err)
			allGood = false
		} else {
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				fmt.Printf("  X Config file missing: %s\n", configPath)
				allGood = false
			} else {
				fmt.Printf("  OK Config file exists: %s\n", configPath)

				cfg, err := config.LoadConfig()
				if err != nil {
					fmt.Printf("  X Config load failed: %v\n", err)
					allGood = false
				} else {
					fmt.Println("  OK Config is valid")
					if cfg.Agents != nil && len(cfg.Agents.List) > 0 {
						fmt.Printf("  OK %d agent(s) configured\n", len(cfg.Agents.List))
					}
				}
			}
		}
		fmt.Println()

		// Check workspace
		fmt.Println("Workspace Structure")
		if _, err := os.Stat(doctorWorkspace); os.IsNotExist(err) {
			fmt.Println("  X Workspace directory missing")
			allGood = false
		} else {
			fmt.Println("  OK Workspace directory exists")
			for _, file := range []string{"AGENTS.md", "SOUL.md"} {
				path := filepath.Join(doctorWorkspace, file)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					fmt.Printf("  X %s missing\n", file)
					allGood = false
				} else {
					fmt.Printf("  OK %s present\n", file)
				}
			}
			memDir := filepath.Join(doctorWorkspace, "memory")
			if _, err := os.Stat(memDir); os.IsNotExist(err) {
				fmt.Println("  X Memory directory missing")
				allGood = false
			} else {
				fmt.Println("  OK Memory directory exists")
			}
		}
		fmt.Println()

		// Check database
		fmt.Println("Database")
		dbPath := filepath.Join(baseDir, "data", "memdoor.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			fmt.Println("  -- No database yet: the gateway creates it on its first start")
		} else {
			info, _ := os.Stat(dbPath)
			fmt.Printf("  OK Database exists (%d KB)\n", info.Size()/1024)
		}
		fmt.Println()

		// Check gateway connectivity
		fmt.Println("Gateway")
		c := NewClient()
		var st struct {
			Version string `json:"version"`
		}
		if err := c.GetJSON("/api/status", &st); err != nil {
			port, local := localGatewayPort()
			switch {
			case local && portAnswers(port):
				fmt.Printf("  X Port %s is taken by something that is not a Memdoor gateway\n", port)
				allGood = false
			case local && serviceState() == "":
				// No service: the gateway ends with the terminal that started
				// it, and the next memdoor starts it again. Not running is the
				// state between windows, not a fault (2026-11-11: an X right
				// after a window closed).
				fmt.Println("  -- Not running now: the next `memdoor` starts it")
			default:
				fmt.Println("  X Gateway not reachable: the bare `memdoor` starts it, or `memdoor gateway`")
				allGood = false
			}
		} else {
			fmt.Printf("  OK Gateway is running (%s)\n", st.Version)
		}
		if state := serviceState(); state != "" {
			_, manager, _ := servicePath()
			fmt.Printf("  OK Service %s (%s): up at boot, restarted if it stops\n", state, manager)
		} else if _, local := localGatewayPort(); local {
			fmt.Println("  -- No service: the gateway ends with your login session (memdoor gateway install keeps it up on a box that stays on)")
		}
		fmt.Println()

		if allGood {
			fmt.Println("All checks passed!")
		} else {
			fmt.Println("Some checks failed: the lines marked X say what. A missing file or directory: memdoor setup.")
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().StringVar(&doctorWorkspace, "workspace-dir", "", "The workspace directory to check (default ~/.memdoor/workspace)")
	rootCmd.AddCommand(doctorCmd)
}

// nothingSetUpYet is a machine where setup has never run: no config file and no
// workspace directory. One or the other missing is a real fault worth the full
// report; neither is simply a new install.
func nothingSetUpYet() bool {
	configPath, err := config.ResolveConfigPath()
	if err != nil {
		return false
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		return false
	}
	baseDir := shared.MemdoorHome()
	ws := doctorWorkspace
	if ws == "" {
		ws = filepath.Join(baseDir, "workspace")
	}
	_, err = os.Stat(ws)
	return os.IsNotExist(err)
}
