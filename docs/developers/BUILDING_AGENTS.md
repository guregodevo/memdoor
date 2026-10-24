# Building Custom Agents

An agent is a row on your gateway: a name, a prompt, a list of tools and a
few switches. The model is not part of it. Every agent runs through a
provider on your own key, and you pin a model from the window. This page
is what you can shape today, named by the code that reads it.

## What an agent is

`domain.Buddy` (`pkg/domain/models.go`), stored in the `buddies` table and
served by `gateway/agent_handlers.go`:

| Field | What it does |
|---|---|
| `name` | the id people type after `@`; lower-case, kebab-case |
| `description` | one line, shown to people (`agent show`, listings) |
| `personality` | one line, the `# Personality` section of the prompt |
| `system_prompt` | the agent's own instructions; the builder adds the rest (below) |
| `tools` | an allow list of tool names; the agent sees these and nothing else |
| `sandbox_scope` | `user` (default), `channel` or `workspace`: whose data its memory and files reach |
| `learning_enabled` | memory retrieval and reflection on (the default) |
| `admin_only` | hidden from and unusable by non-admins; a new agent starts this way |
| `execution_type` | `local` (the gateway's providers) or `remote` (an OpenAI-compatible endpoint of your own) |
| `is_active` | `online` or `offline` in listings |

There is no `model`, `provider`, `temperature` or `max_tokens` to set: the
row's `temperature` and `max_tokens` are fixed defaults the create handler
writes (`handleCreateAgent`), and the model comes from the route (below).
Skills are not a field either; they are files on disk.

## The agents a gateway ships with

`gateway/agent_seeding.go` seeds these into every workspace at setup and
on each boot (`ensureDefaultAgents`), and refreshes their prompts and tools
when a release changes them:

| Agent | Tools | Role |
|---|---|---|
| `coder` | `coderToolPalette`: `bash`, `read_file`, `apply_patch`, `grep`, `jgrep`, `jread`, `glob`, `todo_write`, `todo_read`, `locate`, `skill`, `ask_user_question`, `notes`, `recall`, `web_search`, `web_fetch`, `mcp`, `cron`, `workflow`, `sessions_spawn` | the coding agent; works on the directory the window was launched from |
| `planner` | `read_file`, `grep`, `glob`, `todo_write`, `ask_user_question`, `task_flow` | reads, never edits; writes the spec the coder follows |
| `runner` | `bash`, `read_file` | runs a command and reports what it printed |
| `verifier` | `verify`, `read_file` | build, test, run; PASS or FAIL from the toolchain |
| `narrator` | none | turns the coder's tool stream into plain commentary |
| `chief` | `web_fetch`, `web_search`, `memory`, `channels`, `send_email` | the workspace's chief of staff |

```bash
memdoor agent list          # AGENT ID and NAME
memdoor agent show coder    # id, description, tools
```

Their prompts are in the seeding file; edit the row with `agent update` if
you want them different, and note that a release's refresh rewrites the
seeded prompt and tools of these six names.

## Creating one

`memdoor agent add` posts to `POST /api/agents` (`cmd/cli/cmd/agent.go`,
`handleCreateAgent`); it needs an admin token.

```bash
memdoor agent add code-reviewer \
  --description "Reads a diff and says what is wrong with it" \
  --personality "Precise, specific, never flattering" \
  --system-prompt "You review code. Name the file and line. Say why, not just what. Security first, then correctness, then style." \
  --tools read_file,grep,glob,bash,skill

memdoor agent update code-reviewer --tools read_file,grep,glob,bash,skill,web_search   # replaces the whole list
memdoor agent update code-reviewer --admin-only=false                                  # let everyone use it
memdoor agent delete code-reviewer
```

`agent update` also takes `--learning-enabled`, `--execution-type` and
`--endpoint`. Fields the CLI does not expose (`sandbox_scope`,
`avatar_emoji`, `remote_config`) go through `PUT /api/agents/<name>` with
`curl`; the shapes are in [Client API](CLIENT_API.md#agents).

## Tools

The tool list is an allow list matched by exact name
(`gateway/agent_runtime_tools.go filteredToolsFor`). Two rules on top of
it: an agent with more than one tool and no `memory` entry gets `memory`
added, and the entry `mcp` grants every tool of the MCP servers the person
has connected (`gateway/mcp_tools.go mcpPaletteEntry`). An agent with an
empty list falls back to the `chat` profile of `gateway/config/tool_policy.go`
(`DefaultToolProfile`: web, secrets, context, channels, email). The `group:`
names in that file are for config-file profiles and do not expand inside
an agent row; list the tools themselves.

The tools are defined in `tools/*.go` (`core_tools.go`, `advanced_tools.go`
and the files beside them) and registered on the runtime in
`gateway/agent_runtime_wiring.go`. The ones you will reach for:

| Kind | Tools |
|---|---|
| read | `read_file`, `jread`, `grep`, `jgrep`, `glob`, `locate`, `list_files` |
| write | `apply_patch`, `write_file`, `edit_file`, `search_replace` |
| run | `bash`, `verify`, `chrome_devtools` |
| plan and ask | `todo_write`, `todo_read`, `task_flow`, `ask_user_question`, `notes`, `recall` |
| extend | `skill`, `mcp`, `sessions_spawn`, `cron`, `workflow` |
| reach | `web_search`, `web_fetch`, `channels`, `send_email`, `get_secret` |

The coder's writes are rooted at the directory the window was launched
from; reads go anywhere. `bash` runs on the gateway's machine as the
gateway's user. Give an agent the least it needs: a reviewer reads and
runs tests, it does not need `apply_patch`.

A secret an agent needs (a token for an API it calls from `bash`) is set
with `POST /api/agents/<name>/secrets` and read with `get_secret`; a model
key is never an agent's secret, it belongs to a provider.

## The prompt

Two pieces make the system message. `pkg/prompts/agent_extra_prompt.go
BuildAgentExtraPrompt` is prepended before every request
(`pkg/message/service.go`): `# Personality` from the row, `# Agent
Instructions` from `system_prompt`, the workspace's examples, and the
workspace language. Two files on disk override the row without touching
it: `~/.memdoor/workspaces/<slug>/agents/<name>.prompt.md` replaces the
`system_prompt`, and `<name>.examples.md` beside it is appended
(`pkg/prompts/workspace_examples.go`). Then `gateway/prompts/system.go
SystemPromptBuilder.Build` adds the identity line (`You are agent
'<name>'`) and, for an agent that works on a codebase (the coder, the
planner: `agentWorksOnCodebase`), the project's own `AGENTS.md` from the
directory the turn works in (`gateway/project_instructions.go`) and nothing
else; a conversational agent gets the channel collaboration section
instead. Write the `system_prompt` for what is specific to this agent; the
tools describe themselves, and the project's rules arrive from its
`AGENTS.md`.

## Skills

A skill is a markdown file an agent loads by name with the `skill` tool,
or that you inject as the turn's task with `/skill:<name> [args]` in the
window. The agent needs `skill` in its tool list. Resolution is disk-first,
first hit wins (`gateway/skills/loader.go LoadSkills`,
[Skills Reference](../reference/SKILLS.md)):

1. `<project>/.agents/skills/` (travels with the repo; the coder can write here itself)
2. `./skills/` of the gateway's working directory (a dev checkout)
3. `~/.memdoor/workspace/skills/`
4. `~/.memdoor/skills/`, seeded from the binary at every boot; a file you edited is never overwritten

```bash
cat > ~/.memdoor/skills/review-checklist.md <<'EOF'
# review-checklist
Read the diff with `git diff`. For each file: injection, unchecked errors,
dead code. Answer as a list of file:line, what, why.
EOF
# in the window:  /skill:review-checklist
# or ask:         @code-reviewer use the review-checklist skill on this branch
```

The change is live on the next call; no rebuild, no restart. The built-in
library is `skills/` in the repo (`review`, `workflow`, `chrome`).

## The model

Agents have no model of their own. `memdoor providers` lists where models
come from (a company gateway, a vendor key, OpenRouter, `memdoor connect`),
`memdoor model` shows each agent's ladder, first rung first, and
`memdoor model search <name>` the catalogue with list prices
(`gateway/providers/`, [PROVIDERS.md](../features/PROVIDERS.md)).

`/model` in the window pins a model or a rung for the conversation and keeps
the pin for your later conversations (`gateway/persistent_pin.go`;
`/model auto` removes it). Shift+Tab cycles the reasoning effort; `auto`
lets the decision model pick per turn (`gateway/turn_effort.go`). The same
pin over HTTP is `POST /api/route`.

A remote agent is the one case where a row names a model: set
`execution_type` to `remote` with `remote_config.endpoint` (an
OpenAI-compatible `chat/completions` URL) and `remote_config.model`; the
gateway then calls that endpoint for this agent (`pkg/remote/executor.go`).

## Guards

Three settings bound what any agent may do, all read per call, none needing
a restart:

- `tool_guards` (workspace setting, `gateway/tool_guards.go`): a JSON array
  of `{"tool","pattern","message"}` rules; a call whose input matches the
  Go regexp is refused with the message, which the agent reads and acts on.
  `tool` is a name or `*`.

  ```bash
  TOKEN=$(memdoor auth token)
  curl -s -X PUT http://localhost:18789/api/workspace/settings \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d '{"tool_guards":"[{\"tool\":\"bash\",\"pattern\":\"\\\\bsudo\\\\b\",\"message\":\"sudo is off-limits here\"}]"}'
  ```

- `approve` set to `changes` (same endpoint), or `MEMDOOR_APPROVE=changes` in
  the gateway's environment (`gateway/approval.go`): every `bash`,
  `apply_patch`, `write_file`, `edit_file` and MCP call waits for a yes in
  the picker; no answer in five minutes is a no, so a turn nobody watches
  changes nothing. Reads never ask.

- `--mode plan` on `memdoor agent` (or `permission_mode: "plan"` on the
  message) makes one turn read-only: the agent researches and ends with
  `exit_plan_mode` instead of editing.

`sandbox_scope` on the row keeps a `user` agent out of other people's data;
`admin_only` keeps an agent you are still shaping away from everyone else.

## Testing from the CLI

Test from a scratch directory, never from a repo you care about: the coder
writes where the window was launched.

```bash
mkdir -p ~/memdoor-coder && cd ~/memdoor-coder

# a turn, from a script; the command returns when the turn is done
memdoor agent --message "Review main.go and list what you would change" \
  --channel test --agent-id code-reviewer

# what came back
memdoor messages --channel test --limit 5 --include-threads

# what really happened: tool calls, refusals, guard trips
memdoor logs query --regex "code-reviewer" --since 5m --limit 50
memdoor logs errors --since 5m

# a derailed conversation: undo turns, or start over
memdoor sessions rewind --channel test --turns 1
```

`memdoor agent` takes `--thinking enabled|disabled|auto` and `--mode
plan|acceptEdits`; piped stdin is attached to the message as a code block.
The reply's prose is a claim; the filesystem and the session's tool
results are the receipt. An agent that mentions another (`@verifier run
the tests`) wakes it in the same thread, bounded by the mention depth.

For a change a person sees in the window, drive the real TUI with tmux as
`.agents/skills/test-tui.md` describes.

## Next steps

- [CLI Reference](../reference/CLI.md): `agent add` and `agent update` flags, workspace settings, `logs`
- [Skills Reference](../reference/SKILLS.md): tiers, seeding, agents writing their own skills
- [Client API](CLIENT_API.md): the routes behind every command above
- [Workflows](../features/WORKFLOWS.md): several agents as a DAG, with gates
