# Quick Start

Memdoor is a terminal coding agent (`memdoor tui`) with a decision model in front of the chat model: it judges what each tool returns, which rung of the model ladder a conversation needs and when a turn should stop, so the models you pay for read less. Chat runs on your own provider key.

For the full walkthrough with explanations, see [`GETTING_STARTED.md`](../../GETTING_STARTED.md). For every command and flag, see [`CLI.md`](CLI.md).

## Prerequisites

- macOS Apple Silicon (darwin/arm64), Linux (linux/amd64, linux/arm64 — static binary, no glibc dependency), or Windows 64-bit (`irm https://memdoor.ai/install.ps1 | iex`). Intel Macs have no prebuilt binary — run Linux on that hardware.
- **Your own key.** `OPEN_ROUTER_API_KEY` in the environment, or any provider's through `/connect` (Anthropic, OpenAI, Gemini, Groq, xAI, DeepSeek, Baseten, a company AI gateway — `docs/features/PROVIDERS.md`).

## Install

```bash
# Installs to ~/.local/bin — no sudo (pass --system for /usr/local/bin); adds it to your shell's PATH
curl -fsSL https://memdoor.ai/install.sh | bash
export OPEN_ROUTER_API_KEY=sk-or-...   # or any provider's key, or /connect inside the TUI
```

The first `memdoor tui` starts the gateway and makes the workspace and this
machine's user, with nothing to answer. For scripts and shared gateways,
`memdoor setup --admin-email … --admin-password … --workspace-name …` still
does it explicitly. Nothing is downloaded: the agent runs on your provider key.

**Building from source instead:** clone the repo and run `make build` — produces `./memdoor` for in-repo dev work.

## Start coding — the TUI

```bash
memdoor tui
```

The coding agent works on the directory you launch it from — reads, edits, builds and searches confined to it. Claude-style UX — tool frames, a live thinking indicator, context usage in the header, `Esc` to interrupt a turn, `Shift+Tab` to cycle the reasoning effort. Slash commands without leaving the chat: `/model` (which model answers, on which provider), `/model-search` (pick a provider, then a model), `/connect` (add a provider), `/usage` (what this month cost, and what the decision model kept out of it), `/help`. Your files, sessions and memory stay on your machine; only what a model has to read leaves it, to the provider you chose.

## Connect your own models

```bash
memdoor providers                # each provider, connected or not, ● the one answering
memdoor connect anthropic        # pick, paste the key unseen, probed, kept
memdoor model search glm         # the catalogue with real prices
memdoor mcp add https://mcp.linear.app/mcp   # an MCP server the coder may use
```

In the window: `/model` pins a model, `/model-search` picks a provider then a model, `/connect` adds a provider, `/mcp` the servers.

## Verify

```bash
memdoor auth whoami        # logged in as you?
memdoor doctor             # install and gateway healthy?
memdoor workspace which    # workspace resolution from cwd
```

## Common follow-ups

```bash
# Pipe a build log into an agent
go test ./... 2>&1 | memdoor agent -m "explain failures" -c general -a chief

# Provision another user (admin only — current user must be admin)
memdoor users create --email teammate@example.com --password 'sekret-test-1234' --admin

# Tail structured logs while you experiment
memdoor logs query --since 5m
memdoor logs errors --since 5m
```

## Troubleshooting

- **`agent` returns "agent not found"**: use a real agent ID (`chief`, `coder`, or `planner`). List with `memdoor agent list`.
- **Port already in use**: `make clean stop start`.
