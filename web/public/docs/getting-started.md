# Getting Started

Install, connect a key (OpenRouter, Anthropic, OpenAI, Gemini, DeepSeek, …),
run a first turn. About two minutes. One binary for macOS (Apple Silicon or
Intel), Linux and Windows. The three installers are tested on GitHub's
runners on every change to them.

## Install

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
```

One binary at `~/.local/bin/memdoor`. No sudo, no Docker, no Python. The
download is verified against the published SHA-256; a mismatch is refused.
Windows: `irm https://memdoor.ai/install.ps1 | iex`. Commands run in Git
Bash when installed, else PowerShell; the agent is told which.

## Your key

```bash
export OPENROUTER_API_KEY=sk-or-...    # or ANTHROPIC_API_KEY, OPENAI_API_KEY, DEEPSEEK_API_KEY, …
```

ChatGPT Plus or Pro, no key: `memdoor connect chatgpt` (or `/connect
chatgpt` in the window) opens Sign in with ChatGPT; the plan's allowance
answers the turns. `memdoor connect --remove chatgpt` signs out. A Claude
subscription cannot be used this way (Anthropic's terms); Claude needs an
API key from console.anthropic.com.

No key: run `memdoor`. It looks for a key on this machine (the shell under
any name, the project's `.env`, or, after a yes, keys other coding tools
saved), else signs you in with OpenRouter in the browser (free account
included) or ChatGPT. Same with a gateway on a box over ssh. An OpenRouter
account with no credit runs on a free model from OpenRouter's list: 50
requests a day, or 1,000 once $10 has been bought; `memdoor connect`
prints the limit. Free hosts train on what they are sent: a trial, not
private code.

Or `/connect` in the TUI and paste a key. No other account setup. Billed
at the provider's list prices. Put the export in the shell profile.

Local model: a running [janis](https://github.com/guregodevo/janis)
(`janis pull && janis serve`), Ollama, llama.cpp or LM Studio is a
[provider of its own](/docs/providers). Nothing leaves the machine:

```bash
memdoor connect local          # finds the runtime on this machine; no token
```

On janis the [decision model](/docs/no-babysitting#who-decides-the-decision-model) runs there too, no key.

## First turn

```bash
cd your-project
memdoor
```

`memdoor` opens the window (`memdoor tui` is the same). The first run
starts the local gateway in the background and creates the workspace, no
prompts. A key exported later is picked up on the next run.

The agent works on the launch directory: file tools inside it, commands
starting there. Not a sandbox: a shell command can reach other folders. A
task:

```
> the rate limit in throttle.go is hard-coded to 30 s. Make it configurable by
  environment variable, add a test, and run it.
```

It reads the file, writes the patch, runs the test, reads the output,
fixes what it broke. The footer names the model and its rung.

Keys: `Esc` interrupts. `ctrl+f`: the files panel, what this conversation
touched, newest first. `ctrl+o`: expand a tool frame. `@`: mention a file
or folder. macOS: an image on the clipboard pastes in.

## Or on a box, over ssh

Same first step on a machine you can ssh to with nothing installed: a VPS,
a Mac mini, a home server, Google Cloud Shell. Install memdoor here, then
name the box:

```bash
memdoor --gateway ssh://you@box                      # the window, on the box
memdoor --gateway ssh://you@box connect openrouter   # your key, kept on the box; asked for unseen
memdoor --gateway ssh://you@box run "say hi"         # one turn from a script or CI
```

No gateway on the box: the first command installs memdoor there over ssh
(binary in `~/.local/bin`, no sudo) and runs `memdoor gateway install`,
which registers a service (systemd --user or launchd; without a service
manager, a background process). The window opens on it, in the box's home
unless `--dir` names a project. Each step is printed. Runs live on the box
and continue with the laptop closed. `memdoor --gateway ssh://you@box
resume --last` or the phone reopens the conversation. The box's gateway
listens on its network, token required on every door
(`MEMDOOR_HOST=localhost` restricts it to the box). The ssh door reads the
box's login: no password typed. Rest, phone included: [The
gateway](/docs/gateway).

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

Shell: `memdoor resume` reopens a conversation; `memdoor model search
<name>` browses the catalogue; `memdoor meter` lists every turn with the
model served and its tokens.

## The other doors

Every run is a conversation on the gateway the first run started. Other
doors onto it:

```bash
memdoor run "fix the failing test"        # one turn, no window; exit 0 checked, 2 unverified, 3 it needed you
git diff | memdoor run "review this diff" # what is piped in is the input
memdoor workflow run release --wait       # a workflow from a shell or CI; the exit code is the run's outcome
memdoor cron add --id nightly --schedule "0 6 * * *" --agent coder --message "…"
memdoor resume --last                     # the same run, in a window; into a pipe, as text
memdoor remote --last                     # the link /remote gives, on stdout; the run on your phone
```

Two more: the editor (VS Code, Zed, JetBrains) and the page `/remote`
opens on the phone. A run started by CI can be opened live and answered.
Each door with its exit codes: [Headless, then attached](/docs/headless).

## The decision model

Judged reads, the per-turn toolbox and the stop instead of a loop. Free.
Enabled by an OpenRouter key or a decision key (`memdoor connect typesafe`)
beside any chat provider. With neither, tools return unjudged output.

## Where to next

- **[Headless, then attached](/docs/headless)** — scripts, CI, cron, resume.
- **[The gateway](/docs/gateway)** — the process on your machine every door opens onto.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[The TUI](/docs/tui)** — the screens and the keys.
- **[Remote control](/docs/remote)** — the same terminal, on your phone.
- **[In your editor](/docs/editor)** — the same coder in VS Code, Zed or JetBrains.
- **[Slash commands](/docs/slash-commands)** and **[CLI](/docs/cli)** — reference.
