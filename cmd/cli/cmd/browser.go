package cmd

import (
	"os/exec"
)

// openInBrowser opens u in the default browser (macOS `open`).
func openInBrowser(u string) error {
	return exec.Command("open", u).Start()
}
