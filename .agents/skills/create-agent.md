# Skill: Create Agent

Create Memdoor agents using the CLI. Use this skill when asked to create, configure, or set up new agents.

## CLI Commands

```bash
# Create an agent (tools are set on the same command)
memdoor agent add <name> --description "…" --personality "…" --tools bash,read_file

# Change one later: --tools replaces the whole list
memdoor agent update <name> --tools bash,read_file,apply_patch

# List, show, delete
memdoor agent list
memdoor agent show <name>
memdoor agent delete <name>
```

`agent add` also takes `--system-prompt`. Credentials are not an agent
flag: a key belongs to a provider (`memdoor connect`) or to the gateway's
environment, one place for every agent.

## LLM model

Every agent runs on the gateway's engine. There is no `--model` or
`--provider` flag on `agent add` / `agent update`. The provider comes from
the gateway's keys:

```bash
memdoor providers                # each provider, connected or not
memdoor connect anthropic        # add a provider key
memdoor model                    # each agent's ladder, first rung first
```

`/model` in the window pins a model for one conversation.

## Tool profiles (gateway/config/tool_policy.go)

An agent with no `--tools` gets the **chat** profile — that is the default
(`DefaultToolProfile`): web search/fetch, secrets, context, channels, email.

Profiles, when you set `profile:` through the config file rather than `--tools`:

| profile | what it grants |
|---|---|
| `minimal` | `session_status` only |
| `chat` | web, secrets, context, channels, `send_email` — **the default** |
| `default` | read-only files, sessions, secrets, context, web, `ask_user_question`, invites, email, channels |
| `coding` | all files, bash, sessions, spawn, secrets, context, web, introspection, chrome, `ask_user_question` |
| `messaging` | sessions + context |
| `full` | everything |

`--tools` is an allow list, and it accepts a group as one entry
(`ExpandToolGroups`):

| group | tools |
|---|---|
| `group:fs_read` | read_file, list_files, glob, grep |
| `group:fs_write` | write_file, edit_file, search_replace |
| `group:fs` | both of the above |
| `group:runtime` | bash |
| `group:sessions` | sessions_list, sessions_history, sessions_send, session_status |
| `group:sessions_spawn` | sessions_spawn |
| `group:memory` | memory (never granted by a profile — list it) |
| `group:secrets` | get_secret |
| `group:web` | web_fetch, web_search |
| `group:ui` | chrome_devtools |
| `group:automation` | cron, channels, send_invite |
| `group:productivity` | ask_user_question, todo_write |
| `group:context` | context, status |
| `group:introspection` | agents_list, gateway, agent_log, execution_flow |

Everything the coder turns use lives in the registry (`tools/*.go`):
read_file, jread, jgrep, locate, glob, grep, list_files, apply_patch,
write_file, edit_file, search_replace, bash, todo_write, todo_read, notes,
recall, memory, skill, task, task_flow, exit_plan_mode, sessions_*,
session_status, web_fetch, web_search, chrome_devtools, logs_query,
report_bug, verify, channels, cron, send_email, send_invite,
context, status, agent_log, agents_list, gateway, ask_user_question,
jlogs, notebook_edit.

## Sandbox Scopes

- `user` (default, most secure) - agent accesses only the user's private data
- `channel` - agent accesses channel-shared data + user data
- `workspace` - agent accesses all workspace data

## Examples

### Research agent (default tools are fine)
```bash
memdoor agent add researcher \
  --description "Searches the web and summarizes findings" \
  --personality "Thorough and analytical researcher"
```

### Coding agent (needs write tools + bash)
```bash
memdoor agent add coder \
  --description "Writes and edits code" \
  --personality "Precise and pragmatic developer" \
  --tools group:fs,group:runtime,apply_patch,group:sessions_spawn
```

### Browser automation agent (needs chrome)
```bash
memdoor agent add spider \
  --description "Automates browser tasks" \
  --personality "Efficient web automation specialist" \
  --tools web_fetch,web_search
```

## Guidelines

- Always ask what the agent should do before creating it
- Choose the minimum tools needed for the task
- The chat profile is good for most agents — add files/bash for coding agents
- Use descriptive names (kebab-case): `code-reviewer`, `doc-writer`, `data-analyst`
- Set a personality that matches the agent's purpose
- Credentials come from providers (`memdoor connect`) or the gateway's own
  environment — never paste a key into a system prompt
