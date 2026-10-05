-- Migration 014: rewrite the legacy UUID-keyed default workspace to
-- the id-equals-slug convention every other path expects.
--
-- Pre-2026-05-11 `memdoor setup` wrote workspace.id = the legacy
-- DefaultWorkspaceID UUID ('00000000-0000-0000-0000-000000000001')
-- and workspace.slug = 'default' regardless of the operator's typed
-- workspace name. Every other id=slug install (cyberlaw / hackernews
-- / …) and every layer above the DB — the CLI -w flag,
-- gateway URL parsing in /api/wiki/{slug}/…, pkg/wiki/NewService —
-- expects workspace.id == workspace.slug. The mismatch made
-- `memdoor -w <anything> wiki list` 404 because ListWikis queries
-- WHERE workspace_id=? and the slug carried in the URL never matched
-- the UUID stored in the workspace_id column.
--
-- This migration detects the broken row and rewrites the workspace id
-- in place, cascading the new id to every FK column that referenced
-- the UUID. Idempotent: runs once via schema_migrations tracking, and
-- matches nothing on installs that never had the broken setup.
--
-- Deliberately does NOT rewrite wikis.id (and the dependent
-- wiki_pages.wiki_id / wiki_findings.wiki_id / wiki_claims.wiki_id /
-- wiki_page_refs.wiki_id columns). Those store '<workspace_id>:<slug>'
-- per migration 012, but every read path looks the wiki up by the
-- (workspace_id, slug) UNIQUE — not by id directly — so leaving the
-- legacy prefix in place keeps the cascade consistent (wiki_pages.wiki_id
-- still matches wikis.id) and avoids PK collisions on installs that
-- already have a row with the new <slug>:<slug> shape.

-- ---------------------------------------------------------------
-- Stage 1: identify the broken workspace, if any.
--
-- Match only when the row carries the legacy UUID AND has a
-- meaningful slug (not empty, not 'default', and not already
-- equal to id). Anything else means the install is either
-- already healthy or has a setup we shouldn't touch.
-- ---------------------------------------------------------------
CREATE TEMP TABLE _ws_migrate_014 AS
SELECT id AS old_id, slug AS new_id
FROM workspaces
WHERE id = '00000000-0000-0000-0000-000000000001'
  AND slug != ''
  AND slug != 'default'
  AND slug != id;

-- ---------------------------------------------------------------
-- Stage 2: cascade workspace_id across every FK table.
--
-- One UPDATE per table — same shape, distinct column lists where
-- relevant. The temp-table lookup keeps the mapping deterministic
-- even though there's at most one row (the legacy UUID is a
-- singleton). The IN (...) guard prevents touching rows in any
-- already-healthy workspace.
-- ---------------------------------------------------------------
UPDATE buddies
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

UPDATE channels
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

UPDATE agent_events
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

UPDATE buddy_memory
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

UPDATE sessions
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

UPDATE invites
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

UPDATE cron_jobs
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

UPDATE users
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

UPDATE workspace_settings
SET    workspace_id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspace_id)
WHERE  workspace_id IN (SELECT old_id FROM _ws_migrate_014);

-- (The wiki's tables were rewritten here too until the wiki was deleted on
-- 2026-09-27. A fresh database no longer has them, and a database that ran
-- this migration before then already carries the rewrite.)

-- ---------------------------------------------------------------
-- Stage 3: rewrite the workspace row itself.
--
-- Last so all FKs already point to the new id and the
-- workspaces PK update doesn't dangle anything.
-- ---------------------------------------------------------------
UPDATE workspaces
SET    id = (SELECT new_id FROM _ws_migrate_014 WHERE old_id = workspaces.id)
WHERE  id IN (SELECT old_id FROM _ws_migrate_014);

DROP TABLE _ws_migrate_014;
