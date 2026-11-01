# Overview

Memdoor is a coding agent. It runs as a gateway on your machine, on your key.
Clients: terminal, shell, CI, cron, IDE, phone. Multi-step jobs run as
workflows, the deterministic harness. A decision model in front of the chat
model cuts input and cost.

## The coding agent

`cd your-project && memdoor`. Reads files, writes patches, runs the build,
fixes the errors. File tools and commands are rooted in the launch directory.
Not a sandbox: a shell command can reach other folders. `@` mentions a file or
folder. `memdoor resume` reopens a conversation.

→ **[What is Memdoor](/docs/what-is-memdoor)**, **[Getting started](/docs/getting-started)**

## The decision model

Jev answers typed questions about a turn with a calibrated probability in
under half a second. Judged reads: search hits, file sections and log lines
filtered against the task. Per-call toolbox: tool schemas narrowed to the
request. Stall detection: a turn that stops making progress is ended.
44 paired runs: 74% fewer input tokens on a code question, no change on an
edit or on small fixes, 22 of 22 passed both ways.

→ **[What is Memdoor](/docs/what-is-memdoor)**, **[Why Memdoor](/docs/why)**

## Your key and the models

Export a provider key (`OPENROUTER_API_KEY`, `ANTHROPIC_API_KEY`,
`DEEPSEEK_API_KEY`, …) or run `memdoor connect`. Requests go from your
machine to the provider at list price. Each agent has a model ladder,
cheapest rung first. The footer names the model that answered and why.
`/model` searches tool-capable models with prices, pins one, orders its hosts.

→ **[Your key and the models](/docs/your-key)**, **[Models](/docs/models)**

## Web search

`web_search` returns an answer with numbered sources. `web_fetch` reads one
page. Both run on an OpenRouter key. An answer without a source is refused.
The `chrome` skill sends searches to `web_search` and uses the browser for
pages that need one.

## The terminal

Replies stream. `Esc` interrupts. Typing during a step queues. Each tool call
is a frame; `Ctrl+O` expands it. Scrolling up stays put; a bar reports what
landed below; `End` returns to live. After a gateway restart the TUI
reconnects and sends what was typed meanwhile.

→ **[The TUI](/docs/tui)**, **[Slash commands](/docs/slash-commands)**

## Remote control

`/remote` prints a link and a QR code. The page is the TUI, served by your
gateway, so it works with the window closed. Screen and keystrokes are
end-to-end encrypted with a key held only in the link; the relay forwards
ciphertext. `/remote off` revokes the link. Free on your gateway. The
memdoor.ai relay, for a gateway kept to localhost, is the hosted part.

→ **[Remote control](/docs/remote)**

## Workflows

Describe the steps; the coder writes the workflow. A graph of tasks, each
done when its check passes: a file exists, a command exits 0, or the task's
answer. Independent tasks run in parallel. The graph is drawn live in the
window. Task kinds: agent turn, shell command, one model call. Approval is a
task nobody runs. A failed task shows `✗` and the reason; the run resumes
there. Engine: mario, an open DAG runner.

→ **[Workflows](/docs/workflows)**

## In CI

One step in a GitHub job: review a pull request, write its tests or its
description, explain a failed run, triage an issue, fix a failing suite,
write release notes. The job passes or fails on a check. Runs on your key,
or with no key on a model inside the runner. The runner model is a first
pass; an agent on your key runs the code.

→ **[In CI](/docs/ci)**

## Sessions

Persist, compact when large, rewind when a turn derails.

## Agents, tools, skills

A workspace ships coder, planner, verifier and others. Each is tools and a
prompt, all configuration. Skills are markdown, loaded from disk first.
An edit applies on the next call.

→ **[Agents and skills](/docs/agents-and-skills)**

## Operations

Queryable logs: `memdoor logs query --regex … --since 10m`. Per-turn ledger
of model and cost: `memdoor meter`. One binary, no sudo, runs where Docker
and package managers are blocked.

→ **[CLI](/docs/cli)**, **[Configuration](/docs/configuration)**
