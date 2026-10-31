package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// A directory pinned to a workspace the signed-in account does not own (a
// .memdoor/workspace from the multi-workspace days) opens the account's own:
// the gateway would close the window's socket on the pinned one. What the
// person typed (-w, $MEMDOOR_WORKSPACE) stays, and an answer that is not a
// slug changes nothing.
func TestOwnWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envWorkspaceVar, "")
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(filepath.Join(project, ".memdoor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".memdoor", "workspace"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)

	slug := "mine"
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/whoami" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"user":{"email":"a@b","workspace_id":"uuid-1"},"workspace_slug":"` + slug + `"}`))
	}))
	defer gw.Close()
	saved, savedSlug := gatewayAddr, workspaceSlug
	t.Cleanup(func() { gatewayAddr, workspaceSlug = saved, savedSlug })
	gatewayAddr = gw.URL

	c := NewClient()
	if got := ownWorkspace(c, "stale"); got != "mine" || c.workspaceSlug != "mine" {
		t.Fatalf("stale pin: got %q (client %q), want the account's own", got, c.workspaceSlug)
	}
	for _, ws := range []string{"mine", "uuid-1"} {
		if got := ownWorkspace(NewClient(), ws); got != ws {
			t.Fatalf("%q is the account's: got %q", ws, got)
		}
	}

	slug = `x\u001b[2J`
	if got := ownWorkspace(NewClient(), "stale"); got != "stale" {
		t.Fatalf("a whoami slug that is not a slug became the workspace: %q", got)
	}
	slug = "mine"

	t.Setenv(envWorkspaceVar, "typed")
	if got := ownWorkspace(NewClient(), "typed"); got != "typed" {
		t.Fatalf("$%s was overridden: %q", envWorkspaceVar, got)
	}
}
