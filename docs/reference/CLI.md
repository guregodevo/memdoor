# Memdoor CLI Reference

**Version**: 1.7
**Date**: 2026-08-28

Complete command-line interface reference for Memdoor.

---

## Table of Contents

- [Installation](#installation)
- [Global Flags](#global-flags)
- [Workspace resolution](#workspace-resolution)
- [Command index](#command-index) — every command and subcommand, generated from `memdoor --help`
- [Commands](#commands)
  - [setup](#setup)
  - [doctor](#doctor)
  - [config](#config)
  - [cron](#cron)
  - [workflow](#workflow)
  - [gateway](#gateway)
  - [logs](#logs)
  - [agent](#agent)
  - [messages](#messages)
  - [sessions](#sessions)
  - [channels](#channels)
  - [users](#users)
  - [account](#account) — sign in with your email (a code arrives, you type it)
  - [resume](#resume) — reopen a previous conversation
  - [login](#login) — sign in with your email (short for `account login`)
  - [logout](#logout) — sign out of memdoor.ai; the engine stays signed in
  - [chrome](#chrome)
  - [auth](#auth)
  - [completion](#completion)
- [Workspace and other top-level shortcuts](#workspace--use--which--workspaces)
- [uninstall](#uninstall)

---

## Installation

```bash
# Default — system-wide (sudo)
curl -fsSL https://memdoor.ai/install.sh | bash

# No-sudo / corporate laptop — installs to ~/.local/bin
curl -fsSL https://memdoor.ai/install.sh | bash -s -- --user

# Custom prefix (e.g. /opt/memdoor/bin)
MEMDOOR_PREFIX=/opt/memdoor/bin curl -fsSL https://memdoor.ai/install.sh | bash

# Build from source
go build -o memdoor ./cmd/cli

# Or use make
make build
```

**No-sudo path**: when `--user` (or no sudo command available) is detected, the installer writes the binary to `~/.local/bin/memdoor` and prints a `PATH` advisory if that directory isn't already on your shell's `PATH`. Use this on managed corporate laptops where `/usr/local/bin` writes trip MDM/EDR alerts.

---

## Global Flags

Available for all commands:

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--gateway` | | string | `http://localhost:18789` | Gateway server address |
| `--verbose` | `-v` | bool | `false` | Enable verbose output |
| `--workspace` | `-w` | string | (auto-discovered) | Override the workspace for this invocation. Optional — see resolution chain below. |

### Workspace resolution

Every workspace-scoped command needs to know which workspace to talk to. The CLI resolves it via this precedence chain (first match wins) — same shape as `git`'s repo discovery, `kubectl --context`, `direnv`, and `.nvmrc` / `.python-version`:

1. **`-w/--workspace` flag** — explicit override.
2. **`$MEMDOOR_WORKSPACE` env var** — for shell sessions and CI.
3. **`.memdoor/workspace` file** — single line containing the slug. Discovered by walking up from the current directory to `$HOME` (or filesystem root, whichever comes first). Pinning per-project is the recommended idiom; commit `.memdoor/` so collaborators inherit it.
4. **`workspace_id` in `~/.memdoor/config.json`** — per-install default written by `memdoor setup`.

Use `memdoor workspace use <slug>` to write `./.memdoor/workspace` in the current directory, and `memdoor workspace which` to print the resolved slug + the source string.

When verbose mode is on (`-v`) and the slug came from anywhere other than `-w`, the CLI prints one stderr line at startup so debugging "wait, which workspace am I in?" is one flag away:

```
memdoor: workspace "myproject" (from file /Users/you/work/myproject/.memdoor/workspace)
```

### Ergonomics

Three top-level shortcuts cover the daily switch / inspect / list flow:

```bash
memdoor use <slug>      # = memdoor workspace use <slug>
memdoor which           # = memdoor workspace which (add --bare for PS1 use)
memdoor workspaces      # = memdoor workspace list (active row prefixed with '*')
```

`memdoor workspace recent` prints the 20 most recently-pinned slugs from `~/.memdoor/recent_workspaces`.

**Shell prompt**: there is no `memdoor shell-prompt` command (removed
2026-10-03). `memdoor which --bare` is still the one thing to call from a
prompt function, and `memdoor completion <shell>` still exists:

```bash
memdoor which --bare        # just the slug, for your own PS1 snippet
```

**Tab completion**: cobra's standard completion script wires `memdoor use <TAB>` to suggest from the local workspace list. Source it once in your shell rc:

```bash
source <(memdoor completion zsh)        # zsh
source <(memdoor completion bash)       # bash
memdoor completion fish | source        # fish
```

**Exempt top-level commands** (do not require any workspace at all):

- `workspace` — `which` queries the chain; `use` sets it
- `auth` — runs before a workspace is known
- `gateway` — daemon lifecycle, gateway-wide
- `doctor` — system diagnostics
- `setup` — bootstraps the install before any workspace exists
- `version` — prints the binary version
- `logs` — queries the gateway-wide event log

**Everything else** uses the resolution chain: `agent`, `channels`, `messages`, `sessions`, `cron`, `secrets`, `rag`, `users`.

**Examples**:
```bash
# Pin a workspace once per project, then drop -w from every command
cd ~/work/cyberlaw && memdoor workspace use cyberlaw
memdoor agent sessions chief         # auto-discovers from .memdoor/workspace
memdoor agent --message "..." -c general -a chief

# Or use the env var (handy for one-off shells)
MEMDOOR_WORKSPACE=cyberlaw memdoor channels list

# Or pass -w explicitly (always wins over discovery)
memdoor -w cyberlaw messages -c general

# Confirm what would resolve
memdoor workspace which
# → myproject  (from file /Users/you/work/myproject/.memdoor/workspace)

# Verbose mode announces the source on stderr
memdoor -v channels list
# → memdoor: workspace "myproject" (from file /Users/you/work/myproject/.memdoor/workspace)
```

When nothing resolves, the error names every fix:

```
$ memdoor channels list
Error: no workspace resolved for "channels". Set one of:
  • flag:  memdoor -w <slug> channels ...
  • env:   export MEMDOOR_WORKSPACE=<slug>
  • file:  memdoor workspace use <slug>   (writes ./.memdoor/workspace)
  • config: set workspace_id in ~/.memdoor/config.json
```

---

## Command index

<!-- cli-index:start -->
Generated from the binary's `--help` by `make cli-index` (35 top-level commands).
`memdoor --help` shows 27 of them; the others are hidden from it and run when typed:
`auth`, `billing`, `channels`, `chrome`, `cron`, `messages`, `sessions`, `users`. A command that appears here
and nowhere else in this file is documented by its own `memdoor <command> --help`.

**Workspace & collaboration**

- `account` — Sign in with your email (a code arrives, you type it)
  - `login` — Sign in: a code is emailed, type it here (short: memdoor login)
  - `logout` — Sign out of memdoor.ai on this Mac (the engine stays signed in)
  - `status` — Who is signed in on this Mac, and what the seat is
  - `subscribe` — Subscribe to Pro: remote control and the hosted workflow state, always on your own key; $10 a month
- `agent` — Manage agents and send messages
  - `add` — Add a new agent
  - `delete` — Delete an agent
  - `list` — List all agents
  - `sessions` — List agent sessions
  - `show` — Show agent details
  - `update` — Update an agent's configuration
- `login` — Sign in to memdoor.ai with your email: a six-digit code arrives there, and you
- `logout` — Sign out of memdoor.ai on this Mac (the engine stays signed in)
- `setup` — Onboard a fresh Memdoor install: create the workspace and the
- `use` — Writes <slug> into ./.memdoor/workspace so every memdoor invocation
- `which` — Resolves the workspace slug using the standard chain (flag → env →
- `workspace` — Workspace resolution chain (highest precedence first):
  - `delete` — Permanently delete a workspace (admin only, destructive)
  - `list` — List every workspace you have
  - `recent` — List recently-used workspaces (most recent first)
  - `use` — Pin the current directory to a workspace (writes ./.memdoor/workspace)
  - `which` — Print the resolved workspace and where it came from
- `workspaces` — Lists every workspace this install has touched, from

**Servers & interfaces**

- `gateway` — Start the gateway server

**Config, auth & diagnostics**

- `config` — Manage configuration
  - `get` — Get configuration value by dot-notation path
  - `show` — Display full configuration
  - `validate` — Validate configuration file
- `doctor` — Diagnose workspace and configuration health
- `logs` — View and query gateway logs
  - `errors` — Show recent errors
  - `prune` — Delete old log events and reclaim disk space
  - `query` — Query log events
  - `session` — Show session timeline
  - `stats` — Show log statistics
  - `trace` — Trace a log event

**Advanced & connectors**

- `mcp` — The MCP servers the coder can use, for the project in this directory.
  - `add` — Add a server; it is tested at once, and signed in to if it asks
  - `login` — Sign in to a server (OAuth, in your browser)
  - `logout` — Forget a server's sign-in
  - `off` — Turn a server off
  - `on` — Turn a server on
  - `remove` — Delete a server from its file
  - `search` — Find servers in the official MCP registry
  - `test` — Connect a server now and list its tools
  - `trust` — Let this project's .mcp.json servers start

**Other commands**

- `connect` — Adds a provider to this gateway. The kind is one of: gateway, chatgpt, openrouter, anthropic, openai, gemini, xai, baseten, groq, deepseek, typesafe, custom.
- `conversations` — List recent conversations (what `memdoor resume` picks from)
- `join` — Open a link that /remote printed on another computer, in this terminal
- `meter` — Every model request: the model that answered, its tokens and cost (the ledger /usage sums)
- `model` — Which models answer, and which you can pick.
  - `check` — Probe a model for what a coder turn needs (no argument: the ladder)
  - `providers` — A model's hosts: precision, context, uptime, cost band
  - `search` — Find a model in the catalogue
- `providers` — Every provider this gateway knows — the company AI gateway, Anthropic,
- `resume` — Resume a previous conversation.
- `savings` — What the decision model kept out of your model bill this month.
- `tui` — Launch the Terminal UI
- `uninstall` — Removes Memdoor and its running components:
- `upgrade` — Compares the version baked into this binary (see 'memdoor
- `version` — Prints the version string baked into this binary at build
- `workflow` — A workflow is a directory, .memdoor/workflows/<name>/ in a project (or in
  - `approve` — Complete an external task: your yes, written where the DAG looks for it
  - `changes` — Request changes at a gate: the task reruns with your comment, then the steps after it, and the run comes back to the gate
  - `diff` — The full diff a run made since it started, committed or not: what you approve at its gate
  - `history` — What a workflow did before: every run, newest first (survives a restart)
  - `resume` — Continue a failed or stopped run: what it finished is skipped
  - `run` — Start .memdoor/workflows/<name>/ in this project — a fresh run
  - `status` — Every task's state in a run
  - `stop` — Cancel a run: nothing more starts, the running task is told

**Hidden (notCodingCommands in root.go)**

- `auth` — Authenticate with Memdoor
  - `change-password` — Change your own password (requires current password)
  - `login-direct` — Authenticate with email and password (token persists 30 days)
  - `logout` — Clear saved credentials
  - `register` — Register a new account using an invite token
  - `status` — Show authentication status
  - `token` — Print current auth token (for use in curl/scripts)
  - `whoami` — Show current user
- `billing` — Start the billing service: email sign-in, Pro seats through Stripe
  - `backup` — Copy the ledger and broker state into a dated folder, verify it, prune old ones
- `channels` — Manage channels
  - `add-member` — Add a member to a channel
  - `create` — Create a channel to post into
  - `list` — List all channels
  - `members` — List members of a channel
- `chrome` — Browser automation using a simple DSL (Domain-Specific Language) syntax. Connects to Chrome via DevTools Protocol.
  - `run` — Execute browser script from stdin
- `cron` — Manage scheduled cron jobs
  - `add` — Add a new cron job
  - `history` — Show execution history
  - `list` — List all cron jobs
  - `remove` — Remove a cron job
  - `stats` — Show job statistics
- `messages` — View messages in a channel
- `sessions` — Manage sessions
  - `clear` — Clear conversation history for a session (chat memory wipe)
  - `export` — Export local agent sessions as JSONL (for evals / fine-tuning datasets)
  - `get` — Get session details
  - `list` — List the gateway's live sessions (for conversations you can resume, see `memdoor conversations`)
  - `rewind` — Undo the last N turns of a session (keep the rest)
- `users` — Manage workspace users (admin only)
  - `create` — Create a new user (admin only)
  - `delete` — Delete a user and all their data (admin only)
  - `grant-admin` — Promote user to admin
  - `list` — List all users
  - `revoke-admin` — Demote admin to regular user
  - `set-password` — Set a user's password (admin only)
<!-- cli-index:end -->

---

## Commands

### setup

There are two ways to set up an Memdoor workspace: **Web Setup** (recommended for production) and **CLI Setup** (for development).

#### Web Setup (Recommended)

When you start the gateway without an existing database, Memdoor auto-creates the database and serves a web-based setup wizard. No CLI setup needed.

**How it works**:

1. Start the gateway:
   ```bash
   memdoor gateway --verbose
   ```

   Every turn runs on a provider's API: your own key (OpenRouter, Anthropic,
   OpenAI, …) or a company AI gateway.

2. Open the web UI at `http://localhost:18789`

3. The landing page shows a **"Create Workspace"** button (only when no workspace exists)

4. The setup wizard has two steps:
   - **Step 1: Workspace** -- Choose a workspace name and language
   - **Step 2: Admin Account** -- Set your email, username, and password

5. After setup, you're logged in as admin. The workspace name appears in the sidebar header and login page.

**What it creates**:
- Workspace record with your chosen name
- Admin user account (the email/password you entered)
- SQLite database (auto-created on first start)

**Adding users**: Registration is invite-only. As admin, go to Admin Panel and send invites by email. Invited users get a link with a token to register.

**API endpoints** (used by the wizard):
| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/setup/status` | GET | No | Check if workspace is initialized. Returns `{initialized, workspace_name}` |
| `/api/setup` | POST | No | One-time setup. Creates workspace + admin. Blocked if already initialized |
| `/api/invite/validate` | GET | No | Validate an invite token from URL |

#### CLI Setup (Onboarding)

Onboard a fresh install: create the workspace and the first admin user in one go. On a TTY with no flags, `memdoor setup` runs an **interactive onboarding** that asks for workspace name, admin email, and admin password (input hidden, with confirm). For scripts/CI, pass the credentials as flags.

**Usage**:
```bash
memdoor setup [flags]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--workspace` | string | `~/.memdoor/workspace` | Workspace directory path |
| `--workspace-name` | string | `Memdoor` | Display name for the workspace |
| `--admin-email` | string | — | Email for the first admin user (skips interactive prompt) |
| `--admin-password` | string | — | Password for the first admin user (min 8 chars) |
| `--skip-bootstrap` | bool | `false` | Skip creating bootstrap files |
| `--non-interactive` | bool | `false` | Never prompt — Phase 1 only unless `--admin-email`/`--admin-password` are provided |

**What it does**:

*Phase 1 — always (no gateway required):*
- Creates the workspace directory structure (`data/`, `logs/`, `cron/`, `emails/`, `memory/`, `skills/`)
- Writes a default `config.json` (JSON5 format) if none exists
- Creates bootstrap files in the workspace: `AGENTS.md`, `SOUL.md`, `TOOLS.md`
- Initializes a git repository inside the workspace

*Phase 2 — admin registration (requires gateway running):*
- Calls the public `/api/setup` endpoint to create the workspace row and the first admin user — atomically, in one handler
- Auto-logs in and saves credentials to `~/.memdoor/credentials.json` (token persists 30 days, survives gateway restarts)
- Seeds default `#general` channel, and the Chief of Staff agent

Phase 2 is one-time only — re-running `memdoor setup` after the workspace is initialized prints a hint to use `memdoor users create` instead.

**Modes**:
- **Interactive (default on a TTY)** — prompts for workspace name, admin email, and admin password. Falls back to "next steps" if the gateway isn't reachable.
- **Scripted** — pass `--admin-email` and `--admin-password` (and optionally `--workspace-name`); both must be provided together.
- **Phase 1 only** — pass `--non-interactive` to skip Phase 2 even on a TTY.

**Examples**:
```bash
# Interactive onboarding (recommended for first-time install).
# Start the gateway first, then:
make start
memdoor setup

# Scripted: dirs + admin registration in one shot.
memdoor setup \
  --workspace-name 'Demo' \
  --admin-email you@work.com \
  --admin-password 'sekret-not-shared-1234'

# Phase 1 only (gateway not yet running, headless install)
memdoor setup --non-interactive

# Custom workspace location
memdoor setup --workspace ~/my-workspace
```

---

### doctor

Diagnose workspace and configuration health.

**Usage**:
```bash
memdoor doctor [flags]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--workspace` | string | auto-detected | Workspace directory to check |

**What it checks**:
- Configuration file validity
- Workspace directory structure
- Bootstrap files presence
- Git repository status
- File permissions
- Environment variables

**Example**:
```bash
memdoor doctor
memdoor doctor --workspace ~/my-workspace
```

**Output**:
```
🔍 Memdoor Doctor - Workspace Health Check

 Workspace: /Users/user/.memdoor/workspace

✓ Configuration
  • Config file exists: /Users/user/.memdoor/config.json
  • Config is valid JSON5
  • Default agent configured

✓ Workspace Structure
  • Workspace directory exists
  • AGENTS.md present
  • SOUL.md present
  • Memory directory exists

✓ Git Repository
  • Git initialized
  • .gitignore configured

 All checks passed!
```

---

### config

Manage Memdoor configuration.

**Subcommands**:
- `validate` - Validate configuration file
- `get <path>` - Get configuration value
- `show` - Display full configuration

#### config validate

Validate configuration and show summary.

**Usage**:
```bash
memdoor config validate
```

**Example**:
```bash
memdoor config validate
```

**Output**:
```
✓ Configuration is valid

Summary:
  • Agents: 2
  • Bindings: 3
  • Cron jobs: 1
  • Config path: /Users/user/.memdoor/config.json
```

#### config get

Get configuration value by dot-notation path.

**Usage**:
```bash
memdoor config get <path>
```

**Example**:
```bash
memdoor config get agents.defaults.workspace
memdoor config get agents.list[0].id
memdoor config get cron.enabled
```

**Output**:
```
/Users/user/.memdoor/workspace
```

#### config show

Display full configuration in human-readable format.

**Usage**:
```bash
memdoor config show
```

**Example**:
```bash
memdoor config show
```

---

### workflow

Run a DAG of agent tasks — one YAML per task at `.memdoor/workflows/<name>/<group>/<task>.yaml`
(mario's layout; `docs/features/WORKFLOWS.md`). A task is done when its target
exists; a re-run skips what exists; a run stops *waiting* at an `external`
task until it is approved.

**Subcommands**:
- `run <name|path>` — start a workflow of the project, or any directory of task YAML (`--timeout 90m`)
- `status <run-id>` — every task's state
- `stop <run-id>` — cancel a run
- `resume <run-id>` — continue a failed or stopped run: what it finished is skipped
- `approve <run-id> <task>` — complete an external task (your yes, written where the DAG looks)
- `history <name>` — what a workflow did: every run, newest first, with its state and task count (`--limit`, default 20)

**Every run is kept** in the project's run table (`.memdoor/runs.db`, mario's
sqlite repository). `status` reads the run in hand; `history` reads the table,
so it answers for runs that ended before this gateway started — a restart does
not forget what a workflow did. A table that cannot be opened is reported in
the log and the run goes on without it, rather than taking the gateway down.

`memdoor workflow` alone lists the project's workflows and runs — the project's own under `.memdoor/workflows/`, then the shared library's under `~/.memdoor/workflows/` (a project's own of the same name wins). `--dir` names
the project (default: here). In the TUI, `/workflow` draws the graph as it runs.

```bash
memdoor workflow run release
▶ release started as release-20261002-092420 — 4 tasks
  ○ approve    waiting  (external)
  ○ changelog  waiting  ← tests
  ○ release    waiting  ← changelog, approve
  ○ tests      waiting
memdoor workflow approve release-20261002-092420 approve
```

### cron

Manage scheduled cron jobs.

**Subcommands**:
- `list` - List all cron jobs
- `add` - Add a new cron job
- `remove <job-id>` - Remove a cron job
- `history [job-id]` - Show execution history
- `stats [job-id]` - Show job statistics

#### cron list

List all configured cron jobs.

**Usage**:
```bash
memdoor cron list
```

**Example**:
```bash
memdoor cron list
```

**Output**:
```
JOB ID         SCHEDULE      AGENT      ENABLED  MESSAGE
------         --------      -----      -------  -------
daily-health   0 0 9 * * *   main       yes      Daily health check
weekly-report  0 0 10 * * 1  monitor    yes      Weekly status report
backup         0 0 2 * * *   (default)  no       Nightly backup
```

#### cron add

Add a new cron job.

**Usage**:
```bash
memdoor cron add [flags]
```

**Flags**:
| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--id` | string | ✓ | Unique job identifier |
| `--schedule` | string | ✓ | Cron schedule (5-field crontab, or 6-field with leading seconds) |
| `--message` | string | ✓ (or `--workflow`) | What the agent is asked each time |
| `--workflow` | string | | Run this workflow of the project on the schedule instead of an agent turn |
| `--partition` | string | | With `--workflow`: the run's partition, e.g. `today` (at most once a day); default a fresh run each time |
| `--dir` | string | | With `--workflow`: the project directory (default: the current one) |
| `--agent` | string | | Agent ID (optional) |
| `--enabled` | bool | | Enable immediately (default: true) |

**Cron Schedule Format** — standard 5-field crontab, or 6-field with an optional leading seconds field:
```
[second] minute hour day month weekday
```

**Examples**:
```bash
# Daily at 9:00 AM
memdoor cron add \
  --id daily-health \
  --schedule "0 0 9 * * *" \
  --message "Daily health check" \
  --enabled

# Every 30 minutes
memdoor cron add \
  --id frequent-check \
  --schedule "0 */30 * * * *" \
  --message "Frequent status check"

# A workflow every morning at 7, once a day, in this project (free, local)
memdoor cron add \
  --id peer-comps-daily \
  --schedule "0 7 * * *" \
  --workflow peer-comps \
  --partition today

# Weekly on Monday at midnight
memdoor cron add \
  --id weekly-report \
  --schedule "0 0 0 * * 1" \
  --message "Weekly report" \
  --agent monitor
```

**Output**:
```
✓ Cron job "daily-health" added successfully
  Schedule: 0 0 9 * * *
  Message:  Daily health check
  Status:   Enabled (will run on schedule)

Restart the gateway for changes to take effect (e.g. `make clean stop start`).
```

#### cron remove

Remove a cron job.

**Usage**:
```bash
memdoor cron remove <job-id>
```

**Example**:
```bash
memdoor cron remove daily-health
```

**Output**:
```
✓ Cron job "daily-health" removed successfully

Restart the gateway for changes to take effect (e.g. `make clean stop start`).
```

#### cron history

Show execution history for jobs.

**Usage**:
```bash
memdoor cron history [job-id] [flags]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | 10 | Number of records to show |

**Examples**:
```bash
# Show recent history across all jobs
memdoor cron history

# Show history for specific job
memdoor cron history daily-health

# Show last 20 runs
memdoor cron history daily-health --limit 20
```

**Output**:
```
TIME                 DURATION  STATUS   ERROR
----                 --------  ------   -----
2026-02-20 09:00:15  125ms     SUCCESS
2026-02-19 09:00:12  118ms     SUCCESS
2026-02-18 09:00:08  142ms     FAILED   connection timeout
2026-02-17 09:00:11  131ms     SUCCESS
```

#### cron stats

Show execution statistics for jobs.

**Usage**:
```bash
memdoor cron stats [job-id]
```

**Examples**:
```bash
# Show stats for all jobs
memdoor cron stats

# Show stats for specific job
memdoor cron stats daily-health
```

**Output**:
```
JOB ID         TOTAL  SUCCESS  FAILED  SUCCESS RATE  AVG DURATION  LAST RUN
------         -----  -------  ------  ------------  ------------  --------
daily-health   30     28       2       93.3%         125ms         2 hours ago
weekly-report  4      4        0       100.0%        256ms         3 days ago
```

---

### gateway

Manage the gateway server.

**Usage**:
```bash
memdoor gateway [flags]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--port` | int | 18789 | Server port |
| `--host` | string | `localhost` | Server host |

**What it does**:
- Starts the Memdoor gateway server
- Loads configuration
- Initializes cron scheduler
- Installs the engine: a company AI gateway or vendor key first, then your own provider key (OpenRouter or any `memdoor connect` provider)
- Serves the HTTP/WebSocket API for CLI/TUI and the login/admin web UI

**Example**:
```bash
# Start with defaults
memdoor gateway

# Start on custom port
memdoor gateway --port 8080

# Start with verbose logging
memdoor gateway --verbose
```

---

### Tool guards (workspace setting)

Admin-configured block rules applied to every agent tool call — permission
gates as config, not permission modes. Set the `tool_guards` workspace
setting to a JSON array of rules; a matching call is refused with a
teaching error the agent can act on:

```bash
TOKEN=$(memdoor auth token)
curl -X PUT http://localhost:18789/api/workspace/settings \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"tool_guards":"[{\"tool\":\"bash\",\"pattern\":\"\\\\bsudo\\\\b\",\"message\":\"sudo is off-limits here\"}]"}'
```

`tool` is a tool name or `*`; `pattern` is a Go regexp matched against the
call's input JSON; `message` is optional teaching text. Clear with
`{"tool_guards":""}`. One invalid rule never disables the rest.

`turn_token_budget` (same endpoint, a number of tokens) lets a coding turn run
unattended: once the decision model judges a turn unfinished by its receipts,
the turn continues on the model's own "next" until it is shown done or the
budget is spent ([DECIDE.md](../features/DECIDE.md), settings).

### Approval mode (workspace setting `approve`, or `MEMDOOR_APPROVE`)

Memdoor is always-auto; a company that forbids "the default yolo mode"
(a company policy that forbids an agent acting without asking) gets the one mode beside it. With
`approve` set to `changes` — or `MEMDOOR_APPROVE=changes` in the
environment, which a company preset controls and which wins over the
setting — every `bash`, `apply_patch`, `write_file`, `edit_file` and every
MCP tool call waits for the person's answer in the same picker the agent
uses for its own questions: *"Run: go test ./... — allow?"* with **Yes**,
**Yes, and don't ask again for this tool this session**, **No**. No ends
the call with a result the model acts on ("the person said no … ask what
they want instead"); no answer in five minutes is a no, so a turn nobody
is watching (a channel message, a cron, a workflow task) runs no change
unasked. Reads never ask. Read per call: switching needs no restart.
`gateway/approval.go`; `TestApprovalModeAsksBeforeAChange`.

---

### The coder's read_file

`read_file` pages a text file: the first read returns the first 300 lines
and ends with a note like `[512 more lines in big.go (812 in all).
read_file with offset: 301 to continue]`. `offset` (1-based, default 1) and
`limit` (default 300) read any range; `all: true` returns the whole file.
A read that covers everything comes back unchanged, byte for byte, with no
note. Past the end you get `<file> has N lines; line K is past the end.
read_file with offset: 1 reads from the start.` A page also stops at
128 KB, on a whole line; one line longer than that is cut on a rune
boundary and says so. `bash sed -n` reads a range without a read.

### logs

Query gateway logs over the RPC API. Logs live in `logs.db` (SQLite) and are written via `slog`, so all queries hit the structured event store — not a text file. Requires the gateway to be running.

**Subcommands**:
| Command | Purpose |
|---------|---------|
| `logs query` | Query log events (regex, time range, workspace filter) |
| `logs errors` | Show recent error-level events |
| `logs stats` | Show log store statistics (counts by level, disk usage) |
| `logs trace <event-id>` | Show the full event + surrounding context for a single event |
| `logs session <session-id>` | Replay a session's full timeline |

#### logs query

**Usage**:
```bash
memdoor logs query [flags]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `20` | Max number of events to return |
| `--since` | string | | Time range (e.g. `5m`, `1h`, `24h`) |
| `--regex` | string | | Filter events by regex against the message |
| `--order` | string | | Sort order: `asc` or `desc` |
| `--workspace-filter` | string | | Keep events whose `data.workspace` matches this slug (attached by handlers via slog) |
| `--session` | string | | Only this session's events: a channel conversation (`workspace:…:channel:…`) or a delegated run (`agent:…:subagent:…`) |
| `--run` | string | | Only this run's events (`run-<n>-<agent>`) |
| `--data` | bool | `false` | Show each event's id, `session=`, `run_id=` and data attributes |

**Examples**:
```bash
# Last 20 events
memdoor logs query

# Last 5 minutes of WebSocket activity
memdoor logs query --regex "WebSocket" --since 5m --limit 50

# Debug A2A/mention flow
memdoor logs query --regex "A2A|mention_depth" --since 2m --limit 20

# Scope to one workspace
memdoor logs query --workspace-filter cyberlaw --since 1h

# Which conversation made these tool calls, then only that one's
memdoor logs query --regex "Tool call started" --data --limit 5
memdoor logs query --regex "Tool call" --session "agent:coder:subagent:<id>"
```

**Output**:
```
15:46:02.597 INFO  [Config]      Config loaded
15:46:02.598 DEBUG [Agent]       Agent runtime initialized
15:46:02.601 INFO  [HTTP]        Starting gateway server
15:46:27.205 DEBUG [WebSocket]   Sending compressed message

(4 events)
```

#### logs errors

Return recent `ERROR`-level events. Subset of `logs query` with an error-level filter baked in.

**Usage**:
```bash
memdoor logs errors [flags]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `20` | Max errors to return |
| `--since` | string | | Time range (e.g. `5m`, `1h`) |

**Examples**:
```bash
# Errors in the last 5 minutes
memdoor logs errors --since 5m

# Last 100 errors
memdoor logs errors --limit 100
```

**Output**:
```
2026-04-18 15:46:02 ERROR [WebSocket] client disconnected: context canceled
2026-04-18 15:47:11 ERROR [Agent]     tool execution failed: timeout after 30s

(2 errors)
```

When clean:
```
No errors found
```

#### logs stats

Show log store statistics (event counts by level, retention window, disk usage).

**Usage**:
```bash
memdoor logs stats
```

**Output** (keys vary by gateway version):
```
total_events:   12483
errors:         14
warnings:       52
oldest:         2026-04-17T09:00:00Z
db_size_bytes:  3489120
```

#### logs trace

Show the full record for a single event, including any structured `data` fields that were attached via `slog.Any`. Useful when `logs query` truncates a large payload or you want the raw context of one line.

**Usage**:
```bash
memdoor logs trace <event-id>
memdoor logs trace --event-id <event-id>
```

#### logs session

Replay an entire session's timeline — every event tagged with the given `session_id`. Useful for reconstructing what a single agent run did, end to end.

**Usage**:
```bash
memdoor logs session <session-id>
memdoor logs session --session-id <session-id>
```

**Typical workflow**:
```bash
# 1. Find the session ID
memdoor logs query --regex "session_started" --since 10m

# 2. Replay it
memdoor logs session agent:chief:channel:general
```

**Use Cases**:
- **Debugging compression / WebSocket** — `logs query --regex "compressed|WebSocket"`
- **A2A / mention depth** — `logs query --regex "mention_depth|A2A"`
- **One workspace's activity** — `logs query --workspace-filter <ws> --since 1h`
- **Tool execution failures** — `logs errors --since 5m`
- **Full run replay** — `logs session <session-id>`

**See Also**:
- `memdoor gateway --verbose` — starts the gateway and writes structured events to `logs.db`
- `memdoor doctor` — health checks and diagnostics

---

### agent

Send messages to an agent in a channel, or manage agents. **Requires `-w/--workspace`** (enforced by `requireWorkspaceSlug` in `cmd/cli/cmd/root.go`).

**Alias**: `agents` (e.g., `memdoor agents list` works the same as `memdoor agent list`)

**Usage**:
```bash
memdoor -w WORKSPACE agent --message "Your message" --channel CHANNEL_NAME --agent-id AGENT_ID [flags]
```

**Flags** (all three content flags are functionally required — if any is missing the command prints help and exits):

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--message` | `-m` | string | | Message to send to the agent |
| `--channel` | `-c` | string | | Channel name (e.g. `general`, `test2`) |
| `--agent-id` | `-a` | string | | Agent ID to mention (e.g. `writer`, `chief`) |
| `--thinking` | `-t` | string | `auto` | Thinking mode: `enabled`, `disabled`, `auto` (not validated at the flag level) |

**Subcommands**:
- `list` — List all agents in the workspace
- `show <agent-id>` — Show agent details
- `add <agent-id>` — Add a new agent
- `update <agent-name>` — Update an agent's configuration (`--system-prompt`, `--description`, `--personality`, `--tools`). All agents share the workspace's engine (the provider on your key; `/model` pins one, and the pin stays for every new conversation until `/model auto`); no per-agent provider/model field on the buddy row.
- `delete <agent-name>` — Delete an agent (`--force` to skip confirmation)
- `sessions <agent-id>` — List sessions for an agent

**What it does**:
- Posts message to a channel (Slack-like conversation space)
- Mentions the specified agent using @agent-id syntax
- Messages are persistent and appear in Web UI
- All agents in the channel can see conversation history
- Supports stdin piping for log analysis and batch processing

**Channel Integration**:
- Channels are shared conversation spaces (#general, #test2, etc.)
- CLI messages post to channels and are visible to all members
- Use Web UI at http://localhost:18789 to view responses
- Platform channels: Web, TUI, CLI (interfaces to the platform)

**Examples**:
```bash
# Send a message to the writer agent in #general
memdoor -w memdoor agent --message "What's 2+2?" --channel general --agent-id writer

# Use enabled thinking mode
memdoor -w memdoor agent --message "Analyze the codebase structure" --channel general --agent-id writer --thinking enabled

# Post to a different channel
memdoor -w memdoor agent --message "Status update" --channel test2 --agent-id writer

# Pipe stdin for log analysis (Memdoor-specific feature)
./memdoor logs errors --since 1h | memdoor -w memdoor agent --message "Analyze these errors" --channel general --agent-id writer

# Pipe test output for debugging
go test ./... 2>&1 | memdoor -w memdoor agent --message "Explain failures and suggest fixes" --channel general --agent-id writer

# Read a file and summarize
memdoor -w memdoor agent --message "Read the file docs/README.md and summarize it" --channel general --agent-id writer
```

**Output**:
```
✓ Message posted to #general mentioning @writer

 View responses: memdoor messages --channel general
 Or visit Web UI: http://localhost:18789
```

**Viewing Responses**:
Use the `messages` command to view agent responses:
```bash
# View recent messages in the channel
memdoor messages --channel general --limit 20

# Include threaded replies
memdoor messages --channel general --include-threads
```

Or visit the Web UI at http://localhost:18789 to see real-time responses.

**Stdin Piping** (Memdoor-specific feature):
```bash
# Pipe logs to agent for analysis
./memdoor logs errors --since 1h | memdoor -w memdoor agent --message "Root cause?" --channel general --agent-id writer

# Pipe test failures
go test ./... 2>&1 | memdoor -w memdoor agent --message "Debug these failures" --channel general --agent-id writer

# Pipe git diff
git diff main...feature | memdoor -w memdoor agent --message "Review changes" --channel general --agent-id writer
```

**Thinking Modes**:
- `enabled` - Enable extended thinking for complex tasks
- `disabled` - Fast responses without extended thinking
- `auto` - Let the agent decide based on complexity (default)

**Available Agents** (created by `memdoor setup`):
- `chief`, `verifier` — both share the workspace's engine (the provider on your key)

**Available Channels** (created by `memdoor setup`):
- `general` - Public discussion channel

**Error Handling**:
- Gateway unreachable: Start gateway with `./memdoor gateway --verbose`
- Channel not found: Use `memdoor messages --channel general` to verify channel exists
- Agent not found: Use `memdoor agent list` to see available agents
- Authentication required: Ensure you have a valid token (login via Web UI first)

**See Also**:
- `memdoor tui` - Interactive terminal UI
- `memdoor agent list` - List all configured agents
- `memdoor messages` - View channel messages

---

### agent update

Update an existing agent's configuration.

**Usage**:
```bash
memdoor agent update AGENT_NAME [flags]
```

**Flags**:
| Flag | Type | Description |
|------|------|-------------|
| `--system-prompt` | string | Update the agent's system prompt |
| `--description` | string | Update description |
| `--personality` | string | Update personality |
| `--tools` | strings | Update tool palette (comma-separated) |

All agents share the workspace's engine (`/model` and the ladders pick
the model) — there's no `--model` or `--provider` flag on the buddy row.

**Examples**:
```bash
memdoor agent update writer --personality "Friendly and concise"
memdoor agent update coder --system-prompt "You are a code reviewer. Check the diff, not the summary."
```

---

### agent delete

Delete an agent permanently.

**Usage**:
```bash
memdoor -w WORKSPACE agent delete AGENT_NAME [flags]
```

**Flags**:
| Flag | Type | Description |
|------|------|-------------|
| `--force` | bool | Skip confirmation prompt |

**Examples**:
```bash
memdoor -w memdoor agent delete old-agent
memdoor -w memdoor agent delete temp-agent --force
```

### agent show

Show an agent's configuration (model, system prompt, personality, description).

**Usage**:
```bash
memdoor -w WORKSPACE agent show AGENT_ID
```

### agent sessions

List all sessions owned by an agent — one row per `{channel, user}` pair the agent has conversed in. Useful for debugging stuck sessions or finding a session key to pass to `memdoor sessions clear`.

**Usage**:
```bash
memdoor -w WORKSPACE agent sessions AGENT_ID
```

**Example**:
```bash
memdoor -w memdoor agent sessions chief
```

---

### messages

View messages in a channel.

**Usage**:
```bash
memdoor messages --channel CHANNEL_NAME [flags]
```

**Flags**:
| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--channel` | `-c` | string | (required) | Channel name (e.g., general, test2) |
| `--limit` | `-l` | int | 20 | Number of messages to fetch |
| `--before` | `-b` | string | | Message ID to fetch messages before (pagination) |
| `--include-threads` | | bool | false | Show thread replies (hidden by default) |

**What it does**:
- Fetches messages from a specified channel
- Displays message content, author, and timestamp
- Supports pagination for viewing older messages
- Thread replies are hidden by default; use `--include-threads` to show them

**Examples**:
```bash
# View last 20 messages from #general
memdoor messages --channel general

# View last 50 messages from #test2
memdoor messages --channel test2 --limit 50

# Paginate to older messages
memdoor messages --channel general --before <message-id> --limit 20

# Include thread replies
memdoor messages --channel general --include-threads
```

**Output**:
```
📬 Messages in #general (showing 5 total)

[2026-03-09 10:15:32] human: @writer What's 2+2?

[2026-03-09 10:15:35] agent: 2+2 equals 4.

[2026-03-09 10:16:00] human: @writer Can you help debug this?

  ↳ [2026-03-09 10:16:05] agent: Of course! What's the issue?

  ↳ [2026-03-09 10:16:20] human: Tests are failing

 Tip: Use --before 12345 --limit 20 to fetch older messages
 Tip: Use --include-threads to see thread replies
```

**Features**:
- Shows author type (human/agent)
- Timestamp in readable format
- Thread indicators (↳) for replies
- Pagination support for message history

**Authentication**:
Requires authentication token (obtained by logging into Web UI first).

**See Also**:
- `memdoor agent` - Send messages to agents
- `memdoor tui` - Interactive terminal UI

---

---

### sessions

Manage sessions via RPC.

**Subcommands**:
- `list` - List all sessions
- `get <session-key>` - Get session details
- `clear` - Wipe an agent's conversation history (memory + persistence)
- `rewind` - Undo the last N turns of a session (keep the rest)
- `export` - Export local sessions as JSONL (for evals / datasets)

**Note**: Requires gateway to be running.

#### sessions list

List all active sessions.

**Usage**:
```bash
memdoor sessions list
```

**Example**:
```bash
memdoor sessions list
```

**Output**:
```
SESSION KEY              AGENT  MESSAGES  LAST ACTIVITY
-----------              -----  --------  -------------
agent:main:peer:whatsapp main   45        2 minutes ago
agent:work:peer:slack    work   12        1 hour ago
```

#### sessions clear

Wipe an agent's conversation history for a session — both the
in-memory `Session.Messages` AND the on-disk persistence file
that `LoadRecentMessages` re-reads on the next turn. The session
itself is preserved; just its message log is reset, so the next
turn starts with the system prompt only and no prior context to
pattern-match against.

**When to use**: an agent's prior turns are poisoning its replies.
A typical case: an agent in a DM channel pattern-matches its own
earlier failure response and stops calling tools entirely.

**Usage** (two forms):
```bash
# Channel form — CLI assembles the canonical session key
# (workspace:<wsID>:channel:<chID>) for you.
memdoor -w cyberlaw sessions clear --channel general

# Explicit session key (a scheduled check's session, special-purpose ones).
memdoor sessions clear --session-key agent:coder:cron:poll-9970000
```

**Output**:
```
✅ Cleared session workspace:cyberlaw:channel:3c30c268-1c69-4b76-8081-4232904b7d55
```

The response also reports `memory_cleared` (true if the session
was loaded in the LRU cache) and any `persist_error` if the
on-disk wipe failed; "session not in memory" is NOT an error
because chat sessions are loaded on demand.

---

#### sessions rewind

Undo the last N turns of a session instead of wiping it — the
recovery lever for a derailed turn (a bad patch loop, a poisoned
context). A turn is a user prompt plus everything the agent did in
response; the earlier conversation survives and the next turn
continues from the rewound state.

**Usage**:
```bash
memdoor -w hackernews sessions rewind --channel general
memdoor sessions rewind --channel dev --turns 2
memdoor sessions rewind --session-key "workspace:<ws>:channel:<ch>:agent:coder"
```

#### sessions export

Walk the local session store and write one JSONL record per agent
session — the persisted entries verbatim, wrapped in workspace/
channel/agent metadata. Strictly manual and local: nothing is
uploaded. Intended for building eval and fine-tuning datasets from
real agent trajectories. Sessions contain your prompts, code, and
tool outputs verbatim — review before sharing anywhere.

**Usage**:
```bash
memdoor sessions export                          # all sessions
memdoor sessions export --agent coder --out coder.jsonl
memdoor sessions export --workspace hackernews
```

### channels

Manage channels and view channel members.

**Subcommands**:
- `list` - List all channels
- `members --channel NAME` - List members of a channel
- `add-member` - Add a member to a channel

**What it does**:
- Lists all available channels (#general, #test2, etc.)
- Shows channel members (humans and agents)
- Adds members (users or agents) to channels
- Helps verify which agents are in a channel (important for A2A communication)

#### channels list

List all available channels.

**Usage**:
```bash
memdoor channels list
```

**Example**:
```bash
memdoor channels list
```

**Output**:
```
📋 Channels:

NAME                 DESCRIPTION                              TYPE
───────────────────────────────────────────────────────────────────────────
general              General discussion channel               public
test                                                          public
test2                                                         public

Total: 3 channels
```

#### channels members

List all members (humans and agents) in a channel.

**Usage**:
```bash
memdoor channels members --channel CHANNEL_NAME
```

**Flags**:
| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--channel` | `-c` | string | (required) | Channel name (e.g., general, test2) |

**Examples**:
```bash
# List members of #test2
memdoor channels members --channel test2

# List members of #general
memdoor channels members --channel general
```

**Output**:
```
 Members of #test2

Humans:
  ACTOR_ID                       NAME                 ROLE       STATUS
  ───────────────────────────────────────────────────────────────────────────
  human:current-user             current-user         admin      online
  human:c01929c4-...             Alice                member     online

Agents:
  ACTOR_ID                       NAME                 ROLE       STATUS     EMOJI
  ────────────────────────────────────────────────────────────────────────────────
  agent:writer                   writer               member     online     ✍️
  agent:coder                    coder                member     online     💻
  agent:designer                 designer             member     online     🎨
  agent:analyst                  analyst              member     online     

Total: 6 members (2 humans, 4 agents)

 Tip: These agents can mention each other for collaboration!
   Example: memdoor -w memdoor agent --message "@coder ask @writer for help" --channel test2 --agent-id coder
```

**Features**:
- Separates humans from agents
- Shows actor IDs, names, roles, and status
- Provides helpful A2A collaboration hints when multiple agents present

**Use Cases**:
- **A2A Testing**: Verify which agents are in a channel before testing agent-to-agent mentions
- **Debugging**: Check if an agent is a member when @mentions aren't working
- **Setup Verification**: Confirm agents were added to channels correctly

**See Also**:
- `memdoor agent` - Send messages to agents
- [`features/multi-agent-collaboration.md`](../features/multi-agent-collaboration.md) - A2A `@mention` mechanism and the quick test

#### channels add-member

Add a member (user or agent) to a channel.

**Usage**:
```bash
memdoor channels add-member --channel CHANNEL_NAME --actor ACTOR_ID [--role ROLE]
```

**Flags**:
| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--channel` | `-c` | string | (required) | Channel name |
| `--actor` | `-a` | string | (required) | Actor ID to add (e.g., `user:email` or `agent:name`) |
| `--role` | | string | `member` | Role: `member` or `admin` |

**Examples**:
```bash
# Add an agent to a channel
memdoor channels add-member --channel general --actor agent:chief

# Add a user as admin
memdoor channels add-member --channel general --actor user:alice@localhost --role admin
```

**Output**:
```
Added member to channel
  Channel:   9ba7ee0a-a154-41f1-85ff-6322b6ce4da8
  Actor:     agent:chief
  Role:      member
  Joined at: 2026-03-30T23:59:10+02:00
```

---

### users

Manage workspace users and user roles (admin only).

**Subcommands**:
- `list` - List all users with roles and status
- `grant-admin <email>` - Promote user to admin role
- `revoke-admin <email>` - Demote user to regular user role

**Authentication**:
All users commands require admin privileges. You must be authenticated and have admin role.

**What it does**:
- Lists all users in the workspace with their roles
- Manages user roles (admin/user)
- Enforces admin-only access for security

#### users list

List all users in the workspace with their roles and status.

**Usage**:
```bash
memdoor users list
```

**Example**:
```bash
memdoor users list
```

**Output**:
```
 Users:

EMAIL                                    NAME                           ROLE       VERIFIED
────────────────────────────────────────────────────────────────────────────────────────
user@test                                User                           🔑 admin    
admin@localhost                          Admin                          🔑 admin    
creator@test                                creator                           🔑 admin    
alice@localhost                          Alice                          user       
quentin@localhost                        Quentin                        user       
co@test                                  coco                           user       

Total: 6 users (3 admins, 3 regular users)
```

**Features**:
- Displays email, name, role, and email verified status
- Admin users shown with 🔑 emoji
- Summary count of total users, admins, and regular users
- Sorted by creation date (most recent first)

**Authorization**:
Requires admin role. Regular users will receive "Permission denied: admin access required" error.

#### users grant-admin

Promote a user to workspace administrator role.

**Usage**:
```bash
memdoor users grant-admin <email>
```

**Arguments**:
- `<email>` - Email address of the user to promote

**What it does**:
- Looks up user by email address
- Updates their role from "user" to "admin"
- Grants elevated permissions for managing users, agents, and workspace settings

**Examples**:
```bash
# Promote alice to admin
memdoor users grant-admin alice@localhost

# Promote a custom user
memdoor users grant-admin newadmin@example.com
```

**Output**:
```
 Successfully granted admin role to alice@localhost
```

**Verify**:
```bash
# Verify the role change
memdoor users list | grep alice
```

**Authorization**:
Requires admin role. Only workspace administrators can promote users to admin.

**Use Cases**:
- Onboarding new workspace administrators
- Delegating user management responsibilities
- Granting permissions to create/manage agents

#### users revoke-admin

Demote a user from workspace administrator to regular user role.

**Usage**:
```bash
memdoor users revoke-admin <email>
```

**Arguments**:
- `<email>` - Email address of the admin to demote

**What it does**:
- Looks up admin user by email address
- Updates their role from "admin" to "user"
- Removes elevated permissions for managing users, agents, and workspace settings

**Examples**:
```bash
# Demote alice back to regular user
memdoor users revoke-admin alice@localhost

# Demote a custom admin
memdoor users revoke-admin oldadmin@example.com
```

**Output**:
```
 Successfully revoked admin role from alice@localhost
```

**Verify**:
```bash
# Verify the role change
memdoor users list | grep alice
```

**Authorization**:
Requires admin role. Only workspace administrators can demote other admins.

**Use Cases**:
- Removing admin privileges when no longer needed
- Offboarding administrators
- Enforcing principle of least privilege

**Security Notes**:
- Admin role grants elevated permissions including:
  - Managing user roles
  - Creating/deleting agents
  - Viewing all workspace users
  - Managing workspace settings
- Follow principle of least privilege - grant admin only when necessary
- Regular users cannot see the full user list or modify roles
- All role changes are logged for audit purposes

**Error Handling**:
- User not found: "User not found: <email>"
- Permission denied: "Permission denied: admin access required"
- Authentication required: "Authentication required" — `memdoor login` sets the session up
- API error: "API error: <details>"

**See Also**:
- `memdoor login` - Sign in (sets up this Mac's engine session too)
- `memdoor account status` - Both identities: this Mac's engine user and the memdoor.ai account
- `memdoor agent` - Create agents (admin only)

---

### account

The sign-in. One email, one code; this machine's gateway is prepared behind it
without a question (directories, its own first user, a workspace named like
the account). The command line runs exactly these steps; a person types them.

```bash
memdoor login sara@example.com              # a code is emailed; type it when asked (= account login)
memdoor account status                      # this Mac's engine user, and the memdoor.ai account and seat
memdoor account subscribe                   # Pro ($10/month), in the browser
memdoor logout                              # sign out of memdoor.ai (= account logout)
```

There are two identities, and the status prints both. **memdoor.ai** is the
account that holds the Pro seat; `logout` forgets it. **This Mac** is the user
your own gateway knows you as — the window and the CLI talk to the gateway as
that user. `login` sets it up for you, and `logout` leaves it alone: the agent
on your key keeps working, and nothing would renew that session without a
memdoor.ai account.

Non-interactive (scripts and CI):

```bash
memdoor account login sara@example.com --send-only --json             # step one: mail the code
memdoor account login sara@example.com --code 482913 --json           # step two: verify, prepare the engine
memdoor account status --json
```

`status --json`:

```json
{"signed_in": true, "email": "sara@example.com", "workspace": "sara", "plan": "pro",
 "billing": "monthly", "local_ready": true}
```

`plan` is `free` until a seat is paid, `pro`
with one. The token lives in `~/.memdoor/billing-token`; `~/.memdoor/account.json`
remembers the email so `status` answers offline. Behind it: `POST /billing/v1/auth/email`,
`POST /billing/v1/auth/code`, `GET /billing/v1/me`, `POST /billing/v1/subscribe`.

---

### The agent's own jobs

The coder schedules checks for itself with its `cron` tool (every 20 s, six
times: "is CI green?"); they appear in `memdoor cron list` under the agent
that asked, with their run count, and `memdoor cron remove <id>` stops one
from the shell. Bounds, visibility and the live receipt:
[`../features/cron-jobs.md`](../features/cron-jobs.md).

---

### resume

Reopen a previous conversation. Each `memdoor tui` deliberately starts a FRESH
channel (accumulated history makes a small model imitate its own earlier
replies), so `resume` is the way back to an earlier one.

A resumed session rejoins the same channel and the same agent session: its
title returns to the header, its last exchanges to the transcript, and the
agent continues rather than restarting.

**Usage**:
```bash
memdoor resume              # list this directory's conversations, choose one
memdoor resume --last       # the most recent, without asking
memdoor resume 2            # row 2 from that list
memdoor resume 8488c86b     # by id: the line the window prints as it closes, from any directory
memdoor resume --all        # choose from every directory's
memdoor conversations       # just list (same rows, no prompt)
```

When a window closes after at least one turn, its last line is the way back
in — `Resume this conversation with: memdoor resume 8488c86b`. The id is the
first eight characters of the conversation's channel id; any prefix that
singles out one conversation works, and the list shows it in its ID column.

The index lives at `~/.memdoor/sessions.json`, written by the TUI on each turn.
The gateway holds the transcripts; only the client knows which directory you
launched in and what you first asked, which are what make a conversation
recognisable later. A conversation keeps the title it started with.

`memdoor tui --channel <name>` rejoins a channel directly, which is what
`resume` does once you have picked.

---

### login

Sign in to memdoor.ai with your email. A six-digit code arrives there; type it
when asked. It is `memdoor account login` under its short name, with the same
flags, and it stores a durable token (0600 at `~/.memdoor/billing-token`).

**Usage**:
```bash
memdoor login you@example.com                  # a code is emailed; type it when asked
memdoor login you@example.com --code 482913    # the code given up front
```

The code proves the inbox, and the inbox is the account. (The browser device
flow, `--browser`, was removed on 2026-10-05: its approval page took an email
on trust.)

Signing in is what `/remote` needs (the relay has to know whose terminal it
is), and what a Pro seat signs in with: remote control through the
memdoor.ai relay, and every workflow run's state kept on memdoor.ai (runs
with the laptop closed are coming). Inference is always on
your own key.
Workflows need no sign-in.

---

### logout

Sign out of memdoor.ai on this Mac: the seat's token and the account record
are forgotten, so remote control and the hosted workflow state are off; local
workflows and their local schedules keep running. This Mac's engine
session stays — the agent on your own key keeps working. It is
`memdoor account logout` under its short name.

```bash
memdoor logout
memdoor account status     # This Mac: engine user … · memdoor.ai: not signed in
```

---


### chrome

Browser automation using a simple DSL (Domain-Specific Language) syntax. Connects to Chrome via DevTools Protocol.

**Subcommands**:
- `run` - Execute browser script from stdin

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | int | `30` | Timeout in seconds for operations |

#### chrome run

Execute a browser automation script from stdin.

**Usage**:
```bash
memdoor chrome run <<EOF
<commands>
EOF
```

**DSL Commands**:

| Command | Aliases | Description |
|---------|---------|-------------|
| `nav <url>` | `navigate`, `go` | Navigate to URL |
| `wait <duration>` | | Wait for duration (e.g., `1.5`, `2s`, `1500ms`) |
| `snap <path>` | `screenshot`, `screen` | Take screenshot and save to file |
| `snapshot` | `dom` | Get DOM structure |
| `type <selector> <text>` | `fill` | Type text into element (React-compatible) |
| `click <selector>` | | Click element by CSS selector |
| `click-text <text>` | | Click element by visible text |
| `exec <script>` | `js`, `eval` | Execute JavaScript: an expression (`document.title`) or a function (`() => location.href`) |
| `console` | `logs` | Get console messages |
| `network` | `requests` | Get network requests |
| `press <key>` | | Press keyboard key (e.g., `Enter`, `Escape`) |
| `hover <selector>` | | Hover over element |
| `focus <selector>` | | Focus an element |
| `send <text>` | | Type text and press Enter |
| `reload` | `refresh` | Reload current page |

To search the web, agents use the `web_search` tool
([WEB_SEARCH.md](../features/WEB_SEARCH.md)): search engines answer a headless
browser with a bot challenge.

**Examples**:

```bash
# Login flow
memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
type input[type="email"] user@test.com
type input[type="password"] password123
click button[type="submit"]
wait 3
snap /tmp/logged-in.png
EOF

# Inspect console and network
memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
console
network
EOF

# Inject auth token
memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 1.5
exec localStorage.setItem('auth_token', 'abc123')
reload
wait 4
snap /tmp/screenshot.png
EOF
```

**Output**:
```
✓ [0] Navigated to http://localhost:5173
✓ [1] Waited 2.0s
✓ [2] Typed: ok
✓ [3] Typed: ok
✓ [4] Clicked: ok
✓ [5] Waited 3.0s
✓ [6] Screenshot saved to /tmp/logged-in.png (45231 bytes)

Script complete: 7 commands executed
```

**Prerequisites**:
- Chrome/Chromium must be installed
- Chrome DevTools MCP server must be available

---

### auth

This machine's gateway sign-in, by hand — hidden from help because `memdoor login` sets it up for a person. Scripts and test
steps still type it. The browser flow `auth login` was removed on 2026-10-03:
its callback had no listener, so the token never arrived.

**Subcommands**:
- `login-direct --email <email> --password <pass>` - Sign in to this machine's gateway with a password
- `token` - Print the session token (for curl)
- `change-password`, `register` - The local user's password; a new user from an invite
- `logout` - Clear saved credentials
- `whoami` - Show current user
- `status` - Show authentication status

#### auth login-direct

Authenticate directly with email and password (no browser needed). The session token is saved to `~/.memdoor/credentials.json` and stays valid for **30 days**. Sessions are stored hashed in SQLite, so the token survives gateway restarts (`make clean stop start` is fine — only `memdoor auth logout` or natural expiry clears it).

**Usage**:
```bash
memdoor auth login-direct --email <email> --password <password>
```

**Example**:
```bash
memdoor auth login-direct --email you@work.com --password 'sekret-not-shared-1234'
```

#### auth whoami

Show the currently authenticated user.

**Usage**:
```bash
memdoor auth whoami
```

**Output**:
```
Logged in as: you@work.com (the maintainer)
Role: admin
```

#### auth logout

Clear saved credentials.

**Usage**:
```bash
memdoor auth logout
```

#### auth status

Show whether the CLI is authenticated and when the token expires. Does not hit the gateway — reads `~/.memdoor/credentials.json` only.

**Usage**:
```bash
memdoor auth status
```

**Output** (authenticated):
```
Authenticated as: you@work.com
```

**Output** (not authenticated):
```
Not authenticated
  Run: memdoor auth login-direct --email <email> --password <pass>
```

#### auth token

Print the raw auth token to stdout — handy for piping into `curl` or scripts.

**Usage**:
```bash
memdoor auth token
```

**Example**:
```bash
TOKEN=$(memdoor auth token)
curl -H "Authorization: Bearer $TOKEN" http://localhost:18789/api/agents
```

Fails with `not authenticated` if no credentials are saved.

---

### completion

Generate shell completion scripts. One-time setup; after this, tab-completes every command, subcommand, and flag.

**Usage**:
```bash
memdoor completion <shell>
```

**Supported shells**: `bash`, `zsh`, `fish`, `powershell`.

**Install**:

```bash
# Zsh — add to ~/.zshrc
echo 'source <(memdoor completion zsh)' >> ~/.zshrc

# Bash — add to ~/.bashrc (needs bash-completion installed)
echo 'source <(memdoor completion bash)' >> ~/.bashrc

# Fish
memdoor completion fish > ~/.config/fish/completions/memdoor.fish
```

After reloading your shell, `memdoor <Tab>` lists every top-level command, `memdoor workflow <Tab>` lists its subcommands, and so on. Free from Cobra — no maintenance required as new commands land.

---

## Environment Variables

Memdoor respects the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MEMDOOR_CONFIG` | `~/.memdoor/config.json` | Configuration file path |
| `MEMDOOR_PROFILE` | `default` | Active profile name |
| `MEMDOOR_SYSTEMONE_URL` / `_API_KEY` / `_MODEL` | — | Decision-model provider (System One API: hosted Jev or a local Kev); see `docs/features/DECIDE.md` |

**Providers, each with a list** (2026-10-02, `docs/features/PROVIDERS.md`):
`memdoor providers` lists every provider — the built-in ones from the
environment below and the ones `memdoor connect` added to
`~/.memdoor/providers.json` (0600) — with ● on the one answering and the
credential's source; `memdoor connect [gateway|anthropic|openai|gemini|custom]
[--base URL] [--key value|ENV_NAME|!command] [--model id] [--probe] [--remove id]`
probes (the model list, then one call) before it keeps anything; `/connect
[kind]` in the window is the same flow as a picker, the token drawn as
dots. `/model`
and `/model search` are unchanged and answer across every connected
provider; a pin by id takes the client of the provider that lists it.
**A company's own AI gateway comes first** (2026-10-02, `gateway/providers/
responses.go`): `AI_GATEWAY_BASE_URL` + `AI_GATEWAY_TOKEN` (or any
`*_AI_GATEWAY_TOKEN`) + `MEMDOOR_MODEL` = the provider slug → `POST
<base>/v1/responses` (the OpenAI Responses shape; `AI_GATEWAY_API=chat` for
chat/completions). **Then a vendor key** (`gateway/providers/vendor.go`,
`docs/SECURITY.md`): `ANTHROPIC_API_KEY` (model defaults to `claude-sonnet-5`),
`OPENAI_API_KEY` or `GEMINI_API_KEY` (each needs `MEMDOOR_MODEL`), with
`ANTHROPIC_BASE_URL` / `OPENAI_BASE_URL` / `GEMINI_BASE_URL` for a company AI
gateway, `MEMDOOR_MODEL_CONTEXT` for the window and `MEMDOOR_VENDOR_HEADERS`
(`"Name: value; Name: value"`) for the gateway's cost-attribution tags. With
one set, chat goes to that vendor and nothing else is contacted — no
OpenRouter, no broker, no catalogue, no web search, decisions off unless the
gateway holds its own decision key. `MEMDOOR_VENDOR=off` ignores them. **Attribution** (`providers/attribution.go`):
`MEMDOOR_ATTRIBUTION="user=…;team=…"` + `MEMDOOR_ATTRIBUTION_HEADERS="user=X-Email;…"`
send the person's fields, the project's directory name and the session id as
headers on every vendor/gateway request (default names `X-Memdoor-User/-Team/
-Project/-Session`).
`LLM_DEFAULT_PROVIDER` is retired and ignored. `OPEN_ROUTER_API_KEY` is the
BYOK coding key (chat goes to OpenRouter on it) and runs the decision model too;
a decision key of the gateway's own (`MEMDOOR_SYSTEMONE_API_KEY`,
`TYPESAFE_API_KEY`) runs it next to any other provider.

**Example**:
```bash
export MEMDOOR_CONFIG=~/custom/config.json
export MEMDOOR_PROFILE=production

memdoor gateway
```

---

## Configuration File

Memdoor uses JSON5 format for configuration (supports comments and trailing commas).

**Default Location**: `~/.memdoor/config.json`

**Basic Example**:
```json5
{
  // Memdoor Configuration
  "agents": {
    "defaults": {
      "workspace": "~/.memdoor/workspace",
      // No `model` field — every agent shares the workspace's engine
      // (each agent's ladder; /model pins one).
    },
    "list": [
      { "id": "main", "default": true },
      { "id": "work", "workspace": "/work-workspace" },
    ],
  },

  "cron": {
    "enabled": true,
    "store": "~/.memdoor/cron/jobs.json",
    "jobs": [
      {
        "id": "daily-health",
        "schedule": "0 0 9 * * *",
        "message": "Daily health check",
        "enabled": true,
      },
    ],
  },
}
```

**Config Splitting** (via `$include`):
```json5
{
  "$include": ["./agents.json", "./channels.json"],
  "session": {
    "scope": "per-agent"
  }
}
```

---

### workspace / use / which / workspaces

`memdoor workspace` manages which workspace the CLI targets:

- `memdoor workspace list` — every workspace you have access to (alias: top-level `memdoor workspaces`)
- `memdoor workspace use <slug>` — pin the current directory to a workspace (writes `./.memdoor/workspace`; alias: top-level `memdoor use <slug>`)
- `memdoor workspace which` — print the resolved workspace + the source string (alias: top-level `memdoor which`)
- `memdoor workspace recent` — recently-used workspaces, most recent first
- `memdoor workspace delete <slug>` — permanently delete a workspace (admin only, destructive)

There is no `workspace create`: workspaces come from `memdoor setup`.

The resolution chain is documented above under [Workspace resolution](#workspace-resolution).

---

<a id="operational"></a>

### uninstall

Remove Memdoor from the system. Stops the running gateway, removes the binary, and (by default) deletes `~/.memdoor/` — workspace data, master.key.

**Usage**:
```bash
memdoor uninstall              # interactive, prompts before each step
memdoor uninstall --yes        # scripted; deletes binary + data dir
memdoor uninstall --keep-data  # remove binary, preserve ~/.memdoor
```

There are no external prerequisites to remove — the binary is self-contained.

## Workflow Examples

### Initial Setup
```bash
# 1. Initialize workspace
memdoor setup

# 2. Verify setup
memdoor doctor

# 3. Start gateway
memdoor gateway --verbose
```

### Managing Cron Jobs
```bash
# Add a daily job
memdoor cron add \
  --id daily-check \
  --schedule "0 0 9 * * *" \
  --message "Daily health check"

# List all jobs
memdoor cron list

# View execution history
memdoor cron history daily-check

# View statistics
memdoor cron stats
```

### Working with Configuration
```bash
# Validate config
memdoor config validate

# Get specific value
memdoor config get agents.defaults.workspace

# Show full config
memdoor config show
```

### Interactive Usage
```bash
# Start gateway in background
memdoor gateway &

# Launch TUI
memdoor tui

# Or manage via CLI
memdoor agents list
memdoor sessions list
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Connection error (gateway unreachable) |

---

## Getting Help

**Command Help**:
```bash
memdoor --help
memdoor config --help
memdoor cron add --help
```

**Report Issues**:
- GitHub: [guregodevo/memdoor/issues](https://github.com/guregodevo/memdoor/issues)

---

## See Also

- [Quick Start](QUICK_START.md)
- [Cron Jobs Guide](CRON.md)
- [Skills Reference](SKILLS.md)

---

**CLI Version**: 1.4
