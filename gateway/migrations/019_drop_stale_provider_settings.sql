-- Migration 019: drop the provider rows that name the deleted local engine.
--
-- `memdoor llm auto` used to write one `agent_provider:<agent>` row per agent.
-- The local engine carried two names in its life — `spartacus` before the
-- rename in 2a14072, `llamafit` after — and migration 015 moved rows from the
-- first to the second. The engine is gone (2026-10-03: API only, every LLM
-- request goes through a provider) and the registry knows neither name, so a row
-- saying `llamafit` names a provider no build can resolve.
--
-- Nothing reads these rows any more: the settings API refuses to write them
-- (gateway/workspace_settings_handlers.go), and routing comes from the
-- environment and ~/.memdoor/providers.json. The honest end state is therefore
-- no row at all — no override, the configured default applies. Deleted, not
-- rewritten, because there is no live provider name to rewrite them TO.
--
-- Migration 015 was removed with this one: its only job was writing `llamafit`
-- into these rows. A database that ran it carries the name until this runs.
DELETE FROM workspace_settings
WHERE  key LIKE 'agent_provider:%'
  AND  value IN ('spartacus', 'llamafit');
