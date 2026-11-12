package cmd

import (
	"errors"
	"memdoor/internal/testenv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// A HEADLESS COMMAND SAYS A NEW VERSION IS OUT (#90): once a day, with the
// version it runs, only for run / workflow / cron, never for a dev build.
func TestAHeadlessCommandSaysANewVersionOnceADay(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	savedV, savedF := Version, fetchHeadlessVersion
	t.Cleanup(func() { Version, fetchHeadlessVersion = savedV, savedF })
	Version = "v1.793.0"
	fetches, remote := 0, "v1.800.0"
	fetchHeadlessVersion = func() (string, error) { fetches++; return remote, nil }

	root := &cobra.Command{Use: "memdoor"}
	run := &cobra.Command{Use: "run"}
	wf := &cobra.Command{Use: "workflow"}
	status := &cobra.Command{Use: "status"}
	auth := &cobra.Command{Use: "auth"}
	root.AddCommand(run, wf, auth)
	wf.AddCommand(status)

	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	line := headlessUpdateNotice(run, now)
	if line != "memdoor v1.800.0 is out (this is v1.793.0) — memdoor upgrade installs it" {
		t.Fatalf("the line names both versions: %q", line)
	}
	if got := headlessUpdateNotice(status, now.Add(time.Hour)); got != "" || fetches != 1 {
		t.Fatalf("not again within the day, and no second check: %q, %d checks", got, fetches)
	}
	if got := headlessUpdateNotice(status, now.Add(25*time.Hour)); !strings.Contains(got, "v1.800.0 is out") || fetches != 2 {
		t.Fatalf("a day later, `workflow status` checks and says it again: %q, %d checks", got, fetches)
	}
	if got := headlessUpdateNotice(auth, now.Add(50*time.Hour)); got != "" || fetches != 2 {
		t.Fatalf("a command that is not headless work says nothing and asks nothing: %q", got)
	}
	remote = "v1.793.0"
	if got := headlessUpdateNotice(run, now.Add(75*time.Hour)); got != "" {
		t.Fatalf("on the release, nothing: %q", got)
	}
	Version = "dev"
	if got := headlessUpdateNotice(run, now.Add(100*time.Hour)); got != "" {
		t.Fatalf("a dev build is never told: %q", got)
	}
}

// The headless check is marked ?from=cli, so the metrics leave it out of
// DAU; the window's check is not marked.
func TestTheHeadlessCheckIsMarked(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		_, _ = w.Write([]byte("v1.800.0\n"))
	}))
	defer srv.Close()
	saved := versionEndpoint
	t.Cleanup(func() { versionEndpoint = saved })
	versionEndpoint = srv.URL + "/dl/VERSION"
	// Not on a runner here: CI's own run of this test sets GITHUB_ACTIONS.
	t.Setenv("MEMDOOR_CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("MEMDOOR_OWNER", "")
	if v, err := fetchRemoteVersionFrom("cli", time.Second); err != nil || v != "v1.800.0" {
		t.Fatalf("%q %v", v, err)
	}
	if _, err := fetchRemoteVersion(); err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 || queries[0] != "from=cli" || queries[1] != "" {
		t.Fatalf("headless marked, window not: %q", queries)
	}
}

// On a CI runner the headless check says so: ?from=cli&ci=1 under MEMDOOR_CI
// (install.sh's variable) or GITHUB_ACTIONS (every GitHub runner's), and the
// metrics leave it out of HEADLESS too. Neither set, ?from=cli alone.
func TestTheHeadlessCheckSaysCIOnARunner(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		_, _ = w.Write([]byte("v1.800.0\n"))
	}))
	defer srv.Close()
	saved := versionEndpoint
	t.Cleanup(func() { versionEndpoint = saved })
	versionEndpoint = srv.URL + "/dl/VERSION"
	t.Setenv("MEMDOOR_CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("MEMDOOR_OWNER", "")
	for _, env := range []string{"MEMDOOR_CI", "GITHUB_ACTIONS"} {
		t.Setenv(env, "1")
		if _, err := fetchRemoteVersionFrom("cli", time.Second); err != nil {
			t.Fatal(err)
		}
		t.Setenv(env, "")
	}
	if _, err := fetchRemoteVersionFrom("cli", time.Second); err != nil {
		t.Fatal(err)
	}
	if len(queries) != 3 || queries[0] != "from=cli&ci=1" || queries[1] != "from=cli&ci=1" || queries[2] != "from=cli" {
		t.Fatalf("ci marked under either variable, not without: %q", queries)
	}
}

// The maintainer's own machines say so on every check, the window's and the
// headless one: me=1 under MEMDOOR_OWNER, and the metrics count neither as a
// person. Unset, nothing is added.
func TestTheOwnersMachinesMarkTheirCheck(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		_, _ = w.Write([]byte("v1.800.0\n"))
	}))
	defer srv.Close()
	saved := versionEndpoint
	t.Cleanup(func() { versionEndpoint = saved })
	versionEndpoint = srv.URL + "/dl/VERSION"
	t.Setenv("MEMDOOR_CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("MEMDOOR_OWNER", "1")
	if _, err := fetchRemoteVersion(); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchRemoteVersionFrom("cli", time.Second); err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 || queries[0] != "me=1" || queries[1] != "from=cli&me=1" {
		t.Fatalf("the owner's machines marked, window and headless: %q", queries)
	}
}

// The window never waits on the version check, and asks again after a turn
// once an hour has passed (#290): releases ship several a day.
func TestTheWindowChecksInTheBackgroundAndAfterATurn(t *testing.T) {
	savedV, savedF := Version, fetchWindowVersion
	t.Cleanup(func() {
		Version, fetchWindowVersion = savedV, savedF
		updateNotice, updateSay, updateChecked, updateChecking = "", "", time.Time{}, false
	})
	updateNotice, updateSay, updateChecked, updateChecking = "", "", time.Time{}, false
	Version = "v1.821.0"
	remote, fetches := "v1.822.0", 0
	release := make(chan struct{})
	fetchWindowVersion = func() (string, error) { <-release; fetches++; return remote, nil }
	settled := func() string {
		t.Helper()
		for i := 0; i < 200; i++ {
			updateMu.Lock()
			busy := updateChecking
			updateMu.Unlock()
			if !busy {
				updateMu.Lock()
				defer updateMu.Unlock()
				return updateNotice
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("the check never ended")
		return ""
	}

	if got, say := tuiUpdateNotice(); got != "" || say != "" {
		t.Fatalf("the first call answers at once, before the check: %q %q", got, say)
	}
	release <- struct{}{}
	if got := settled(); !strings.Contains(got, "v1.822.0 is out (this is v1.821.0)") {
		t.Fatalf("after the check: %q", got)
	}
	if _, say := tuiUpdateNotice(); !strings.Contains(say, "/update installs it and reopens this conversation") ||
		!strings.Contains(say, "https://github.com/guregodevo/memdoor/releases/tag/v1.822.0") {
		t.Fatalf("the conversation's line names /update and links the release: %q", say)
	}

	remote = "v1.823.0"
	tuiTurnEnded()
	if got := settled(); fetches != 1 || !strings.Contains(got, "v1.822.0") {
		t.Fatalf("a turn within the hour asks nothing: fetches=%d %q", fetches, got)
	}
	updateMu.Lock()
	updateChecked = time.Now().Add(-updateAfterTurn - time.Minute)
	updateMu.Unlock()
	tuiTurnEnded()
	release <- struct{}{}
	if got := settled(); fetches != 2 || !strings.Contains(got, "v1.823.0 is out") {
		t.Fatalf("a turn an hour later asks again: fetches=%d %q", fetches, got)
	}

	fetchWindowVersion = func() (string, error) { fetches++; return "", errors.New("offline") }
	updateMu.Lock()
	updateChecked = time.Time{}
	updateMu.Unlock()
	tuiUpdateNotice()
	if got := settled(); !strings.Contains(got, "v1.823.0 is out") {
		t.Fatalf("a failed check keeps the last answer: %q", got)
	}

	Version, updateChecked, updateNotice, updateSay = "dev", time.Time{}, "", ""
	before := fetches
	tuiTurnEnded()
	if got, say := tuiUpdateNotice(); got != "" || say != "" || fetches != before {
		t.Fatalf("a dev build is never told: %q %q fetches %d -> %d", got, say, before, fetches)
	}
}
