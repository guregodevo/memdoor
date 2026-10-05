// `memdoor upgrade` — check memdoor.ai/dl/VERSION, compare to the
// version baked into this binary, run install.sh if newer.
//
// Why a thin wrapper around install.sh rather than re-implementing
// the swap logic here: scripts/install.sh already owns the gnarly
// part — detect existing install, SIGTERM the gateway, SIGKILL fallback, strip macOS
// quarantine, sudo/no-sudo branching. Forking that into Go would
// double the surface and split the source of truth. Better to keep
// install.sh canonical and have this command answer the "is there a
// newer build?" question, then delegate.
package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// versionEndpoint is the canonical place deploy.sh publishes the
// git-describe of the latest production build. Plain text, one line,
// no trailing whitespace. Kept as a var (not const) so tests / staging
// can override without recompiling.
var versionEndpoint = "https://memdoor.ai/dl/VERSION"

// installScriptURL is the curl|bash entrypoint install.sh publishes
// to. Same script handles fresh-install and upgrade-in-place (see
// scripts/install.sh, the "Upgrade-in-place" block).
var installScriptURL = "https://memdoor.ai/install.sh"

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Check for a newer memdoor build and install it via memdoor.ai/install.sh",
	Long: `Compares the version baked into this binary (see 'memdoor
version') against the latest published build at memdoor.ai/dl/VERSION.
When a newer build is available, this command pipes
memdoor.ai/install.sh into bash — same upgrade-in-place path the
curl|bash installer uses, including the SIGTERM-then-swap dance that
stops the running gateway before replacing the binary.

Flags:
  --check     Print the version delta and exit 0 if up-to-date / exit 1
              if an upgrade is available. No download, no install.
              Suitable for cron / shell prompts.
  --force     Run the installer even when the local version matches
              remote — useful for re-applying after a botched swap
              or when the version string says "dev" (locally built).
  --yes       Skip the interactive confirm before running the installer.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkOnly, _ := cmd.Flags().GetBool("check")
		force, _ := cmd.Flags().GetBool("force")
		yes, _ := cmd.Flags().GetBool("yes")

		remote, err := fetchRemoteVersion()
		if err != nil {
			return fmt.Errorf("fetch %s: %w", versionEndpoint, err)
		}
		local := Version

		fmt.Printf("Local:  %s\n", local)
		fmt.Printf("Remote: %s\n", remote)

		// "dev" or empty is always considered out-of-date for the
		// purposes of the version check — a locally-built binary
		// without ldflags has no provenance to compare against, so
		// the safe answer is "yes, you can upgrade to a real build".
		// --force keeps the door open for re-installing the same
		// version (e.g. after a botched swap that left a corrupted
		// binary on disk).
		isDev := local == "" || local == "dev"
		upToDate := !isDev && local == remote

		if upToDate && !force {
			fmt.Println("✓ Already on latest.")
			return nil
		}

		if checkOnly {
			if upToDate {
				return nil
			}
			fmt.Println("→ Upgrade available. Run 'memdoor upgrade' to install.")
			os.Exit(1)
		}

		if isDev {
			fmt.Println("Note: local version is \"dev\" — installer will replace this development binary with the published build.")
		}

		if !yes {
			fmt.Print("Proceed with install? [Y/n]: ")
			var ans string
			fmt.Scanln(&ans)
			ans = strings.TrimSpace(strings.ToLower(ans))
			if ans != "" && ans != "y" && ans != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		// Delegate to install.sh. Stream output through so the user
		// sees the same progress they'd get from the curl|bash flow.
		// install.sh owns every post-swap step, so every install path heals
		// the same way, including `curl|bash install.sh` users who never
		// invoke `memdoor upgrade`.
		fmt.Println("→ Running installer (curl -fsSL " + installScriptURL + " | bash)…")
		pipe := fmt.Sprintf("curl -fsSL %s | bash", installScriptURL)
		c := exec.Command("bash", "-c", pipe)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin
		if err := c.Run(); err != nil {
			return err
		}
		return nil
	},
}

// fetchRemoteVersion reads the one-line plaintext VERSION file the
// deploy publishes alongside the binary. Trims surrounding whitespace
// since CDNs and editors occasionally add a trailing newline.
func fetchRemoteVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(versionEndpoint)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(body))
	if v == "" {
		return "", fmt.Errorf("VERSION endpoint returned empty body")
	}
	return v, nil
}

func init() {
	upgradeCmd.Flags().Bool("check", false, "Print the version delta and exit 0/1 — no download, no install")
	upgradeCmd.Flags().Bool("force", false, "Run the installer even when local matches remote")
	upgradeCmd.Flags().Bool("yes", false, "Skip the interactive confirm before running the installer")
	rootCmd.AddCommand(upgradeCmd)
}
