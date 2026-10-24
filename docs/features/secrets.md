# Secrets

Two places a credential can live without landing in `config.json` or git.
Neither has a command of its own (`memdoor secrets` was removed on
2026-10-03); both are reached from the config file and from the agent's
tools.

## A SecretRef in config.json

Wherever `config.json` takes a credential, a reference can stand in for the
value:

```json5
{
  "channels": {
    "slack": {
      "botToken": {"source": "keychain", "id": "slack-bot-token"}
    }
  }
}
```

The loader resolves it when the config is read (`gateway/config/loader.go`,
`pkg/secrets/resolver.go`). Plain strings and `${ENV_VAR}` substitutions
still work beside it.

| `source` | Where the value comes from | Notes |
|---|---|---|
| `env` | the environment variable named by `id` | CI, containers |
| `file` | the file at `id` | Docker and Kubernetes secret mounts |
| `exec` | the stdout of the command in `id` | `op read …` and the like; read-only |
| `keychain` | the macOS keychain item `id` | macOS only; absent elsewhere |

A reference that cannot be resolved fails the config load, naming the field
(`channels.slack.botToken: …`), never a value.

## Agent secrets

Each agent has its own keyring in the database (`AgentSecretRepository`,
`pkg/repository/agent_secret.go`; SQLite in
`pkg/repository/sqlite/agent_secrets.go`). An agent reads its own entries
through the `get_secret` tool (`tools/secret_tool.go`) and nothing else can:

- values are scoped by agent id, so one agent's credentials never reach
  another's turn, and deleting the agent deletes them;
- values never appear in an API response (`Value` is `json:"-"`);
- the tool's description tells the model never to print what it read, and
  anything that looks like a key, a token or a password is redacted from
  tool output before it enters the transcript, from streamed bash output and
  from `/share` (`pkg/secrets/redact.go`).

## Provider keys are not here

The key a model runs on is not a secret in this sense: `memdoor connect`
keeps it in `~/.memdoor/providers.json` as a value, an `ENV_NAME` or a
`!command`, and the built-in providers read theirs from the gateway's
environment. See [PROVIDERS.md](PROVIDERS.md).
