# Agents and skills

## The agents

A workspace ships with a small team. Each has its own tools, prompt, and sessions.

| Agent | What it does |
|---|---|
| `coder` | The coding agent — the read, edit, build, verify loop |
| `planner` | Produces a plan for a task; `/go` hands it to the coder |
| `chief` | Workspace chief of staff — general tasks and delegation |
| `verifier` | Checks work against what was asked |
| `runner` | Executes commands on behalf of another agent |
| `narrator` | Announces what's happening in a channel |

`memdoor tui` talks to the coder; start a message with `@planner` (or another agent's name) to send it there instead. `memdoor agent list` shows what's in your workspace. Agents can mention each other (`@coder`, `@chief`) and reply in threads. A scheduled check's answer, or a spawned subagent's report, wakes the conversation that asked for it.

Agents are configuration, not code: tool allowlists, model, temperature, and prompt live in the database per agent, and `memdoor agent update` changes them.

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
| `ask_user_question` | Interrupt with a numbered picker when a decision is genuinely needed |
| `see` | Look at an image or frames of a video with a vision model — a screenshot, a thumbnail, what is on screen |
| `notes` | The coder's memory for a project: a file it reads at the start of a turn and appends to as it works |
| `recall` | Return a tool output word for word after the transcript has been compacted |
| `jgrep`, `jread` | The judged versions of `grep` and `read_file`: the decision model scores every match or section against the task and returns the ones that count |
| `web_search`, `web_fetch` | Look something up on the web and read a page in full, on your own OpenRouter key; an answer with no source is refused |

Everything runs on your machine, inside the folder the TUI was launched from. What leaves it is the prompt and the excerpts the model has to read.

**Blocking a tool.** The `tool_guards` workspace setting takes regex rules per tool, so you can forbid specific invocations (a destructive command shape, a path you don't want touched) without removing the tool.

## Skills

A skill is a markdown file of instructions the agent loads when it's relevant — a checklist for reviews, the conventions in your repo.

**They resolve from disk first**, in this order:

2. `<project>/.agents/skills/`
3. `~/.memdoor/skills/` (seeded on first boot)
4. the built-in defaults

Edit a file and the next call uses it — no rebuild, no restart. A skill you drop into `.agents/skills/` appears in the TUI's `/` dropdown immediately.

**The built-in skills** — seeded into `~/.memdoor/skills/` on first boot and shipped inside the binary:

| Skill | What it is |
|---|---|
| `chrome` | Reading pages that need a real browser, driven through the `chrome` CLI |
| `review` | Read and understand an existing repo before changing it |
| `workflow` | Build a workflow's task files from what the person asked for, then run it |

**Running one deliberately.** `/skill:review src/auth.go` makes that skill the turn's task, rather than hoping the model decides to load it.

**Agents write their own.** When an agent works out a procedure worth keeping, it can save it as a skill and use it next time. The files land in the same directories you edit by hand.

## What it remembers

The agent's notes are working memory for one conversation: what it worked
out, which take was the good one. They end with the conversation. Something
you want kept for every later conversation in the project, say so:

```
> keep this as a rule: run the tests with -race, and never add a dependency
```

It is written as a rule into the project's `AGENTS.md`, or into
`.memdoor/AGENTS.md` (git-ignored) when the project has none, and read at
every turn. A rule for every project goes in `~/.memdoor/workspace/AGENTS.md`,
which every conversation reads. Memdoor does not write such files on its own
after a turn: a rule is yours, said once.

## Next

- **[The TUI](/docs/tui)** — `/agents`, `/skill:`, and the question picker, in screens.
