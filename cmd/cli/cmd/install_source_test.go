package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestTheInstallSourceIsMemdoorAIUnlessGitHubIsAskedFor(t *testing.T) {
	if _, ok, err := parseInstallSource(envOf(nil)); ok || err != nil {
		t.Fatalf("unset: ok=%v err=%v", ok, err)
	}
	g, ok, err := parseInstallSource(envOf(map[string]string{installSourceEnv: "github"}))
	if !ok || err != nil || g.repo != defaultGitHubRepo || g.tag != "" {
		t.Fatalf("github: %+v %v %v", g, ok, err)
	}
	g, ok, _ = parseInstallSource(envOf(map[string]string{installSourceEnv: "github", githubRepositoryEnv: "me/memdoor-fork", githubReleaseEnv: "v1.2.3"}))
	if !ok || g.repo != "me/memdoor-fork" || g.tag != "v1.2.3" {
		t.Fatalf("%+v", g)
	}
	for _, bad := range []map[string]string{
		{installSourceEnv: "s3"},
		{installSourceEnv: "github", githubRepositoryEnv: "me/x; rm -rf ~"},
		{installSourceEnv: "github", githubRepositoryEnv: "nouser"},
		{installSourceEnv: "github", githubReleaseEnv: "v1 && curl evil"},
	} {
		if _, _, err := parseInstallSource(envOf(bad)); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

// The latest tag is read from the redirect releases/latest answers with, and
// the installer line installs that tag with that tag's installer.
func TestTheLatestGitHubReleaseIsReadFromItsRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/fork/releases/latest" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/me/fork/releases/tag/v9.8.7", http.StatusFound)
	}))
	defer srv.Close()
	saved := githubBase
	githubBase = srv.URL
	t.Cleanup(func() { githubBase = saved })

	g := githubRelease{repo: "me/fork"}
	tag, err := g.resolvedTag(time.Second)
	if err != nil || tag != "v9.8.7" {
		t.Fatalf("%q %v", tag, err)
	}
	line := g.installerLine(tag, false)
	want := "curl -fsSL " + srv.URL + "/me/fork/raw/v9.8.7/scripts/install.sh | MEMDOOR_INSTALL_SOURCE=github MEMDOOR_GITHUB_REPOSITORY=me/fork MEMDOOR_GITHUB_RELEASE=v9.8.7 bash"
	if line != want {
		t.Fatalf("\n got %s\nwant %s", line, want)
	}
	if ps := g.installerLine(tag, true); !strings.Contains(ps, "$env:MEMDOOR_GITHUB_RELEASE='v9.8.7'") || !strings.HasSuffix(ps, "/me/fork/raw/v9.8.7/scripts/install.ps1 | iex") {
		t.Fatalf("%s", ps)
	}
	if _, err := (githubRelease{repo: "me/none"}).resolvedTag(time.Second); err == nil {
		t.Fatal("a repository with no release is an error")
	}
	if tag, _ := (githubRelease{repo: "me/fork", tag: "v1.0.0"}).resolvedTag(time.Second); tag != "v1.0.0" {
		t.Fatalf("a named tag is used as it is: %q", tag)
	}
}
