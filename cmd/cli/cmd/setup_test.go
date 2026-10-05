package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestPersistWorkspaceSlug guards the post-setup back-fill of config.json's
// workspace_id with the authoritative workspace SLUG. Regression: setup once
// wrote the workspace DIRECTORY PATH into workspace_id, so resolveWorkspaceSlug
// returned a path and every post-setup command built malformed API URLs
// (`/api/messages//path/to/workspace` → 404). The slug must replace it while
// every other config field survives the round-trip.
func TestPersistWorkspaceSlug(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	// A config as phase-1 setup leaves it: workspace_id holds the dir PATH (the
	// bug), and other fields (agents.defaults.workspace, database) are populated.
	wsPath := filepath.Join(dir, ".memdoor", "workspace")
	dbPath := filepath.Join(dir, ".memdoor", "data", "memdoor.db")
	initial := map[string]any{
		"workspace_id": wsPath,
		"agents":       map[string]any{"defaults": map[string]any{"workspace": wsPath}},
		"database":     map[string]any{"type": "sqlite", "sqlite": map[string]any{"path": dbPath}},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	persistWorkspaceSlug(configPath, "trading")

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("config is not valid JSON after persist: %v", err)
	}

	if got["workspace_id"] != "trading" {
		t.Errorf("workspace_id = %v, want %q (the slug, not the path)", got["workspace_id"], "trading")
	}
	// The agent workspace dir path is intentionally a path — it must NOT have
	// been clobbered by the slug back-fill.
	defaults, _ := got["agents"].(map[string]any)["defaults"].(map[string]any)
	if defaults == nil || defaults["workspace"] != wsPath {
		t.Errorf("agents.defaults.workspace = %v, want preserved path %q", defaults, wsPath)
	}
	if _, ok := got["database"]; !ok {
		t.Errorf("database block was dropped by the config round-trip")
	}

	// Empty slug is a no-op (never blank out a configured workspace_id).
	persistWorkspaceSlug(configPath, "")
	raw2, _ := os.ReadFile(configPath)
	var got2 map[string]any
	_ = json.Unmarshal(raw2, &got2)
	if got2["workspace_id"] != "trading" {
		t.Errorf("empty slug must be a no-op; workspace_id = %v, want %q", got2["workspace_id"], "trading")
	}
}
