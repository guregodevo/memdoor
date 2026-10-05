-- Migration 009: Create cron_history and subagent_runs tables
-- Replaces file-based cron history and subagent registry with SQLite-backed storage
-- Enables Raft replication across cluster nodes

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
