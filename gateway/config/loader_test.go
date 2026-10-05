package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfigJSON5 tests loading JSON5 config with comments and trailing commas
func TestLoadConfigJSON5(t *testing.T) {
	// Create temp JSON5 config file
	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// JSON5 with comments and trailing commas (not valid standard JSON)
	json5Content := `{
  // Agent configuration
  "agents": {
    "defaults": {
      "workspace": "/tmp/test-workspace",  // trailing comma is OK in JSON5
      "model": "claude-sonnet-4-5",
    },
    "list": [
      {
        "id": "main",
        "default": true,
        "workspace": "/tmp/test-workspace",  // another trailing comma
      },
    ],
  },
}`

	if _, err := tmpfile.WriteString(json5Content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	// Load config
	cfg, err := LoadConfigFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load JSON5 config: %v", err)
	}

	// Verify loaded values
	if cfg.Agents == nil {
		t.Fatal("Agents config is nil")
	}

	if cfg.Agents.Defaults == nil {
		t.Fatal("Agent defaults is nil")
	}

	if cfg.Agents.Defaults.Workspace != "/tmp/test-workspace" {
		t.Errorf("Expected workspace '/tmp/test-workspace', got '%s'", cfg.Agents.Defaults.Workspace)
	}

	if len(cfg.Agents.List) != 1 {
		t.Fatalf("Expected 1 agent, got %d", len(cfg.Agents.List))
	}

	if cfg.Agents.List[0].ID != "main" {
		t.Errorf("Expected agent ID 'main', got '%s'", cfg.Agents.List[0].ID)
	}
}

// TestLoadConfigYAML tests loading YAML config (legacy format)
func TestLoadConfigYAML(t *testing.T) {
	// Create temp YAML config file
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	yamlContent := `agents:
  defaults:
    workspace: /tmp/test-workspace
    model: claude-sonnet-4-5
  list:
    - id: main
      default: true
      workspace: /tmp/test-workspace
`

	if _, err := tmpfile.WriteString(yamlContent); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	// Load config
	cfg, err := LoadConfigFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load YAML config: %v", err)
	}

	// Verify loaded values
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Fatal("Agents config is nil")
	}

	if cfg.Agents.Defaults.Workspace != "/tmp/test-workspace" {
		t.Errorf("Expected workspace '/tmp/test-workspace', got '%s'", cfg.Agents.Defaults.Workspace)
	}
}

// TestLoadConfigStandardJSON tests loading standard JSON (no comments, no trailing commas)
func TestLoadConfigStandardJSON(t *testing.T) {
	// Create temp JSON config file
	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Standard JSON (strict, no trailing commas or comments)
	jsonContent := `{
  "agents": {
    "defaults": {
      "workspace": "/tmp/test-workspace",
      "model": "claude-sonnet-4-5"
    },
    "list": [
      {
        "id": "main",
        "default": true,
        "workspace": "/tmp/test-workspace"
      }
    ]
  }
}`

	if _, err := tmpfile.WriteString(jsonContent); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	// Load config
	cfg, err := LoadConfigFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load JSON config: %v", err)
	}

	// Verify loaded values
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Fatal("Agents config is nil")
	}

	if cfg.Agents.Defaults.Workspace != "/tmp/test-workspace" {
		t.Errorf("Expected workspace '/tmp/test-workspace', got '%s'", cfg.Agents.Defaults.Workspace)
	}
}

// TestSaveConfigJSON tests saving config as JSON
func TestSaveConfigJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// Create test config
	cfg := &Config{
		Agents: &AgentsConfig{
			Defaults: &AgentDefaultsConfig{
				Workspace: "/tmp/test",
			},
			List: []AgentConfig{
				{
					ID:        "main",
					Default:   true,
					Workspace: "/tmp/test",
				},
			},
		},
	}

	// Save config
	if err := SaveConfigToFile(cfg, configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load it back
	loadedCfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	// Verify values match
	if loadedCfg.Agents.Defaults.Workspace != cfg.Agents.Defaults.Workspace {
		t.Errorf("Workspace mismatch after save/load")
	}
}

// TestSaveConfigYAML tests saving config as YAML
func TestSaveConfigYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create test config
	cfg := &Config{
		Agents: &AgentsConfig{
			Defaults: &AgentDefaultsConfig{
				Workspace: "/tmp/test",
			},
			List: []AgentConfig{
				{
					ID:        "main",
					Default:   true,
					Workspace: "/tmp/test",
				},
			},
		},
	}

	// Save config
	if err := SaveConfigToFile(cfg, configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load it back
	loadedCfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	// Verify values match
	if loadedCfg.Agents.Defaults.Workspace != cfg.Agents.Defaults.Workspace {
		t.Errorf("Workspace mismatch after save/load")
	}
}

// TestResolveConfigPath_JSON_Priority tests that JSON config is preferred over YAML
func TestResolveConfigPath_JSON_Priority(t *testing.T) {
	// Create temp home directory
	tmpHome, err := os.MkdirTemp("", "home-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	// Override home directory for this test
	originalHome, _ := os.LookupEnv("HOME")
	os.Setenv("HOME", tmpHome)
	defer func() {
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		} else {
			os.Unsetenv("HOME")
		}
	}()

	// Create .memdoor directory
	configDir := filepath.Join(tmpHome, ".memdoor")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create both JSON and YAML config files
	jsonPath := filepath.Join(configDir, "config.json")
	yamlPath := filepath.Join(configDir, "config.yaml")

	if err := os.WriteFile(jsonPath, []byte(`{"agents":{}}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(yamlPath, []byte(`agents: {}`), 0600); err != nil {
		t.Fatal(err)
	}

	// Resolve config path
	resolvedPath, err := ResolveConfigPath()
	if err != nil {
		t.Fatalf("Failed to resolve config path: %v", err)
	}

	// Should prefer JSON over YAML
	if resolvedPath != jsonPath {
		t.Errorf("Expected JSON config to be preferred, got %s instead of %s", resolvedPath, jsonPath)
	}
}

// TestLoadConfigNonExistent tests that loading a non-existent config returns defaults
func TestLoadConfigNonExistent(t *testing.T) {
	cfg, err := LoadConfigFromFile("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("Expected no error for non-existent config, got: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected default config, got nil")
	}

	// Should have default agent
	if cfg.Agents == nil || len(cfg.Agents.List) == 0 {
		t.Fatal("Default config should have at least one agent")
	}
}

// TestExpandUserPath tests ~ expansion
func TestExpandUserPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde with path",
			input:    "~/test",
			expected: "", // Will be filled with home dir
		},
		{
			name:     "just tilde",
			input:    "~",
			expected: "", // Will be filled with home dir
		},
		{
			name:     "no tilde",
			input:    "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "relative path",
			input:    "relative/path",
			expected: "relative/path",
		},
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.expected
			if tt.input == "~" {
				expected = home
			} else if tt.input == "~/test" {
				expected = filepath.Join(home, "test")
			}

			result := ExpandUserPath(tt.input)
			if result != expected {
				t.Errorf("ExpandUserPath(%s) = %s, expected %s", tt.input, result, expected)
			}
		})
	}
}
