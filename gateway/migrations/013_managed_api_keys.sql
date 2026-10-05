-- Per-install API keys for the Memdoor-managed cheap-tier proxy.
--
-- Replaces the earlier "single shared bearer baked into install.sh"
-- design. Now each install hits POST /api/managed/register on first
-- `llm auto`, gets a unique key back, stores it locally encrypted.
-- Server validates the bearer against this table on every proxy
-- call, increments quotas, can revoke individual keys.
--
-- key_hash: SHA-256 of the issued token. The plaintext token is
--   shown to the install exactly once (in the register response) and
--   never persisted server-side. Compromised DB → attacker sees who
--   was registered, but can't impersonate them.
-- install_id: short UUID, the public-facing identifier. Logged on
--   every request for usage attribution. Useful for "I see weird
--   traffic from install_id=abc, ban it" without dealing with hashes.
-- calls_today / cost_microusd_today: rolling 24h counters. Reset
--   when (now - window_start) > 86400. Cheap, no cron job needed —
--   the proxy resets inline on the next request after the window
--   expires.
CREATE TABLE IF NOT EXISTS managed_api_keys (
    key_hash             TEXT PRIMARY KEY,
    install_id           TEXT NOT NULL UNIQUE,
    created_at           INTEGER NOT NULL,
    last_used_at         INTEGER,
    calls_today          INTEGER NOT NULL DEFAULT 0,
    cost_microusd_today  INTEGER NOT NULL DEFAULT 0,
    window_start         INTEGER NOT NULL DEFAULT 0,
    revoked              INTEGER NOT NULL DEFAULT 0,
    hw_fingerprint       TEXT,
    create_ip            TEXT
);

CREATE INDEX IF NOT EXISTS idx_managed_api_keys_install_id ON managed_api_keys(install_id);
CREATE INDEX IF NOT EXISTS idx_managed_api_keys_revoked    ON managed_api_keys(revoked);

-- Registration rate-limit ledger. One row per (ip, hour-bucket); the
-- count column is the number of registrations from that IP in that
-- hour. Crude but works — caps how fast an attacker can mint fresh
-- keys to bypass per-key quotas. Default policy: 10 keys / IP / hour.
CREATE TABLE IF NOT EXISTS managed_register_ledger (
    ip          TEXT NOT NULL,
    hour_bucket INTEGER NOT NULL,
    count       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (ip, hour_bucket)
);
