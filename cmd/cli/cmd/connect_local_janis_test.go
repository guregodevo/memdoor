package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	sumOld = "1111111111111111111111111111111111111111111111111111111111111111"
	sumNew = "2222222222222222222222222222222222222222222222222222222222222222"
)

func TestJanisAgeDecidesUpdateTellOrKeep(t *testing.T) {
	cases := []struct {
		name        string
		have, latst string
		ours        bool
		want        janisVerdict
		says        string
	}{
		{"current", sumNew, sumNew, true, janisKeep, ""},
		{"older, the installer's", sumOld, sumNew, true, janisRenew, "updating /h/.local/bin/janis"},
		{"older, put there another way", sumOld, sumNew, false, janisTell, "so it stays"},
		{"the release unreachable", sumOld, "", true, janisKeep, "could not be read; keeping"},
		{"this janis unknown", "", sumNew, true, janisKeep, "cannot tell which janis release"},
	}
	for _, c := range cases {
		v, line := janisAge("/h/.local/bin/janis", c.have, c.latst, c.ours)
		if v != c.want || (c.says == "") != (line == "") || !strings.Contains(line, c.says) {
			t.Errorf("%s: got %v %q, want %v saying %q", c.name, v, line, c.want, c.says)
		}
		if v == janisTell && !strings.Contains(line, janisInstall) {
			t.Errorf("%s: the line names the update command: %q", c.name, line)
		}
	}
}

// fakeRelease is a release check with no network: what is installed, the
// latest checksum, and what got recorded.
func fakeRelease(home, have, latest string, said *strings.Builder, recorded *string) janisRelease {
	return janisRelease{
		home:      home,
		installed: func(string) string { return have },
		latest:    func() string { return latest },
		record:    func(s string) { *recorded = s },
		say:       func(s string) { said.WriteString(s) },
	}
}

func TestInstallJanisUpdatesAnOldJanisOnlyWhereTheInstallerPutsIt(t *testing.T) {
	ours := "/h/.local/bin/janis"
	var ran []string
	run := func(script string, tee io.Writer) error { ran = append(ran, script); return nil }

	var said strings.Builder
	var recorded string
	if err := installJanis(run, ours, fakeRelease(ours, sumOld, sumNew, &said, &recorded)); err != nil {
		t.Fatal(err)
	}
	if want := []string{janisInstall, ours + " pull", ours + " serve --detach"}; fmt.Sprint(ran) != fmt.Sprint(want) {
		t.Fatalf("an old janis of the installer's is updated, then pulls and serves:\n got %q\nwant %q", ran, want)
	}
	if recorded != sumNew {
		t.Fatalf("the update records the release it installed: %q", recorded)
	}

	for _, c := range []struct{ path, have, latest string }{
		{ours, sumNew, sumNew},                      // current
		{"/opt/homebrew/bin/janis", sumOld, sumNew}, // older, not the installer's
		{ours, sumOld, ""},                          // release unreachable
		{ours, "", sumNew},                          // cannot tell
	} {
		ran, recorded = nil, ""
		said.Reset()
		if err := installJanis(run, c.path, fakeRelease(ours, c.have, c.latest, &said, &recorded)); err != nil {
			t.Fatal(err)
		}
		if len(ran) != 2 || ran[0] != c.path+" pull" || recorded != "" {
			t.Fatalf("%+v: no installer, the janis there pulls and serves: %q (recorded %q)", c, ran, recorded)
		}
	}
}

func TestAFailedJanisUpdateKeepsTheJanisThere(t *testing.T) {
	ours := "/h/.local/bin/janis"
	var ran []string
	run := func(script string, tee io.Writer) error {
		ran = append(ran, script)
		if script == janisInstall {
			return errors.New("exit status 22")
		}
		return nil
	}
	var said strings.Builder
	var recorded string
	if err := installJanis(run, ours, fakeRelease(ours, sumOld, sumNew, &said, &recorded)); err != nil {
		t.Fatalf("a failed update never fails the connect: %v", err)
	}
	if len(ran) != 3 || ran[1] != ours+" pull" || recorded != "" {
		t.Fatalf("after a failed update the old janis pulls and serves, nothing recorded: %q %q", ran, recorded)
	}
	if !strings.Contains(said.String(), "keeping "+ours) {
		t.Fatalf("the failure is said: %q", said.String())
	}
}

func TestAFreshInstallRecordsTheRelease(t *testing.T) {
	run := func(string, io.Writer) error { return nil }
	var said strings.Builder
	var recorded string
	if err := installJanis(run, "", fakeRelease("/h/.local/bin/janis", "", sumNew, &said, &recorded)); err != nil {
		t.Fatal(err)
	}
	if recorded != sumNew || said.Len() != 0 {
		t.Fatalf("a fresh install records the release, quietly: %q %q", recorded, said.String())
	}
}

func TestJanisAssetIsTheInstallersFile(t *testing.T) {
	for in, want := range map[[2]string]string{
		{"linux", "amd64"}:   "janis-linux-amd64",
		{"linux", "arm64"}:   "janis-linux-arm64",
		{"darwin", "arm64"}:  "janis-darwin-arm64.tar.gz",
		{"darwin", "amd64"}:  "",
		{"windows", "amd64"}: "",
	} {
		if got := janisAsset(in[0], in[1]); got != want {
			t.Errorf("%v: %q, want %q", in, got, want)
		}
	}
}

func TestReleaseChecksumIsReadFromTheSha256File(t *testing.T) {
	body, status := sumNew+"  janis-linux-amd64\n", http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	if got := releaseSHA256(srv.URL); got != sumNew {
		t.Fatalf("the first field of the sha256 file: %q", got)
	}
	status = http.StatusNotFound
	if got := releaseSHA256(srv.URL); got != "" {
		t.Fatalf("a missing file is unknown: %q", got)
	}
	status, body = http.StatusOK, "<html>"
	if got := releaseSHA256(srv.URL); got != "" {
		t.Fatalf("a page that is not a checksum is unknown: %q", got)
	}

	f := filepath.Join(t.TempDir(), "janis")
	if err := os.WriteFile(f, []byte("janis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := fileSHA256(f); !sha256Hex.MatchString(got) || fileSHA256(f+"x") != "" {
		t.Fatalf("a file's checksum, \"\" for a missing one: %q", got)
	}
}
