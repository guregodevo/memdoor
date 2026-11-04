package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"memdoor/pkg/shared"
)

// janisReleaseURL is where janis's latest release keeps its files: the
// installer, each build, and each build's sha256.
const janisReleaseURL = "https://github.com/guregodevo/janis/releases/latest/download/"

// janisRecordFile keeps the checksum of the janis build memdoor last
// installed. A Mac build is a tarball, so the binary's own checksum says
// nothing about the release; the record does.
const janisRecordFile = "janis-release.sha256"

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// janisAsset is the release file janis's installer fetches on goos/goarch,
// "" where janis has no build.
func janisAsset(goos, goarch string) string {
	switch {
	case goos == "linux" && (goarch == "amd64" || goarch == "arm64"):
		return "janis-linux-" + goarch
	case goos == "darwin" && goarch == "arm64":
		return "janis-darwin-arm64.tar.gz"
	}
	return ""
}

// janisRelease compares the janis about to be started with the latest
// release. A zero value checks nothing.
type janisRelease struct {
	home      string                   // where janis's installer puts the binary
	installed func(path string) string // the release checksum of the janis at path, "" when unknown
	latest    func() string            // the latest release's checksum, "" when unreachable
	record    func(sum string)         // keeps the checksum of what the installer put in place
	say       func(string)
}

// janisVerdict is what to do with the janis found on PATH.
type janisVerdict int

const (
	janisKeep  janisVerdict = iota // current, or nothing to compare it with
	janisRenew                     // older, and where the installer puts it: run the installer
	janisTell                      // older, put there some other way: say how to update
)

// janisAge decides from the checksum of the janis at path (have) and of the
// latest release (latest), either "" when unknown, and whether path is where
// the installer puts janis. The line is what to tell the person, "" for nothing.
func janisAge(path, have, latest string, ours bool) (janisVerdict, string) {
	switch {
	case latest == "":
		return janisKeep, "· janis's latest release could not be read; keeping " + path + "\n"
	case have == "":
		return janisKeep, "· cannot tell which janis release " + path + " is; the latest installs with: " + janisInstall + "\n"
	case have == latest:
		return janisKeep, ""
	case ours:
		return janisRenew, "· a newer janis is out; updating " + path + "\n"
	}
	return janisTell, "· a newer janis is out; " + path + " was not put there by its installer, so it stays. Update it the way it was installed, or: " + janisInstall + "\n"
}

func (r janisRelease) checks() bool { return r.latest != nil }

func (r janisRelease) tell(line string) {
	if line != "" && r.say != nil {
		r.say(line)
	}
}

// keep records the latest release's checksum once its installer has run.
func (r janisRelease) keep(latest string) {
	if latest == "" && r.checks() {
		latest = r.latest()
	}
	if latest != "" && r.record != nil {
		r.record(latest)
	}
}

// renew updates the janis at path when it is older than the latest release
// and the installer put it there. A failed check or update keeps it: the
// connect goes on with the janis there is.
func (r janisRelease) renew(run runShell, path string) {
	if !r.checks() {
		return
	}
	latest := r.latest()
	have := ""
	if latest != "" {
		have = r.installed(path)
	}
	verdict, line := janisAge(path, have, latest, filepath.Clean(path) == filepath.Clean(r.home))
	r.tell(line)
	if verdict != janisRenew {
		return
	}
	if err := run(janisInstall, nil); err != nil {
		r.tell("✗ the janis update did not finish (" + err.Error() + "); keeping " + path + "\n")
		return
	}
	r.keep(latest)
}

// thisJanisRelease checks against the release built for this machine.
func thisJanisRelease(say func(string)) janisRelease {
	asset := janisAsset(runtime.GOOS, runtime.GOARCH)
	if asset == "" {
		return janisRelease{}
	}
	home, _ := os.UserHomeDir()
	ours := filepath.Join(home, ".local", "bin", "janis")
	record := shared.MemdoorHome(janisRecordFile)
	return janisRelease{
		home: ours,
		installed: func(path string) string {
			if !strings.HasSuffix(asset, ".tar.gz") {
				return fileSHA256(path)
			}
			if filepath.Clean(path) != ours {
				return ""
			}
			b, _ := os.ReadFile(record)
			return checksumIn(string(b))
		},
		latest: func() string { return releaseSHA256(janisReleaseURL + asset + ".sha256") },
		record: func(sum string) {
			_ = os.MkdirAll(filepath.Dir(record), 0o755)
			_ = os.WriteFile(record, []byte(sum+"\n"), 0o644)
		},
		say: say,
	}
}

// checksumIn is the first field of a sha256 line ("<hex>  <file>"), "" when
// it is not one.
func checksumIn(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 || !sha256Hex.MatchString(f[0]) {
		return ""
	}
	return f[0]
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func releaseSHA256(url string) string {
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return checksumIn(string(b))
}
