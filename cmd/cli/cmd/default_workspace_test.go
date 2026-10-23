package cmd

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRecents(t *testing.T, home string, slugs ...string) {
	t.Helper()
	dir := filepath.Join(home, ".memdoor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, s := range slugs {
		body += s + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, recentWorkspacesFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A QUESTION WITH ONE POSSIBLE ANSWER IS NOT A QUESTION (Greg, 2026-09-27,
// walking onboarding: "there is a default workspace"). Somebody with a single
// workspace was refused in any directory they had not pinned, and handed four
// ways to name the only workspace they have.
func TestTheOnlyWorkspaceIsTheDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envWorkspaceVar, "")
	writeRecents(t, home, "shipyard")
	if slug, ok := soleWorkspace(); !ok || slug != "shipyard" {
		t.Fatalf("the only workspace should be the default: %q ok=%v", slug, ok)
	}
}

// With several, the ambiguity is real. The error must then be worth reading:
// the slugs the person actually has, not a recital of four ways to set one.
func TestSeveralWorkspacesNameThemselves(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envWorkspaceVar, "")
	writeRecents(t, home, "shipyard", "hackernews")
	if _, ok := soleWorkspace(); ok {
		t.Fatal("two workspaces must not resolve to a default")
	}
	msg := noWorkspaceErrorMessage("agent").Error()
	for _, want := range []string{"shipyard", "hackernews", "workspace use"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the error should mention %q: %s", want, msg)
		}
	}
}

// And on a machine where nothing has happened yet, the answer is the first
// run (install.sh and the README promise no setup command, 2026-10-08; a
// visitor who ran `memdoor providers` first was sent to one, 2026-10-21) —
// not a lecture about flags, env vars and config keys.
func TestFreshMachineIsToldToSetUp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envWorkspaceVar, "")
	if _, ok := soleWorkspace(); ok {
		t.Fatal("an empty machine has no default workspace")
	}
	msg := noWorkspaceErrorMessage("agent").Error()
	if !strings.Contains(msg, "&& memdoor") || strings.Contains(msg, "memdoor setup") {
		t.Errorf("a fresh machine is told to set up: %s", msg)
	}
	if strings.Contains(msg, "config: set workspace_id") {
		t.Errorf("the four-way recital is gone: %s", msg)
	}
}

// A fresh laptop against a box: nothing resolves here, and the box has one
// workspace. An exempt command (run, workflow, remote) takes that one instead
// of sending the box a turn with no workspace, which it refused with
// "workspace is required" (the ssh door, 2026-10-23).
func TestARemoteGatewaysOnlyWorkspaceIsTheDefaultForRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envWorkspaceVar, "")
	t.Chdir(t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/setup/status" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"initialized":true,"workspace_slug":"box"}`))
	}))
	defer srv.Close()
	savedAddr, savedSlug := gatewayAddr, workspaceSlug
	gatewayAddr, workspaceSlug = strings.Replace(srv.URL, "127.0.0.1", "box.lan", 1), ""
	t.Cleanup(func() { gatewayAddr, workspaceSlug = savedAddr, savedSlug })
	// box.lan is not this machine: the request goes to the test server.
	http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return net.Dial(network, strings.TrimPrefix(srv.URL, "http://"))
	}
	t.Cleanup(func() { http.DefaultTransport.(*http.Transport).DialContext = nil })
	run, _, err := rootCmd.Find([]string{"run"})
	if err != nil {
		t.Fatal(err)
	}
	if err := requireWorkspaceSlug(run, nil); err != nil {
		t.Fatal(err)
	}
	if workspaceSlug != "box" {
		t.Fatalf("run on a box with one workspace takes it: %q", workspaceSlug)
	}
}
