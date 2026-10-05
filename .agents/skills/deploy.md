---
name: deploy
description: Deploy Memdoor to production VPS
---

# Deploy Skill

Deploy Memdoor to production at `memdoor.ai`.

## Full Deploy (code + rebuild)

```bash
source .envrc && ./scripts/deploy.sh
```

Syncs code, builds on VPS, restarts gateway. Does NOT sync data or the DB. The
flag that used to push a workspace's data (reset → rsync → reindex) went with
the CLI it called, so deploy takes no other flag now.

## Sync Database

Stop gateway first to avoid corruption:

```bash
source .envrc
sqlite3 ~/.memdoor/data/memdoor.db "PRAGMA wal_checkpoint(TRUNCATE);"
ssh $VPS_HOST "systemctl stop memdoor && rm -f /root/.memdoor/data/memdoor.db-wal /root/.memdoor/data/memdoor.db-shm"
scp ~/.memdoor/data/memdoor.db $VPS_HOST:/root/.memdoor/data/memdoor.db
ssh $VPS_HOST "systemctl start memdoor"
```

## Environment

- **VPS_HOST**: `$VPS_HOST` (from `.envrc`)
- **LLM**: the VPS serves no model. Every user's model calls run on their own provider key, from their own gateway. The `memdoor-billing` unit holds sign-in, plans and Stripe; its secrets live in `/etc/memdoor/billing.env`, its state (`billing.db`, `billing-auth.json`) in `/var/lib/memdoor/`. Never under `/opt/memdoor` — the deploy rsyncs with `--delete`. See `docs/internal/OPS.md`.
- **Gateway**: port 18789, behind Nginx with TLS (routes are the inline heredoc in `scripts/deploy.sh`)

## Verify After Deploy

```bash
# Health check
curl https://memdoor.ai/health

# Check agents
source .envrc && ssh $VPS_HOST "cd /opt/memdoor && LD_LIBRARY_PATH=/usr/local/lib ./memdoor auth login-direct --email "$ADMIN_EMAIL" --password "$ADMIN_PASSWORD" && LD_LIBRARY_PATH=/usr/local/lib ./memdoor agent list"

# Check that the gateway knows its providers
source .envrc && ssh $VPS_HOST "cd /opt/memdoor && LD_LIBRARY_PATH=/usr/local/lib ./memdoor providers"

# Check logs
ssh $VPS_HOST "journalctl -u memdoor -n 30 --no-pager"

# Screenshot landing page
echo 'navigate https://memdoor.ai
wait 3
screenshot /tmp/verify.png' | ./memdoor chrome run
```

## Common Issues

- **Billing service crash-loops after deploy**: `/etc/memdoor/billing.env` is missing or unreadable (a deploy once erased it — live incident 2026-08-18). Restore it from backup; it is not part of the rsync. Runbook: `docs/internal/OPS.md`.
- **A model is missing from the catalogue**: the catalogue is data, `/var/lib/memdoor/catalog.json`, overlaying the built-ins — no release needed (`docs/internal/OPS.md`, "Adding a model without a release").
- **DB corruption after sync**: always stop gateway before syncing DB, checkpoint WAL first
- **A page link in an old bookmark 404s**: that surface was deleted on
  2026-09-27 (its tables are no longer created —
  `pkg/repository/sqlite/factory.go`), so it is a stale bookmark, not
  something a deploy fixes
- **Stale chat widget**: users need to clear localStorage (`wc_token_*`) or widget auto-recovers on 401/403
