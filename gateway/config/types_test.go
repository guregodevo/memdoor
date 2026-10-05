package config

import (
	"testing"
)

// TestValidateConfig_DuplicateAgentID tests duplicate agent ID detection
func TestValidateConfig_DuplicateAgentID(t *testing.T) {
	cfg := &Config{
		Agents: &AgentsConfig{
			List: []AgentConfig{
				{ID: "main", Workspace: "/workspace1"},
				{ID: "main", Workspace: "/workspace2"}, // duplicate
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate agent ID, got nil")
	}
	if err.Error() != "duplicate agent ID: main" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestValidateConfig_InvalidBindingAgentRef tests binding validation
func TestValidateConfig_InvalidBindingAgentRef(t *testing.T) {
	cfg := &Config{
		Agents: &AgentsConfig{
			List: []AgentConfig{
				{ID: "main", Workspace: "/workspace"},
			},
		},
		Bindings: []Binding{
			{
				AgentID: "nonexistent",
				Match: BindingMatch{
					Channel: "whatsapp",
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid binding agent reference, got nil")
	}
	if err.Error() != "binding 0 references unknown agent: nonexistent" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestValidateConfig_InvalidCronJobAgentRef tests cron job validation
func TestValidateConfig_InvalidCronJobAgentRef(t *testing.T) {
	cfg := &Config{
		Agents: &AgentsConfig{
			List: []AgentConfig{
				{ID: "main", Workspace: "/workspace"},
			},
		},
		Cron: &CronConfig{
			Jobs: []CronJob{
				{
					ID:       "test-job",
					AgentID:  "nonexistent",
					Schedule: "0 9 * * *",
					Message:  "test",
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid cron job agent reference, got nil")
	}
	if err.Error() != "cron job test-job references unknown agent: nonexistent" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestValidateConfig_MissingWorkspace tests workspace validation
func TestValidateConfig_MissingWorkspace(t *testing.T) {
	cfg := &Config{
		Agents: &AgentsConfig{
			List: []AgentConfig{
				{ID: "main", Workspace: ""}, // missing workspace
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing workspace, got nil")
	}
	if err.Error() != "agent main: workspace is required (either per-agent or in defaults)" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestValidateConfig_ValidConfig tests a valid configuration
func TestValidateConfig_ValidConfig(t *testing.T) {
	cfg := &Config{
		Agents: &AgentsConfig{
			Defaults: &AgentDefaultsConfig{
				Workspace: "/default-workspace",
			},
			List: []AgentConfig{
				{ID: "main", Workspace: "/workspace1", Default: true},
				{ID: "work", Workspace: "/workspace2"},
			},
		},
		Bindings: []Binding{
			{
				AgentID: "work",
				Match: BindingMatch{
					Channel:   "whatsapp",
					AccountID: "business",
				},
			},
		},
		Cron: &CronConfig{
			Jobs: []CronJob{
				{
					ID:       "morning-job",
					AgentID:  "main",
					Schedule: "0 7 * * *",
					Message:  "Good morning",
				},
			},
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}
}

// TestGetDefaultAgent tests default agent retrieval
func TestGetDefaultAgent(t *testing.T) {
	cfg := &Config{
		Agents: &AgentsConfig{
			List: []AgentConfig{
				{ID: "first", Workspace: "/workspace1"},
				{ID: "main", Workspace: "/workspace2", Default: true},
				{ID: "third", Workspace: "/workspace3"},
			},
		},
	}

	agent := cfg.GetDefaultAgent()
	if agent == nil {
		t.Fatal("expected default agent, got nil")
	}
	if agent.ID != "main" {
		t.Errorf("expected default agent 'main', got '%s'", agent.ID)
	}
}

// TestGetDefaultAgent_NoDefault tests fallback to first agent
func TestGetDefaultAgent_NoDefault(t *testing.T) {
	cfg := &Config{
		Agents: &AgentsConfig{
			List: []AgentConfig{
				{ID: "first", Workspace: "/workspace1"},
				{ID: "second", Workspace: "/workspace2"},
			},
		},
	}

	agent := cfg.GetDefaultAgent()
	if agent == nil {
		t.Fatal("expected fallback to first agent, got nil")
	}
	if agent.ID != "first" {
		t.Errorf("expected first agent as default, got '%s'", agent.ID)
	}
}

// TestGetAgent tests agent retrieval by ID
func TestGetAgent(t *testing.T) {
	cfg := &Config{
		Agents: &AgentsConfig{
			List: []AgentConfig{
				{ID: "main", Workspace: "/workspace1"},
				{ID: "work", Workspace: "/workspace2"},
			},
		},
	}

	agent := cfg.GetAgent("work")
	if agent == nil {
		t.Fatal("expected agent 'work', got nil")
	}
	if agent.ID != "work" {
		t.Errorf("expected agent 'work', got '%s'", agent.ID)
	}

	agent = cfg.GetAgent("nonexistent")
	if agent != nil {
		t.Errorf("expected nil for nonexistent agent, got %v", agent)
	}
}

// TestIsToolAllowed tests tool filtering
func TestIsToolAllowed(t *testing.T) {
	tests := []struct {
		name     string
		agent    AgentConfig
		toolName string
		expected bool
	}{
		{
			name: "no restrictions - allow all",
			agent: AgentConfig{
				ID:        "main",
				Workspace: "/workspace",
			},
			toolName: "read",
			expected: true,
		},
		{
			name: "allowlist - tool in list",
			agent: AgentConfig{
				ID:        "main",
				Workspace: "/workspace",
				Tools: &AgentToolsConfig{
					Allow: []string{"read", "write"},
				},
			},
			toolName: "read",
			expected: true,
		},
		{
			name: "allowlist - tool not in list",
			agent: AgentConfig{
				ID:        "main",
				Workspace: "/workspace",
				Tools: &AgentToolsConfig{
					Allow: []string{"read", "write"},
				},
			},
			toolName: "exec",
			expected: false,
		},
		{
			name: "denylist - tool in list",
			agent: AgentConfig{
				ID:        "main",
				Workspace: "/workspace",
				Tools: &AgentToolsConfig{
					Deny: []string{"exec", "bash"},
				},
			},
			toolName: "exec",
			expected: false,
		},
		{
			name: "denylist - tool not in list",
			agent: AgentConfig{
				ID:        "main",
				Workspace: "/workspace",
				Tools: &AgentToolsConfig{
					Deny: []string{"exec", "bash"},
				},
			},
			toolName: "read",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.agent.IsToolAllowed(tt.toolName)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
