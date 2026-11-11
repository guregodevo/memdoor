package cmd

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Where memdoor installs and upgrades from. memdoor.ai by default. With
// MEMDOOR_INSTALL_SOURCE=github, a GitHub release instead: every release
// carries the binaries and SHA256SUMS, so a machine that cannot reach
// memdoor.ai, or a fork that publishes its own builds, has an install and an
// upgrade path. MEMDOOR_GITHUB_REPOSITORY (default guregodevo/memdoor) names
// the repository and MEMDOOR_GITHUB_RELEASE a tag (default the latest).
// scripts/install.sh and install.ps1 read the same three.

const (
	installSourceEnv    = "MEMDOOR_INSTALL_SOURCE"
	githubRepositoryEnv = "MEMDOOR_GITHUB_REPOSITORY"
	githubReleaseEnv    = "MEMDOOR_GITHUB_RELEASE"
	installSourceGitHub = "github"
	defaultGitHubRepo   = "guregodevo/memdoor"
)

// githubBase is github.com; a var so a test can stand in for it.
var githubBase = "https://github.com"

var (
	githubRepoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	githubTagPattern  = regexp.MustCompile(`^[A-Za-z0-9_.+-]+$`)
)

// githubRelease is a release to install from: a repository and a tag ("" =
// the latest). Both are checked, since they go into the installer's command
// line.
type githubRelease struct {
	repo string
	tag  string
}

// parseInstallSource reads the environment: ok false for memdoor.ai.
func parseInstallSource(getenv func(string) string) (githubRelease, bool, error) {
	switch src := strings.TrimSpace(getenv(installSourceEnv)); src {
	case "", "memdoor.ai":
		return githubRelease{}, false, nil
	case installSourceGitHub:
	default:
		return githubRelease{}, false, fmt.Errorf("%s=%s: set it to github, or unset it for memdoor.ai", installSourceEnv, src)
	}
	g := githubRelease{repo: strings.TrimSpace(getenv(githubRepositoryEnv)), tag: strings.TrimSpace(getenv(githubReleaseEnv))}
	if g.repo == "" {
		g.repo = defaultGitHubRepo
	}
	if !githubRepoPattern.MatchString(g.repo) {
		return githubRelease{}, false, fmt.Errorf("%s=%s is not owner/repository", githubRepositoryEnv, g.repo)
	}
	if g.tag != "" && !githubTagPattern.MatchString(g.tag) {
		return githubRelease{}, false, fmt.Errorf("%s=%s is not a release tag", githubReleaseEnv, g.tag)
	}
	return g, true, nil
}

// resolvedTag is the release's tag: the one named, else the latest, read from
// the redirect github.com/<repo>/releases/latest answers with (no API call).
func (g githubRelease) resolvedTag(timeout time.Duration) (string, error) {
	if g.tag != "" {
		return g.tag, nil
	}
	c := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	u := githubBase + "/" + g.repo + "/releases/latest"
	resp, err := c.Get(u)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	i := strings.LastIndex(loc, "/releases/tag/")
	if i < 0 {
		return "", fmt.Errorf("%s has no release (HTTP %d)", u, resp.StatusCode)
	}
	tag := loc[i+len("/releases/tag/"):]
	if !githubTagPattern.MatchString(tag) {
		return "", fmt.Errorf("%s names an odd tag %q", u, tag)
	}
	return tag, nil
}

// installerLine is the one-liner that installs this release's build with the
// installer of the same tag.
func (g githubRelease) installerLine(tag string, windows bool) string {
	if windows {
		return fmt.Sprintf("$env:%s='%s'; $env:%s='%s'; $env:%s='%s'; irm %s/%s/raw/%s/scripts/install.ps1 | iex",
			installSourceEnv, installSourceGitHub, githubRepositoryEnv, g.repo, githubReleaseEnv, tag, githubBase, g.repo, tag)
	}
	return fmt.Sprintf("curl -fsSL %s/%s/raw/%s/scripts/install.sh | %s=%s %s=%s %s=%s bash",
		githubBase, g.repo, tag, installSourceEnv, installSourceGitHub, githubRepositoryEnv, g.repo, githubReleaseEnv, tag)
}

// githubTag caches the latest tag for the length of one command, so the
// version shown and the build installed are the same release.
var githubTag struct {
	once sync.Once
	tag  string
	err  error
}

func githubTagOnce(g githubRelease, timeout time.Duration) (string, error) {
	githubTag.once.Do(func() { githubTag.tag, githubTag.err = g.resolvedTag(timeout) })
	return githubTag.tag, githubTag.err
}

func installSource() (githubRelease, bool, error) { return parseInstallSource(os.Getenv) }

// versionSource is where the latest version is read, for an error message.
func versionSource() string {
	if g, ok, err := installSource(); err == nil && ok {
		return githubBase + "/" + g.repo + "/releases/latest"
	}
	return versionEndpoint
}
