package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"memdoor/gateway/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("configuration invalid: %w", err)
		}

		fmt.Println("Configuration is valid")
		fmt.Println()
		fmt.Println("Summary:")
		if cfg.Agents != nil {
			fmt.Printf("  Agents:    %d\n", len(cfg.Agents.List))
		}
		fmt.Printf("  Bindings:  %d\n", len(cfg.Bindings))
		if cfg.Cron != nil {
			fmt.Printf("  Cron jobs: %d\n", len(cfg.Cron.Jobs))
		}
		configPath, _ := config.ResolveConfigPath()
		fmt.Printf("  Config:    %s\n", configPath)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display full configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [path]",
	Short: "Get configuration value by dot-notation path",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Convert to map for dot-notation access
		data, _ := json.Marshal(cfg)
		var m map[string]interface{}
		_ = json.Unmarshal(data, &m)

		value := getNestedValue(m, args[0])
		if value == nil {
			return fmt.Errorf("path not found: %s", args[0])
		}

		switch v := value.(type) {
		case string:
			fmt.Println(v)
		default:
			out, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(out))
		}
		return nil
	},
}

func getNestedValue(m map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = m

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}
	return current
}

func init() {
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	rootCmd.AddCommand(configCmd)
}
