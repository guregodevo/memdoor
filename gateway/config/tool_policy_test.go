package config

import (
	"testing"
)

func TestAgentConfig_IsToolAllowed(t *testing.T) {
	tests := []struct {
		name     string
		config   *AgentConfig
		toolName string
		allowed  bool
	}{
		{
			name: "no restrictions - all allowed",
			config: &AgentConfig{
				ID:    "test",
				Tools: nil,
			},
			toolName: "read_file",
			allowed:  true,
		},
		{
			name: "profile minimal - session_status allowed",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Profile: "minimal",
				},
			},
			toolName: "session_status",
			allowed:  true,
		},
		{
			name: "profile minimal - read_file denied",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Profile: "minimal",
				},
			},
			toolName: "read_file",
			allowed:  false,
		},
		{
			name: "explicit allow - read_file allowed",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Allow: []string{"read_file", "write_file"},
				},
			},
			toolName: "read_file",
			allowed:  true,
		},
		{
			name: "explicit allow - bash denied",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Allow: []string{"read_file", "write_file"},
				},
			},
			toolName: "bash",
			allowed:  false,
		},
		{
			name: "explicit deny - read_file denied",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Deny: []string{"bash", "read_file"},
				},
			},
			toolName: "read_file",
			allowed:  false,
		},
		{
			name: "explicit deny - write_file allowed",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Deny: []string{"bash"},
				},
			},
			toolName: "write_file",
			allowed:  true,
		},
		{
			name: "profile + explicit allow - both allowed",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Profile: "minimal",
					Allow:   []string{"web_fetch"},
				},
			},
			toolName: "web_fetch",
			allowed:  true,
		},
		{
			name: "deny overrides allow",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Allow: []string{"read_file", "write_file"},
					Deny:  []string{"write_file"},
				},
			},
			toolName: "write_file",
			allowed:  false,
		},
		{
			name: "group:fs expands to individual tools",
			config: &AgentConfig{
				ID: "test",
				Tools: &AgentToolsConfig{
					Allow: []string{"group:fs"},
				},
			},
			toolName: "read_file",
			allowed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsToolAllowed(tt.toolName)
			if result != tt.allowed {
				t.Errorf("IsToolAllowed(%s) = %v, want %v", tt.toolName, result, tt.allowed)
			}
		})
	}
}

func TestNormalizeToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no normalization needed",
			input:    "read_file",
			expected: "read_file",
		},
		{
			name:     "exec alias to bash",
			input:    "exec",
			expected: "bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeToolName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeToolName(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
