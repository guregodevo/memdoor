# Agent Instructions for Memdoor Development

## Quick Reference

### Common Commands
```bash
# Build & Test
make build                    # Build binary (kills processes, cleans, builds)
make test                     # Run all unit tests
make up                       # Rebuild and restart server
make cli-index                # Regenerate the command index in docs/reference/CLI.md from --help

# Testing
./memdoor agent --message "test" --channel test2 --agent-id chief
./memdoor messages --channel test2 --limit 5
./memdoor logs query --limit 20
./memdoor logs errors --since 5m

# Debugging
./memdoor logs query --regex "PATTERN" --limit N --since Xm
grep -r "duplicate_code" .    # Search before coding

# Plans (2026-10-04): free = the agent, the decision model AND workflows (local
# cron schedules included) on your own machine and key; Pro = $10/month seat:
# remote control through the memdoor.ai relay, the hosted workflow state (mario-state,
# 2026-10-05: every Pro run's state on memdoor.ai); runs with the laptop closed are coming.
# A seat NEVER supplies a model key (Greg 2026-10-04) — always the user's key;
# the gateway never leases a model (no MEMDOOR_BRAIN_CLASS / MEMDOOR_BYOK any
# more). The BROKER owns the seat: token ->
# workspace -> plan. No GPU pool, no catalogue, no warm pools (all removed).

# PROVIDERS, EACH WITH A LIST (2026-10-02, docs/features/PROVIDERS.md) — providers/registry.go:
# built-ins from the env (gateway, anthropic, openai, gemini, openrouter) + ~/.memdoor/providers.json
# (memdoor connect: probe first, key as value|ENV_NAME|!cmd). /model and /model search UNCHANGED,
# they answer across providers; a pin by id takes that provider's client (factory.go GetClientFor).
./memdoor providers              # each provider, connected or not, ● the one answering
./memdoor connect anthropic      # pick, paste, probed, kept
# A COMPANY'S OWN AI GATEWAY FIRST (2026-10-02) — AI_GATEWAY_BASE_URL + *_AI_GATEWAY_TOKEN +
# MEMDOOR_MODEL=<provider slug> → POST <base>/v1/responses (OpenAI Responses shape,
# providers/responses.go; AI_GATEWAY_API=chat for chat/completions). THEN a vendor key:
# ANTHROPIC_API_KEY / OPENAI_API_KEY / GEMINI_API_KEY
# (+ *_BASE_URL for a company AI gateway, MEMDOOR_MODEL, MEMDOOR_VENDOR_HEADERS for cost
# tags): that vendor and NOTHING else — no OpenRouter, no broker, no catalogue, no web
# search, decisions off. providers/vendor.go + anthropic.go; docs/SECURITY.md lists every
# host the binary names and gateway/egress_test.go fails the build on a new one.
# BYOK (2026-09-27) — OPEN_ROUTER_API_KEY in the gateway's env IS the product:
# chat goes straight to OpenRouter on that key (providers/byok.go: local ladders,
# data_collection deny, require_parameters, price sort; /api/models reads
# OpenRouter directly).
./memdoor model                 # each agent's ladder, first rung first
./memdoor model search glm      # the catalogue with REAL list prices
# web_search (2026-09-30) — OpenRouter's search server tool on the same key,
# one search per call, refuses an answer without sources; coder has it with
# web_fetch. docs/features/WEB_SEARCH.md. Chrome can't search (bot challenges).

# Decisions (Jev via OpenRouter, 2026-09-25) — docs/features/DECIDE.md, ADR-0015
# No on/off. JEV IS NOT PRO (2026-10-03): a decision key (MEMDOOR_SYSTEMONE_API_KEY),
# else the person's OPEN_ROUTER_API_KEY (on by default); never the broker.
# Vendor key: only an explicit decision key (autoDecisionModel, decision_model.go).
# Settings, read per call: decision_tool_routing=on   decision_message_gate=on
# memdoor decide was removed 2026-10-03; `memdoor savings` counts the judged reads (proof decisions run)
# Agent tools: jgrep jread (coder); jlogs in no palette

# THE HEARTBEAT IS A WAKE (2026-10-05, OpenClaw's model): a conversation is woken by what it was
# waiting on — a spawned run's report, a scheduled check's answer — as a turn of its own
# (gateway/server_jobs.go wakeConversation). A check with nothing to report answers HEARTBEAT_OK
# and wakes nobody; an answer identical to the last run's wakes nobody. The hourly per-agent
# checklist (its runner, table, API, tool, settings) is deleted; there is no HEARTBEAT.md.
# Subagents (2026-10-05) — the coder has `sessions_spawn`: a self-contained subtask runs in
# parallel in its own session (same project dir); its result WAKES the conversation that spawned
# it and, while it runs, the status line shows "coder · Bash 10s…" from the `subagent` beats
# (agent_runtime_progress.go).
# Scheduled checks (2026-10-01) — docs/features/cron-jobs.md
# The coder has a `cron` tool: it schedules its own re-check ("every 20s, 6
# times: is CI green?") instead of sleeping in bash; each run answers into the
# window that asked (⏱), runs in that project, and stops itself at its bound.
./memdoor cron list              # what is scheduled, with the agent and schedule
./memdoor cron add --id nightly --schedule "0 9 * * *" --agent coder --message "…"

# Workflows (2026-10-02) — docs/features/WORKFLOWS.md, ADR-0019
# A DAG of agent tasks on mario (guregodevo/mario, imported, changed UPSTREAM on
# its workflow-engine branch — never copied). The YAML is MARIO'S: one file per
# task at .memdoor/workflows/<name>/<group>/<task>.yaml, `type: agent`, `requires:
# [{table_pattern: x}]`, `external: true` = made outside, never triggered — the
# run WAITS there until `a`/approve, then the same run is triggered again. Done =
# target exists (file / command / the answer). Missing something? change mario or
# mario-llm, not pkg/workflow ("no need to reinvent the wheel").
./memdoor workflow run release   # /workflow in the TUI draws the DAG live; Esc stops this window's run
# Reusable: a project's .memdoor/workflows/<name>/, or the library ~/.memdoor/workflows/<name>/ (any project)
# The coder has a `workflow` tool (list/run/status/stop/approve) — gateway/workflow_tool.go —
# and the `workflow` SKILL (skills/workflow.md): the person says what they want, the coder
# builds the files. Never put the format in a prompt to the user.
# FREE LOCALLY (2026-10-04, "everyone can run schedule with local cron"): no seat check;
# `memdoor cron add --workflow <name> [--partition today]` schedules one (gateway/server_jobs.go
# scheduledWorkflow). Pro = the hosted workflow state (one table per workspace on memdoor.ai: every
# project and person on it, so an external of one workflow is a task another produced); laptop-off runs coming.
# STATE IS MARIO'S: pkg/workflow = the agent TaskFactory + duck-typed callbacks (Runner,
# Events, Logger); the gateway reads Execution.Status() from mario's repository and keeps
# no task state. Missing/wrong behaviour → change mario, never add state or logic here.

# Remote control (2026-09-28) — docs/features/REMOTE.md
# /remote in the TUI -> https://memdoor.ai/r/<id>#k=<key>; the page IS the TUI
# (a second program on the same conversation, xterm.js on the phone), sealed
# end to end. The relay is on memdoor.ai and takes the ACCOUNT token.
# A new websocket path needs its own nginx location in scripts/deploy.sh.
# MEMDOOR_REMOTE_URL=http://localhost:18789 tests it against a local gateway.

# Models & sessions (post 2026-08-16)
# API ONLY (2026-10-03): no local model in Memdoor. The local runtime is janis
# (github.com/guregodevo/janis, an Ollama for MLX); the clipper and the local-model
# integration live in github.com/guregodevo/clipper (../clipper), kept for a plugin.
./memdoor sessions rewind --channel X --turns 1   # undo derailed turns (vs clear = wipe)
./memdoor sessions export     # local sessions -> JSONL (eval/dataset feedstock)
# /skill:<name> [args] in the TUI injects a skill as the turn's task (deterministic)
# tool_guards workspace setting = regex block rules per tool (see docs/reference/CLI.md)
```

### Critical Patterns
- **Search first:** `grep -r "pattern" .` before adding code
- **Factory returns interface:** Never expose concrete types
- **Duck typing:** Use for circular dependencies
- **CLI testing only:** Never use sqlite3 directly
- **Squash commits:** Before pushing

### Skills Available
- `.agents/skills/coding-principles.md` - Memdoor coding principles (always-on)
- `.agents/skills/refactoring.md` - Refactoring patterns (eliminate duplication, DDD)
- `.agents/skills/quick-test.md` - Pre-commit testing workflow (always-on)
- `.agents/skills/ddd-workflow.md` - DDD patterns and examples
- `.agents/skills/roadmap.md` - Roadmap management (MUST/SHOULD/COULD)
- `.agents/skills/verify-assumptions.md` - Verify runtime changes via CLI and logs (always-on)
- `.agents/skills/test-tui.md` - Drive the real TUI from tmux: which gateway, which keys, what to assert

---

## Documentation
- Read `docs/` directory for project documentation and architecture details
- **`docs/roadmap/MUST.md`** — READ FIRST. The epoch note at the top states what
  Memdoor is now (2026-09-27): a **coding agent for solo devs that cuts their
  model bill, on THEIR OWN key** (BYOK — `gateway/providers/byok.go`; any
  provider's key works). The decision model (Jev) is what does the cutting:
  judged reads, a per-turn toolbox, a stop instead of a cap, `/model` with real
  list prices. Free is the agent **and the decision model** on their key (Greg,
  2026-10-03: "jev decision model is not pro"), and so are workflows and
  their local schedules (2026-10-04); Pro is $10/month: remote control, and
  the hosted scheduler when it comes. A seat never supplies a key: every model
  and decision call is on the user's own (`autoDecisionModel` never picks the
  broker; every chat request goes through a provider, `providers/factory.go`).
  Command line only — one binary, no app, no feature flags. The
  operator/administration commands are left out of `--help`
  (`root.go: notCodingCommands`). The clipper moved to its own repo (2026-10-03). Hidden commands still run when typed. Then
  the current queue.
- Read `docs/reference/CLI.md`
- Read `docs/reference/SKILLS.md` for skill resolution, editing, seeding, and self-extension

## Channel Management
- Use `test` channel for CLI testing and development work

## Workspace Structure
- See `~/.memdoor/workspace` for bootstrap files (AGENTS.md, SOUL.md, TOOLS.md, etc.) and agent workspace directories

## Workspace Resolution (CLI)
The CLI auto-discovers the active workspace via this chain (first match wins): `-w` flag → `$MEMDOOR_WORKSPACE` env → `.memdoor/workspace` file walked up from cwd to `$HOME` → `workspace_id` in `~/.memdoor/config.json`. Use `memdoor workspace use <slug>` to pin a project dir, `memdoor workspace which` to confirm what would resolve. Most CLI examples in this file pass `-w` explicitly for clarity, but the flag is **optional** when discovery works.

## Authentication
- `memdoor setup` no longer creates default users (alice/quentin removed 2026-05-02). Provision the first admin via:
  - `memdoor setup --admin-email <email> --admin-password <pass> --workspace-name <name>` (one-shot, requires gateway running)
  - Or use `memdoor users create --email --password [--admin]` once any admin exists
- Quick login: `./memdoor auth login-direct --email <email> --password <pass>`
- Check current user: `./memdoor auth whoami`

## Coding Principles
**See:** `.agents/skills/coding-principles.md` for complete coding guidelines

Key principles (11 rules):
1. Search before coding (avoid duplicates / code reuse)
2. Test via CLI (never direct DB/HTTP)
3. Use factory pattern (fail fast, initialize all fields)
4. Return interface (never concrete types)
5. Duck typing (for circular refs)
6. No obvious comments (self-documenting code)
7. Use logger (never fmt.Println)
8. Use constants (no magic strings)
9. Follow DDD (read section below)
10. Value objects, not bare strings — model identities (Slug, WorkspaceID, ChannelID) as named types; put behavior on the type; validate at the boundary with a fail-fast `Parse*` constructor
11. Change the Go interface first, run `go build ./...`, and fix exactly what the compiler lists (implementations, mocks) instead of searching for them

The three rules that catch the most real bugs: **type-safety (10), fail-fast constructors (3), code reuse (1).**

## Server Management
- Use `make up` to rebuild and restart the server (builds the binary, launches the
  gateway in the background writing `gateway.log`; it does NOT start the web UI —
  use `make start` for that). It sources `./.envrc` itself (direnv is not installed
  here) and, once up, prints the providers the gateway connected, ● the one
  answering; "no provider connected" means the keys were in neither the shell nor
  `.envrc` (2026-10-03: a bare-shell `make up` came up with zero providers). Load
  keys in your own shell with `set -a; . ./.envrc; set +a` — never print one.
- DO NOT manually kill processes or restart services — and NEVER `pkill -f "memdoor tui"`:
  that kills the USER's interactive TUI session too. Kill only your own tmux session
  (`tmux kill-session -t <yours>`).
- `make up` takes 1-3 minutes (web build included) — run it in the background and poll
  `curl -s http://localhost:18789/api/status` for readiness.
- After a gateway restart, connected TUIs auto-reconnect (banner → "✓ reconnected") and
  flush prompts queued while offline; events emitted during the gap are LOST (no replay),
  so a turn that was mid-run may have finished invisibly.
- The CODER must never run `make up`, `make start`, `make stop` (or any target that stops
  the gateway) from its own turn: the gateway it would restart is the one running the turn,
  which dies mid-flight. It verifies with `go build ./...`, `go test ./...` or `make build`
  (binary only) and leaves the restart to the user. Enforced on this machine's workspace by
  the `tool_guards` rules `scripts/tool_guards_make_up.sh` prints.

## Coder / TUI Semantics (Claude-CLI style)
- **The coder works on the directory the TUI was LAUNCHED from** (pwd): `cd <project> &&
  memdoor tui -w <ws>`. The TUI sends its launch dir with every turn; the gateway roots
  the coder's WRITES there (write_file, edit_file, apply_patch, bash cd, locate index, verify dir);
  reads go anywhere (2026-10-05, "like omp": oh-my-pi confines no read, and bash could read it anyway).
  A guard's first trip (loop-breaker, 30 reads with no change) is a nudge in the model's next input;
  the second ends the turn.
  No workdir sent (CLI channel messages) → `~/memdoor-coder`.
- Approval mode (2026-10-02, gateway/approval.go): `MEMDOOR_APPROVE=changes` or the
  `approve` workspace setting asks before bash / writes / MCP tools through the picker
  (Yes · always this session · No); unanswered 5 min = No. The company-policy "no yolo".
- Always-auto otherwise: no permission modes; the ONLY interruption is the `ask_user_question`
  picker. Esc interrupts a running turn; typing mid-run queues; End jumps to latest;
  ctrl+o expands tool frames; Shift+Tab cycles the reasoning effort (auto = the
  decision model picks per turn, else high — gateway/turn_effort.go).
- Skills are DISK-FIRST: the gateway seeds the embedded library to `~/.memdoor/skills`
  at boot (user edits there win forever, no rebuild needed) and the coder also loads
  `<project>/.agents/skills/`. Repo `./skills/*.md` edits still need a rebuild only to
  update the SEED; a dev gateway running from the repo picks them up directly.
- To test a deployed binary from an arbitrary dir: copy to a NEW path (fresh inode),
  `codesign --force --sign -` it (see Testing Traps), and run from there.

## Making a demo (2026-10-02)
Every demo on memdoor.ai — the hero, the gallery, the casts in the docs — is a
REAL `memdoor tui` session, never a scripted animation (Greg: "we need better
demo", "excellent demos, we need more like this in docs as well"). The method,
in `scripts/demo/`:
- `take.sh <session> <out.cast> "<script>" "<end marker>" <seconds>` — a clean
  checkout (`git archive HEAD` → /tmp/memdoor-<session>, `git init`), a 96x30
  tmux pane running `memdoor tui` (cmux vars unset, `MEMDOOR_NO_BROWSER=1`),
  the script's lines (`type <text>` a key at a time, `key <tmux key>`,
  `sleep <s>`), and the pane sampled 4×/s with `tmux capture-pane -e` by
  `sample-cast.py` into an asciinema v2 file until the marker shows (`■ ` for
  a workflow run, `⏱ poll` for a cron, `answered by` for a turn).
- `trim-cast.py in out 6 2.5` — while only a clock ticks keep one frame per
  6 s, cap gaps at 2.5 s (the gate take: 363 s/3.4 MB → 163 s/516 KB).
- Put the file in `web/public/`, add a card in `DemoGallery.tsx` (title, the
  sentence, a `poster: npt:m:ss` still) and `![caption](/x.cast)` in the doc
  page; the player is `Cast.tsx`, click-to-play, 1.5× speed.
- A workflow sentence must say "workflow"; a gate take needs the approval sent
  from the shell when ⏸ shows. The `/remote` recorder returned no frames on
  2026-10-02 — use the sampler. Run against the user's gateway (18789) only
  when asked; never from the repo (a scratch checkout, as the script does).

## Visual Design Rules (TUI palette)
- **Hue says what a thing is; luminance says how much it matters.** Never
  express hierarchy (primary / secondary / structure) through hue. Full
  record: `docs/adr/0018-hue-is-what-luminance-is-how-much.md`.
- Use ONLY the palette constants in `cmd/tui/ui/model_view.go`:
  `colText` (body text you mean to be read), `colDim` (secondary text,
  WCAG AA), `colFaint` (line numbers, struck-out items), `colRule`
  (gutters, borders, separators — NEVER text), `colFill` (the one tinted
  surface: bands, bars, code blocks). Raw `lipgloss.Color("NNN")` greys are
  banned; hue tokens (`colOK`, `colErr`, `colRun`, `colRead`, `colWrite`,
  `colPlan`, `colAccent`) colour WHAT a line is, not how loud it is.
- Selection is `colSelFG` on `colSelBG` (the shared pair in
  `cmd/tui/ui/model_view.go`) — luminance, not hue, so text on the fill is
  legible; the → cursor carries the accent. Every selection surface (slash
  dropdown, MCP panel, route picker, page strip, model picker) uses this
  pair.
- Every palette change must pass `go test ./cmd/tui/ui/ -run TestThePalette`
  (`palette_test.go` measures WCAG contrast: text floors against the page,
  a ceiling on `colRule`, a legible selected row, and an OK-vs-Err
  luminance gap for colour-blind safety). Never adjust a colour without
  running it.

## Testing

### Available Agents
**Authorized agents** for testing (exist in database):
- `chief` - Main agent (Chief of Staff)
- `coder`, `planner`, `runner`, `verifier`, `narrator` - The coding agents
- `test-agent` - Test agent

**DO NOT use `main`** - it is not authorized and will cause "buddy not found" errors
**Note:** Agent names must match buddies in the database. Use `memdoor agent list` to see current agents.

### Sending Messages to Agents
Use the CLI to send messages and test agent interactions:

```bash
# Send a message to an agent in a channel
./memdoor agent --message "Your message here" --channel CHANNEL_NAME --agent-id AGENT_ID

# Examples:
./memdoor agent --message "Test message" --channel test2 --agent-id chief
./memdoor agent --message "Debug this issue" --channel general --agent-id chief

# Required flags:
#   --message, -m    The message text to send
#   --channel, -c    The channel name (e.g., test2, general)
#   --agent-id, -a   The agent ID to mention (use from authorized list above)

# Optional flags:
#   --verbose, -v    Show verbose output
#   --thinking       Set thinking mode: enabled|disabled|auto (default: auto)
```

### Viewing Messages
View messages in a channel using the messages command:

```bash
# View last 20 messages (default)
./memdoor messages --channel test2

# View specific number of messages
./memdoor messages --channel test2 --limit 50

# View messages with pagination (older messages)
./memdoor messages --channel test2 --before MESSAGE_ID --limit 20

# Include thread replies
./memdoor messages --channel test2 --include-threads
```

### Agent-to-Agent (A2A) Communication Testing
There is no dedicated A2A skill file (the `skills/test-a2a.md` this section
used to cite does not exist). `docs/features/multi-agent-collaboration.md`
describes the `@mention` mechanism; the procedure is the quick test below.

Quick A2A test:
```bash
# Test one agent mentioning another
./memdoor agent --message "Hey test-agent! Please literally type: 'Let me ask @chief for help.'" --channel general --agent-id test-agent
sleep 15
./memdoor messages --channel test2 --limit 3

# Verify A2A worked
./memdoor logs query --regex "A2A|mention_depth" --limit 20 --since 2m
```

### General Testing Guidelines
- DO NOT use direct HTTP calls or manual database queries for testing
- Always use the CLI commands for sending messages and viewing responses
- Use `./memdoor logs query` to debug issues (see Logging section)

### NEVER TEST THE CODER FROM THE REPO
The coder works on the directory the TUI was **launched from**. Testing
from `~/Dev/aktapus` means its files land in the repo — three times this
weekend it wrote a stray `main.go` (repo root, then `gateway/client/`,
breaking the build: two package declarations in one directory).

Always test from a scratch dir:

    mkdir -p ~/memdoor-coder && cd ~/memdoor-coder && memdoor tui -w <ws>

Clean it between runs (`rm -f ~/memdoor-coder/*.go`) — stale files with
`func main` confuse `locate` and collide under `go build ./...`.

### Testing Traps (learned the hard way — read before e2e testing)
For a change a person sees or types, follow `.agents/skills/test-tui.md`: it has
the whole procedure (your own gateway on another port, confirming which binary
serves it, driving tmux, what to assert, negative paths).

- **macOS codesign SIGKILL**: NEVER `cp` a rebuilt binary over an existing one — the per-inode
  signature cache poisons it and every start dies instantly with ZERO output. Copy to a new name
  then `mv -f` (fresh inode), or `codesign --force --sign - <binary>` after copying. A brand-new
  directory per deploy (e.g. `/tmp/<hash>/`) avoids it entirely.
- **`./memdoor agent` BLOCKS** until the turn finishes — run it in the background or it eats your
  2-minute command timeout. Same for `memdoor gateway` (foreground); `timeout` and `setsid` do
  not exist on macOS.
- **Drive the TUI with tmux**, not a hand-rolled PTY: `tmux new-session -d -s coder -x 140 -y 44
  "./memdoor tui -w <ws>"`, `tmux send-keys -t coder "<prompt>" Enter`, `tmux capture-pane -t
  coder -p` for the real rendered screen (`-e` to check colors). Answer pickers with
  `tmux send-keys -t coder "1"`. Start the TUI only AFTER the gateway answers `/api/status`.
- **Clean state between coder tests**: `rm -f ~/memdoor-coder/*.go` (stale files with `func main`
  confuse locate and collide under `go build ./...` — verify the SPECIFIC file with
  `go run <file>.go`) and delete `~/.memdoor/workspaces/*/channels/*/agents/coder/sessions.json`
  for a truly fresh session.
- **Dispatch delay + engine contention**: a turn can sit 30–120s before the first tool call —
  other turns (scheduled checks, spawned runs) share the provider's rate limit. Don't diagnose a hang
  before checking gateway CPU and `AGENT BUILD` log lines.
- **Log-reading traps**: scheduled checks and spawned runs pollute grep windows — a weird "raw
  reply" may be one of those, not the coder. `ERROR Tool execution failed` during coder testing is
  usually an EXPECTED in-loop failure; read the WARN `Tool call failed: <tool> — <reason>` detail
  before treating it as a regression.
- **Skills resolve DISK-FIRST** (chain: `<project>/.agents/skills` →
  gateway-cwd `./skills` → `~/.memdoor/workspace/skills` → `~/.memdoor/skills` (seeded at
  boot) → embedded fallback). Edit `~/.memdoor/skills/<name>.md` and the next skill call
  sees it — no rebuild. The embedded copy only matters for never-booted machines.
- **Coder ground truth is the filesystem**, not the transcript: assert results by running the file
  in `~/memdoor-coder`, and read the coder's session JSON (tool_use/tool_result pairs) to see what
  REALLY happened — the model's final prose may claim work it never did (the honesty guards flag
  this, but verify).

## Logging
- Use `./memdoor logs query` to check logs
- Use `./memdoor logs errors` to check errors
- NEVER read log files directly with `cat`, `tail`, or similar commands
- NEVER use sqlite3 or direct database queries - use CLI tools or logs
- ALWAYS write logs using the centralized logger (slog)
- DO NOT use `fmt.Println` or other direct output methods

### Debugging Tips
- Use `./memdoor logs query --regex "PATTERN" --limit N` to search logs
- Use `--since Xm` flag to limit time range (e.g., `--since 5m` for last 5 minutes)
- Add debug logging with `log.Debug()` to trace data flow
- Use `slog.Any()` for complex objects, `slog.Int64()` for IDs
- Log at key decision points: metadata extraction, threading logic, broadcasts

### Telemetry (WARN+ → memdoor.ai monitoring inbox)
- WARN/ERROR events flow through `gateway/telemetry` to https://memdoor.ai when `MEMDOOR_TELEMETRY_ENABLED=1`
- Agents can emit structured anomalies via the `report_bug` tool (universal palette)
- Sender opt-in (default OFF); same `memdoor logs query` UX works against the receiver
- Full design + env-var contract: [`docs/internal/TELEMETRY.md`](docs/internal/TELEMETRY.md)

## Architecture Patterns

### DDD (Domain-Driven Design)
- `pkg/` is the domain (entities, value objects, services, repositories); `gateway/` is infrastructure + composition (HTTP, WebSocket, DB, engines, billing)
- **`pkg/` never imports `gateway/`** — enforced by grep: `grep -rl '"memdoor/gateway/' pkg | grep -v _test.go` must be empty
- `gateway/` imports `pkg/` services, repositories and value objects freely (about half of it does); what it must not do is put business rules in handlers or bypass a domain constructor
- Use **duck typing** only for CALLBACKS from the domain into the gateway (a circular dependency): declare the interface in the consumer — `SystemAnnouncementPoster` in `gateway/server.go`, satisfied by `pkg/message/service.go`
- Use Factory method that return interface 
- User Predicate

### Thread-Safe Concurrency
- Pass session objects directly instead of SessionManager lookups
- Avoids race conditions when multiple agents work concurrently
- Session metadata flows: simpleSessionContext → AgentExecutorAdapter → tools

### Message Threading
- Use `parent_message_id` to create Slack-like threaded replies
- Flow: User message → Agent response → Subagent spawns → Completion announces → Threaded reply
- Key fields: `parent_message_id` in session metadata, job context, and message entities

## Production Operations

memdoor.ai runs TWO units from the same binary: `memdoor` (gateway) and
`memdoor-billing` (sign-in/Stripe/plans, memdoor.ai only). **Runtime
state must never live under `/opt/memdoor`** — the deploy rsyncs with
`--delete` and will erase it (live incident 2026-08-18). Secrets live in
`/etc/memdoor/billing.env`, ledgers in `/var/lib/memdoor/`. nginx routes
are written from an INLINE heredoc in `scripts/deploy.sh`, not from
`scripts/nginx-memdoor.conf`. Full runbook: `docs/internal/OPS.md`.
**Every LLM request goes through a provider** (2026-10-04, Greg: "no
exception. all request for llm goes through provider"): `providers.ClientFactory`
resolves a pinned model to the provider that lists it, else the engine chosen at
start (a company's vendor key, else the person's own OpenRouter key) to its
provider, else the first provider added with `memdoor connect`. No gateway
leases a model and no seat supplies a key; the broker's brain (Qwen3.8-Omni-Flash,
ADR-0011/0012/0017, the Hermes text protocol) is deleted. A gateway with no key
has no model until one is connected. There is no local model.

## Git Workflow
- ALWAYS test changes before committing
- Build the project to ensure no compilation errors: `make build`
- Test critical paths:
  - Send test messages: `./memdoor agent --message "test" --channel test2 --agent-id chief`
  - View messages: `./memdoor messages --channel test --limit 5 --include-threads`
  - Check logs for errors: `./memdoor logs query --limit 20`
- **Chrome CLI verification**: After deploying or changing UI/web pages, ALWAYS verify with the chrome CLI before assuming it works:
  ```bash
  echo 'navigate https://memdoor.ai/
  wait 2
  screenshot /tmp/verify.png' | ./memdoor chrome run
  ```
  Then read the screenshot to confirm the page renders correctly. Never trust curl alone for UI changes.
- Only commit after verifying all changes work correctly
- **Squash commits**: Combine multiple related commits into a single logical commit before pushing
  - Use `git rebase -i` to squash work-in-progress commits
  - Each final commit should represent one complete feature or fix
  - Write clear, descriptive commit messages that explain the "why" not just the "what"

## Rules kept by Memdoor

- Context windows and output caps are DATA, never constants: a provider's own list, then models.dev (providers/reference.go, daily, on disk, off the network on a company key), then the provider default marked `~` (a guess, warned once per model in the log). Never add a model's window to code; `memdoor providers --models <id>` shows where each figure came from, `--refresh` re-reads. (Greg, 2026-10-02: "very important to maintain it".)

- Review after apply (2026-10-03): after a fix is applied, run the review skill on the diff AND sweep untouched files for the same class of claim ("no MCP server", "rented GPU", "no model-vendor API key" survived two rewrites each). Three rounds in a row a cheap model found a real leftover; the second pass costs cents.
- No word-scoring heuristics or tuned thresholds for narrowing the coder's prompt: when no decision model, send an exact outline (preamble + ## headings with line numbers) instead.
- Skills are agent-agnostic: use .agents/skills/ (never .claude/skills) for repo skills; custom slash commands live in .agents/commands/ (.claude/commands/ is only a legacy fallback); .claude/ is gitignored.
- TUI palette: only constants in cmd/tui/ui/model_view.go (colText/colDim/colFaint/colRule/colFill, colSelFG/colSelBG); raw lipgloss.Color("NNN") greys banned; any colour change must pass `go test ./cmd/tui/ui/ -run TestThePalette`. Full record: docs/adr/0018.
