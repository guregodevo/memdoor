package cmd

import (
	"bufio"
	"fmt"
	"io"

	"os/exec"
	"path/filepath"
	"strings"
)

// The nightly line: team-backlog on a cron, one open issue a night.
const nightlySchedule = "0 2 * * *"

// githubRepoAt is the directory when it is inside a git repository whose
// origin is on GitHub and gh is signed in: where team-backlog can work.
// Empty otherwise, and the nightly offer is not shown: setup run from a
// home directory, or a repository gh cannot read or push, is not a backlog.
func githubRepoAt(dir string) string {
	if dir == "" {
		return ""
	}
	top, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	origin, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil || !strings.Contains(string(origin), "github.com") {
		return ""
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return ""
	}
	if err := exec.Command("gh", "auth", "status").Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(string(top))
}

// offerNightly is setup's last offer, in a repository on GitHub (#172): the
// cron that works its open issues at night, team-solve on one issue a night,
// a pull request for each fix the Check passed, at most five of its own open.
// Nobody adds a cron they are not shown. Declining is one key, and the
// default: a schedule that opens pull requests on a repository is accepted
// on purpose. Accepted, it is the same schedule cron add would write.
func offerNightly(in *bufio.Reader, out io.Writer, repo string) error {
	if repo == "" {
		return nil
	}
	fmt.Fprintf(out, "\nWork %s's open issues at night? One a night: team-solve on it, a pull request when its Check passed, a comment on the issue when not; at most 5 of its pull requests open; about $0.02 an issue on DeepSeek V4.1 Flash. [y/N] ", filepath.Base(repo))
	line, _ := in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		if err := addCronJob(NewClient(), "team-backlog", nightlySchedule, "/workflow run team-backlog", repo, "", true); err != nil {
			return fmt.Errorf("the nightly schedule: %w (later: memdoor cron add --workflow team-backlog %q)", err, nightlySchedule)
		}
		fmt.Fprintf(out, "  Nightly at 02:00: team-backlog in %s. memdoor cron list shows it, memdoor cron remove team-backlog ends it.\n", repo)
		return nil
	}
	fmt.Fprintf(out, "  Later, in the repository: memdoor cron add --workflow team-backlog %q\n", nightlySchedule)
	return nil
}
