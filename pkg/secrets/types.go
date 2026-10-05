package secrets

// SecretRef references a secret stored in an external provider.
// In config JSON: {"source": "env", "id": "ANTHROPIC_API_KEY"}
type SecretRef struct {
	Source string `json:"source"` // Provider name: "env", "keychain", "file", "exec"
	ID     string `json:"id"`     // Provider-specific identifier
}

// SecretScope restricts who can access a secret
type SecretScope struct {
	AgentID string `json:"agent_id,omitempty"`
	ToolID  string `json:"tool_id,omitempty"`
}

// AuditFinding represents a plaintext secret detected in config
type AuditFinding struct {
	Location    string // Config path, e.g. "channels.slack.botToken"
	Severity    string // "critical", "warning"
	Description string
	Fix         string // Suggested remediation command
}

// AuditSeverityCritical indicates a high-risk plaintext secret
const AuditSeverityCritical = "critical"

// AuditSeverityWarning indicates a medium-risk plaintext secret
const AuditSeverityWarning = "warning"
