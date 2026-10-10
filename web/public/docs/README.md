# Memdoor docs

A coding agent in your terminal, running on **your own key, any provider's**,
with a decision model in front of the chat model to cut what it reads and what
it costs. Measured over twenty paired runs: 49% fewer input tokens on a question
about the codebase, 26% fewer on an edit, every answer still correct. The
agent, the decision model and workflows are free on your own key; Pro is $10 a
month for remote control and workflow state kept on memdoor.ai.

```
curl -fsSL https://memdoor.ai/install.sh | bash
export OPEN_ROUTER_API_KEY=sk-or-...   # or any provider's key, or: memdoor connect
cd your-project && memdoor tui
```

## Start here

- **[What is Memdoor](/docs/what-is-memdoor)** — the short answer.
- **[Getting started](/docs/getting-started)** — install, key, first turn.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, real prices.

## Guides

- **[The TUI](/docs/tui)** — the screens, captured from a running session.
- **[Remote control](/docs/remote)** — `/remote`: the same terminal, on your phone.
- **[In your editor](/docs/editor)** — the coder in VS Code, Zed or JetBrains (`memdoor acp`).
- **[Models](/docs/models)** — picking and pinning a model.
- **[Agents and skills](/docs/agents-and-skills)** — who does the work, what tools they hold, how to extend them.

## Reference

- **[Slash commands](/docs/slash-commands)** — everything you can type in the TUI.
- **[Where your data goes](/docs/security)** — every host the binary can contact, and when; the decision model is a hosted model.
- **[CLI](/docs/cli)** — every command, grouped by what you're doing.
- **[Configuration](/docs/configuration)** — environment variables, file locations, workspace resolution.

## Background

- **[How it works](/docs/how-it-works)** — the architecture, and where the decision model sits.
- **[Why](/docs/why)** — the argument, in full.

## Common questions

**What do I need?** A terminal and a provider key (OpenRouter, Anthropic,
OpenAI, Gemini, DeepSeek, …), on an Apple Silicon Mac or
Linux: one binary. A Windows build exists but has not been run yet.

**Do you resell me tokens?** No. You hold the account at your provider and pay
their list price directly; nothing of your code passes through memdoor.ai and nothing
is added to your bill. The $10 is never for inference on your key; workflows are free.

**Does my code leave my machine?** Your files, sessions, memory and the output of
every command stay local. What leaves is the prompt and the excerpts a model must
read, with `data_collection: deny` on every request and training hosts excluded
(a model you pin whose id ends in `:free` is the exception: its hosts train, and
the pin says so).
Without an OpenRouter or decision key nothing is judged off-machine either.

**Is there a token or call limit?** No cap on tool calls. The decision model ends
a turn that has stopped making progress, which is what a cap was trying to do
badly.

**Does it work on a locked-down laptop?** Usually — a single binary with
no Docker, no Python, no package manager. See [Getting started](/docs/getting-started).
