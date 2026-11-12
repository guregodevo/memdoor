package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The TUI's side of /update (cmd/tui/ui/update.go): the version check the
// footer shows, the installer, and the re-exec into the same conversation.

var (
	updateMu       sync.Mutex
	updateChecked  time.Time
	updateNotice   string
	updateSay      string
	updateChecking bool
)

// How often the window asks for the published version: when it opens and
// then daily, and after a turn ends once an hour has passed (#290). Releases
// ship several a day; a check only after a turn means an idle window left
// open overnight is not a window the next day in memdoor-ops' count.
const (
	updateEvery     = 24 * time.Hour
	updateAfterTurn = time.Hour
)

// fetchWindowVersion is the window's check; a test replaces it.
var fetchWindowVersion = fetchRemoteVersion

// tuiUpdateNotice is the footer's "memdoor vX is out (this is vY) —
// /update installs it" and the conversation's line, with the release's
// link, when a newer build is published; "" otherwise. It never waits on
// the network: a check that is due runs in the background and its answer
// shows on a later status tick. A "dev" binary is never nagged.
func tuiUpdateNotice() (footer, say string) {
	updateMu.Lock()
	defer updateMu.Unlock()
	if time.Since(updateChecked) >= updateEvery {
		startUpdateCheckLocked()
	}
	return updateNotice, updateSay
}

// updateSayLine is what the conversation says once per version (#290):
// what to type, and the release page that says what changed.
func updateSayLine(remote, local string) string {
	return fmt.Sprintf("memdoor %s is out (this is %s). /update installs it and reopens this conversation. What changed: %s",
		remote, local, releaseURL(remote))
}

// releaseURL is the GitHub release of tag, in the repository the builds
// come from (MEMDOOR_GITHUB_REPOSITORY under a GitHub install source).
func releaseURL(tag string) string {
	repo := defaultGitHubRepo
	if g, ok, err := installSource(); err == nil && ok {
		repo = g.repo
	}
	return githubBase + "/" + repo + "/releases/tag/" + tag
}

// tuiTurnEnded asks again after a turn, at most once an hour.
func tuiTurnEnded() {
	updateMu.Lock()
	defer updateMu.Unlock()
	if time.Since(updateChecked) >= updateAfterTurn {
		startUpdateCheckLocked()
	}
}

// startUpdateCheckLocked runs one check in the background. An error keeps
// the last answer: the check is a convenience, not a gate.
func startUpdateCheckLocked() {
	if updateChecking || Version == "" || Version == "dev" {
		return
	}
	updateChecking, updateChecked = true, time.Now()
	fetch, local := fetchWindowVersion, Version
	go func() {
		remote, err := fetch()
		updateMu.Lock()
		defer updateMu.Unlock()
		updateChecking = false
		switch {
		case err != nil || remote == "":
		case versionNewer(remote, local):
			updateNotice = updateLine(remote, local, "/update")
			updateSay = updateSayLine(remote, local)
		default:
			updateNotice, updateSay = "", ""
		}
	}()
}

// versionNewer reports whether remote is a later release than local. Any
// difference used to count, so a build ahead of the release (v1.330.0-3-g…,
// what `make up` installs) was told to "update" to the older v1.330.0
// (2026-09-28, TUI session). Compared on major.minor.patch; the same numbers
// are not newer, whatever the suffix.
func versionNewer(remote, local string) bool {
	r, okR := versionNumbers(remote)
	l, okL := versionNumbers(local)
	if !okR || !okL {
		return remote != local
	}
	for i := range r {
		if r[i] != l[i] {
			return r[i] > l[i]
		}
	}
	return false
}

func versionNumbers(v string) ([3]int, bool) {
	var n [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return n, false
	}
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil {
			return n, false
		}
		n[i] = x
	}
	return n, true
}

// tuiUpdate runs the installer memdoor upgrade runs (memdoor.ai/install.sh),
// output captured, and says what was installed.
func tuiUpdate() (string, error) {
	var out bytes.Buffer
	c := installerCommand()
	c.Stdout, c.Stderr = &out, &out
	c.Env = append(os.Environ(), "MEMDOOR_NONINTERACTIVE=1")
	if err := c.Run(); err != nil {
		tail := out.String()
		if len(tail) > 600 {
			tail = "…" + tail[len(tail)-600:]
		}
		return "", fmt.Errorf("installer: %v\n%s", err, strings.TrimSpace(tail))
	}
	updateMu.Lock()
	updateNotice, updateSay, updateChecked = "", "", time.Now()
	updateMu.Unlock()
	remote, _ := fetchRemoteVersion()
	if remote == "" {
		remote = "the newest build"
	}
	return "Installed " + remote + ".", nil
}

// reexecIntoConversation replaces this process with the new binary,
// resuming the conversation the window was in.
func reexecIntoConversation() error {
	// The installer writes the binary on the PATH (~/.local/bin/memdoor by
	// default); a window launched from a build elsewhere must still come
	// back as the installed one, else it relaunches the old build.
	exe, err := exec.LookPath("memdoor")
	if err != nil {
		if exe, err = os.Executable(); err != nil {
			return err
		}
	}
	return syscall.Exec(exe, []string{exe, "resume", "--last"}, os.Environ())
}
