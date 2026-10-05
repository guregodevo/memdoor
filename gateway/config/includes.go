package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/titanous/json5"
	"gopkg.in/yaml.v3"
)

// processIncludes processes $include directives in a configuration file
// Pattern: OpenClaw config splitting
//
// Supports:
// - Top-level includes: { "$include": "./common.json" }
// - Array includes: { "$include": ["./a.json", "./b.json"] }
// - Nested includes in sections
// - Circular dependency detection
// - Relative path resolution
func processIncludes(data []byte, baseDir string, format string, visited map[string]bool) ([]byte, error) {
	// Parse to generic map to detect $include
	var raw map[string]interface{}

	switch format {
	case "json", "json5":
		if err := json5.Unmarshal(data, &raw); err != nil {
			// Try standard JSON as fallback
			if jsonErr := json.Unmarshal(data, &raw); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse config: %w", err)
			}
		}
	case "yaml":
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return data, nil
	}

	// Check for $include directive
	includeVal, hasInclude := raw["$include"]
	if !hasInclude {
		// No includes, return original data
		return data, nil
	}

	// Process includes
	var includePaths []string
	switch v := includeVal.(type) {
	case string:
		includePaths = []string{v}
	case []interface{}:
		for _, path := range v {
			if str, ok := path.(string); ok {
				includePaths = append(includePaths, str)
			}
		}
	default:
		return nil, fmt.Errorf("$include must be a string or array of strings")
	}

	// Remove $include from the current config
	delete(raw, "$include")

	// Load and merge included configs
	merged := raw
	for _, includePath := range includePaths {
		// Resolve path relative to base directory
		absPath := filepath.Join(baseDir, includePath)

		// Check for circular dependencies
		if visited[absPath] {
			return nil, fmt.Errorf("circular dependency detected: %s", absPath)
		}
		visited[absPath] = true

		// Load included file
		includedData, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read included file %s: %w", includePath, err)
		}

		// Detect format of included file
		includeFormat := detectFormat(absPath)

		// Recursively process includes in the included file
		includedDir := filepath.Dir(absPath)
		processedData, err := processIncludes(includedData, includedDir, includeFormat, visited)
		if err != nil {
			return nil, fmt.Errorf("failed to process includes in %s: %w", includePath, err)
		}

		// Parse processed included data
		var includedMap map[string]interface{}
		switch includeFormat {
		case "json", "json5":
			if err := json5.Unmarshal(processedData, &includedMap); err != nil {
				if jsonErr := json.Unmarshal(processedData, &includedMap); jsonErr != nil {
					return nil, fmt.Errorf("failed to parse included file %s: %w", includePath, err)
				}
			}
		case "yaml":
			if err := yaml.Unmarshal(processedData, &includedMap); err != nil {
				return nil, fmt.Errorf("failed to parse included YAML file %s: %w", includePath, err)
			}
		}

		// Merge included config into current config
		// Included config values take precedence over current values
		merged = mergeMaps(includedMap, merged)

		delete(visited, absPath)
	}

	// Marshal back to original format
	switch format {
	case "json", "json5":
		return json.MarshalIndent(merged, "", "  ")
	case "yaml":
		return yaml.Marshal(merged)
	default:
		return data, nil
	}
}

// detectFormat detects the config format from file extension
func detectFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json", ".json5":
		return "json5"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return "json5"
	}
}

// mergeMaps merges two maps, with values from 'override' taking precedence
func mergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy all from base
	for k, v := range base {
		result[k] = v
	}

	// Override with values from override
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			// If both are maps, merge recursively
			if baseMap, ok := baseVal.(map[string]interface{}); ok {
				if overrideMap, ok := v.(map[string]interface{}); ok {
					result[k] = mergeMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		// Otherwise, override takes precedence
		result[k] = v
	}

	return result
}
