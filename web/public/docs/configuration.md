# Configuration

Memdoor works without configuration. These are the levers when you need them.

## Where things live

| Path | What's there |
|---|---|
| `~/.local/bin/memdoor` | The binary (the installer puts it here; no sudo) |
| `~/.memdoor/` | Config, credentials, skills, workspaces |
| `~/.memdoor/config.json` | Global config, including the fallback workspace |
| `~/.memdoor/providers.json` | The providers `memdoor connect` kept (the key as you typed it: a value, a variable's NAME or `!command`) |
| `~/.memdoor/model.json` | The model you pinned with `/model`, per person; `/model auto` removes it |
| `~/.memdoor/meter.jsonl` | Every turn: model served, tokens, duration, cost |
| `~/.memdoor/skills/` | Global skills, seeded on first boot — edit freely |
| `<project>/.memdoor/workspace` | Pins a directory to a workspace |
| `<project>/.agents/skills/` | Skills scoped to one project |

## Environment variables

### Scope

| Variable | Effect |
|---|---|
| `OPEN_ROUTER_API_KEY` | Your OpenRouter key: turns go straight to OpenRouter on your account, and the decision model uses it too. Any provider's key works the same for turns: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `DEEPSEEK_API_KEY`, … or `memdoor connect` ([providers](/docs/providers)) |
| `MEMDOOR_WORKSPACE` | Workspace slug, when you don't want directory discovery |
| `MEMDOOR_GATEWAY` | Gateway address the CLI talks to |
| `MEMDOOR_MODEL` | The model on a vendor key or a company gateway (OpenAI, Gemini and the gateway need it named) |
| `AI_GATEWAY_BASE_URL` + `AI_GATEWAY_TOKEN` (or any `*_AI_GATEWAY_TOKEN`) | Your company's own AI gateway: `POST <base>/v1/responses` with `MEMDOOR_MODEL` as the provider slug, nothing else contacted; `AI_GATEWAY_API=chat` for chat/completions |

### Agents and skills

| Variable | Effect |
|---|---|
| `MEMDOOR_APPROVE=changes` | Approval mode: every command, file write and MCP tool call waits for your yes; wins over the `approve` workspace setting |
| `MEMDOOR_SYSTEMONE_API_KEY` | A decision-model key of your own (`memdoor connect typesafe` keeps one too); without it the decision model runs on your OpenRouter key, or not at all |
| `MEMDOOR_SYSTEMONE_URL`, `MEMDOOR_SYSTEMONE_MODEL` | Where that key is sent (default `api.typesafe.ai`) and which decision model answers |

### Operations

| Variable | Effect |
|---|---|
| `MEMDOOR_HOST` | The address the gateway binds: `0.0.0.0` by default (this machine and its network, so a phone reaches it); `localhost` keeps it to this machine |
| `MEMDOOR_REMOTE_URL` | The relay a `/remote` link uses for a gateway kept to localhost (default `https://memdoor.ai`; only memdoor.ai or this machine is accepted) |
| `MEMDOOR_TELEMETRY_ENABLED` | Send warnings and errors upstream (off by default) |
| `MEMDOOR_BILLING_TOKEN` | The `memdoor-billing` service on memdoor.ai only; a developer's gateway never reads it |

## Workspace settings

Per-workspace settings live in the database; `memdoor config show` prints the configuration, and an admin sets workspace settings through the gateway's workspace-settings API. The one worth knowing:

**`tool_guards`** — rules per tool that refuse a matching call: never `.env`, never `git push`, never `sudo`. Set with `memdoor guards add --tool bash --pattern '\bgit push\b' --message '…'`; the whole story is in [Boundaries](/docs/boundaries).

**`approve`** (or `MEMDOOR_APPROVE=changes` in the environment, which wins) — approval mode, for a company that forbids "yolo" agents: before every command, every file write and every MCP tool call, the picker asks *"Run: go test ./... — allow?"* — Yes, Yes-for-this-session, or No. No ends the call with a result the agent acts on; no answer in five minutes is a no. Reads never ask. Off by default.

## Blocked machines

The installer needs only `curl` and a shell. There's no Docker, Python, package manager, or API key anywhere in the install path, and the binary is self-contained (on macOS the installer ad-hoc signs it with `codesign --force --sign -`; it is not notarized) — which is why it runs on locked-down corporate machines where the alternatives can't be installed at all.

## Next

- **[CLI](/docs/cli)** — the commands these settings affect.
- **[Getting started](/docs/getting-started)** — the default path, where none of this is needed.
