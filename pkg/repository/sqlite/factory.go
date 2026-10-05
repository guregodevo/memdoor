package sqlite

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"memdoor/pkg/channel"
	"memdoor/pkg/filestore"
	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
	"memdoor/pkg/repository"
	_ "memdoor/pkg/sqlitedriver"
)

// SQLiteFactory creates SQLite-backed repositories
// This is the single-tenant implementation (no workspace filtering)
type SQLiteFactory struct {
	db *sql.DB

	// Cached repository instances
	workspaces         repository.WorkspaceRepository
	users              repository.UserRepository
	channels           repository.ChannelRepository
	channelMemberships channel.MembershipRepository
	buddies            repository.BuddyRepository
	messages           message.Repository
	reactions          reaction.Repository
	agentEvents        repository.AgentEventRepository
	buddyMemory        repository.BuddyMemoryRepository
	sessions           repository.SessionRepository
	cronJobs           repository.CronJobRepository
	cronHistory        repository.CronHistoryRepository
	subagentRuns       repository.SubagentRunRepository
	agentTurns         repository.AgentTurnRepository
	agentMemories      repository.AgentMemoryRepository
	agentSecrets       repository.AgentSecretRepository
	files              filestore.Repository
}

// NewSQLiteFactory creates a new SQLite repository factory
func NewSQLiteFactory(dbPath string) (repository.RepositoryFactory, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// Configure SQLite for better performance
	db.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes well
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Initialize schema
	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	factory := &SQLiteFactory{
		db:                 db,
		workspaces:         NewWorkspaceRepository(db),
		users:              NewUserRepository(db),
		channels:           NewChannelRepository(db),
		channelMemberships: NewMembershipRepository(db),
		buddies:            NewBuddyRepository(db),
		messages:           NewMessageRepository(db),
		reactions:          NewReactionRepository(db),
		agentEvents:        NewAgentEventRepository(db),
		buddyMemory:        NewBuddyMemoryRepository(db),
		sessions:           NewSessionRepository(db),
		cronJobs:           NewCronJobRepository(db),
		cronHistory:        NewCronHistoryRepository(db),
		subagentRuns:       NewSubagentRunRepository(db),
		agentTurns:         NewAgentTurnRepository(db),
		agentMemories:      NewAgentMemoryRepository(db),
		agentSecrets:       newAgentSecretRepository(db),
		files:              NewFileRepository(db),
	}

	return factory, nil
}

func (f *SQLiteFactory) Workspaces() repository.WorkspaceRepository { return f.workspaces }
func (f *SQLiteFactory) Users() repository.UserRepository           { return f.users }
func (f *SQLiteFactory) Channels() repository.ChannelRepository     { return f.channels }
func (f *SQLiteFactory) ChannelMemberships() channel.MembershipRepository {
	return f.channelMemberships
}
func (f *SQLiteFactory) Buddies() repository.BuddyRepository             { return f.buddies }
func (f *SQLiteFactory) Messages() message.Repository                    { return f.messages }
func (f *SQLiteFactory) Reactions() reaction.Repository                  { return f.reactions }
func (f *SQLiteFactory) AgentEvents() repository.AgentEventRepository    { return f.agentEvents }
func (f *SQLiteFactory) BuddyMemory() repository.BuddyMemoryRepository   { return f.buddyMemory }
func (f *SQLiteFactory) Sessions() repository.SessionRepository          { return f.sessions }
func (f *SQLiteFactory) CronJobs() repository.CronJobRepository          { return f.cronJobs }
func (f *SQLiteFactory) CronHistory() repository.CronHistoryRepository   { return f.cronHistory }
func (f *SQLiteFactory) SubagentRuns() repository.SubagentRunRepository  { return f.subagentRuns }
func (f *SQLiteFactory) AgentTurns() repository.AgentTurnRepository      { return f.agentTurns }
func (f *SQLiteFactory) AgentMemories() repository.AgentMemoryRepository { return f.agentMemories }
func (f *SQLiteFactory) AgentSecrets() repository.AgentSecretRepository  { return f.agentSecrets }
func (f *SQLiteFactory) Files() filestore.Repository                     { return f.files }

// DB returns the raw database connection
func (f *SQLiteFactory) DB() interface{} {
	return f.db
}

// Close closes the database connection
func (f *SQLiteFactory) Close() error {
	if f.db != nil {
		return f.db.Close()
	}
	return nil
}

// initSchema creates tables if they don't exist
func initSchema(db *sql.DB) error {
	schema := `
	-- Workspaces table (multi-tenant support)
	CREATE TABLE IF NOT EXISTS workspaces (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		owner_id TEXT,
		language TEXT DEFAULT 'en',
		-- VESTIGIAL. The billing plan is resolved by the BROKER from the
		-- workspace (gateway/billingsvc/auth.go), because a self-hosted
		-- gateway carries no billing code and could never enforce an
		-- entitlement kept here. Nothing reads or writes this column; it is
		-- left exactly as it shipped rather than rebuilt, since changing a
		-- CHECK constraint in SQLite means recreating a live table for a
		-- value nobody consults.
		plan TEXT DEFAULT 'free' CHECK (plan IN ('free', 'pro', 'enterprise')),
		status TEXT DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Users table
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT,
		name TEXT,
		avatar_url TEXT,
		email_verified INTEGER DEFAULT 0,
		workspace_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login_at DATETIME
	);

	-- Channels table (conversation spaces)
	CREATE TABLE IF NOT EXISTS channels (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL, -- Always set, but not used for filtering in SQLite
		name TEXT UNIQUE NOT NULL,
		description TEXT,
		is_private INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		archived_at DATETIME
	);

	-- Channel memberships table
	CREATE TABLE IF NOT EXISTS channel_memberships (
		channel_id TEXT NOT NULL,
		actor_id TEXT NOT NULL,
		role TEXT NOT NULL CHECK (role IN ('member', 'admin')),
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		invited_by TEXT,
		PRIMARY KEY (channel_id, actor_id),
		FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE
	);

	-- Buddies table (AI agents). LLM provider/model are NOT stored
	-- per-row — they live in workspace_settings.provider_key:* (byok)
	-- and resolve to a single workspace-wide value at request time.
	CREATE TABLE IF NOT EXISTS buddies (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT UNIQUE NOT NULL,
		avatar_emoji TEXT DEFAULT '🤖',
		icon TEXT DEFAULT 'octopus', -- Animal mascot icon
		description TEXT,
		personality TEXT,
		system_prompt TEXT,
		temperature REAL DEFAULT 1.0,
		max_tokens INTEGER DEFAULT 8000,
		skills TEXT, -- JSON array
		tools TEXT,  -- JSON array
		sandbox_scope TEXT DEFAULT 'user' CHECK (sandbox_scope IN ('user', 'channel', 'workspace')),
		learning_enabled INTEGER DEFAULT 0,
		is_active INTEGER DEFAULT 1,
		created_by TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (created_by) REFERENCES users(id)
	);

	-- Messages table (matches pkg/message domain model)
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id TEXT NOT NULL,
		author_id TEXT NOT NULL, -- ActorID format: 'human:alice' or 'agent:writer'
		content_text TEXT NOT NULL,
		content_mentions TEXT, -- JSON array of mentions
		parent_id INTEGER, -- For threading (reply to another message)
		is_read INTEGER DEFAULT 0, -- Boolean: 0 = unread, 1 = read
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME, -- NULL if never edited
		FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE,
		FOREIGN KEY (parent_id) REFERENCES messages(id) ON DELETE SET NULL
	);

	-- Message reactions table
	CREATE TABLE IF NOT EXISTS reactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id INTEGER NOT NULL,
		user_id TEXT NOT NULL, -- ActorID format: 'human:alice' or 'agent:writer'
		emoji TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (message_id, user_id, emoji), -- Prevent duplicate reactions
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
	);

	-- Index for fast reaction lookups by message
	CREATE INDEX IF NOT EXISTS idx_reactions_message_id ON reactions(message_id);

	-- Agent events table (execution logs)
	CREATE TABLE IF NOT EXISTS agent_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id TEXT NOT NULL,
		buddy_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		parent_id INTEGER,
		event_type TEXT NOT NULL,
		event_data TEXT NOT NULL, -- JSON
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		causality_id TEXT,
		FOREIGN KEY (buddy_id) REFERENCES buddies(id),
		FOREIGN KEY (parent_id) REFERENCES agent_events(id)
	);

	-- Buddy memory table (RAG storage)
	CREATE TABLE IF NOT EXISTS buddy_memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id TEXT NOT NULL,
		buddy_id TEXT NOT NULL,
		memory_type TEXT NOT NULL,
		content TEXT NOT NULL,
		metadata TEXT, -- JSON
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME,
		FOREIGN KEY (buddy_id) REFERENCES buddies(id)
	);

	-- Sessions table
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		session_type TEXT NOT NULL,
		token_hash TEXT,
		metadata TEXT, -- JSON
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		last_activity DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	-- Workspace settings (key-value store, scoped by workspace)
	-- workspace_id='' for global settings
	CREATE TABLE IF NOT EXISTS workspace_settings (
		workspace_id TEXT NOT NULL DEFAULT '',
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (workspace_id, key)
	);

	-- Per-agent scoped secrets
	CREATE TABLE IF NOT EXISTS agent_secrets (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		name TEXT NOT NULL,
		value TEXT NOT NULL,
		created_by TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (agent_id) REFERENCES buddies(id) ON DELETE CASCADE,
		UNIQUE(agent_id, name)
	);

	-- Files table (file attachments for messages)
	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		message_id INTEGER,
		channel_id TEXT NOT NULL,
		uploader_id TEXT NOT NULL,
		filename TEXT NOT NULL,
		mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
		size_bytes INTEGER NOT NULL,
		storage_path TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE SET NULL,
		FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_files_message ON files(message_id);
	CREATE INDEX IF NOT EXISTS idx_files_channel ON files(channel_id);
	CREATE INDEX IF NOT EXISTS idx_files_created ON files(created_at);

	-- File binary data (stored in SQLite for Raft replication)
	CREATE TABLE IF NOT EXISTS file_data (
		id TEXT PRIMARY KEY,
		data BLOB NOT NULL,
		FOREIGN KEY (id) REFERENCES files(id) ON DELETE CASCADE
	);

	-- Email verification tokens
	CREATE TABLE IF NOT EXISTS email_verification_tokens (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- Password reset tokens
	CREATE TABLE IF NOT EXISTS password_reset_tokens (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		used_at DATETIME,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- Invite tokens (invite-only registration)
	CREATE TABLE IF NOT EXISTS invites (
		token TEXT PRIMARY KEY,
		email TEXT NOT NULL,
		workspace_id TEXT NOT NULL,
		invited_by TEXT NOT NULL,
		channel_name TEXT,
		expires_at DATETIME NOT NULL,
		used_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Campaigns (shareable signup links for fan acquisition)
	CREATE TABLE IF NOT EXISTS campaigns (
		id TEXT PRIMARY KEY,
		slug TEXT UNIQUE NOT NULL,
		creator_name TEXT NOT NULL,
		channel_name TEXT NOT NULL,
		message TEXT,
		is_active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Campaign signups (tracks which users signed up via which campaign)
	CREATE TABLE IF NOT EXISTS campaign_signups (
		id TEXT PRIMARY KEY,
		campaign_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		email TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (campaign_id) REFERENCES campaigns(id),
		UNIQUE(campaign_id, email)
	);

	CREATE INDEX IF NOT EXISTS idx_campaign_signups_campaign ON campaign_signups(campaign_id);
	CREATE INDEX IF NOT EXISTS idx_campaigns_slug ON campaigns(slug);

	-- Cron jobs table (scheduled agent tasks)
	CREATE TABLE IF NOT EXISTS cron_jobs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL DEFAULT 'default',
		agent_id TEXT NOT NULL DEFAULT '',
		schedule TEXT NOT NULL,
		message TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		workdir TEXT NOT NULL DEFAULT '',
		session_key TEXT NOT NULL DEFAULT '',
		max_runs INTEGER NOT NULL DEFAULT 0,
		run_count INTEGER NOT NULL DEFAULT 0,
		expires_at INTEGER NOT NULL DEFAULT 0
	);

	-- Cron execution history
	CREATE TABLE IF NOT EXISTS cron_history (
		id TEXT PRIMARY KEY,
		job_id TEXT NOT NULL,
		agent_id TEXT NOT NULL DEFAULT '',
		session_key TEXT NOT NULL DEFAULT '',
		start_time INTEGER NOT NULL,
		end_time INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL,
		success INTEGER NOT NULL DEFAULT 0,
		error TEXT NOT NULL DEFAULT '',
		message TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_cron_history_job ON cron_history(job_id, end_time DESC);
	CREATE INDEX IF NOT EXISTS idx_cron_history_time ON cron_history(end_time DESC);

	-- Subagent lifecycle tracking
	CREATE TABLE IF NOT EXISTS subagent_runs (
		run_id TEXT PRIMARY KEY,
		child_session_key TEXT NOT NULL,
		requester_session_key TEXT NOT NULL DEFAULT '',
		requester_display_key TEXT NOT NULL DEFAULT '',
		task TEXT NOT NULL DEFAULT '',
		cleanup TEXT NOT NULL DEFAULT '',
		label TEXT NOT NULL DEFAULT '',
		parent_message_id INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		started_at INTEGER,
		ended_at INTEGER,
		outcome_status TEXT NOT NULL DEFAULT '',
		outcome_error TEXT NOT NULL DEFAULT '',
		archive_at_ms INTEGER NOT NULL DEFAULT 0,
		cleanup_completed_at INTEGER,
		cleanup_handled INTEGER NOT NULL DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_subagent_runs_child ON subagent_runs(child_session_key);
	CREATE INDEX IF NOT EXISTS idx_subagent_runs_created ON subagent_runs(created_at DESC);

	-- Turn ledger: every agent dispatch gets a row BEFORE anything can die
	-- silently; UNIQUE key makes dispatch idempotent.
	CREATE TABLE IF NOT EXISTS agent_turns (
		turn_id TEXT PRIMARY KEY,
		channel_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		trigger_message_id INTEGER NOT NULL DEFAULT 0,
		state TEXT NOT NULL,
		dispatched_at INTEGER NOT NULL,
		started_at INTEGER,
		ended_at INTEGER,
		response_message_id INTEGER NOT NULL DEFAULT 0,
		error TEXT NOT NULL DEFAULT '',
		UNIQUE(channel_id, agent_id, trigger_message_id)
	);
	CREATE INDEX IF NOT EXISTS idx_agent_turns_state ON agent_turns(state, dispatched_at);
	CREATE INDEX IF NOT EXISTS idx_agent_turns_channel ON agent_turns(channel_id, dispatched_at DESC);

	-- Prepaid credits: append-only integer cents; (kind, ref) unique makes
	-- Stripe-webhook replays idempotent.
	CREATE TABLE IF NOT EXISTS credit_entries (
		entry_id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
		ref TEXT NOT NULL DEFAULT '',
		note TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_credit_kind_ref ON credit_entries(kind, ref) WHERE ref != '';
	CREATE INDEX IF NOT EXISTS idx_credit_workspace ON credit_entries(workspace_id, created_at DESC);

	-- Agent memories table (per-agent BM25 keyword search, replaces separate SQLite files)
	CREATE TABLE IF NOT EXISTS agent_memories (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		content TEXT NOT NULL,
		tags TEXT,     -- JSON array
		metadata TEXT, -- JSON object
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_agent_memories_agent ON agent_memories(agent_id, created_at DESC);

	-- Indexes for performance
	CREATE INDEX IF NOT EXISTS idx_agent_secrets_agent ON agent_secrets(agent_id);
	CREATE INDEX IF NOT EXISTS idx_channel_memberships_actor ON channel_memberships(actor_id);
	CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages(channel_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_messages_parent ON messages(parent_id);
	CREATE INDEX IF NOT EXISTS idx_agent_events_buddy ON agent_events(buddy_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_agent_events_session ON agent_events(session_id);
	CREATE INDEX IF NOT EXISTS idx_buddy_memory_buddy ON buddy_memory(buddy_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

	-- The tables of the feature deleted on 2026-09-27 are not created
	-- here: a database from before that date keeps them until migration
	-- 018 drops them (gateway/migrations/018_drop_wiki.sql).
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Run migrations for existing databases
	return runMigrations(db)
}

// runMigrations applies schema migrations to existing databases
func runMigrations(db *sql.DB) error {
	// Migration: a cron job an AGENT scheduled carries where to work, where to
	// answer, and its bounds. Older rows have none of it and behave as before.
	for _, stmt := range []string{
		`ALTER TABLE cron_jobs ADD COLUMN workdir TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE cron_jobs ADD COLUMN session_key TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE cron_jobs ADD COLUMN max_runs INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE cron_jobs ADD COLUMN run_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE cron_jobs ADD COLUMN expires_at INTEGER NOT NULL DEFAULT 0`,
	} {
		_, _ = db.Exec(stmt) // a duplicate column is the normal case
	}

	// Migration: Add sandbox_scope column to buddies table (if it doesn't exist)
	// This handles databases created before the sandbox scope feature
	_, err := db.Exec(`
		ALTER TABLE buddies ADD COLUMN sandbox_scope TEXT DEFAULT 'user' CHECK (sandbox_scope IN ('user', 'channel', 'workspace'));
	`)
	// Ignore error if column already exists (SQLite returns error for duplicate column)
	// This is safe because we're using CREATE TABLE IF NOT EXISTS above
	_ = err

	// Migration: Add remote agent support columns (execution_type, remote_config)
	// This enables OpenAI-compatible remote agents
	_, err = db.Exec(`
		ALTER TABLE buddies ADD COLUMN execution_type TEXT DEFAULT 'local' CHECK (execution_type IN ('local', 'remote'));
	`)
	_ = err // Ignore if column already exists

	_, err = db.Exec(`
		ALTER TABLE buddies ADD COLUMN remote_config TEXT; -- JSON for RemoteAgentConfig
	`)
	_ = err // Ignore if column already exists

	// Migration: Add icon column to buddies table
	// This adds animal mascot icon support for visual identification
	_, err = db.Exec(`
		ALTER TABLE buddies ADD COLUMN icon TEXT DEFAULT 'octopus';
	`)
	_ = err // Ignore if column already exists

	// Migration: Update default icon from orangutan to octopus
	_, err = db.Exec(`UPDATE buddies SET icon = 'octopus' WHERE icon = 'orangutan';`)
	_ = err

	// Migration: Add learning_enabled column to buddies table
	// This enables the learning loop (reflection + memory retrieval) for personal agents
	_, err = db.Exec(`
		ALTER TABLE buddies ADD COLUMN learning_enabled INTEGER DEFAULT 0;
	`)
	_ = err // Ignore if column already exists

	// Migration: Add workspace_id column to users table
	// This enables workspace isolation for multi-tenant support
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN workspace_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
	`)
	_ = err // Ignore if column already exists

	// Migration: Add username column to users table
	// This enables @username mentions (unique, required for new users)
	_, err = db.Exec(`
		ALTER TABLE users ADD COLUMN username TEXT;
	`)
	_ = err // Ignore if column already exists

	// Migration: Create unique index on username (for existing databases)
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
	`)
	_ = err // Ignore if index already exists

	// Migration: Populate username from email for existing users
	// Extract local part of email (before @) as initial username
	_, err = db.Exec(`
		UPDATE users SET username = SUBSTR(email, 1, INSTR(email, '@') - 1) WHERE username IS NULL;
	`)
	_ = err // Ignore if already populated

	// Migration 1: Rename room_memberships to channel_memberships
	// Check if the new table already exists (migration already completed)
	var newTableName string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='channel_memberships'`).Scan(&newTableName)
	if err == nil {
		// New table already exists, migration already completed
		// Check if old table still exists and drop it if needed
		var oldTableName string
		err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='room_memberships'`).Scan(&oldTableName)
		if err == nil {
			// Old table still exists alongside new one, drop the old one
			_, err = db.Exec(`DROP TABLE room_memberships`)
			if err != nil {
				// Not critical, just log it
				_ = err
			}
		}
	} else {
		// New table doesn't exist, check if old table exists to rename
		var oldTableName string
		err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='room_memberships'`).Scan(&oldTableName)
		if err == nil {
			// Old table exists, rename it
			_, err = db.Exec(`ALTER TABLE room_memberships RENAME TO channel_memberships`)
			if err != nil {
				return fmt.Errorf("failed to rename room_memberships to channel_memberships: %w", err)
			}
		}
		// Ignore error if neither table exists (new database will create from schema)
	}

	// Migration 2: Rename rooms to channels
	// Check if the new table already exists (migration already completed)
	var channelsTableName string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='channels'`).Scan(&channelsTableName)
	if err == nil {
		// New table already exists, migration already completed
		// Check if old table still exists and drop it if needed
		var roomsTableName string
		err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='rooms'`).Scan(&roomsTableName)
		if err == nil {
			// Old table still exists alongside new one, drop the old one
			_, err = db.Exec(`DROP TABLE rooms`)
			if err != nil {
				// Not critical, just log it
				_ = err
			}
		}
	} else {
		// New table doesn't exist, check if old table exists to rename
		var roomsTableName string
		err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='rooms'`).Scan(&roomsTableName)
		if err == nil {
			// Old table exists, rename it
			_, err = db.Exec(`ALTER TABLE rooms RENAME TO channels`)
			if err != nil {
				return fmt.Errorf("failed to rename rooms to channels: %w", err)
			}
		}
		// Ignore error if neither table exists (new database will create from schema)
	}

	// Migration: Add updated_at column to messages table for edit support
	_, err = db.Exec(`
		ALTER TABLE messages ADD COLUMN updated_at DATETIME;
	`)
	_ = err // Ignore if column already exists

	// Migration: Create campaigns table (fan acquisition links)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS campaigns (
			id TEXT PRIMARY KEY,
			slug TEXT UNIQUE NOT NULL,
			creator_name TEXT NOT NULL,
			channel_name TEXT NOT NULL,
			message TEXT,
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	_ = err

	// Migration: Add landing_config JSON to campaigns
	_, err = db.Exec(`ALTER TABLE campaigns ADD COLUMN landing_config TEXT`)
	_ = err

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS campaign_signups (
			id TEXT PRIMARY KEY,
			campaign_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			email TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (campaign_id) REFERENCES campaigns(id),
			UNIQUE(campaign_id, email)
		);
	`)
	_ = err

	db.Exec(`CREATE INDEX IF NOT EXISTS idx_campaign_signups_campaign ON campaign_signups(campaign_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_campaigns_slug ON campaigns(slug)`)

	// Migration: Create invites table for invite-only registration
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS invites (
			token TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			invited_by TEXT NOT NULL,
			channel_name TEXT,
			expires_at DATETIME NOT NULL,
			used_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	_ = err // Ignore if table already exists

	// Migration: Create cron_jobs table (migrate from file-based storage)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cron_jobs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL DEFAULT 'default',
			agent_id TEXT NOT NULL DEFAULT '',
			schedule TEXT NOT NULL,
			message TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
	`)
	_ = err // Ignore if table already exists

	// Migration: Create FTS5 virtual table for agent_memories (BM25 search).
	// FTS5 may not be compiled into every SQLite build — when it isn't,
	// the CREATE returns an error and we MUST NOT create the sync
	// triggers below, or every INSERT into agent_memories will fail
	// AND every ALTER TABLE in the DB (e.g. migrations 011/012) will
	// fail trigger re-validation with "no such table:
	// agent_memories_fts." Belt-and-suspenders: also drop any
	// pre-existing dangling triggers from older builds.
	_, ftsErr := db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS agent_memories_fts USING fts5(
			id UNINDEXED,
			content,
			tags,
			content='agent_memories',
			content_rowid='rowid'
		)
	`)
	if ftsErr != nil {
		// FTS5 unavailable — drop any leftover triggers from a prior
		// build so ALTER TABLE migrations don't fail trigger
		// validation. Memory search degrades to LIKE-only (handled
		// elsewhere in agent_memory.go) but the rest of the gateway
		// keeps working.
		db.Exec("DROP TRIGGER IF EXISTS agent_memories_ai")
		db.Exec("DROP TRIGGER IF EXISTS agent_memories_ad")
		db.Exec("DROP TRIGGER IF EXISTS agent_memories_au")
	} else {
		// FTS5 sync triggers for agent_memories.
		db.Exec("DROP TRIGGER IF EXISTS agent_memories_ai")
		db.Exec(`CREATE TRIGGER IF NOT EXISTS agent_memories_ai AFTER INSERT ON agent_memories BEGIN
		INSERT INTO agent_memories_fts(rowid, id, content, tags)
		VALUES (new.rowid, new.id, new.content, new.tags);
	END`)
		db.Exec("DROP TRIGGER IF EXISTS agent_memories_ad")
		db.Exec(`CREATE TRIGGER IF NOT EXISTS agent_memories_ad AFTER DELETE ON agent_memories BEGIN
		DELETE FROM agent_memories_fts WHERE rowid = old.rowid;
	END`)
		db.Exec("DROP TRIGGER IF EXISTS agent_memories_au")
		db.Exec(`CREATE TRIGGER IF NOT EXISTS agent_memories_au AFTER UPDATE ON agent_memories BEGIN
		DELETE FROM agent_memories_fts WHERE rowid = old.rowid;
		INSERT INTO agent_memories_fts(rowid, id, content, tags)
		VALUES (new.rowid, new.id, new.content, new.tags);
	END`)
	}

	// Migration: Add admin_only and avatar_url columns to buddies
	_, _ = db.Exec(`ALTER TABLE buddies ADD COLUMN admin_only INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE buddies ADD COLUMN avatar_url TEXT DEFAULT ''`)

	// Every agent is admin_only by default.
	_, _ = db.Exec(`UPDATE buddies SET admin_only = 1 WHERE admin_only = 0`)

	// Migration: drop the per-buddy LLM provider/model columns —
	// vestigial after the byok refactor (LLM provider/model are
	// workspace-wide via workspace_settings.provider_key:*).
	//
	// SQLite's DROP COLUMN does an implicit table rebuild and
	// re-parses every trigger in the database. The agent_memories
	// FTS5 triggers reference fts5 modules that aren't always
	// visible during migration, so the rebuild fails with
	// "no such module: fts5" — even though FTS5 is enabled at
	// runtime. Workaround: drop the triggers, run the ALTER, then
	// recreate them. The triggers below mirror the ones created
	// earlier in this function.
	dropFTSTriggers := func() {
		db.Exec(`DROP TRIGGER IF EXISTS agent_memories_ai`)
		db.Exec(`DROP TRIGGER IF EXISTS agent_memories_ad`)
		db.Exec(`DROP TRIGGER IF EXISTS agent_memories_au`)
	}
	recreateFTSTriggers := func() {
		// ONLY WHERE FTS5 EXISTS. The block that creates the virtual table
		// above is careful never to leave triggers behind on a build
		// without FTS5, because every INSERT into agent_memories would then
		// fail against a table that is not there. This block undid that
		// care by recreating them unconditionally, so a binary built
		// without -DSQLITE_ENABLE_FTS5 could not write a memory at all.
		if ftsErr != nil {
			return
		}
		db.Exec(`CREATE TRIGGER IF NOT EXISTS agent_memories_ai AFTER INSERT ON agent_memories BEGIN
			INSERT INTO agent_memories_fts(rowid, id, content, tags)
			VALUES (new.rowid, new.id, new.content, new.tags);
		END`)
		db.Exec(`CREATE TRIGGER IF NOT EXISTS agent_memories_ad AFTER DELETE ON agent_memories BEGIN
			DELETE FROM agent_memories_fts WHERE rowid = old.rowid;
		END`)
		db.Exec(`CREATE TRIGGER IF NOT EXISTS agent_memories_au AFTER UPDATE ON agent_memories BEGIN
			DELETE FROM agent_memories_fts WHERE rowid = old.rowid;
			INSERT INTO agent_memories_fts(rowid, id, content, tags)
			VALUES (new.rowid, new.id, new.content, new.tags);
		END`)
		// The index is derived from the rows, and while the triggers were
		// off it stopped following them — a write in that window is
		// invisible to it, and an ALTER that rebuilds the table renumbers
		// the rowids it points at. Either leaves search failing with
		// "missing row N from content table" (live 2026-09-05, where it
		// surfaced as "Tool memory failed"). Rebuilding from the rows is
		// cheap here and makes the migration self-healing.
		if _, err := db.Exec(`INSERT INTO agent_memories_fts(agent_memories_fts) VALUES('rebuild')`); err != nil {
			slog.Warn("migration: agent memory search index could not be rebuilt",
				slog.String("error", err.Error()))
		}
	}
	dropFTSTriggers()
	for _, col := range []string{"model_provider", "model_name"} {
		if _, err := db.Exec(`ALTER TABLE buddies DROP COLUMN ` + col); err != nil &&
			!strings.Contains(err.Error(), "no such column") {
			slog.Warn("migration: ALTER TABLE buddies DROP COLUMN failed",
				slog.String("column", col), slog.String("error", err.Error()))
		}
	}
	recreateFTSTriggers()

	// Migration: Add language column to workspaces table
	_, _ = db.Exec(`ALTER TABLE workspaces ADD COLUMN language TEXT DEFAULT 'en'`)

	// Migration: Add workspace_id to workspace_settings (composite PK)
	// Check if migration needed by looking for workspace_id column
	var colCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('workspace_settings') WHERE name='workspace_id'`).Scan(&colCount)
	if colCount == 0 {
		tx, err := db.Begin()
		if err == nil {
			// Recreate table with workspace_id in composite PK
			tx.Exec(`CREATE TABLE workspace_settings_new (
				workspace_id TEXT NOT NULL DEFAULT '',
				key TEXT NOT NULL,
				value TEXT NOT NULL,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (workspace_id, key)
			)`)
			// Copy existing rows as global (workspace_id='')
			tx.Exec(`INSERT INTO workspace_settings_new (workspace_id, key, value, updated_at)
				SELECT '', key, value, updated_at FROM workspace_settings`)
			tx.Exec(`DROP TABLE workspace_settings`)
			tx.Exec(`ALTER TABLE workspace_settings_new RENAME TO workspace_settings`)
			tx.Commit()
		}
	}

	return nil
}
