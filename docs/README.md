# Memdoor Documentation

**Choose your path: Developer or AI Agent**

Memdoor is a terminal layer over the models you already pay for, with a
decision model in front of them to cut the bill: Jev judges what each tool
returns, which rung of the model ladder a conversation needs, and when a
turn has stopped making progress ([features/DECIDE.md](features/DECIDE.md),
[ADR-0015](adr/0015-decisions-through-jev-on-openrouter.md)). Chat goes
through a provider on the person's own key (OpenRouter, a vendor, a company
AI gateway, or one added with `memdoor connect`).
Pro is remote control and the hosted scheduler. The CLI and TUI are the product; the web UI is login/admin.
The coder uses the person's MCP servers ([features/MCP.md](features/MCP.md), 2026-10-01) and runs
on any provider's key — the company's gateway, Anthropic, OpenAI, Gemini, Groq, xAI, DeepSeek,
Baseten or OpenRouter ([features/PROVIDERS.md](features/PROVIDERS.md), 2026-10-02). The current direction is the epoch note at the
top of [`roadmap/MUST.md`](roadmap/MUST.md); plans, pools and the
catalogue are in `internal/PLANS_AND_CAPACITY.md` (removed 2026-10-03).

---

## Get started

- **[Top-level README](../README.md)** — One-paragraph pitch + 60-second install
- **[GETTING_STARTED.md](../GETTING_STARTED.md)** — Zero-to-first-session walkthrough
- **[CLI Reference](reference/CLI.md)** — Every command, every flag
- **[Quick Start](reference/QUICK_START.md)** — Minimal first-run path

---

## For Contributors

**Want to CONTRIBUTE to Memdoor? Start here:**

### Developer Resources

- **[Architecture](reference/ARCHITECTURE.md)** — System design and DDD patterns
- **[Client API](developers/CLIENT_API.md)** — REST & WebSocket API reference
- **[Building Agents](developers/BUILDING_AGENTS.md)** — Create custom AI agents

### Architecture Deep Dives

- **[Multi-Agent Architecture](architecture/MULTI_AGENT_ARCHITECTURE.md)** — Agent isolation model
- **[Execution Context](architecture/EXECUTION_CONTEXT.md)** — Session metadata flow
- **[Config System](architecture/CONFIG_SYSTEM_DESIGN.md)** — JSON5 + `$include` loader
- **[Agent Sandbox Security](architecture/AGENT_SANDBOX_SECURITY.md)** — Tool-level isolation
- **[Always-Valid Pattern](architecture/ALWAYS_VALID_PATTERN.md)** — Constructor factories that fail fast
- **[ADRs](adr/)** — [0001 managed lane](adr/0001-managed-lane-architecture.md) · [0002 inference stays direct](adr/0002-inference-stays-direct.md) · [0003 supplier risk](adr/0003-supplier-risk-and-multi-provider.md)

### Operations

- **[Operations runbook](internal/OPS.md)** — Running memdoor.ai: units, paths, secrets, deploy traps
- **[Telemetry](internal/TELEMETRY.md)** — Shipping local-gateway events to memdoor.ai

---

## For AI Agents

### Agent Resources

- **[Client API](developers/CLIENT_API.md)** — REST & WebSocket APIs
- **[Skills](reference/SKILLS.md)** — Skill resolution, editing, seeding, self-extension

---

## Reference Documentation

Complete technical reference (`docs/reference/`):

- **[CLI Reference](reference/CLI.md)** — All CLI commands and options
- **[Quick Start](reference/QUICK_START.md)** — Minimal first-run path
- **[Architecture](reference/ARCHITECTURE.md)** — System design, DDD, patterns
- **[Authorization](reference/AUTHORIZATION.md)** — Channel ACL, roles, permissions
- **[Cron Jobs](reference/CRON.md)** — Scheduled agent tasks
- **[Logs](reference/LOGS.md)** — Structured logging and debugging
- **[Logs Quick Reference](reference/LOGS_QUICK_REF.md)** — Common log queries
- **[Queue](reference/QUEUE.md)** — Message queuing and concurrency
- **[Skills](reference/SKILLS.md)** — Skill resolution, editing, seeding, self-extension
- **[Gateway API (generated)](reference/gateway/README.md)** — HTTP/WebSocket endpoint reference

---

## Features

Feature-focused walkthroughs (`docs/features/`):

- **[Authentication](features/authentication.md)**
- **[Browser Automation](features/BROWSER_AUTOMATION.md)** — `memdoor chrome run` DSL
- **[Channels & Threading](features/channels-and-threading.md)**
- **[Decisions](features/DECIDE.md)** — Jev through OpenRouter: jgrep / jread / jlogs, per-turn tool routing, the message gate
- **[Cron Jobs](features/cron-jobs.md)** — lightweight overview (reference: `reference/CRON.md`); the coder's own `cron` tool
- **[Workflows](features/WORKFLOWS.md)** — a DAG of agent tasks from a sentence, on mario (Pro)
- **[MCP servers](features/MCP.md)** — `/mcp`: the person's servers, OAuth, the registry
- **[Providers](features/PROVIDERS.md)** — each provider with its own model list, `/connect`, the company gateway, attribution
- **[Hooks](features/HOOKS.md)** — Memdoor's own hooks beside `tool_guards`
- **[Remote control](features/REMOTE.md)** — the terminal on your phone, sealed end to end; `/share`
- **[Web search](features/WEB_SEARCH.md)** — OpenRouter's search tool on the person's key
- **[Hashline edits](features/HASHLINE.md)** — line-anchored edits as an alternative to the patch format
- **[Direct Messages (E2E)](features/DIRECT_MESSAGES_E2E.md)**
- **[Logging](features/logging.md)**
- **[Multi-Agent Collaboration](features/multi-agent-collaboration.md)** — `@mention`-based A2A
- **[Secrets](features/secrets.md)** — Credentials and SecretRef resolution
- **[Stdin Piping](features/stdin-piping.md)** — pipe logs/diffs into agents
- **[Thinking Mode](features/thinking-mode.md)**

---

## Roadmap & Strategy

- **[Roadmap](roadmap/ROADMAP.md)** — Vision and priorities
- **[MUST / SHOULD / COULD](roadmap/MUST.md)** — Priority tiers; MUST.md opens with the 2026-09-27 epoch note
- **[ADRs](adr/)** — managed lane, direct inference, supplier risk
- **[Authentication Roadmap](roadmap/AUTHENTICATION.md)**

---

## Advanced / Internal

- **[Session Architecture](SESSION_ARCHITECTURE.md)**
- **[SQLite Logs Design](SQLITE_LOGS_DESIGN.md)**
- **[Memory Tool](MEMORY_TOOL.md)**
- **[Secure Credentials](SECURE_CREDENTIALS.md)**
- **[Sandbox Implementation](SANDBOX_IMPLEMENTATION_GUIDE.md)**
- **[OpenClaw Organization](openclaw/OPENCLAW_ORGANIZATION.md)**

---

## Testing

- **[Workspace Isolation Tests](testing/WORKSPACE_ISOLATION_TESTS.md)**

---

## Need Help?

- **GitHub Issues** — [Report bugs](https://github.com/guregodevo/memdoor/issues)
- **Top-level README** — [`../README.md`](../README.md)

---
