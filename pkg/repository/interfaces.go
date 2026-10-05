package repository

import (
	"context"
	"time"

	"memdoor/pkg/channel"
	"memdoor/pkg/domain"
	"memdoor/pkg/filestore"
	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
	"memdoor/pkg/shared"

	"github.com/google/uuid"
)

// WorkspaceRepository manages workspaces (tenants)
// NOTE: Not used in SQLite implementation (single-tenant)
type WorkspaceRepository interface {
	Create(ctx context.Context, workspace *domain.Workspace) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error)
	List(ctx context.Context) ([]*domain.Workspace, error)
	Update(ctx context.Context, workspace *domain.Workspace) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserRepository manages users
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)       // Changed from uuid.UUID to string
	GetByIDs(ctx context.Context, ids []string) ([]*domain.User, error) // Changed from uuid.UUID to string
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error) // Lookup user by username for @mentions
	List(ctx context.Context) ([]*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id string) error // Changed from uuid.UUID to string
}

// ChannelRepository manages channels
// In SQLite: Returns all channels (no workspace filter)
// In PostgreSQL: Returns only channels in current workspace (from context)
type ChannelRepository interface {
	Create(ctx context.Context, channel *domain.Channel) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Channel, error)
	GetByName(ctx context.Context, name string) (*domain.Channel, error)
	List(ctx context.Context, page shared.OffsetPage) ([]*domain.Channel, error)
	ListActive(ctx context.Context, page shared.OffsetPage) ([]*domain.Channel, error) // Exclude archived
	Update(ctx context.Context, channel *domain.Channel) error
	Archive(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error

	// GetWorkspaceSlug resolves a workspace UUID to its slug. Used by the
	// message service to populate SandboxContext.WorkspaceSlug when the
	// upstream ExecutionContext doesn't carry it (the A2A path — where the
	// fallback shared.MustNewExecutionContext only takes (actorID, workspaceID)
	// without a slug). Bug parked in SHOULD.md from 2026-05-01, fixed
	// 2026-05-02.
	GetWorkspaceSlug(ctx context.Context, workspaceID uuid.UUID) (string, error)
}

// BuddyRepository manages AI agents (buddies)
// In SQLite: Returns all buddies (no workspace filter)
// In PostgreSQL: Returns only buddies in current workspace (from context)
type BuddyRepository interface {
	Create(ctx context.Context, buddy *domain.Buddy) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Buddy, error)
	GetByName(ctx context.Context, name string) (*domain.Buddy, error)
	GetByNames(ctx context.Context, names []string) ([]*domain.Buddy, error) // Batch load by names
	List(ctx context.Context, page shared.OffsetPage) ([]*domain.Buddy, error)
	ListActive(ctx context.Context, page shared.OffsetPage) ([]*domain.Buddy, error) // Only active buddies
	Update(ctx context.Context, buddy *domain.Buddy) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByName(ctx context.Context, name string) error // Fallback for stale-id state — see SQLite impl notes
}

// MessageRepository manages chat messages
// In SQLite: Queries all messages (no workspace filter)
// In PostgreSQL: Queries only messages in current workspace (from context)
type MessageRepository interface {
	Create(ctx context.Context, message *domain.Message) error
	GetByID(ctx context.Context, id int64) (*domain.Message, error)
	GetByChannelID(ctx context.Context, channelID uuid.UUID, limit int, offset int) ([]*domain.Message, error)
	GetByThreadID(ctx context.Context, threadID int64, limit int) ([]*domain.Message, error)
	Update(ctx context.Context, message *domain.Message) error
	SoftDelete(ctx context.Context, id int64) error // Sets deleted_at
	Delete(ctx context.Context, id int64) error     // Hard delete
}

// AgentEventRepository manages agent execution logs
// In SQLite: Queries all events (no workspace filter)
// In PostgreSQL: Queries only events in current workspace (from context)
type AgentEventRepository interface {
	Create(ctx context.Context, event *domain.AgentEvent) error
	GetByID(ctx context.Context, id int64) (*domain.AgentEvent, error)
	GetBySessionID(ctx context.Context, sessionID uuid.UUID, limit int) ([]*domain.AgentEvent, error)
	GetByBuddyID(ctx context.Context, buddyID uuid.UUID, limit int) ([]*domain.AgentEvent, error)
	List(ctx context.Context, limit int, offset int) ([]*domain.AgentEvent, error)
	Delete(ctx context.Context, id int64) error
}

// BuddyMemoryRepository manages buddy memory (RAG storage)
// In SQLite: Queries all memory (no workspace filter)
// In PostgreSQL: Queries only memory in current workspace (from context)
type BuddyMemoryRepository interface {
	Create(ctx context.Context, memory *domain.BuddyMemory) error
	GetByID(ctx context.Context, id int64) (*domain.BuddyMemory, error)
	GetByBuddyID(ctx context.Context, buddyID uuid.UUID, limit int) ([]*domain.BuddyMemory, error)
	Search(ctx context.Context, buddyID uuid.UUID, query string, limit int) ([]*domain.BuddyMemory, error) // Vector search later
	Delete(ctx context.Context, id int64) error
	DeleteExpired(ctx context.Context) (int64, error) // Delete expired memories
}

// AgentMemoryRepository manages per-agent tool-accessible memories (BM25 search)
// Replaces separate per-agent SQLite files for Raft replication
type AgentMemoryRepository interface {
	Store(ctx context.Context, mem *domain.AgentMemory) error
	GetByID(ctx context.Context, id string) (*domain.AgentMemory, error)
	Search(ctx context.Context, agentID, ftsQuery string, limit int) ([]*domain.AgentMemory, []float64, error)
	List(ctx context.Context, agentID string, tags []string) ([]*domain.AgentMemory, error)
	Update(ctx context.Context, mem *domain.AgentMemory) error
	Delete(ctx context.Context, id string) error
}

// SessionRepository manages user/agent sessions
// In SQLite: Queries all sessions (no workspace filter)
// In PostgreSQL: Queries only sessions in current workspace (from context)
type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Session, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Session, error)
	Update(ctx context.Context, session *domain.Session) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error) // Delete expired sessions
}

// GetChecklist returns all enabled checklist items for an agent
// Returns items ordered by the order field

// Create adds a new heartbeat item

// Update modifies an existing heartbeat item

// Delete removes a heartbeat item

// GetByID retrieves a single heartbeat item

// List returns all heartbeat items (including disabled) for an agent

// ReorderItems updates the order field for multiple items atomically

// ListWorkspacesWithItems returns the distinct workspace_ids that
// have at least one enabled heartbeat item for the given agent. The
// heartbeat runner uses this to iterate workspaces beyond the
// agent's own workspace — an agent that lives in the default
// workspace may have self-maintenance items registered for every
// user workspace.

// CronJobRepository manages scheduled cron jobs
type CronJobRepository interface {
	Create(ctx context.Context, job *domain.CronJob) error
	Update(ctx context.Context, job *domain.CronJob) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*domain.CronJob, error)
	List(ctx context.Context, workspaceID string) ([]*domain.CronJob, error)
}

// CronHistoryRepository manages cron job execution history
type CronHistoryRepository interface {
	Record(ctx context.Context, record *domain.CronRunRecord) error
	ListByJob(ctx context.Context, jobID string, limit int) ([]*domain.CronRunRecord, error)
	ListRecent(ctx context.Context, limit int) ([]*domain.CronRunRecord, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// SubagentRunRepository manages subagent lifecycle records
type SubagentRunRepository interface {
	Create(ctx context.Context, run *domain.SubagentRun) error
	Update(ctx context.Context, run *domain.SubagentRun) error
	GetByID(ctx context.Context, runID string) (*domain.SubagentRun, error)
	GetByChildSession(ctx context.Context, childSessionKey string) (*domain.SubagentRun, error)
	List(ctx context.Context) ([]*domain.SubagentRun, error)
	Delete(ctx context.Context, runID string) error
	DeleteArchived(ctx context.Context, beforeMs int64) (int64, error)
}

// AgentTurnRepository is the turn ledger: every agent dispatch gets a row
// before anything can die silently, idempotent on (channel, agent,
// trigger message). The janitor reads Stuck; reconnecting clients read
// Since.
type AgentTurnRepository interface {
	Begin(ctx context.Context, t *domain.AgentTurn) (created bool, err error)
	SetState(ctx context.Context, turnID string, state domain.TurnState, errMsg string, responseMessageID int64) error
	Stuck(ctx context.Context, before time.Time) ([]*domain.AgentTurn, error)
	Since(ctx context.Context, channelID string, since time.Time) ([]*domain.AgentTurn, error)
}

// RepositoryFactory creates repository implementations based on config
// This is the main abstraction - swap implementations by changing config
type RepositoryFactory interface {
	Workspaces() WorkspaceRepository
	Users() UserRepository
	Channels() ChannelRepository
	ChannelMemberships() channel.MembershipRepository // Channel membership management
	Buddies() BuddyRepository
	Messages() message.Repository   // New DDD message repository
	Reactions() reaction.Repository // Message reactions
	AgentEvents() AgentEventRepository
	BuddyMemory() BuddyMemoryRepository
	Sessions() SessionRepository
	CronJobs() CronJobRepository          // Scheduled cron jobs
	CronHistory() CronHistoryRepository   // Cron execution history
	SubagentRuns() SubagentRunRepository  // Subagent lifecycle tracking
	AgentTurns() AgentTurnRepository      // Turn ledger (silence is impossible)
	AgentMemories() AgentMemoryRepository // Per-agent tool memories (BM25)
	AgentSecrets() AgentSecretRepository  // Per-agent scoped secrets
	Files() filestore.Repository          // File attachment metadata
	DB() interface{}                      // Get raw database connection (type varies by implementation)
	Close() error                         // Close database connections
}
