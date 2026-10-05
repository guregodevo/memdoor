-- Account SSH public keys — the identity layer for the SSH transport
-- (clone/push/pull over ssh://, the git@host model).
--
-- A user registers one or more public keys; the gateway's embedded SSH server
-- maps a presented key's fingerprint back to the owning user on connect, then
-- runs the SAME workspace ACL the HTTP transport uses. Only fingerprints +
-- public keys are stored — never anything secret. A DB leak reveals WHO can
-- push, never lets anyone impersonate them (the private key never leaves the
-- client).
--
-- fingerprint:  ssh.FingerprintSHA256 of the marshaled public key
--   ("SHA256:base64..."). Primary key — one row per distinct key, and the same
--   public key can't be registered twice.
-- user_id:      owner; the connection authenticates AS this user. Their
--   workspace binding becomes the ExecutionContext the ACL trusts.
-- public_key:   the full authorized_keys line ("ssh-ed25519 AAAA... label").
--   Kept for `memdoor keys list` display and audit.
-- label:        human tag ("greg's laptop") so users can tell keys apart.
-- created_at / last_used_at: epoch seconds; last_used_at updates on each
--   successful auth for "which keys are actually in use" hygiene.
CREATE TABLE IF NOT EXISTS user_ssh_keys (
    fingerprint  TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    public_key   TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    last_used_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_user_ssh_keys_user_id ON user_ssh_keys(user_id);
