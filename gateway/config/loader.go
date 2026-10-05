package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"memdoor/pkg/secrets"
	"memdoor/pkg/shared"

	"github.com/titanous/json5"
	"gopkg.in/yaml.v3"
)

// Configuration file loading and resolution
// Pattern: OpenClaw src/config/io.ts
//
// Supports:
// - JSON5 format (preferred, supports comments and trailing commas)
// - YAML format (legacy, for backward compatibility)
// - JSON format (standard JSON, subset of JSON5)
// - Environment variable substitution
// - ~ expansion for home directory
// - Default configuration paths

const (
	// DefaultConfigFileName is the name of the config file (JSON5 preferred)
	DefaultConfigFileName = "config.json"

	// LegacyConfigFileName is the old YAML config file name
	LegacyConfigFileName = "config.yaml"
)

// LoadConfig loads the Memdoor configuration from the default location
// Pattern: OpenClaw config loading with defaults
func LoadConfig() (*Config, error) {
	configPath, err := ResolveConfigPath()
	if err != nil {
		return nil, err
	}

	return LoadConfigFromFile(configPath)
}

// LoadConfigFromFile loads configuration from a specific file path
// Supports JSON5, JSON, and YAML formats (auto-detected by file extension)
// Processes $include directives for config splitting
func LoadConfigFromFile(path string) (*Config, error) {
	// Expand ~ to home directory
	expandedPath := ExpandUserPath(path)

	// Read file
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		// If file doesn't exist, return default config
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", expandedPath, err)
	}

	// Detect format
	ext := strings.ToLower(filepath.Ext(expandedPath))
	format := "json5"
	if ext == ".yaml" || ext == ".yml" {
		format = "yaml"
	}

	// Process $include directives
	baseDir := filepath.Dir(expandedPath)
	visited := make(map[string]bool)
	visited[expandedPath] = true

	processedData, err := processIncludes(data, baseDir, format, visited)
	if err != nil {
		return nil, fmt.Errorf("failed to process includes in %s: %w", expandedPath, err)
	}

	// Parse processed config
	var config Config

	switch format {
	case "yaml":
		// YAML format (legacy)
		if err := yaml.Unmarshal(processedData, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config file %s: %w", expandedPath, err)
		}

	case "json5":
		// JSON5 format (preferred) or standard JSON
		// Try JSON5 first (supports comments, trailing commas)
		if err := json5.Unmarshal(processedData, &config); err != nil {
			// Fallback to standard JSON
			if jsonErr := json.Unmarshal(processedData, &config); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse JSON5/JSON config file %s: %w (also tried JSON: %v)", expandedPath, err, jsonErr)
			}
		}

	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .json, .json5, .yaml, .yml)", ext)
	}

	// Substitute environment variables
	if err := substituteEnvVars(&config); err != nil {
		return nil, fmt.Errorf("failed to substitute environment variables: %w", err)
	}

	// Apply defaults
	applyDefaults(&config)

	// Resolve secret references in sensitive fields
	if err := resolveSecretRefs(&config); err != nil {
		return nil, fmt.Errorf("failed to resolve secret references: %w", err)
	}

	return &config, nil
}

// ResolveConfigPath finds the configuration file path
// The current directory's .memdoor/ first, then ~/.memdoor/ (shared.ProjectOrHome);
// config.json (JSON5) before the legacy config.yaml.
func ResolveConfigPath() (string, error) {
	for _, name := range []string{DefaultConfigFileName, LegacyConfigFileName} {
		if p := shared.ProjectOrHome("", name); fileExists(p) {
			return p, nil
		}
	}
	return shared.MemdoorHome(DefaultConfigFileName), nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ExpandUserPath expands ~ to the user's home directory
// Pattern: OpenClaw path expansion
func ExpandUserPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return home
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}

	return path
}

// substituteEnvVars replaces ${VAR} placeholders with environment variable values
// Pattern: OpenClaw environment variable substitution
func substituteEnvVars(config *Config) error {
	// Substitute in session store path
	if config.Session != nil && config.Session.Store != "" {
		config.Session.Store = os.ExpandEnv(config.Session.Store)
	}

	// Substitute in cron store path
	if config.Cron != nil && config.Cron.Store != "" {
		config.Cron.Store = os.ExpandEnv(config.Cron.Store)
	}

	// Substitute in agent workspace paths
	if config.Agents != nil {
		if config.Agents.Defaults != nil && config.Agents.Defaults.Workspace != "" {
			config.Agents.Defaults.Workspace = os.ExpandEnv(config.Agents.Defaults.Workspace)
		}

		for i := range config.Agents.List {
			if config.Agents.List[i].Workspace != "" {
				config.Agents.List[i].Workspace = os.ExpandEnv(config.Agents.List[i].Workspace)
			}
			if config.Agents.List[i].AgentDir != "" {
				config.Agents.List[i].AgentDir = os.ExpandEnv(config.Agents.List[i].AgentDir)
			}
		}
	}

	return nil
}

// applyDefaults applies default values to the configuration
// Pattern: OpenClaw defaults application
func applyDefaults(config *Config) {
	// Apply agent defaults
	if config.Agents == nil {
		config.Agents = &AgentsConfig{}
	}

	// Ensure at least one agent exists (main)
	if len(config.Agents.List) == 0 {
		config.Agents.List = []AgentConfig{
			{
				ID:      "main",
				Default: true,
				Name:    "Main Agent",
			},
		}
	}

	// Apply session defaults
	if config.Session == nil {
		base := shared.MemdoorHome()
		config.Session = &SessionConfig{
			Store:   filepath.Join(base, "agents", "{agentId}", "sessions.json"),
			MainKey: "main",
			Scope:   "per-agent",
		}
	}

	// Expand ~ in session store path
	if config.Session.Store != "" {
		config.Session.Store = ExpandUserPath(config.Session.Store)
	}

	// Apply cron defaults
	if config.Cron == nil {
		base := shared.MemdoorHome()
		config.Cron = &CronConfig{
			Enabled: true,
			Store:   filepath.Join(base, "cron", "jobs.json"),
		}
	}

	// Expand ~ in cron store path
	if config.Cron.Store != "" {
		config.Cron.Store = ExpandUserPath(config.Cron.Store)
	}

	// Apply database defaults
	if config.Database == nil {
		dataDir := shared.MemdoorHome("data")
		defaultDBPath := filepath.Join(dataDir, "memdoor.db")
		config.Database = &DatabaseConfig{
			Type: "sqlite",
			SQLite: &SQLiteDatabaseConfig{
				Path: defaultDBPath,
			},
		}
	}

	// Expand ~ in database path
	if config.Database != nil && config.Database.SQLite != nil && config.Database.SQLite.Path != "" {
		config.Database.SQLite.Path = ExpandUserPath(config.Database.SQLite.Path)
	}
}

// DefaultConfig returns a default configuration
// Pattern: OpenClaw default configuration
func DefaultConfig() *Config {
	base := shared.MemdoorHome()
	defaultWorkspace := filepath.Join(base, "workspace")
	dataDir := shared.MemdoorHome("data")
	defaultDBPath := filepath.Join(dataDir, "memdoor.db")

	config := &Config{
		Agents: &AgentsConfig{
			Defaults: &AgentDefaultsConfig{
				Workspace: defaultWorkspace,
				// No Model — the provider decides. See AgentDefaultsConfig.
			},
			List: []AgentConfig{
				{
					ID:        "main",
					Default:   true,
					Name:      "Main Agent",
					Workspace: defaultWorkspace,
				},
			},
		},
		Session: &SessionConfig{
			Store:   filepath.Join(base, "agents", "{agentId}", "sessions.json"),
			MainKey: "main",
			Scope:   "per-agent",
		},
		Cron: &CronConfig{
			Enabled: true,
			Store:   filepath.Join(base, "cron", "jobs.json"),
			Jobs:    []CronJob{},
		},
		Database: &DatabaseConfig{
			Type: "sqlite",
			SQLite: &SQLiteDatabaseConfig{
				Path: defaultDBPath,
			},
		},
	}

	return config
}

// SaveConfigToFile saves the configuration to a specific file
// Saves as JSON5 for .json/.json5 files, YAML for .yaml/.yml files
func SaveConfigToFile(config *Config, path string) error {
	// Expand ~ to home directory
	expandedPath := ExpandUserPath(path)

	// Ensure directory exists
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	var data []byte
	var err error

	// Detect format by extension
	ext := strings.ToLower(filepath.Ext(expandedPath))
	switch ext {
	case ".yaml", ".yml":
		// Save as YAML (legacy)
		data, err = yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config to YAML: %w", err)
		}

	case ".json", ".json5", "":
		// Save as JSON with indentation (JSON5-compatible)
		// Note: json5 package doesn't have Marshal, so we use standard JSON
		// which is valid JSON5. Users can add comments and trailing commas manually.
		data, err = json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %w", err)
		}

	default:
		return fmt.Errorf("unsupported config file format: %s (supported: .json, .json5, .yaml, .yml)", ext)
	}

	// Write to file
	if err := os.WriteFile(expandedPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", expandedPath, err)
	}

	return nil
}

// resolveSecretRefs resolves SecretRef JSON values in sensitive config fields.
// Fields that contain JSON like {"source":"keychain","id":"my-key"} are resolved
// to the actual secret value. Plain strings pass through unchanged.
func resolveSecretRefs(config *Config) error {
	resolver := secrets.DefaultResolver()
	ctx := context.Background()

	// Resolve Slack channel secrets
	if config.Channels != nil && config.Channels.Slack != nil {
		slack := config.Channels.Slack
		var err error
		if slack.BotToken, err = resolver.ResolveString(ctx, slack.BotToken); err != nil {
			return fmt.Errorf("channels.slack.botToken: %w", err)
		}
		if slack.AppToken, err = resolver.ResolveString(ctx, slack.AppToken); err != nil {
			return fmt.Errorf("channels.slack.appToken: %w", err)
		}
		if slack.SigningSecret, err = resolver.ResolveString(ctx, slack.SigningSecret); err != nil {
			return fmt.Errorf("channels.slack.signingSecret: %w", err)
		}
	}

	// Resolve database secrets
	if config.Database != nil && config.Database.Postgres != nil {
		var err error
		if config.Database.Postgres.URL, err = resolver.ResolveString(ctx, config.Database.Postgres.URL); err != nil {
			return fmt.Errorf("database.postgres.url: %w", err)
		}
	}

	return nil
}
