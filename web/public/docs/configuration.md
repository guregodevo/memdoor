# Configuration

Memdoor works without configuration. These are the levers when you need them.

## Where things live

| Path | What's there |
|---|---|
| `~/.local/bin/memdoor` | The binary (the installer puts it here; no sudo) |
| `~/.memdoor/` | Config, credentials, skills, workspaces |
| `~/.memdoor/config.json` | Global config, including the fallback workspace |
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

### Agents and skills

| Variable | Effect |
|---|---|

### Operations

| Variable | Effect |
|---|---|
| `MEMDOOR_TELEMETRY_ENABLED` | Send warnings and errors upstream (off by default) |

## Workspace settings

Per-workspace settings live in the database; `memdoor config show` prints the configuration, and an admin sets workspace settings through the gateway's workspace-settings API. The one worth knowing:

**`tool_guards`** — regex rules per tool that block matching invocations. Use it to forbid a command shape or protect a path without taking the tool away from the agent.

**`approve`** (or `MEMDOOR_APPROVE=changes` in the environment, which wins) — approval mode, for a company that forbids "yolo" agents: before every command, every file write and every MCP tool call, the picker asks *"Run: go test ./... — allow?"* — Yes, Yes-for-this-session, or No. No ends the call with a result the agent acts on; no answer in five minutes is a no. Reads never ask. Off by default.

## Blocked machines

The installer needs only `curl` and a shell. There's no Docker, Python, package manager, or API key anywhere in the install path, and the binary is signed and self-contained — which is why it runs on locked-down corporate machines where the alternatives can't be installed at all.

## Next

- **[CLI](/docs/cli)** — the commands these settings affect.
- **[Getting started](/docs/getting-started)** — the default path, where none of this is needed.
