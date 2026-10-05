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
	updateMu      sync.Mutex
	updateChecked time.Time
	updateNotice  string
)

// tuiUpdateNotice is "memdoor vX is out — /update installs it" when a
// newer build is published, checked at most once a day; "" otherwise. A
// "dev" binary is never nagged. Errors are silence: the check is a
// convenience, not a gate.
func tuiUpdateNotice() string {
	updateMu.Lock()
	defer updateMu.Unlock()
	if time.Since(updateChecked) < 24*time.Hour {
		return updateNotice
	}
	updateChecked = time.Now()
	updateNotice = ""
	if Version == "" || Version == "dev" {
		return ""
	}
	remote, err := fetchRemoteVersion()
	if err != nil || remote == "" || !versionNewer(remote, Version) {
		return ""
	}
	updateNotice = "memdoor " + remote + " is out — /update installs it"
	return updateNotice
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
	c := exec.Command("bash", "-c", "curl -fsSL "+installScriptURL+" | bash")
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
	updateNotice, updateChecked = "", time.Now()
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
