package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(vars map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := vars[k]; return v, ok }
}

// A Claude Code .mcp.json reads as is; the project's entry wins over the
// user's of the same name; variables expand, with defaults.
func TestServersFromBothFiles(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(proj, ".mcp.json"), []byte(`{"mcpServers":{
		"github":{"type":"http","url":"https://api.githubcopilot.com/mcp/","headers":{"Authorization":"Bearer ${GITHUB_TOKEN}"}},
		"db":{"command":"uvx","args":["mcp-db","--port","${PORT:-5432}"],"timeout":20000}}}`), 0o644)
	os.WriteFile(filepath.Join(home, "mcp.json"), []byte(`{"mcpServers":{
		"github":{"command":"should-lose"},
		"notes":{"command":"notes-mcp"}}}`), 0o600)
	st := NewFileStore(home)
	servers, err := st.Servers(proj)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Server{}
	for _, s := range servers {
		got[s.Name] = s
	}
	if len(servers) != 3 || got["github"].Scope != ScopeProject || got["notes"].Scope != ScopeUser {
		t.Fatalf("servers %+v", servers)
	}
	sp, err := got["github"].Spec(proj, env(map[string]string{"GITHUB_TOKEN": "ghp_x"}))
	if err != nil || sp.Headers["Authorization"] != "Bearer ghp_x" || sp.URL == "" {
		t.Fatalf("spec %+v, err %v", sp, err)
	}
	sp, err = got["db"].Spec(proj, env(nil))
	if err != nil || strings.Join(sp.Args, " ") != "mcp-db --port 5432" || sp.Dir != proj {
		t.Fatalf("spec %+v, err %v", sp, err)
	}
	if _, err := got["github"].Spec(proj, env(nil)); err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("a missing variable must be named, got %v", err)
	}
}

// Writes keep what the file holds that this package does not model, and
// turning a server off and on round-trips.
func TestEditsKeepUnknownFields(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	p := filepath.Join(proj, ".mcp.json")
	os.WriteFile(p, []byte(`{"$schema":"x","mcpServers":{"db":{"command":"uvx","timeout":20000}}}`), 0o644)
	st := NewFileStore(home)
	if err := st.Add(ScopeProject, proj, "web", Entry{URL: "https://mcp.example.com/mcp"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDisabled(ScopeProject, proj, "db", true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	for _, want := range []string{`"$schema": "x"`, `"timeout": 20000`, `"disabled": true`, `"web"`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("missing %s in:\n%s", want, b)
		}
	}
	if err := st.SetDisabled(ScopeProject, proj, "db", false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "disabled") {
		t.Fatalf("still disabled:\n%s", b)
	}
	if err := st.Remove(ScopeProject, proj, "web"); err != nil {
		t.Fatal(err)
	}
	if err := st.Add(ScopeUser, proj, "bad name", Entry{Command: "x"}); err == nil {
		t.Fatal("a name with a space must be refused")
	}
	if fi, _ := os.Stat(filepath.Join(home, "mcp.json")); fi != nil {
		t.Fatal("a refused add must not create the file")
	}
	if err := st.Add(ScopeUser, proj, "k", Entry{Command: "x", Headers: map[string]string{"Authorization": "k"}}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(home, "mcp.json")); fi.Mode().Perm() != 0o600 {
		t.Fatalf("the user file must be private: %v", fi.Mode())
	}
}

// A project's servers are trusted as they are: a change asks again.
func TestTrustCoversTheProjectServersAsTheyAre(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	st := NewFileStore(home)
	if ok, _ := st.Trusted(proj); !ok {
		t.Fatal("a project without servers needs no trust")
	}
	st.Add(ScopeProject, proj, "db", Entry{Command: "uvx", Args: []string{"mcp-db"}})
	if ok, _ := st.Trusted(proj); ok {
		t.Fatal("a new project server must not be trusted yet")
	}
	if err := st.Trust(proj); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.Trusted(proj); !ok {
		t.Fatal("trusted after Trust")
	}
	if err := st.SetDisabled(ScopeProject, proj, "db", true); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.Trusted(proj); !ok {
		t.Fatal("turning a server off is not a change to what runs")
	}
	st.Add(ScopeProject, proj, "db", Entry{Command: "curl", Args: []string{"evil.sh"}})
	if ok, _ := st.Trusted(proj); ok {
		t.Fatal("a changed command must ask again")
	}
}

// A file that ends up holding no servers is deleted, so a refused add leaves
// no `{"mcpServers": {}}` behind to be committed — unless the person keeps
// something else in it.
func TestAnEmptiedFileIsDeletedUnlessItHoldsSomethingElse(t *testing.T) {
	home, proj := t.TempDir(), t.TempDir()
	p := filepath.Join(proj, ".mcp.json")
	st := NewFileStore(home)
	if err := st.Add(ScopeProject, proj, "one", Entry{Command: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove(ScopeProject, proj, "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		b, _ := os.ReadFile(p)
		t.Fatalf("the file should be gone, it holds:\n%s", b)
	}

	os.WriteFile(p, []byte(`{"$schema":"x","mcpServers":{"one":{"command":"x"}}}`), 0o644)
	if err := st.Remove(ScopeProject, proj, "one"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("a file with the person's own keys must stay: %v", err)
	}
	if !strings.Contains(string(b), `"$schema": "x"`) {
		t.Fatalf("their keys are gone:\n%s", b)
	}
}
