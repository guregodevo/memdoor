package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProcessIncludes_SingleFile tests including a single file
func TestProcessIncludes_SingleFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "include-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create base config with $include
	baseConfig := filepath.Join(tmpDir, "config.json")
	baseContent := `{
  "$include": "./agents.json",
  "session": {
    "scope": "per-agent"
  }
}`
	if err := os.WriteFile(baseConfig, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create included config
	agentsConfig := filepath.Join(tmpDir, "agents.json")
	agentsContent := `{
  "agents": {
    "defaults": {
      "workspace": "/included-workspace",
      "model": "claude-sonnet-4-5"
    }
  }
}`
	if err := os.WriteFile(agentsConfig, []byte(agentsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := LoadConfigFromFile(baseConfig)
	if err != nil {
		t.Fatalf("Failed to load config with include: %v", err)
	}

	// Verify merged values
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Fatal("Agents config not loaded from included file")
	}

	if cfg.Agents.Defaults.Workspace != "/included-workspace" {
		t.Errorf("Expected workspace '/included-workspace', got '%s'", cfg.Agents.Defaults.Workspace)
	}

	if cfg.Session == nil || cfg.Session.Scope != "per-agent" {
		t.Error("Session config from base file not preserved")
	}
}

// TestProcessIncludes_MultipleFiles tests including multiple files (array)
func TestProcessIncludes_MultipleFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "include-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create base config with multiple includes
	baseConfig := filepath.Join(tmpDir, "config.json")
	baseContent := `{
  "$include": ["./agents.json", "./session.json"]
}`
	if err := os.WriteFile(baseConfig, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create agents config
	agentsConfig := filepath.Join(tmpDir, "agents.json")
	agentsContent := `{
  "agents": {
    "defaults": {
      "workspace": "/workspace"
    }
  }
}`
	if err := os.WriteFile(agentsConfig, []byte(agentsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create session config
	sessionConfig := filepath.Join(tmpDir, "session.json")
	sessionContent := `{
  "session": {
    "scope": "global"
  }
}`
	if err := os.WriteFile(sessionConfig, []byte(sessionContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := LoadConfigFromFile(baseConfig)
	if err != nil {
		t.Fatalf("Failed to load config with multiple includes: %v", err)
	}

	// Verify both includes were loaded
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Fatal("Agents config not loaded")
	}

	if cfg.Session == nil || cfg.Session.Scope != "global" {
		t.Fatal("Session config not loaded")
	}
}

// TestProcessIncludes_NestedIncludes tests includes within included files
func TestProcessIncludes_NestedIncludes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "include-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create base config
	baseConfig := filepath.Join(tmpDir, "config.json")
	baseContent := `{
  "$include": "./level1.json"
}`
	if err := os.WriteFile(baseConfig, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create level1 config (has its own include)
	level1Config := filepath.Join(tmpDir, "level1.json")
	level1Content := `{
  "$include": "./level2.json",
  "session": {
    "scope": "level1"
  }
}`
	if err := os.WriteFile(level1Config, []byte(level1Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create level2 config
	level2Config := filepath.Join(tmpDir, "level2.json")
	level2Content := `{
  "agents": {
    "defaults": {
      "workspace": "/nested-workspace"
    }
  }
}`
	if err := os.WriteFile(level2Config, []byte(level2Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := LoadConfigFromFile(baseConfig)
	if err != nil {
		t.Fatalf("Failed to load config with nested includes: %v", err)
	}

	// Verify nested include was processed
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Fatal("Agents config not loaded from nested include")
	}

	if cfg.Agents.Defaults.Workspace != "/nested-workspace" {
		t.Errorf("Expected workspace '/nested-workspace', got '%s'", cfg.Agents.Defaults.Workspace)
	}

	if cfg.Session == nil || cfg.Session.Scope != "level1" {
		t.Error("Session config from level1 not preserved")
	}
}

// TestProcessIncludes_CircularDependency tests circular dependency detection
func TestProcessIncludes_CircularDependency(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "include-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config A that includes B
	configA := filepath.Join(tmpDir, "a.json")
	contentA := `{
  "$include": "./b.json"
}`
	if err := os.WriteFile(configA, []byte(contentA), 0644); err != nil {
		t.Fatal(err)
	}

	// Create config B that includes A (circular!)
	configB := filepath.Join(tmpDir, "b.json")
	contentB := `{
  "$include": "./a.json"
}`
	if err := os.WriteFile(configB, []byte(contentB), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config should fail with circular dependency error
	_, err = LoadConfigFromFile(configA)
	if err == nil {
		t.Fatal("Expected error for circular dependency, got none")
	}

	if !contains(err.Error(), "circular dependency") {
		t.Errorf("Expected 'circular dependency' error, got: %v", err)
	}
}

// TestProcessIncludes_OverrideBehavior tests that base config overrides included config
func TestProcessIncludes_OverrideBehavior(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "include-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create base config with override
	baseConfig := filepath.Join(tmpDir, "config.json")
	baseContent := `{
  "$include": "./defaults.json",
  "agents": {
    "defaults": {
      "workspace": "/override-workspace"
    }
  }
}`
	if err := os.WriteFile(baseConfig, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create defaults config
	defaultsConfig := filepath.Join(tmpDir, "defaults.json")
	defaultsContent := `{
  "agents": {
    "defaults": {
      "workspace": "/default-workspace"
    }
  }
}`
	if err := os.WriteFile(defaultsConfig, []byte(defaultsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := LoadConfigFromFile(baseConfig)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Base config should override included config
	if cfg.Agents.Defaults.Workspace != "/override-workspace" {
		t.Errorf("Expected workspace '/override-workspace' (from base), got '%s'", cfg.Agents.Defaults.Workspace)
	}
}

// TestProcessIncludes_YAMLFormat tests $include with YAML files
func TestProcessIncludes_YAMLFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "include-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create base YAML config
	baseConfig := filepath.Join(tmpDir, "config.yaml")
	baseContent := `$include: ./agents.yaml
session:
  scope: per-agent
`
	if err := os.WriteFile(baseConfig, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create included YAML config
	agentsConfig := filepath.Join(tmpDir, "agents.yaml")
	agentsContent := `agents:
  defaults:
    workspace: /yaml-workspace
    model: claude-sonnet-4-5
`
	if err := os.WriteFile(agentsConfig, []byte(agentsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := LoadConfigFromFile(baseConfig)
	if err != nil {
		t.Fatalf("Failed to load YAML config with include: %v", err)
	}

	// Verify merged values
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Fatal("Agents config not loaded from included YAML file")
	}

	if cfg.Agents.Defaults.Workspace != "/yaml-workspace" {
		t.Errorf("Expected workspace '/yaml-workspace', got '%s'", cfg.Agents.Defaults.Workspace)
	}
}

// TestProcessIncludes_NoInclude tests that configs without $include still work
func TestProcessIncludes_NoInclude(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "include-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config without $include
	configPath := filepath.Join(tmpDir, "config.json")
	content := `{
  "agents": {
    "defaults": {
      "workspace": "/simple-workspace"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config without includes: %v", err)
	}

	// Verify values
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Fatal("Agents config not loaded")
	}

	if cfg.Agents.Defaults.Workspace != "/simple-workspace" {
		t.Errorf("Expected workspace '/simple-workspace', got '%s'", cfg.Agents.Defaults.Workspace)
	}
}

// TestProcessIncludes_MissingFile tests error handling for missing included files
func TestProcessIncludes_MissingFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "include-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config that includes non-existent file
	configPath := filepath.Join(tmpDir, "config.json")
	content := `{
  "$include": "./nonexistent.json"
}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config should fail
	_, err = LoadConfigFromFile(configPath)
	if err == nil {
		t.Fatal("Expected error for missing included file, got none")
	}

	if !contains(err.Error(), "failed to read included file") {
		t.Errorf("Expected 'failed to read included file' error, got: %v", err)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
