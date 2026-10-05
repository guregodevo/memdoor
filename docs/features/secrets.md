# Secrets & Credentials

> **The `memdoor secrets` command was removed on 2026-10-03** (less code). The
> runtime feature below still exists; the command examples in this page no
> longer run.

**Version**: 1.0
**Date**: 2026-04-18

Memdoor stores credentials (API keys, tokens) outside of `config.json` so plaintext secrets never land in git. Two scopes exist:

- **Workspace-level secrets** — provider credentials used to resolve `SecretRef` values in `config.json` at load time. Managed via `memdoor secrets`.
- **Agent-level secrets** — per-agent key/value store exposed to tools as the `secret` tool. Managed via the agent runtime (`pkg/repository/agent_secret.go`).

## Providers

Workspace secrets use pluggable backends selected via `--source`:

| Provider | Read | Write | Typical Use |
|----------|------|-------|-------------|
| `env` | ✓ | ✓ | CI/CD, containers |
| `keychain` | ✓ | ✓ | Local development (macOS) |
| `file` | ✓ | ✓ | Docker/Kubernetes secrets mounts |
| `exec` | ✓ | — | 1Password CLI (`op read …`) and similar |

`exec` is read-only: the secret is the stdout of running a command.

## SecretRef in config

Instead of `"botToken": "xoxb-plain-token"`, reference a stored secret:

```json5
{
  "channels": {
    "slack": {
      "botToken": {"source": "keychain", "id": "slack-bot-token"}
    }
  }
}
```

Resolution happens automatically at config load. Plain strings and `${ENV_VAR}` substitutions still work for backward compatibility.

## Agent secrets

Each agent has its own isolated keyring (`AgentSecretRepository`) so one agent's credentials can't leak to another. Values are:

- Write-only from the CLI side (read back only through the agent's `secret` tool).
- Never serialized to JSON responses (`Value` has `json:"-"`).
- Scoped by `agent_id` — deleting the agent deletes its secrets.

## CLI

```bash
# Scan config for plaintext credentials
memdoor secrets audit

# Store + retrieve
memdoor secrets set slack-bot-token xoxb-... --source keychain
memdoor secrets get slack-bot-token --source keychain --reveal

# Pipe from stdin
echo "sk-ant-..." | memdoor secrets set anthropic-key --source keychain
```

Full flag reference: [`reference/CLI.md#secrets`](../reference/CLI.md#secrets).

## Files

- `cmd/cli/cmd/secrets.go` — CLI surface
- `gateway/providers/workspace_secrets.go` — SecretRef resolution
- `pkg/repository/agent_secret.go` — agent-level keyring interface
- `pkg/repository/sqlite/agent_secrets.go` — SQLite backend
- `tools/secret_tool.go` — the `secret` tool exposed to agents
