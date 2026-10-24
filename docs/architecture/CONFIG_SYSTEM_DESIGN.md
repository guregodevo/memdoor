# Configuration

Memdoor reads one optional file, `config.json`, and starts without it. The
loader is `gateway/config/loader.go`; the schema is the `Config` struct in
`gateway/config/types.go`; `$include` is `gateway/config/includes.go`. What
a person tunes day to day is not in that file: the model is a provider's
(`memdoor connect`, `/model`), and the gates (`approve`, `tool_guards`, the
decision switches) are workspace settings in the database. This page says
what the file holds, how it is read, and where the rest lives.

## Where the file lives

`config.ResolveConfigPath()` looks, in order, for `<cwd>/.memdoor/config.json`,
`~/.memdoor/config.json`, then the legacy `config.yaml` in the same two
places (`shared.ProjectOrHome` and `shared.MemdoorHome` in
`pkg/shared/paths.go`; `~/.memdoor` is built there and nowhere else). When
none exists the path is `~/.memdoor/config.json`, and
`LoadConfigFromFile` answers `DefaultConfig()` for a missing file. The gateway
(`gateway/server.go`, the `Config loaded` log line) falls back to
`DefaultConfig()` on any load error too, with a warning.

A `.json` or `.json5` file is parsed as JSON5 (`github.com/titanous/json5`:
comments and trailing commas), then as plain JSON if that fails; `.yaml` and
`.yml` are parsed with `gopkg.in/yaml.v3`. `SaveConfigToFile` writes indented
JSON, mode 0600; it is what `memdoor setup` and `memdoor auth login*` use to
write `workspace_id`.

## A minimal configuration

None. A gateway with no file runs with `DefaultConfig()`: one agent, `main`,
on the workspace `~/.memdoor/workspace`; sessions at
`~/.memdoor/agents/{agentId}/sessions.json`; cron on, its legacy store at
`~/.memdoor/cron/jobs.json`; SQLite at `~/.memdoor/data/memdoor.db`. The
same defaults are filled into a file that leaves a section out
(`applyDefaults`), so a file is only ever the part a person changed.

```json5
// ~/.memdoor/config.json: everything not written here is the default
{
  workspace_id: "my-workspace",
  agents: {
    defaults: { workspace: "~/.memdoor/workspace" },
    list: [
      { id: "coder", default: true, tools: { profile: "coding" } },
    ],
  },
}
```

## How a file is read

`LoadConfigFromFile` runs five steps in this order:

1. `processIncludes` splices every `$include` (below).
2. The result is parsed into `Config`.
3. `substituteEnvVars` expands `${VAR}` with `os.ExpandEnv` in exactly five
   fields: `session.store`, `cron.store`, `agents.defaults.workspace`, and
   each agent's `workspace` and `agent_dir`. An unset variable expands to
   an empty string. No other field sees the environment.
4. `applyDefaults` fills the missing sections and expands a leading `~` in
   `session.store`, `cron.store` and `database.sqlite.path`
   (`ExpandUserPath`; the config path itself is expanded the same way).
5. `resolveSecretRefs` turns secret references into values (below).

`Config.Validate()` runs under `memdoor config validate`, not at boot: it
requires an `agents` section, unique agent ids, a workspace on each agent
or in `agents.defaults`, and that every binding and cron job names an agent
in the list.

### `$include`

A document may carry a top-level `$include`, a path or a list of paths,
relative to the directory of the file that names them. Each included file
is read, parsed in its own format (by extension), and may include further
files; a file already on the current chain is a circular dependency and an
error. The included maps are merged under the including document, maps
recursively and every other value whole, and the including file wins over
what it includes (`mergeMaps`; `TestProcessIncludes_OverrideBehavior` in
`gateway/config/includes_test.go`). A `$include` inside a section is not
processed; only the top level of a document is looked at.

```json5
{
  $include: ["./agents.json", "./cron.json"],
  workspace_id: "my-workspace",   // wins over the same key in an included file
}
```

### Secret references

A string field may hold a JSON object instead of a value:
`{"source": "env", "id": "SLACK_BOT_TOKEN"}` (`secrets.SecretRef`,
`pkg/secrets/types.go`). `secrets.DefaultResolver()`
(`pkg/secrets/factory.go`) knows four sources: `env` (the variable's name),
`file` (a path whose content is the secret), `exec` (a command run through
the shell, its output the secret, `pkg/secrets/providers/exec.go`) and, on
macOS, `keychain` (the item's name). A string that is not such an object
passes through unchanged (`Resolver.ResolveString`, `pkg/secrets/resolver.go`).

The loader resolves references in four fields only: `channels.slack.botToken`,
`channels.slack.appToken`, `channels.slack.signingSecret` and
`database.postgres.url`. Nothing reads those fields today (see `channels`
and `database` below), so the mechanism is in place ahead of a consumer.
Provider keys never go through it: `memdoor connect` keeps them in
`~/.memdoor/providers.json` as a literal, an environment variable's name or a
`!command`, and resolves them at use.

## The sections

The top level of `Config` (`gateway/config/types.go`), with the name each
section has in the file:

| Key | Type | Read by |
|---|---|---|
| `workspace_id` | string | the CLI's workspace resolution |
| `agents` | `AgentsConfig` | the agent runtime at boot |
| `bindings` | `[]Binding` | nothing (parsed and validated only) |
| `session` | `SessionConfig` | `gateway/session_persistence.go` |
| `cron` | `CronConfig` | `gateway/cron/scheduler.go` |
| `queue` | `QueueConfig` | nothing (parsed only) |
| `channels` | `ChannelsConfig` | `gateway/channels/factory.go`, which builds no adapter |
| `tools` | `ToolsConfig` | `gateway/a2a_integration.go` |
| `agent` | `AgentConfig` | `gateway/server.go`, its `id` only |
| `database` | `DatabaseConfig` | `gateway/repository/factory.go` |

**`workspace_id`** is the slug of this machine's workspace, written by
`memdoor setup` once the gateway has registered it (`persistWorkspaceSlug`,
`cmd/cli/cmd/setup.go`) and by `memdoor auth login*` (`pinWorkspace`,
`cmd/cli/cmd/auth.go`). It is the last step of the resolution chain in
`cmd/cli/cmd/workspace_discovery.go`: `-w`, then `$MEMDOOR_WORKSPACE`, then
a `.memdoor/workspace` file walked up from the current directory, then this
field; `memdoor workspace which` prints which one answered.

**`agents`** has `defaults` and `list`. `AgentDefaultsConfig` holds
`workspace`, `tools`, `identity` and `compaction`; each `AgentConfig` in the
list holds `id`, `default`, `name`, `workspace`, `agent_dir`, `skills`,
`tools`, `identity` and `compaction`. There is no model field, on purpose:
the model is the provider's, chosen by a `/model` pin, else the engine
chosen at start from the environment's keys, else the first provider added
with `memdoor connect` (`providers.ClientFactory`,
`gateway/providers/factory.go`; `docs/features/PROVIDERS.md`). At boot the
gateway builds its runtime from `Config.GetDefaultAgent()`: the entry marked
`default`, else the first. The agents the CLI lists and mentions
(`memdoor agent list`, `@coder`) are buddies in SQLite, not this list; the
file-backed agent store was removed because it made a second source of
truth (`gateway/repository/factory.go`).

- `tools` (`AgentToolsConfig`): `profile`, `allow`, `deny`. A deny wins; an
  allow list, merged with the profile, is then the whole set; with neither,
  every tool is allowed (`AgentConfig.IsToolAllowed`, applied in
  `gateway/agent_runtime_tools.go`). Profiles are in
  `gateway/config/tool_policy.go`: `minimal`, `chat` (the default),
  `default`, `coding`, `messaging`, `full`; a name may be a group such as
  `group:fs`, `group:runtime`, `group:web` or `group:sessions_spawn`, which
  `ExpandToolGroups` spells out.
- `compaction` (`CompactionConfig`, the agent's over `defaults`,
  `resolveCompactionConfig` in `gateway/agent_adapter.go`): `enabled`
  (unset is on), `compactionPercent` (default 60 of the window),
  `thresholdTokens` (a fixed trigger that wins over the percent),
  `keepRecentTokens` (what `/compact` keeps word for word, default 12000)
  and `memoryFlush`.
- `identity` (`IdentityConfig`): `name`, `emoji`, `avatar`.
- `workspace` is the agent's bootstrap directory (AGENTS.md, SOUL.md,
  skills); it is not where the coder writes, which is the directory the
  TUI was launched from.

**`bindings`** (`Binding`: `agent_id`, `match` with `channel`,
`account_id`, `peer`) are parsed, validated against the agent list and
counted by `memdoor config validate`; no code routes on them.

**`session`** (`SessionConfig`): `store`, a path template with `{agentId}`;
`main_key` (default `main`); `scope` (`per-agent` by default);
`max_tool_result_bytes` (default 50000, `DefaultMaxToolResultBytes`) and
`truncation_message_tail`, the two that bound a tool result kept in a session
(`gateway/session_persistence.go`).

**`cron`** (`CronConfig`): `enabled` (default true; off, no scheduler is
started, `gateway/server.go`), `store`, and `jobs`, each a `CronJob` with
`id`, `schedule`, `agent_id`, `message`, `enabled`. Jobs written in the file
are registered at start as legacy (`registerConfiguredJobs`,
`gateway/cron/scheduler.go`); the jobs people and agents add, with
`memdoor cron add` and the `cron` tool, live in SQLite
(`cron.NewDBJobStore`) and carry the extra fields `workdir`, `session_key`,
`max_runs`, `run_count` and `expires_at` (`docs/features/cron-jobs.md`).

**`queue`** (`QueueConfig`: `mode`, `debounce_ms`, `cap`, `drop_policy`,
`lanes`) has getters with defaults in `types.go` and no caller outside the
package.

**`channels`** (`ChannelsConfig`) declares `whatsapp` and `slack` structs.
`InitializeChannels` in `gateway/channels/factory.go` builds nothing from
them: an enabled Slack block logs that the adapter is not implemented, and
there is no WhatsApp adapter. The channels a gateway serves are its own: the
TUI's websocket, the CLI, the remote page, cron, peers.

**`tools.agentToAgent`** (`A2AConfig`): `enabled`, `allow` as a list of
`{from, to}` with `*` as a wildcard, `maxPingPongTurns`. `loadA2APolicy` in
`gateway/a2a_integration.go` turns it into the authorization policy for
`sessions_send`; absent, `authorization.DefaultA2APolicy()` applies.

**`agent`** is the single-agent form kept for old files; only its `id` is
read, to wire `sessions_send` (`gateway/server.go`).

**`database`** (`DatabaseConfig`): `type` and `sqlite.path` (default
`~/.memdoor/data/memdoor.db`). `file` and `postgres` blocks parse, but
`NewRepositoryFactory` in `gateway/repository/factory.go` accepts `sqlite`
and nothing else.

## Environment variables

The file carries no address, port or key; those come from flags and the
environment, and none of them overrides a value in the file.

| Variable | What it sets | Where it is read |
|---|---|---|
| `MEMDOOR_HOST` | the address the gateway binds; default `0.0.0.0`, `localhost` keeps it to the machine; `--host` wins | `cmd/cli/cmd/gateway.go` |
| `MEMDOOR_PORT` | the service's port for the installer and the ssh door; the binary itself takes `--port` (18789) | `scripts/install.sh`, `cmd/cli/cmd/gateway_ssh.go` |
| `MEMDOOR_GATEWAY` | the address the CLI and TUI dial: a URL, `host:port`, a bare host, or `ssh://[user@]box[:port]`; `--gateway` wins | `cmd/cli/cmd/root.go` |
| `MEMDOOR_WORKSPACE` | the workspace slug, second in the resolution chain | `cmd/cli/cmd/workspace_discovery.go` |
| `MEMDOOR_APPROVE` | approval mode (`changes` asks before bash, writes and MCP tools); wins over the `approve` setting | `gateway/approval.go` |
| `OPEN_ROUTER_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `AI_GATEWAY_BASE_URL` with `*_AI_GATEWAY_TOKEN`, `MEMDOOR_MODEL`, `MEMDOOR_VENDOR_HEADERS`, `MEMDOOR_MODEL_CONTEXT` | the built-in providers and the engine chosen at start | `gateway/providers/registry.go`, `vendor.go`; `docs/features/PROVIDERS.md` |
| `MEMDOOR_SYSTEMONE_API_KEY` (or `TYPESAFE_API_KEY`), `MEMDOOR_SYSTEMONE_URL`, `MEMDOOR_SYSTEMONE_MODEL` | an explicit decision model; else the person's OpenRouter key judges | `gateway/decisions.go`; `docs/features/DECIDE.md` |
| `MEMDOOR_NO_BROWSER` | never open a browser from `memdoor connect` and `memdoor mcp` | `cmd/cli/cmd/providers.go`, `mcp.go` |
| `MEMDOOR_TELEMETRY_ENABLED` | WARN and above sent to memdoor.ai, off by default | `docs/internal/TELEMETRY.md` |

`docs/SECURITY.md` lists every host the binary names; `gateway/egress_test.go`
fails the build on a new one.

## Workspace settings, outside the file

The gates are rows in the `workspace_settings` table, read on every call so
a change needs no restart, written by an admin through
`PUT /api/workspace/settings` (`gateway/workspace_settings_handlers.go`;
`globalSettingKeys` is the whole list and a key not in it is refused):

- `tool_guards`: regexp block rules per tool, set with `memdoor guards add
  --tool bash --pattern '\bsudo\b'` (`cmd/cli/cmd/guards.go`,
  `gateway/tool_guards.go`).
- `approve`: `changes` or `off` (`gateway/approval.go`); `MEMDOOR_APPROVE`
  wins when set.
- `edit_format`: `patch` or `hashline` (`docs/features/HASHLINE.md`).
- The decision model's switches (`gateway/decisions.go`,
  `gateway/tool_routing.go`): `decision_message_gate` and
  `decision_gate_threshold`, `decision_tool_routing` and `tool_families`,
  `decision_turn_verdict`, `decision_turn_stop`,
  `decision_result_acceptance` and `decision_result_acceptance_threshold`.

Only the guards have a command; the others are set with the PUT, as
`docs/reference/CLI.md` shows with a curl.

## The other files under `~/.memdoor`

`providers.json` is what `memdoor connect` keeps (mode 0600; the key as
given, never resolved onto disk; `gateway/providers/registry.go`);
`workspace/` holds the bootstrap files and the agents' directories;
`skills/` is the seeded skill library (`docs/reference/SKILLS.md`);
`data/memdoor.db` is the database; `workflows/<name>/` is the workflow
library (`docs/features/WORKFLOWS.md`); MCP servers are a project's
`.mcp.json` and `~/.memdoor/mcp.json` (`pkg/mcp/config.go`). A project's own
`.memdoor/` is looked at first for the config file, workflows and the
`workspace` pin.

## Commands

`memdoor config validate` runs `Validate()` and prints the counts (agents,
bindings, cron jobs) and the path it read; `memdoor config show` prints the
loaded `Config` as JSON with the defaults filled in; `memdoor config get
agents.defaults.workspace` reads one value by dot path. All three are in
`cmd/cli/cmd/config_cmd.go`.
