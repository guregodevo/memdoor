# Overview

Everything Memdoor does and where each part is documented. Memdoor is a coding
agent on your gateway, on your key at any provider. It runs from Terminal,
a shell, CI, cron, your IDE or phone. A multi-step job runs as a workflow,
the deterministic harness: each step checked by its output, a gate where
you want one, resumed where it failed. A decision model in front of the chat model cuts what it reads and what
it costs.

## The coding agent

`cd your-project && memdoor`. It reads your files, writes patches, runs the
build, reads the errors and fixes what it broke. It works in the directory you
launched it from: its file tools stay inside it and its commands start there.
It is not a sandbox, since a shell command can still reach other folders. `@`
mentions files and folders. `memdoor resume` returns to an earlier
conversation.

→ **[What is Memdoor](/docs/what-is-memdoor)**, **[Getting started](/docs/getting-started)**

## The decision model

A decision model (Jev) answers typed questions about the turn with a calibrated
probability in under half a second. The harness acts on the answer. Judged
reads return the search hits, file sections and log lines the task needs. The
tool schemas sent per call are narrowed to the kind of work the request needs.
A turn that has stopped making progress is ended.
Measured over 44 paired runs: 74% fewer input tokens on a code question, no
change on an edit or on small fixes, 22 of 22 passed each way.

→ **[What is Memdoor](/docs/what-is-memdoor)**, **[Why Memdoor](/docs/why)**

## Your key and the models

Export a provider key (`OPENROUTER_API_KEY`, `ANTHROPIC_API_KEY`,
`DEEPSEEK_API_KEY`, …) or run `memdoor connect`. Turns go straight from your
machine to that provider at its list price. Each agent has a ladder with the
cheapest rung first. The footer names the model that answered and why. `/model`
searches the catalogue of tool-capable models with real prices, pins one and
orders its hosts.

→ **[Your key and the models](/docs/your-key)**, **[Models](/docs/models)**

## Web search

`web_search` looks something up on the web and answers with numbered sources. `web_fetch` reads one of those pages in full. The coder has both. They run on an OpenRouter key and no other account. An answer with no source is refused, so the agent cannot pass memory off as a search. The `chrome` skill sends searches to `web_search` too and keeps the browser for pages that need one.

## The terminal

Replies stream as they generate. `Esc` interrupts. Typing during a step queues.
Every tool call renders as a frame you can expand with `Ctrl+O`. Scrolling up
never pulls you back down: a bar says what landed below and `End` returns to
live. If the gateway restarts the TUI reconnects and sends what you typed while
it was away.

→ **[The TUI](/docs/tui)**, **[Slash commands](/docs/slash-commands)**

## Remote control

`/remote` gives the conversation a link and a QR code. The page it opens on your
phone is the TUI, run by your gateway, so it works with the window
closed. Watch a turn, answer its question, stop it, start the next one. The
screen and your keystrokes are end-to-end encrypted with a key that is only in
the link. The relay, your gateway or memdoor.ai, forwards what it cannot read.
`/remote off` revokes the link. On your gateway it is free. Only the
memdoor.ai relay for a gateway kept to localhost is part of the hosted gateway.

→ **[Remote control](/docs/remote)**

## Workflows

Say what you want done in steps and the coder builds a workflow: a graph of
tasks, each proven done by something you can check (a file that must exist,
a command that must exit 0, or the task's own answer). Independent tasks run
in parallel and the graph is drawn in your window as it goes. Three kinds of
task: an agent turn, a shell command, one model call. Your approval is a task
nobody runs. A failed task reads `✗` with the reason and the run resumes from
there. The engine is mario, an open DAG runner.

→ **[Workflows](/docs/workflows)**

## In CI

One step in a GitHub job reviews every pull request, writes its tests or its
description, explains a failed run, triages a new issue, fixes a failing suite
or writes release notes. The job passes or fails on a check. The model runs on
your key, or with no key on a model inside the runner. The runner model is a free first pass for the obvious. An agent on your key
runs the code to prove what it finds.

→ **[In CI](/docs/ci)**

## Sessions

Sessions persist, compact when they grow, and rewind when a turn derails.

## Agents, tools, skills

A workspace ships with a team: coder, planner, verifier and others, each
with its tools and prompt, all of it configuration. Skills are markdown
instructions resolved from disk first. Editing one takes effect on the next
call.

→ **[Agents and skills](/docs/agents-and-skills)**

## Operations

Structured, queryable logs (`memdoor logs query --regex … --since 10m`). A
per-turn ledger of what each model served and cost (`memdoor meter`). A
single binary that installs without sudo and runs where Docker and
package managers are blocked.

→ **[CLI](/docs/cli)**, **[Configuration](/docs/configuration)**
