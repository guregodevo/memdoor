package domain

import (
	"time"

	"memdoor/pkg/sandbox"

	"github.com/google/uuid"
)

// DefaultWorkspaceID is the single workspace ID for single-tenant deployments
// In single-tenant mode, all users, channels, and buddies belong to this workspace
// This enables future multi-tenant scaling while keeping current implementation simple
const DefaultWorkspaceID = "00000000-0000-0000-0000-000000000001"

// Value Objects - DDD pattern for common field groups
// These are embedded in domain entities to eliminate duplication

// Timestamps represents creation and update timestamps
type Timestamps struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Archivable represents soft-delete via archiving
type Archivable struct {
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

// IsArchived is a DDD Specification/Predicate pattern
// Revealing interface: makes domain logic explicit and reusable
func (a Archivable) IsArchived() bool {
	return a.ArchivedAt != nil
}

// Archive marks the entity as archived (soft-delete)
func (a *Archivable) Archive() {
	now := time.Now()
	a.ArchivedAt = &now
}

// Unarchive restores an archived entity
func (a *Archivable) Unarchive() {
	a.ArchivedAt = nil
}

// SoftDeletable represents soft-delete via deletion timestamp
type SoftDeletable struct {
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// WorkspaceScoped represents entities that belong to a workspace (multi-tenancy)
type WorkspaceScoped struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
}

// BelongsToWorkspace is a DDD Specification/Predicate pattern
// Usage: if channel.BelongsToWorkspace(userWorkspaceID) { ... }
func (w WorkspaceScoped) BelongsToWorkspace(workspaceID uuid.UUID) bool {
	return w.WorkspaceID == workspaceID
}

// IsInDefaultWorkspace checks if entity belongs to the default/nil workspace (SQLite mode)
func (w WorkspaceScoped) IsInDefaultWorkspace() bool {
	return w.WorkspaceID == uuid.Nil
}

// SetWorkspace assigns the entity to a workspace
func (w *WorkspaceScoped) SetWorkspace(workspaceID uuid.UUID) {
	w.WorkspaceID = workspaceID
}

// Workspace represents a tenant (multi-tenant concept, not used in SQLite implementation)
type Workspace struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Slug     string    `json:"slug"`
	OwnerID  uuid.UUID `json:"owner_id"`
	Language string    `json:"language"` // ISO 639-1 code (e.g. "en", "ar")
	Plan     string    `json:"plan"`     // 'free', 'pro', 'enterprise'
	Status   string    `json:"status"`   // 'active', 'suspended', 'deleted'
	Timestamps
}

// BaseUser contains common user fields shared across all bounded contexts
// Use embedding to reuse these fields instead of duplicating them
type BaseUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Username      string `json:"username"` // Unique username for @mentions (e.g., "john")
	Name          string `json:"name"`     // Display name (e.g., "John Doe")
	AvatarURL     string `json:"avatar_url"`
	EmailVerified bool   `json:"email_verified"`
	Timestamps
}

// UserRole represents a user's role in the workspace
type UserRole string

const (
	UserRoleAdmin UserRole = "admin" // Workspace administrator
	UserRoleUser  UserRole = "user"  // Regular user
)

// User represents a human user (domain model with all fields)
type User struct {
	BaseUser
	PasswordHash *string    `json:"-"` // Never serialize password
	WorkspaceID  string     `json:"workspace_id"`
	Role         UserRole   `json:"role"` // User role (admin/user)
	LastLoginAt  *time.Time `json:"last_login_at"`
}

// Channel represents a communication channel (like Slack channels)
type Channel struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	IsPrivate   bool      `json:"is_private"`
	WorkspaceScoped
	Timestamps
	Archivable
}

// Buddy represents an AI agent
type Buddy struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	AvatarEmoji  string    `json:"avatar_emoji"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Icon         string    `json:"icon"` // Animal mascot icon (fox, owl, panda, etc.)
	Description  *string   `json:"description"`
	Personality  *string   `json:"personality"`
	SystemPrompt *string   `json:"system_prompt"`

	// Execution type determines how the agent runs
	ExecutionType string `json:"execution_type"` // "local" or "remote"

	// LLM provider/model for local agents are NOT stored here.
	// They live in workspace_settings.provider_key:* (byok) and
	// resolve to a single workspace-wide provider/model at request
	// time. Per-agent model overrides were removed in the byok
	// refactor; remote agents still carry their own endpoint+model
	// in RemoteConfig.
	RemoteConfig *RemoteAgentConfig `json:"remote_config,omitempty"` // Only for execution_type="remote"

	// Common configuration fields
	Temperature     float64              `json:"temperature"`
	MaxTokens       int                  `json:"max_tokens"`
	Skills          []string             `json:"skills"`        // JSON array
	Tools           []string             `json:"tools"`         // JSON array
	SandboxScope    sandbox.SandboxScope `json:"sandbox_scope"` // user/channel/workspace
	LearningEnabled bool                 `json:"learning_enabled"`
	IsActive        bool                 `json:"is_active"`
	AdminOnly       bool                 `json:"admin_only"` // Only visible to workspace admins
	CreatedBy       uuid.UUID            `json:"created_by"`
	WorkspaceScoped
	Timestamps
}

// RemoteAgentConfig holds configuration for remote agents using OpenAI-compatible protocol
type RemoteAgentConfig struct {
	Endpoint string  `json:"endpoint"` // OpenAI-compatible endpoint URL (e.g., https://api.openai.com/v1/chat/completions)
	APIKey   *string `json:"api_key"`  // Optional API key (can use environment variable)
	Model    string  `json:"model"`    // Model name to pass in requests (e.g., "gpt-4")

	// Optional advanced settings
	Timeout       int               `json:"timeout"`        // Request timeout in seconds (default: 180)
	MaxRetries    int               `json:"max_retries"`    // Retry count for failed requests (default: 3)
	CustomHeaders map[string]string `json:"custom_headers"` // Additional HTTP headers for authentication or routing

	// Sampling parameters (optional; zero means use endpoint default)
	Temperature      float64 `json:"temperature,omitempty"`
	TopP             float64 `json:"top_p,omitempty"`
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`

	// Tool use control (optional): "auto", "required", "none"
	ToolChoice string `json:"tool_choice,omitempty"`
}

// Message represents a chat message
type Message struct {
	ID          int64      `json:"id"`
	ChannelID   uuid.UUID  `json:"channel_id"`
	UserID      *uuid.UUID `json:"user_id"`  // NULL if sent by buddy
	BuddyID     *uuid.UUID `json:"buddy_id"` // NULL if sent by user
	Content     string     `json:"content"`
	ContentType string     `json:"content_type"` // 'text', 'markdown', 'code'
	ThreadID    *int64     `json:"thread_id"`    // For threading
	WorkspaceScoped
	Timestamps
	SoftDeletable
}

// AgentEvent represents an agent execution event (from Milestone 1)
type AgentEvent struct {
	ID           int64     `json:"id"`
	BuddyID      uuid.UUID `json:"buddy_id"`
	SessionID    uuid.UUID `json:"session_id"`
	ParentID     *int64    `json:"parent_id"`
	EventType    string    `json:"event_type"` // 'tool_call', 'completion', 'error'
	EventData    string    `json:"event_data"` // JSON
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CausalityID  *string   `json:"causality_id"`
	WorkspaceScoped
	CreatedAt time.Time `json:"created_at"` // Only CreatedAt, no UpdatedAt for events
}

// BuddyMemory represents RAG memory storage
type BuddyMemory struct {
	ID         int64      `json:"id"`
	BuddyID    uuid.UUID  `json:"buddy_id"`
	MemoryType string     `json:"memory_type"` // 'conversation', 'fact', 'preference'
	Content    string     `json:"content"`
	Metadata   string     `json:"metadata"` // JSON
	ExpiresAt  *time.Time `json:"expires_at"`
	WorkspaceScoped
	CreatedAt time.Time `json:"created_at"` // Only CreatedAt, no UpdatedAt for memory
}

// AgentMemory represents per-agent tool-accessible memory (BM25 keyword search)
// Stored in the main DB for Raft replication (replaces separate per-agent SQLite files)
type AgentMemory struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	Content   string `json:"content"`
	Tags      string `json:"tags"`     // JSON array
	Metadata  string `json:"metadata"` // JSON object
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// Session represents a user or agent session
type Session struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	SessionType  string    `json:"session_type"` // 'websocket', 'agent', 'api'
	TokenHash    *string   `json:"-"`            // Never serialize
	Metadata     string    `json:"metadata"`     // JSON
	ExpiresAt    time.Time `json:"expires_at"`
	LastActivity time.Time `json:"last_activity"`
	WorkspaceScoped
	CreatedAt time.Time `json:"created_at"` // Only CreatedAt for sessions
}
