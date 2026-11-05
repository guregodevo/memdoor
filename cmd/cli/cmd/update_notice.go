package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// A NEW VERSION IS NEVER MISSED (#90). The window says it in its footer; a
// headless command (`memdoor run`, `workflow …`, `cron`: a script, a cron
// job, CI on a box) said nothing, and a fix reached only the people who opened
// the window. Now those commands check at most once a day and print one line
// on stderr, never on stdout (so --json is unchanged). The check asks the
// same version file the window reads, marked ?from=cli so memdoor-ops'
// metrics keep counting windows only; nothing about the person is sent.

// updateCheckFile keeps the last answer: when it was asked and what it said.
const updateCheckFile = "update-check.json"

// headlessUpdateCommands are the top-level commands that say it.
var headlessUpdateCommands = map[string]bool{"run": true, "workflow": true, "cron": true}

// updateLine is the notice, the same words in the window and headless.
func updateLine(remote, local, how string) string {
	return fmt.Sprintf("memdoor %s is out (this is %s) — %s installs it", remote, local, how)
}

type updateCheck struct {
	Checked time.Time `json:"checked"`
	Remote  string    `json:"remote"`
}

// fetchHeadlessVersion is the check itself; a test replaces it.
var fetchHeadlessVersion = func() (string, error) { return fetchRemoteVersionFrom("cli", 3*time.Second) }

// headlessUpdateNotice is the line for cmd, "" when it says nothing: not a
// headless command, a dev build, no newer release, or no answer. It asks the
// network at most once a day, whatever the answer.
func headlessUpdateNotice(cmd *cobra.Command, now time.Time) string {
	if Version == "" || Version == "dev" || !headlessUpdateCommands[topCommand(cmd)] {
		return ""
	}
	var last updateCheck
	if b, err := os.ReadFile(homePath(updateCheckFile)); err == nil {
		_ = json.Unmarshal(b, &last)
	}
	if now.Sub(last.Checked) >= 24*time.Hour {
		remote, err := fetchHeadlessVersion()
		if err != nil {
			remote = ""
		}
		last = updateCheck{Checked: now, Remote: remote}
		if b, err := json.Marshal(last); err == nil {
			_ = writeHomeFile(updateCheckFile, b, 0o600)
		}
	} else {
		// Said once a day: the answer kept is not said again within it.
		return ""
	}
	if last.Remote == "" || !versionNewer(last.Remote, Version) {
		return ""
	}
	return updateLine(last.Remote, Version, "memdoor upgrade")
}

// topCommand is the subcommand right under the root ("run" for
// `memdoor run …`, "workflow" for `memdoor workflow status …`).
func topCommand(cmd *cobra.Command) string {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Parent() != nil && c.Parent().Parent() == nil {
			return c.Name()
		}
	}
	return ""
}
