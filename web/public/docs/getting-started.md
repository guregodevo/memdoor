# Getting Started

From nothing to a coding agent working in your project, on your own API key
(OpenRouter, Anthropic, OpenAI, Gemini, DeepSeek, …), in about two minutes. Everything here happens in a
terminal, on a Mac (Apple Silicon or Intel), Linux or Windows, one binary. The three installers run on GitHub's
runners on every change to them.

## Install

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
```

One binary at `~/.local/bin/memdoor`, no sudo, no Docker, no Python. The
installer verifies the download against the published SHA-256 and refuses to
install if it does not match. On Windows: `irm https://memdoor.ai/install.ps1 |
iex`; the agent runs its commands in Git Bash when it is installed, otherwise
in PowerShell, and it is told which.

## Your key

```bash
export OPEN_ROUTER_API_KEY=sk-or-...   # or ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, …
```

On a ChatGPT Plus or Pro plan? Skip the key: `memdoor connect chatgpt` (or
`/connect chatgpt` in the window) opens Sign in with ChatGPT, and your plan's
allowance answers Memdoor's turns; `memdoor connect --remove chatgpt` signs
out. A Claude subscription cannot be used this way (Anthropic's terms forbid
it); Claude needs an API key from console.anthropic.com.

No money on a key yet? A free OpenRouter account is enough to try it: in the
window, `/model nvidia/nemotron-3-super-120b-a12b:free` pins a free model
(50 requests a day; 1,000 once $10 has ever been bought). Free hosts train on
what they are sent, and Memdoor says so at the pin: a trial, not private code.

No key at hand? Skip it: type `/connect` inside the TUI and paste one. This
is the whole account setup: your key, your bill, at the provider's list
prices. Put the export in your shell profile so it is there next time.

No key at all, a model on this machine: a running [janis](https://github.com/guregodevo/janis)
(`janis pull && janis serve`), Ollama, llama.cpp or LM Studio is a
[provider of its own](/docs/providers), and nothing leaves the machine:

```bash
memdoor connect local          # finds the runtime on this machine; no token
```

On janis the [decision model](/docs/decide) runs there too, with no key.

## First turn

```bash
cd your-project
memdoor
```

The bare command opens the window (`memdoor tui` still works). The first run
sets this machine up by itself: it starts the local gateway in
the background and makes the workspace, with nothing to answer. A key you
export later reaches it the next time you run `memdoor`.

The agent works on the directory you launched it from: its file tools stay inside it and its commands start there — not a sandbox: a shell command can still reach other folders. Type a task:

```
> the rate limit in throttle.go is hard-coded to 30 s. Make it configurable by
  environment variable, add a test, and run it.
```

It reads the file, writes the patch, runs the test, reads the output and fixes
what it broke. Watch the footer: it names the model that answered and the rung
of the ladder it came from.

`Esc` interrupts a running turn. `ctrl+f` opens the files panel: what this
conversation touched, newest first. `ctrl+o` expands a tool frame to its full
output. `@` mentions a file or folder, and on macOS an image on the clipboard
pastes in.

## Or on a box, over ssh

The same first step works on a machine you can ssh to, with nothing
installed there: a VPS, a Mac mini, a home server, Google Cloud Shell.
Install memdoor here, then name the box:

```bash
memdoor --gateway ssh://you@box                      # the window, on the box
memdoor --gateway ssh://you@box connect openrouter   # your key, kept on the box; asked for unseen
memdoor --gateway ssh://you@box run "say hi"         # one turn from a script or CI
```

The first command finds no gateway on the box and sets it up over the same
ssh: memdoor.ai's installer puts the binary in `~/.local/bin` there (no
sudo), `memdoor gateway install` makes it a service (systemd --user,
launchd; where there is no service manager it starts in the background),
and the window opens on it, in the box's home unless `--dir` names a
project there. Each step is printed as it happens. The runs then live on
the box: close the laptop and the work goes on; `memdoor --gateway
ssh://you@box resume --last` or your phone opens the same conversation.
The box's gateway listens on its own network, every door gated by the
account token (`MEMDOOR_HOST=localhost` keeps it to the box), and the box's
own login is read over ssh, so no password is typed. [The gateway](/docs/gateway) has
the rest, the phone included.

## The commands worth knowing first

| Command | What it does |
|---|---|
| `/model` | Which model is answering, the ladder, and how to pin one |
| `/model-search <text>` | Find a tool-capable model with its real price, and pin it |
| `/files` or `ctrl+f` | The project's files, the ones this conversation read or changed first, with what happened to each; enter previews, `ctrl+d` shows the diff |
| `/usage` | What this workspace used this month: turns, tokens, cost on your key |
| `/remote` | Open this conversation on your phone, as the terminal itself |
| `/update` | Install the published build and reopen this conversation on it |
| `/help` | Everything else |

From the shell: `memdoor resume` reopens a previous conversation, `memdoor model
search <name>` browses the catalogue, and `memdoor meter` lists every turn
with the model served and its tokens.

## The other doors

The window is one door onto the gateway the first run started. Every run is a
conversation on it, and each of these opens the same one:

```bash
memdoor run "fix the failing test"        # one turn, no window; exit 0 checked, 2 unverified, 3 it needed you
git diff | memdoor run "review this diff" # what is piped in is the input
memdoor workflow run release --wait       # a workflow from a shell or CI; the exit code is the run's outcome
memdoor cron add --id nightly --schedule "0 6 * * *" --agent coder --message "…"
memdoor resume --last                     # the same run, in a window; into a pipe, as text
memdoor remote --last                     # the link /remote gives, on stdout; the run on your phone
```

Your editor (VS Code, Zed, JetBrains) and the page `/remote` opens on your
phone are two more. The run a CI job started is the one you open live and
answer. [Headless, then attached](/docs/headless) walks through every door
with the exit codes.

## The decision model

The judged reads, the per-turn toolbox and the stop-instead-of-loop all come
from a decision model, and they are what make the same work cost less. It is
free and runs on your own key: an OpenRouter key turns it on, and so does a
decision key (`memdoor connect typesafe`) next to any chat provider. With
neither, nothing is judged and every tool returns its unjudged output.

## Where to next

- **[Headless, then attached](/docs/headless)** — one gateway, every door: scripts, CI, cron, resume.
- **[The gateway](/docs/gateway)** — the process on your machine every door opens onto.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[The TUI](/docs/tui)** — the screens and the keys.
- **[Remote control](/docs/remote)** — the same terminal, on your phone.
- **[In your editor](/docs/editor)** — the same coder in VS Code, Zed or JetBrains.
- **[Slash commands](/docs/slash-commands)** and **[CLI](/docs/cli)** — reference.
