# Memdoor docs

A coding agent that runs from Terminal, a shell, CI, cron, your IDE or
phone, on your gateway, on your key, any provider. A multi-step job runs
as a workflow, the deterministic harness: each step checked by its output,
the run stops only where you decide. A decision model in front of the chat
model cuts input and cost: 44 paired runs, 74% fewer input tokens on a
codebase question, no change on edits, 22 of 22 passed each way. The agent,
the decision model, workflows and every door onto your gateway are open
source (Apache-2.0) and free.

```
curl -fsSL https://memdoor.ai/install.sh | bash
export OPENROUTER_API_KEY=sk-or-...    # or any provider's key, or: memdoor connect
cd your-project && memdoor
```

## Start here

- **[What is Memdoor](/docs/what-is-memdoor)** — the short answer.
- **[Getting started](/docs/getting-started)** — install, key, first turn.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, real prices.

## Open-weight models

- **[No key: run on an open-weight model](/docs/open-weight)** — Ollama, llama.cpp, vLLM, LM Studio or janis on your machine; `memdoor connect local`; the window, the model, the team on your GPU.
- **[Team solve: the result](/docs/team-solve)** — five Qwen3.8-27B agents at 9.8 of 10 against Fable 5's 8.0, every run published.

## Guides

- **[The TUI](/docs/tui)** — the screens, captured from a running session.
- **[Remote control](/docs/remote)** — `/remote`: the same terminal, on your phone.
- **[In your editor](/docs/editor)** — the coder in VS Code, Zed or JetBrains (`memdoor acp`).
- **[Headless, then attached](/docs/headless)** — scheduled checks, workflows from CI, editor sessions: every run is a conversation you can come back into.
- **[In CI](/docs/ci)** — reviews, tests, failure explanations, issue triage and release notes with `guregodevo/memdoor-action`, on your key or on a model inside the runner.
- **[Boundaries](/docs/boundaries)** — what the agent may touch: writes in the project, `memdoor guards` rules, approval mode, gates.
- **[Models](/docs/models)** — picking and pinning a model.
- **[Agents and skills](/docs/agents-and-skills)** — who does the work, what tools they hold, how to extend them.

## Reference

- **[Slash commands](/docs/slash-commands)** — everything you can type in the TUI.
- **[Where your data goes](/docs/security)** — the hosts the binary can contact, and when; the decision model is a hosted model.
- **[CLI](/docs/cli)** — the commands, grouped by what you're doing.
- **[Configuration](/docs/configuration)** — environment variables, file locations, workspace resolution.

## Background

- **[How it works](/docs/how-it-works)** — the architecture, and where the decision model sits.
- **[Why](/docs/why)** — the argument, in full.

## Common questions

**What do I need?** A terminal and a provider key (OpenRouter, Anthropic,
OpenAI, Gemini, DeepSeek, …). Mac, Linux or Windows. One binary.

**Do you resell me tokens?** No. You hold the provider account and pay list
price. No code passes through memdoor.ai.

**Does my code leave my machine?** Files, sessions, memory and command output
stay local. The prompt and the excerpts a model must read leave, with
`data_collection: deny` on every request and training hosts excluded.
Exception: a `:free` model, pinned or the one an OpenRouter account with no
credit answers on. Its hosts train; Memdoor says so at the pin or when the
key is connected. Without an OpenRouter or decision key nothing is judged
off-machine either.

**Is there a token or call limit?** No cap on tool calls. The decision model
ends a turn that has stopped making progress.

**Does it work on a locked-down laptop?** One binary, no Docker, no Python,
no package manager. See [Getting started](/docs/getting-started).
