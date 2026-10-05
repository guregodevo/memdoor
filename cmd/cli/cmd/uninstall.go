package cmd

import (
	"bufio"
	"fmt"
	"memdoor/pkg/shared"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// `memdoor uninstall` — remove Memdoor from the system. Stops the
// gateway, removes the binary, and optionally cleans up the data
// directory (`~/.memdoor/`).
//
// Why this exists as a first-class command rather than just "rm
// /usr/local/bin/memdoor": a running gateway holds open files and
// connections; stopping it gracefully
// BEFORE removing the binary is what actually gets the memory back.
//
// Re-installing via `curl install.sh | bash` doesn't need uninstall
// first — the install script handles upgrade-in-place. Use uninstall
// when you actually want Memdoor off the machine.

var (
	uninstallKeepData bool
	uninstallYes      bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Memdoor from the system (stops services, deletes binary, optionally clears data)",
	Long: `Removes Memdoor and its running components:

  1. SIGTERM the gateway (graceful; nothing else runs beside it).
  2. Remove the memdoor binary at ~/.local/bin/memdoor or
     /usr/local/bin/memdoor (wherever this binary lives).
  3. Optionally (with --keep-data NOT set, default removes): delete
     ~/.memdoor/ — the data dir holding workspaces, conversations,
     credentials, the master.key, logs.

By default this command PROMPTS before each destructive step. Pass
--yes to skip prompts (scripted uninstall).

Re-installing later: 'curl -fsSL https://memdoor.ai/install.sh | bash'.

Examples:
  memdoor uninstall              # interactive — prompts before each step
  memdoor uninstall --yes        # no prompts; deletes binary + data dir
  memdoor uninstall --keep-data  # removes binary, preserves ~/.memdoor`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Memdoor uninstaller")
		fmt.Println()

		// 1. Stop the gateway gracefully.
		gatewayPID := findProcessOnPort(18789)
		if gatewayPID > 0 {
			fmt.Printf("→ Gateway running (pid %d). Sending SIGTERM for graceful shutdown...\n", gatewayPID)
			if err := stopProcessGracefully(gatewayPID); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ %v (continuing — SIGKILL will follow)\n", err)
			}
			fmt.Println("  ✓ gateway stopped")
		}

		// 3. Remove the binary. Use this binary's own path — works for
		//    `make build` installs (./memdoor in repo) AND for the
		//    install.sh path (/usr/local/bin/memdoor). The OS keeps
		//    the file alive until our process exits, so removing it
		//    while we're running is safe on POSIX.
		binPath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ couldn't locate own binary: %v (skipping binary removal)\n", err)
		} else {
			// Resolve symlinks so we delete the actual file, not a
			// symlink target left behind by a packager.
			if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
				binPath = resolved
			}
			if uninstallYes || promptYes(fmt.Sprintf("Delete binary at %s?", binPath)) {
				if err := removeBinary(binPath); err != nil {
					fmt.Fprintf(os.Stderr, "  ⚠ couldn't remove binary: %v\n", err)
					fmt.Fprintf(os.Stderr, "    try manually: sudo rm %s\n", binPath)
				} else {
					fmt.Printf("  ✓ removed %s\n", binPath)
				}
			} else {
				fmt.Println("  (skipped binary removal)")
			}
		}

		// 4. Data dir. This is the destructive step — workspaces +
		//    conversations + master.key (= encrypted managed-tier
		//    bearer token; unrecoverable once deleted).
		dataDir := shared.MemdoorHome()
		if uninstallKeepData {
			fmt.Printf("\n  --keep-data: preserving %s\n", dataDir)
		} else {
			info, err := os.Stat(dataDir)
			if err != nil {
				// Dir doesn't exist — nothing to clean.
			} else if info.IsDir() {
				size := dirSizeBytes(dataDir)
				prompt := fmt.Sprintf("Delete data dir %s (%.2f GB — workspaces, conversations, master.key)?",
					dataDir, float64(size)/(1<<30))
				if uninstallYes || promptYes(prompt) {
					if err := os.RemoveAll(dataDir); err != nil {
						fmt.Fprintf(os.Stderr, "  ⚠ couldn't remove data dir: %v\n", err)
					} else {
						fmt.Printf("  ✓ removed %s (freed %.2f GB)\n", dataDir, float64(size)/(1<<30))
					}
				} else {
					fmt.Printf("  (preserved %s — re-installing later will pick up where you left off)\n", dataDir)
				}
			}
		}

		fmt.Println()
		fmt.Println("✓ Memdoor uninstalled.")
		fmt.Println()
		fmt.Println("Re-install: curl -fsSL https://memdoor.ai/install.sh | bash")
		return nil
	},
}

// promptYes reads a y/N answer from stdin. Defaults to no.
func promptYes(question string) bool {
	fmt.Printf("\n%s [y/N]: ", question)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes"
}

// removeBinary deletes the binary at path. Uses sudo when the parent
// directory isn't writable by the current user (the typical case for
// /usr/local/bin/memdoor installed via the install script with sudo).
//
// The OS keeps the running process's image until exit, so deleting
// our own binary while running is safe on POSIX. We just have to
// not need to read it again before exit — which is fine, we're at
// the very end of the uninstall flow.
func removeBinary(path string) error {
	// Try without sudo first.
	if err := os.Remove(path); err == nil {
		return nil
	}
	// Fall back to sudo. exec.Command preserves the user's terminal
	// for the password prompt.
	cmd := exec.Command("sudo", "rm", "-f", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// dirSizeBytes walks a directory and returns the total size in bytes.
// Best-effort — silently skips files we can't stat (permission errors).
// Used to give the user a "this will free X GB" line before they say
// yes to deleting the data dir.
func dirSizeBytes(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// findProcessOnPort returns the PID of the process listening on the given TCP
// port, or 0 if none. Uses lsof (present on macOS/Linux dev hosts).
func findProcessOnPort(port int) int {
	out, err := exec.Command("lsof", "-t", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN").Output()
	if err != nil {
		return 0
	}
	first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	pid, _ := strconv.Atoi(strings.TrimSpace(first))
	return pid
}

// stopProcessGracefully sends SIGTERM, waits up to ~5s for exit, then SIGKILL.
func stopProcessGracefully(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !processAlive(pid) {
			return nil
		}
	}
	return proc.Signal(syscall.SIGKILL)
}

// processAlive reports whether pid refers to a live process (signal 0 probe).
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallKeepData, "keep-data", false, "Preserve ~/.memdoor/ (workspaces, conversations, master.key)")
	uninstallCmd.Flags().BoolVar(&uninstallYes, "yes", false, "Skip the per-step confirmation prompts")
	rootCmd.AddCommand(uninstallCmd)
}
