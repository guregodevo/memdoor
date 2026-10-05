-- Migration 008: Create cron_jobs table for SQLite-backed cron storage
-- Enables Raft replication of cron jobs across cluster nodes

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
