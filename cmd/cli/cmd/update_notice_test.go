package cmd

import (
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
	t.Setenv("HOME", t.TempDir())
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
