// Version metadata baked into the binary at build time via -ldflags.
// The Makefile (gateway/Makefile build) and scripts/deploy.sh both
// pass `-X 'memdoor/cmd/cli/cmd.<var>=<value>'` so a release binary
// can answer "what am I?" without needing the source tree.
//
// Defaults stay at "dev" / "" so `go build` without ldflags still
// produces a working binary — useful for `go run` and dev rebuilds.
// Production binaries shipped via memdoor.ai/dl/* always carry a
// real git-describe string.
package cmd

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// Version is the git-describe of the build (e.g. "v1.466.0-wiki-12-gd3fc10c"
// or "d3fc10c" when there's no recent tag). "dev" when built without
// ldflags. `memdoor upgrade` compares this string against the VERSION
// file at memdoor.ai/dl/VERSION to decide whether a newer binary is
// available.
var Version = "dev"

// Commit is the short SHA of the build. Redundant with Version's
// suffix when at a tagged release, but always populated for the
// "what's actually on disk" reading at debug time.
var Commit = ""

// BuildDate is the UTC timestamp of the build (RFC3339). Useful for
// "is my binary from yesterday or last month" without parsing the
// git tree.
var BuildDate = ""

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the memdoor version, build commit, and build date",
	Long: `Prints the version string baked into this binary at build
time. Use this when reporting a bug, comparing two installs, or
deciding whether to run 'memdoor upgrade'.

Output shape:
  memdoor <version> (<commit>, built <date>, <go-version> <os/arch>)

"dev" in the version slot means the binary was built without the
ldflags wiring — typically a 'go run' or a developer rebuild on a
branch that doesn't go through gateway/Makefile.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		if asJSON {
			fmt.Printf(`{"version":%q,"commit":%q,"build_date":%q,"go":%q,"platform":%q}`+"\n",
				Version, Commit, BuildDate,
				runtime.Version(),
				runtime.GOOS+"/"+runtime.GOARCH,
			)
			return nil
		}
		fmt.Println(humanVersion())
		return nil
	},
}

// humanVersion is the shared human-readable build string, used by both the
// `version` subcommand and the root `--version` flag so they never drift.
func humanVersion() string {
	var b strings.Builder
	fmt.Fprintf(&b, "memdoor %s", Version)
	if Commit != "" && Commit != Version {
		fmt.Fprintf(&b, " (%s", Commit)
		if BuildDate != "" {
			fmt.Fprintf(&b, ", built %s", BuildDate)
		}
		fmt.Fprintf(&b, ")")
	} else if BuildDate != "" {
		fmt.Fprintf(&b, " (built %s)", BuildDate)
	}
	fmt.Fprintf(&b, "\n  %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return b.String()
}

func init() {
	versionCmd.Flags().Bool("json", false, "Emit version info as a single JSON line — for scripts and 'memdoor doctor'-style probes")
	rootCmd.AddCommand(versionCmd)

	// Make `memdoor --version` work (the first thing most users type),
	// printing the same string as the `version` subcommand. `-v` is taken
	// by --verbose, so cobra registers the long flag only — no clash.
	rootCmd.Version = humanVersion()
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}
