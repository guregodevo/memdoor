package config

import (
	"fmt"
	"memdoor/pkg/shared"
	"path/filepath"
	"time"
)

// Agent configuration types following OpenClaw's pattern
// Pattern: OpenClaw src/config/types.agents.ts
//
// This package defines the configuration schema for Memdoor,
// enabling per-agent workspace isolation, tool filtering, and model configuration.

// Config represents the complete configuration for Memdoor
type Config struct {
	WorkspaceID string          `yaml:"workspace_id,omitempty" json:"workspace_id,omitempty"`
	Agents      *AgentsConfig   `yaml:"agents,omitempty" json:"agents,omitempty"`
	Bindings    []Binding       `yaml:"bindings,omitempty" json:"bindings,omitempty"`
	Session     *SessionConfig  `yaml:"session,omitempty" json:"session,omitempty"`
	Cron        *CronConfig     `yaml:"cron,omitempty" json:"cron,omitempty"`
	Queue       *QueueConfig    `yaml:"queue,omitempty" json:"queue,omitempty"`
	Channels    *ChannelsConfig `yaml:"channels,omitempty" json:"channels,omitempty"`
	Tools       *ToolsConfig    `yaml:"tools,omitempty" json:"tools,omitempty"`       // Week 27: A2A config
	Agent       *AgentConfig    `yaml:"agent,omitempty" json:"agent,omitempty"`       // Single agent config (backwards compat)
	Database    *DatabaseConfig `yaml:"database,omitempty" json:"database,omitempty"` // Database configuration
}

// AgentsConfig contains agent-related configuration
type AgentsConfig struct {
	Defaults *AgentDefaultsConfig `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	List     []AgentConfig        `yaml:"list,omitempty" json:"list,omitempty"`
}

// AgentDefaultsConfig provides default values for agents.
//
// Model selection is intentionally absent — the model is the
// provider's (providers.ClientFactory: a pin, else the engine chosen at
// start, else the first connected provider); a config-file Model field
// would be a stale shadow with no role to play.
type AgentDefaultsConfig struct {
	Workspace  string            `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	Tools      *AgentToolsConfig `yaml:"tools,omitempty" json:"tools,omitempty"`
	Identity   *IdentityConfig   `yaml:"identity,omitempty" json:"identity,omitempty"`
	Compaction *CompactionConfig `yaml:"compaction,omitempty" json:"compaction,omitempty"`
}

// AgentConfig represents a single agent's configuration.
//
// Model selection is intentionally absent — see AgentDefaultsConfig.
type AgentConfig struct {
	ID         string            `yaml:"id" json:"id"`
	Default    bool              `yaml:"default,omitempty" json:"default,omitempty"`
	Name       string            `yaml:"name,omitempty" json:"name,omitempty"`
	Workspace  string            `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	AgentDir   string            `yaml:"agent_dir,omitempty" json:"agent_dir,omitempty"`
	Skills     []string          `yaml:"skills,omitempty" json:"skills,omitempty"`
	Tools      *AgentToolsConfig `yaml:"tools,omitempty" json:"tools,omitempty"`
	Identity   *IdentityConfig   `yaml:"identity,omitempty" json:"identity,omitempty"`
	Compaction *CompactionConfig `yaml:"compaction,omitempty" json:"compaction,omitempty"`
}

// AgentToolsConfig defines tool filtering for an agent
// Pattern: OpenClaw tool policy configuration with profiles
type AgentToolsConfig struct {
	Profile string   `yaml:"profile,omitempty" json:"profile,omitempty"` // Tool profile: "minimal", "default", "coding", "messaging", "full"
	Allow   []string `yaml:"allow,omitempty" json:"allow,omitempty"`     // Whitelist: only these tools (merged with profile)
	Deny    []string `yaml:"deny,omitempty" json:"deny,omitempty"`       // Blacklist: exclude these tools
}

// Binding defines how messages are routed to agents
// Pattern: OpenClaw multi-agent binding configuration
// Priority: peer match > account match > channel match > default agent
type Binding struct {
	AgentID string       `yaml:"agent_id" json:"agentId"`
	Match   BindingMatch `yaml:"match" json:"match"`
}

// BindingMatch specifies the matching criteria for a binding
type BindingMatch struct {
	Channel   string     `yaml:"channel,omitempty" json:"channel,omitempty"`      // Channel name: "whatsapp", "telegram", "slack"
	AccountID string     `yaml:"account_id,omitempty" json:"accountId,omitempty"` // Account ID for multi-account channels
	Peer      *PeerMatch `yaml:"peer,omitempty" json:"peer,omitempty"`            // Specific peer (DM or group)
}

// PeerMatch specifies a specific peer to match (DM or group)
type PeerMatch struct {
	Kind string `yaml:"kind" json:"kind"` // "dm" or "group"
	ID   string `yaml:"id" json:"id"`     // Peer ID (phone number for WhatsApp DM, JID for group)
}

// IdentityConfig configures agent identity (name, emoji, etc.)
type IdentityConfig struct {
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
	Emoji  string `yaml:"emoji,omitempty" json:"emoji,omitempty"`
	Avatar string `yaml:"avatar,omitempty" json:"avatar,omitempty"`
}

// Duration: "1h", "30m", etc.

// CompactionConfig configures automatic context compaction
// Pattern: OpenClaw src/config/types.agents.ts compaction settings
type CompactionConfig struct {
	Enabled           *bool              `yaml:"enabled,omitempty" json:"enabled,omitempty"`                     // Compact at the threshold (default: true); a turn over the window is fitted regardless
	CompactionPercent int                `yaml:"compactionPercent,omitempty" json:"compactionPercent,omitempty"` // Trigger compaction at % of context window (default: 60, never past 200K tokens)
	ThresholdTokens   int                `yaml:"thresholdTokens,omitempty" json:"thresholdTokens,omitempty"`     // A fixed trigger in tokens; takes precedence over compactionPercent
	KeepRecentTokens  int                `yaml:"keepRecentTokens,omitempty" json:"keepRecentTokens,omitempty"`   // Recent conversation /compact keeps word for word (default: 12000)
	MemoryFlush       *MemoryFlushConfig `yaml:"memoryFlush,omitempty" json:"memoryFlush,omitempty"`             // Pre-compaction memory flush
}

// On reports whether threshold compaction is on: unset is on.
func (c *CompactionConfig) On() bool { return c == nil || c.Enabled == nil || *c.Enabled }

// MemoryFlushConfig configures automatic memory flush before compaction
// Pattern: OpenClaw src/auto-reply/reply/memory-flush.ts
type MemoryFlushConfig struct {
	Enabled      bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`           // Enable memory flush (default: true)
	Prompt       string `yaml:"prompt,omitempty" json:"prompt,omitempty"`             // User prompt for memory flush
	SystemPrompt string `yaml:"systemPrompt,omitempty" json:"systemPrompt,omitempty"` // System prompt for memory flush
}

// Session truncation defaults
const (
	DefaultMaxToolResultBytes    = 50000
	DefaultTruncationMessageTail = "\n\n[... truncated for context management ...]"
)

// SessionConfig configures session storage
// Pattern: OpenClaw session configuration
type SessionConfig struct {
	Store                 string `yaml:"store,omitempty" json:"store,omitempty"`                                     // Path template: ~/.memdoor/agents/{agentId}/sessions.json
	MainKey               string `yaml:"main_key,omitempty" json:"main_key,omitempty"`                               // Main session key suffix (default: "main")
	Scope                 string `yaml:"scope,omitempty" json:"scope,omitempty"`                                     // "global" or "per-agent"
	MaxToolResultBytes    int    `yaml:"max_tool_result_bytes,omitempty" json:"max_tool_result_bytes,omitempty"`     // Maximum bytes per tool result (default: 50KB)
	TruncationMessageTail string `yaml:"truncation_message_tail,omitempty" json:"truncation_message_tail,omitempty"` // Message appended when truncated
}

// CronConfig configures cron job scheduling
// Pattern: OpenClaw cron configuration
type CronConfig struct {
	Enabled bool      `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Store   string    `yaml:"store,omitempty" json:"store,omitempty"` // Path to cron job store
	Jobs    []CronJob `yaml:"jobs,omitempty" json:"jobs,omitempty"`
}

// CronJob represents a scheduled task
// Pattern: OpenClaw cron job format with agent assignment
type CronJob struct {
	ID       string `yaml:"id" json:"id"`
	Schedule string `yaml:"schedule" json:"schedule"`                     // Cron expression: "0 9 * * *"
	AgentID  string `yaml:"agent_id,omitempty" json:"agent_id,omitempty"` // Which agent runs this job
	Message  string `yaml:"message" json:"message"`
	Enabled  bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Set when an AGENT scheduled the job rather than a person: where to work,
	// where to answer, and the bounds that stop it.
	Workdir    string    `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	SessionKey string    `yaml:"session_key,omitempty" json:"session_key,omitempty"`
	MaxRuns    int       `yaml:"max_runs,omitempty" json:"max_runs,omitempty"`
	RunCount   int       `yaml:"run_count,omitempty" json:"run_count,omitempty"`
	ExpiresAt  time.Time `yaml:"expires_at,omitempty" json:"expires_at,omitempty"`
}

// QueueConfig configures the command queue system
// Pattern: OpenClaw src/auto-reply/reply/queue/types.ts
type QueueConfig struct {
	Mode       string       `yaml:"mode,omitempty" json:"mode,omitempty"`               // Queue mode: steer, followup, collect (default), steer-backlog, interrupt
	DebounceMs int          `yaml:"debounce_ms,omitempty" json:"debounce_ms,omitempty"` // Debounce timeout in milliseconds (default: 1000)
	Cap        int          `yaml:"cap,omitempty" json:"cap,omitempty"`                 // Max queue size (default: 20)
	DropPolicy string       `yaml:"drop_policy,omitempty" json:"drop_policy,omitempty"` // Drop policy: old, new, summarize (default)
	Lanes      *LanesConfig `yaml:"lanes,omitempty" json:"lanes,omitempty"`             // Lane-specific configuration
}

// LanesConfig configures global lane concurrency limits
// Pattern: OpenClaw src/process/lanes.ts
type LanesConfig struct {
	MainMaxConcurrent     int `yaml:"main_max_concurrent,omitempty" json:"main_max_concurrent,omitempty"`         // Max concurrent jobs in main lane (default: 4)
	CronMaxConcurrent     int `yaml:"cron_max_concurrent,omitempty" json:"cron_max_concurrent,omitempty"`         // Max concurrent jobs in cron lane (default: 4)
	SubagentMaxConcurrent int `yaml:"subagent_max_concurrent,omitempty" json:"subagent_max_concurrent,omitempty"` // Max concurrent jobs in subagent lane (default: 8)
	NestedMaxConcurrent   int `yaml:"nested_max_concurrent,omitempty" json:"nested_max_concurrent,omitempty"`     // Max concurrent jobs in nested lane (default: 8)
}

// GetWorkspace returns the workspace path for an agent
// Returns empty string if not configured
func (ac *AgentConfig) GetWorkspace() string {
	return ac.Workspace
}

// GetAgentDir returns the agent directory path
// Returns empty string if not configured
func (ac *AgentConfig) GetAgentDir() string {
	return ac.AgentDir
}

// GetTools returns the tool configuration for an agent
// Returns nil if not configured
func (ac *AgentConfig) GetTools() *AgentToolsConfig {
	return ac.Tools
}

// GetIdentity returns the identity configuration for an agent
// Returns nil if not configured
func (ac *AgentConfig) GetIdentity() *IdentityConfig {
	return ac.Identity
}

// HasToolsProfile checks if the agent has a tool profile configured
func (ac *AgentConfig) HasToolsProfile() bool {
	return ac.Tools != nil && ac.Tools.Profile != ""
}

// HasToolsAllowList checks if the agent has a tool allowlist configured
func (ac *AgentConfig) HasToolsAllowList() bool {
	return ac.Tools != nil && len(ac.Tools.Allow) > 0
}

// HasToolsDenyList checks if the agent has a tool denylist configured
func (ac *AgentConfig) HasToolsDenyList() bool {
	return ac.Tools != nil && len(ac.Tools.Deny) > 0
}

// GetEffectiveAllowList returns the merged allowlist from profile + explicit allow
// Pattern: OpenClaw tool policy resolution
func (ac *AgentConfig) GetEffectiveAllowList() []string {
	if ac.Tools == nil {
		return nil
	}

	allowList := make([]string, 0)

	// Start with profile allowlist (if any)
	if ac.Tools.Profile != "" {
		profilePolicy := ResolveToolProfilePolicy(ac.Tools.Profile)
		if profilePolicy != nil && len(profilePolicy.Allow) > 0 {
			allowList = append(allowList, profilePolicy.Allow...)
		}
	}

	// Merge explicit allow list
	if len(ac.Tools.Allow) > 0 {
		allowList = append(allowList, ac.Tools.Allow...)
	}

	// Expand tool groups
	if len(allowList) > 0 {
		return ExpandToolGroups(allowList)
	}

	return nil
}

// GetEffectiveDenyList returns the expanded denylist
// Pattern: OpenClaw tool policy resolution
func (ac *AgentConfig) GetEffectiveDenyList() []string {
	if ac.Tools == nil || len(ac.Tools.Deny) == 0 {
		return nil
	}

	// Expand tool groups in deny list
	return ExpandToolGroups(ac.Tools.Deny)
}

// IsToolAllowed checks if a tool is allowed for this agent
// Pattern: OpenClaw tool policy resolution with profiles
// Rules:
//  1. If denylist contains the tool: denied
//  2. If allowlist is set: only tools in allowlist are allowed
//  3. If no allowlist: all tools allowed (except denied)
func (ac *AgentConfig) IsToolAllowed(toolName string) bool {
	normalizedName := NormalizeToolName(toolName)

	// Get effective deny list (expanded with groups)
	denyList := ac.GetEffectiveDenyList()
	if len(denyList) > 0 {
		for _, denied := range denyList {
			if NormalizeToolName(denied) == normalizedName {
				return false
			}
		}
	}

	// Get effective allow list (profile + explicit, expanded with groups)
	allowList := ac.GetEffectiveAllowList()
	if len(allowList) > 0 {
		for _, allowed := range allowList {
			if NormalizeToolName(allowed) == normalizedName {
				return true
			}
		}
		return false
	}

	// No restrictions: all tools allowed (except denied)
	return true
}

// GetDefaultAgent finds the default agent in the configuration
// Returns the first agent marked as default, or the first agent in the list
// Pattern: OpenClaw src/agents/agent-scope.ts:61-73
func (sc *Config) GetDefaultAgent() *AgentConfig {
	if sc.Agents == nil || len(sc.Agents.List) == 0 {
		return nil
	}

	// Look for agent marked as default
	for i := range sc.Agents.List {
		if sc.Agents.List[i].Default {
			return &sc.Agents.List[i]
		}
	}

	// Return first agent if no default is set
	return &sc.Agents.List[0]
}

// GetAgent finds an agent by ID
// Returns nil if not found
func (sc *Config) GetAgent(agentID string) *AgentConfig {
	if sc.Agents == nil {
		return nil
	}

	for i := range sc.Agents.List {
		if sc.Agents.List[i].ID == agentID {
			return &sc.Agents.List[i]
		}
	}

	return nil
}

// ListAgentIDs returns a list of all configured agent IDs
func (sc *Config) ListAgentIDs() []string {
	if sc.Agents == nil {
		return nil
	}

	ids := make([]string, 0, len(sc.Agents.List))
	for _, agent := range sc.Agents.List {
		ids = append(ids, agent.ID)
	}
	return ids
}

// GetCronJobsForAgent returns all cron jobs assigned to a specific agent
func (sc *Config) GetCronJobsForAgent(agentID string) []CronJob {
	if sc.Cron == nil {
		return nil
	}

	jobs := make([]CronJob, 0)
	for _, job := range sc.Cron.Jobs {
		if job.AgentID == agentID {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// Validate performs basic validation on the configuration
// Pattern: OpenClaw config validation
func (sc *Config) Validate() error {
	if sc.Agents == nil {
		return fmt.Errorf("agents configuration is required")
	}

	// Build agent ID map for reference checking
	agentIDs := make(map[string]bool)
	for _, agent := range sc.Agents.List {
		// Check for duplicate agent IDs
		if agentIDs[agent.ID] {
			return fmt.Errorf("duplicate agent ID: %s", agent.ID)
		}
		agentIDs[agent.ID] = true

		// Validate workspace path is set
		if agent.Workspace == "" && (sc.Agents.Defaults == nil || sc.Agents.Defaults.Workspace == "") {
			return fmt.Errorf("agent %s: workspace is required (either per-agent or in defaults)", agent.ID)
		}
	}

	// Validate bindings reference existing agents
	for i, binding := range sc.Bindings {
		if !agentIDs[binding.AgentID] {
			return fmt.Errorf("binding %d references unknown agent: %s", i, binding.AgentID)
		}
	}

	// Validate cron job agent references
	if sc.Cron != nil {
		for _, job := range sc.Cron.Jobs {
			if job.AgentID != "" && !agentIDs[job.AgentID] {
				return fmt.Errorf("cron job %s references unknown agent: %s", job.ID, job.AgentID)
			}
		}
	}

	return nil
}

// GetQueueMode returns the configured queue mode (default: "collect")
func (qc *QueueConfig) GetQueueMode() string {
	if qc == nil || qc.Mode == "" {
		return "collect"
	}
	return qc.Mode
}

// GetDebounceMs returns the configured debounce timeout (default: 1000ms)
func (qc *QueueConfig) GetDebounceMs() int {
	if qc == nil || qc.DebounceMs == 0 {
		return 1000
	}
	return qc.DebounceMs
}

// GetCap returns the configured queue cap (default: 20)
func (qc *QueueConfig) GetCap() int {
	if qc == nil || qc.Cap == 0 {
		return 20
	}
	return qc.Cap
}

// GetDropPolicy returns the configured drop policy (default: "summarize")
func (qc *QueueConfig) GetDropPolicy() string {
	if qc == nil || qc.DropPolicy == "" {
		return "summarize"
	}
	return qc.DropPolicy
}

// GetMainMaxConcurrent returns the configured main lane max concurrent (default: 4)
func (lc *LanesConfig) GetMainMaxConcurrent() int {
	if lc == nil || lc.MainMaxConcurrent == 0 {
		return 4
	}
	return lc.MainMaxConcurrent
}

// GetCronMaxConcurrent returns the configured cron lane max concurrent (default: 4)
func (lc *LanesConfig) GetCronMaxConcurrent() int {
	if lc == nil || lc.CronMaxConcurrent == 0 {
		return 4
	}
	return lc.CronMaxConcurrent
}

// GetSubagentMaxConcurrent returns the configured subagent lane max concurrent (default: 8)
func (lc *LanesConfig) GetSubagentMaxConcurrent() int {
	if lc == nil || lc.SubagentMaxConcurrent == 0 {
		return 8
	}
	return lc.SubagentMaxConcurrent
}

// GetNestedMaxConcurrent returns the configured nested lane max concurrent (default: 8)
func (lc *LanesConfig) GetNestedMaxConcurrent() int {
	if lc == nil || lc.NestedMaxConcurrent == 0 {
		return 8
	}
	return lc.NestedMaxConcurrent
}

// ChannelsConfig holds all channel adapter configurations
// Pattern: Interface-based channel initialization for extensibility
type ChannelsConfig struct {
	WhatsApp *WhatsAppChannelConfig `yaml:"whatsapp,omitempty" json:"whatsapp,omitempty"`
	Slack    *SlackChannelConfig    `yaml:"slack,omitempty" json:"slack,omitempty"`
	// Future channels: Telegram, Discord, etc.
}

// WhatsAppChannelConfig configures WhatsApp channel adapter
type WhatsAppChannelConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	DeviceName    string `yaml:"device_name,omitempty" json:"deviceName,omitempty"`
	DBPath        string `yaml:"db_path,omitempty" json:"dbPath,omitempty"`
	DMPolicy      string `yaml:"dm_policy,omitempty" json:"dmPolicy,omitempty"` // "open" or "pairing"
	BotName       string `yaml:"bot_name,omitempty" json:"botName,omitempty"`
	AutoReconnect bool   `yaml:"auto_reconnect,omitempty" json:"autoReconnect,omitempty"`
}

// SlackChannelConfig configures Slack channel adapter
type SlackChannelConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	BotToken      string `yaml:"bot_token,omitempty" json:"botToken,omitempty"`           // xoxb-... token
	AppToken      string `yaml:"app_token,omitempty" json:"appToken,omitempty"`           // xapp-... token (Socket Mode)
	SigningSecret string `yaml:"signing_secret,omitempty" json:"signingSecret,omitempty"` // For event verification
	DMPolicy      string `yaml:"dm_policy,omitempty" json:"dmPolicy,omitempty"`           // "open" or "pairing"
	BotName       string `yaml:"bot_name,omitempty" json:"botName,omitempty"`
}

// ToolsConfig contains tool-related configuration
// Pattern: OpenClaw src/config/types.tools.ts
type ToolsConfig struct {
	AgentToAgent *A2AConfig `yaml:"agentToAgent,omitempty" json:"agentToAgent,omitempty"` // Week 27: A2A messaging
}

// A2AConfig configures Agent-to-Agent messaging
// Pattern: OpenClaw src/agents/tool-policy.ts A2A policy
type A2AConfig struct {
	Enabled          bool      `yaml:"enabled" json:"enabled"`                                       // Enable A2A messaging (default: false)
	Allow            []A2ARule `yaml:"allow,omitempty" json:"allow,omitempty"`                       // Permission rules
	MaxPingPongTurns int       `yaml:"maxPingPongTurns,omitempty" json:"maxPingPongTurns,omitempty"` // Max back-and-forth turns (default: 0)
}

// A2ARule defines a single Agent-to-Agent permission rule
// Pattern: OpenClaw A2A policy allow rules
type A2ARule struct {
	From string `yaml:"from" json:"from"` // Agent ID or "*" (wildcard)
	To   string `yaml:"to" json:"to"`     // Agent ID or "*" (wildcard)
}

// DatabaseConfig configures database backend (SQLite, File, or PostgreSQL)
// Pattern: Repository pattern with factory for easy migration
type DatabaseConfig struct {
	Type     string                  `yaml:"type" json:"type"`                             // "sqlite", "file", or "postgres"
	SQLite   *SQLiteDatabaseConfig   `yaml:"sqlite,omitempty" json:"sqlite,omitempty"`     // SQLite configuration
	File     *FileDatabaseConfig     `yaml:"file,omitempty" json:"file,omitempty"`         // File-based configuration
	Postgres *PostgresDatabaseConfig `yaml:"postgres,omitempty" json:"postgres,omitempty"` // PostgreSQL configuration
}

// SQLiteDatabaseConfig configures SQLite database
type SQLiteDatabaseConfig struct {
	Path string `yaml:"path" json:"path"` // Path to SQLite database file (default: ~/.memdoor/data/memdoor.db)
}

// FileDatabaseConfig configures file-based storage (using Memdoor config file)
type FileDatabaseConfig struct {
	Path string `yaml:"path" json:"path"` // Path to Memdoor config file (default: ~/.memdoor/config.json)
}

// PostgresDatabaseConfig configures PostgreSQL database
type PostgresDatabaseConfig struct {
	URL string `yaml:"url" json:"url"` // PostgreSQL connection string (e.g., postgres://user:pass@localhost:5432/cobuddy)
}

// GetDatabaseType returns the database type (default: "sqlite")
func (dc *DatabaseConfig) GetDatabaseType() string {
	if dc == nil || dc.Type == "" {
		return "sqlite"
	}
	return dc.Type
}

// GetSQLitePath returns the SQLite database path with defaults
func (dc *DatabaseConfig) GetSQLitePath() string {
	if dc == nil || dc.SQLite == nil || dc.SQLite.Path == "" {
		baseDir := shared.MemdoorHome()
		return filepath.Join(baseDir, "data", "memdoor.db")
	}
	return dc.SQLite.Path
}

// GetPostgresURL returns the PostgreSQL connection URL
func (dc *DatabaseConfig) GetPostgresURL() string {
	if dc == nil || dc.Postgres == nil {
		return ""
	}
	return dc.Postgres.URL
}

// GetFilePath returns the file-based storage config path with defaults
func (dc *DatabaseConfig) GetFilePath() string {
	if dc == nil || dc.File == nil || dc.File.Path == "" {
		baseDir := shared.MemdoorHome()
		return filepath.Join(baseDir, "config.json")
	}
	return dc.File.Path
}
