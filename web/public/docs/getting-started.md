# Getting Started

From nothing to a coding agent working in your project, on your own API key
(OpenRouter, Anthropic, OpenAI, Gemini, DeepSeek, …), in about two minutes. Everything here happens in a
terminal, on an Apple Silicon Mac, Linux or Windows, one binary. The three installers run on GitHub's
runners on every change to them.

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

No money on the key yet? A free OpenRouter account is enough to try it: in the
window, `/model nvidia/nemotron-3-super-120b-a12b:free` pins a free model
(50 requests a day; 1,000 once $10 has ever been bought). Free hosts train on
what they are sent, and Memdoor says so at the pin: a trial, not private code.

No key at hand? Skip it: type `/connect` inside the TUI and paste one. This
is the whole account setup: your key, your bill, at the provider's list
prices. Put the export in your shell profile so it is there next time.

## First turn

```bash
cd your-project
memdoor tui
```

The first run sets this machine up by itself: it starts the local gateway in
the background and makes the workspace, with nothing to answer. A key you
export later reaches it the next time you run `memdoor tui`.

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
