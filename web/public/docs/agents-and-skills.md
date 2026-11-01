# Agents and skills

## The agents

A workspace ships with a team. Each has its tools, prompt and sessions.

| Agent | What it does |
|---|---|
| `coder` | The coding agent — the read, edit, build, verify loop, with `cron`, `workflow`, MCP and `sessions_spawn` for subtasks |
| `planner` | Reads and searches, asks, and produces a plan (`read_file`, `grep`, `glob`, `todo_write`, `ask_user_question`, `task_flow`); `/go` hands it to the coder |
| `chief` | A general assistant: `web_search`, `web_fetch`, `memory`, `send_email` |
| `verifier` | Runs the build, the tests or the program through `verify` and reports what the toolchain said (`verify`, `read_file`) |
| `runner` | Runs a command and reports its real output; it does not write code (`bash`, `read_file`) |
| `narrator` | Narrates the coder's tool activity for a person; no tools |

`memdoor tui` talks to the coder. `@planner` (or another agent's name) at
the start of a message sends it there. `memdoor agent list` shows what is
on your gateway. Agents mention each other (`@coder`, `@chief`); the
mentioned agent's answer comes back as the reply. A scheduled check's answer
or a spawned subagent's report wakes the conversation that asked.

Agents are configuration: tool allowlist, prompt, description and
personality, in the database per agent, changed with `memdoor agent
update`. No agent has a model of its own. Each runs on its ladder or on the
model pinned with `/model`.

```
memdoor agent list
memdoor agent -m "summarize today's changes" -c general -a chief
memdoor agent update coder --tools bash,read_file,apply_patch
```

## Tools

The coding agent's:

| Tool | What it does |
|---|---|
| `bash` | Run a command in the project directory |
| `read_file` | Read a file |
| `apply_patch` | Edit or create files — the build runs after every patch, and a failure comes back to the agent |
| `grep`, `glob`, `locate` | Find code by content, by name, by symbol |
| `todo_write`, `todo_read` | Track a multi-step plan visibly in the TUI |
| `skill` | Load a skill's instructions on demand |
| `ask_user_question` | Interrupt with a numbered picker when a decision is needed |
| `notes` | The coder's memory for a project: a file it reads at the start of a turn and appends to as it works |
| `recall` | Return a tool output word for word after the transcript has been compacted |
| `jgrep`, `jread` | The judged versions of `grep` and `read_file`: the decision model scores every match or section against the task and returns the ones that count |
| `web_search`, `web_fetch` | Look something up on the web and read a page in full, on an OpenRouter key; an answer with no source is refused |
| `mcp`, `cron`, `workflow`, `sessions_spawn` | Call a connected MCP server's tool, schedule its own re-check, run or approve a workflow, run a subtask in a session of its own |

Everything runs on your machine inside the folder the TUI was launched
from. The prompt and the excerpts the model reads leave it; nothing else.

Blocking a tool: the `tool_guards` workspace setting takes regex rules per
tool. A rule refuses specific invocations (a destructive command shape, a
path) without removing the tool.

## Skills

A skill is a markdown file of instructions the agent loads when relevant:
a review checklist, the conventions of a repo.

Resolved from disk first, in this order:

1. `<project>/.agents/skills/`
2. `~/.memdoor/workspace/skills/`
3. `~/.memdoor/skills/` (seeded on first boot)
4. the built-in defaults

An edit applies on the next call. No rebuild, no restart. A skill dropped
into `.agents/skills/` appears in the TUI's `/` dropdown at once.

Built-in skills, seeded into `~/.memdoor/skills/` on first boot and shipped
in the binary:

| Skill | What it is |
|---|---|
| `chrome` | Reading pages that need a real browser, driven through the `chrome` CLI |
| `review` | Read and understand an existing repo before changing it |
| `workflow` | Build a workflow's task files from what the person asked for, then run it |

Running one deliberately: `/skill:review src/auth.go` makes that skill the
turn's task.

Agents write their own: a procedure worth keeping is saved as a skill in the
same directories, for the next time.

## What it remembers

The agent's notes are working memory for one conversation and end with it.
A rule for every later conversation in the project:

```
> keep this as a rule: run the tests with -race, and never add a dependency
```

Written into the project's `AGENTS.md` and read at every turn. With no
`AGENTS.md` it goes into `.memdoor/AGENTS.md`, git-ignored. A rule for every
project: `~/.memdoor/workspace/AGENTS.md`. Memdoor writes such files only
when told.

## Next

- **[The TUI](/docs/tui)** — `/agents`, `/skill:`, and the question picker, in screens.
