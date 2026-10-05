package cmd

import (
	"os/exec"
)

// openInBrowser opens u in the default browser (macOS `open`).
func openInBrowser(u string) error {
	return exec.Command("open", u).Start()
}

// workspaceArg is the workspace named on the command line, else the one
// that resolves here, else "".
func workspaceArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	if slug, _, ok := resolveWorkspaceSlug(); ok {
		return slug
	}
	return ""
}
