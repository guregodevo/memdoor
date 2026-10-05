# Getting Started

From nothing to a coding agent working in your project, on your own OpenRouter
key, in about two minutes. Everything here happens in a terminal, on an Apple Silicon Mac or Linux, one
binary. A Windows build exists but has not been run yet.

## Install

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
```

One binary at `~/.local/bin/memdoor`, no sudo, no Docker, no Python. The
installer verifies the download against the published SHA-256 and refuses to
install if it does not match. On Windows: `irm https://memdoor.ai/install.ps1 |
iex`.

## Your key

```bash
export OPEN_ROUTER_API_KEY=sk-or-...   # or ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, …
```

Or `memdoor connect`, which asks for a provider and a key, probes it and keeps
it. This is the whole account setup: your key, your bill, at the provider's
list prices. Put it in your shell profile so it is there next time.

## Set up

```bash
memdoor setup
```

Interactive, once: a name for the workspace, an email and password for this
machine. It starts the local gateway in the background. For
scripts, pass everything as flags: `memdoor setup --workspace-name 'My Project'
--admin-email you@example.com --admin-password '…'`.

## First turn

```bash
cd your-project
memdoor tui
```

The agent works on the directory you launched it from: its file tools stay inside it and its commands start there — not a sandbox: a shell command can still reach other folders. Type a task:

```
> the rate limit in throttle.go is hard-coded to 30 s. Make it configurable by
  environment variable, add a test, and run it.
```

It reads the file, writes the patch, runs the test, reads the output and fixes
what it broke. Watch the footer: it names the model that answered and the rung
of the ladder it came from.

`Esc` interrupts a running turn. `ctrl+o` expands a tool frame to its full
output. `@` mentions a file or folder, and on macOS an image on the clipboard
pastes in.

## The commands worth knowing first

| Command | What it does |
|---|---|
| `/model` | Which model is answering, the ladder, and how to pin one |
| `/model-search <text>` | Find a tool-capable model with its real price, and pin it |
| `/usage` | What this workspace used this month: turns, tokens, cost on your key |
| `/remote` | Open this conversation on your phone, as the terminal itself |
| `/update` | Install the published build and reopen this conversation on it |
| `/help` | Everything else |

From the shell: `memdoor resume` reopens a previous conversation, `memdoor model
search <name>` browses the catalogue, and `memdoor meter` lists every turn
with the model served and its tokens.

## The decision model

The judged reads, the per-turn toolbox and the stop-instead-of-loop all come
from a decision model, and they are what make the same work cost less. It is
free and runs on your own key: an OpenRouter key turns it on, and so does a
decision key (`memdoor connect typesafe`) next to any chat provider. With
neither, nothing is judged and every tool returns its unjudged output.

## Where to next

- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[The TUI](/docs/tui)** — the screens and the keys.
- **[Remote control](/docs/remote)** — the same terminal, on your phone.
- **[Slash commands](/docs/slash-commands)** and **[CLI](/docs/cli)** — reference.
