# Getting Started

Install, connect a key (OpenRouter, Anthropic, OpenAI, Gemini, DeepSeek, …)
and run a first turn, in a terminal, in about two minutes. One binary for
macOS (Apple Silicon or Intel), Linux and Windows. The three installers are
tested on GitHub's runners on every change to them.

## Install

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
```

One binary at `~/.local/bin/memdoor`. No sudo, no Docker, no Python. The
installer verifies the download against the published SHA-256 and refuses to
install if it does not match. On Windows: `irm https://memdoor.ai/install.ps1 |
iex`. The agent runs its commands in Git Bash when it is installed and otherwise
in PowerShell. It is told which.

## Your key

```bash
export OPENROUTER_API_KEY=sk-or-...    # or ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, …
```

On a ChatGPT Plus or Pro plan you can skip the key. `memdoor connect chatgpt` (or
`/connect chatgpt` in the window) opens Sign in with ChatGPT and your plan's
allowance answers Memdoor's turns. `memdoor connect --remove chatgpt` signs
out. A Claude subscription cannot be used this way because Anthropic's terms forbid
it. Claude needs an API key from console.anthropic.com.

With no key, run `memdoor`. It finds a key on this machine: in your
shell under any name, in the project's `.env`, or after a yes among the keys other coding
tools saved. Otherwise it signs you in with OpenRouter in the browser (a free account
included) or with ChatGPT. That works with a gateway on this machine or on a box
over ssh. An OpenRouter account with no credit runs on a free model picked
from OpenRouter's list: 50 requests a day, or 1,000 once $10 has been
bought. `memdoor connect` prints the limit. Free hosts train on what they are
sent, so use them for a trial and not for private code.

Or type `/connect` in the TUI and paste a key. There is no other account
setup: your key, billed at the provider's list prices. Put the export in your
shell profile.

With no key and a model on this machine, a running [janis](https://github.com/guregodevo/janis)
(`janis pull && janis serve`), Ollama, llama.cpp or LM Studio is a
[provider of its own](/docs/providers). Nothing leaves the machine:

```bash
memdoor connect local          # finds the runtime on this machine; no token
```

On janis the [decision model](/docs/no-babysitting#who-decides-the-decision-model) runs there too, with no key.

## First turn

```bash
cd your-project
memdoor
```

`memdoor` opens the window (`memdoor tui` is the same). The first run starts
the local gateway in the background and creates the workspace without
prompting. A key exported later is picked up the next time you run `memdoor`.

The agent works on the directory you launched it from. Its file tools stay inside it and its commands start there. It is not a sandbox: a shell command can still reach other folders. Type a task:

```
> the rate limit in throttle.go is hard-coded to 30 s. Make it configurable by
  environment variable, add a test, and run it.
```

It reads the file, writes the patch, runs the test, reads the output and fixes
what it broke. The footer names the model that answered and the rung
of the ladder it came from.

`Esc` interrupts a running turn. `ctrl+f` opens the files panel: what this
conversation touched, newest first. `ctrl+o` expands a tool frame to its full
output. `@` mentions a file or folder. On macOS an image on the clipboard
pastes in.

## Or on a box, over ssh

The same first step works on a machine you can ssh to with nothing
installed there: a VPS, a Mac mini, a home server, Google Cloud Shell.
Install memdoor here, then name the box:

```bash
memdoor --gateway ssh://you@box                      # the window, on the box
memdoor --gateway ssh://you@box connect openrouter   # your key, kept on the box; asked for unseen
memdoor --gateway ssh://you@box run "say hi"         # one turn from a script or CI
```

When no gateway answers on the box, the first command installs memdoor
there over the same ssh (the installer puts the binary in `~/.local/bin`, no
sudo) and runs `memdoor gateway install`, which registers a service (systemd
--user or launchd; without a service manager it starts the gateway in the
background). The window then opens on it, in the box's home unless `--dir`
names a project there. Each step is printed. Runs live on the box and
continue with the laptop closed. `memdoor --gateway ssh://you@box resume
--last` or your phone reopens the conversation. The box's gateway listens on
its network with the account token required on every door
(`MEMDOOR_HOST=localhost` restricts it to the box). The ssh door reads the
box's login, so no password is typed. [The gateway](/docs/gateway) has the
rest, the phone included.

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

From the shell: `memdoor resume` reopens a previous conversation. `memdoor model
search <name>` browses the catalogue. `memdoor meter` lists every turn
with the model served and its tokens.

## The other doors

Every run is a conversation on the gateway the first run started. Each of
these is another door onto it:

```bash
memdoor run "fix the failing test"        # one turn, no window; exit 0 checked, 2 unverified, 3 it needed you
git diff | memdoor run "review this diff" # what is piped in is the input
memdoor workflow run release --wait       # a workflow from a shell or CI; the exit code is the run's outcome
memdoor cron add --id nightly --schedule "0 6 * * *" --agent coder --message "…"
memdoor resume --last                     # the same run, in a window; into a pipe, as text
memdoor remote --last                     # the link /remote gives, on stdout; the run on your phone
```

Your editor (VS Code, Zed, JetBrains) and the page `/remote` opens on your
phone are two more. A run started by CI can be opened live and answered.
[Headless, then attached](/docs/headless) lists each door with its exit
codes.

## The decision model

Judged reads, the per-turn toolbox and the stop instead of a loop come from
the decision model. It is free and is enabled by an OpenRouter key or a
decision key (`memdoor connect typesafe`) beside any chat provider. With
neither, tools return unjudged output.

## Where to next

- **[Headless, then attached](/docs/headless)** — scripts, CI, cron, resume.
- **[The gateway](/docs/gateway)** — the process on your machine every door opens onto.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[The TUI](/docs/tui)** — the screens and the keys.
- **[Remote control](/docs/remote)** — the same terminal, on your phone.
- **[In your editor](/docs/editor)** — the same coder in VS Code, Zed or JetBrains.
- **[Slash commands](/docs/slash-commands)** and **[CLI](/docs/cli)** — reference.
