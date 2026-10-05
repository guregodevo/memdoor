-- Migration 020: drop the periodic checklist's table.
--
-- The hourly heartbeat runner — a checklist per agent, run in its own
-- session and posted to a channel — was deleted on 2026-10-05. The heartbeat
-- is now the wake of a conversation by what it was waiting on (a spawned
-- run's report, a scheduled check's answer; gateway/server_jobs.go
-- wakeConversation), and keeps no table. A fresh database no longer creates
-- heartbeat_items; this drops it where it exists. IF EXISTS: a no-op after.
DROP INDEX IF EXISTS idx_heartbeat_workspace_agent;
DROP INDEX IF EXISTS idx_heartbeat_channel;
DROP TABLE IF EXISTS heartbeat_items;
