# Getting Started with Memdoor

From zero to a first coding session in about two minutes. Everything in this guide runs in the terminal. The served
version of this page is [memdoor.ai/docs/getting-started](https://memdoor.ai/docs/getting-started)
(source: [`web/public/docs/getting-started.md`](web/public/docs/getting-started.md)).

## What you are installing

Memdoor is a coding agent in your terminal with a **decision model in front of
the chat model** to cut
what it reads. Jev judges which grep hits, file sections and log lines a
task needs, which rung of the model ladder a conversation is on, and
when a turn has stopped making progress; the chat model sees a fraction of
the text and answers the same ([`docs/features/DECIDE.md`](docs/features/DECIDE.md)).
The models come from your own key — OpenRouter, Anthropic, OpenAI, Gemini,
Groq, xAI, DeepSeek, Baseten or your company's AI gateway — at the provider's
list price, shown when you choose. The decision model and workflows run on
the same key; all of it is free.

## Prerequisites

| Platform | Installer |
|---|---|
| macOS, Apple Silicon (`darwin/arm64`) | `install.sh` |
| Linux (`linux/amd64`, `linux/arm64`) | `install.sh` |
| Windows 64-bit (`windows/amd64`) | `install.ps1` |

Intel Macs have no download — run Linux on that hardware. No Docker, Python, or package manager is needed on any
platform.

---

## 1. Install

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
```

On Windows: `irm https://memdoor.ai/install.ps1 | iex`.

The installer ([`scripts/install.sh`](scripts/install.sh)) detects your OS and
arch, downloads the matching binary from `memdoor.ai/dl/`, verifies its SHA-256
against the published `SHA256SUMS` (it refuses to install if the manifest or
hash is missing or wrong), and installs to **`~/.local/bin/memdoor` — no sudo,
by default**. It prints a `PATH` hint if `~/.local/bin` isn't on yours. On
macOS it also ad-hoc signs the binary and clears the quarantine attribute.

Other modes, first match wins: `MEMDOOR_PREFIX=/some/dir` (explicit location),
`--system` (`/usr/local/bin`, requires sudo), or an existing user-writable
install on `PATH` (upgraded in place). Re-running the installer upgrades; it
stops a running gateway first and tells you to restart it.

## 2. Your key

```bash
export OPEN_ROUTER_API_KEY=sk-or-...   # or ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, DEEPSEEK_API_KEY, …
```

Any provider's key works; `/connect` inside the window adds one too. Nothing
is downloaded: every model runs on that key, at the provider's list price.

On a ChatGPT Plus or Pro plan? Skip the key: `memdoor connect chatgpt` (or
`/connect chatgpt` in the window) opens Sign in with ChatGPT, and your plan's
allowance answers Memdoor's turns; `memdoor connect --remove chatgpt` signs
out. A Claude subscription cannot be used this way (Anthropic's terms forbid
it); Claude needs an API key from console.anthropic.com.

No money on a key yet? A free OpenRouter account is enough to try it: in the
window, `/model nvidia/nemotron-3-super-120b-a12b:free` pins a free model
(50 requests a day; 1,000 once $10 has ever been bought). Free hosts train on
what they are sent, and Memdoor says so at the pin: a trial, not private code.

## 3. Start coding

```bash
cd your-project
memdoor tui
```

The agent works on the directory you launched from: its file tools stay inside
it and its commands start there (not a sandbox: a shell command can still reach
other folders). Type a task; watch the tool frames as it works.
`Esc` interrupts, `ctrl+o` expands tool frames,
`/model` shows which model answers and pins one, `/usage` shows what the
workspace used this month, `/update` installs the published build. `memdoor resume` reopens
a previous conversation.

Pinning the workspace is optional but convenient: `memdoor workspace use
<slug>` writes `./.memdoor/workspace`, and every command auto-discovers it by
walking up from the current directory (like `git` or `direnv`), so `-w`
becomes optional. `memdoor workspace which` confirms what would resolve.

## 4. The model ladder

Every agent has a ladder of models on your key, cheapest first, and the
decision model runs on the same key:

```bash
memdoor model                    # each agent's ladder, rung 1 first, list prices per million tokens
memdoor model search glm         # the catalogue: tool-capable models on OpenRouter, newest first
memdoor model providers z-ai/glm-5.3   # who serves it: precision, uptime, price, tool calls
```

In the window the footer says which model answered and why (`rung 1/3 ·
first rung · held for this session`). `/model 2` pins a rung for the
conversation, `/model z-ai/glm-5.3 price` pins any catalogue model with its
hosts sorted cheapest first (`throughput`, `latency`, `default`, or `order
host1,host2` for your own order), `/model auto` hands control back, and
`/model-search <text>` finds a model to pin without leaving the chat.

What keeps the bill down is not the ladder alone. Every tool that could
flood the context asks the decision model first (`jread`, `jgrep`, `jlogs`),
so the model reads what the task needs; there is no cap on
tool calls per turn, because the decision model stops a turn that has
stopped making progress; a provider's rate limit is waited out rather than
failed. `memdoor meter` lists every turn with the
model served and its tokens; `/usage` sums the month. The seat is flat: the
prices you see when choosing are the hosts' list prices, not a bill.

## Optional: tab completion

```bash
echo 'source <(memdoor completion zsh)' >> ~/.zshrc      # zsh
echo 'source <(memdoor completion bash)' >> ~/.bashrc    # bash (needs bash-completion)
memdoor completion fish > ~/.config/fish/completions/memdoor.fish
```

## Power user

```bash
memdoor agent --message "explain failures" --channel general --agent-id chief   # message a specific agent
go test ./... 2>&1 | memdoor agent -m "Explain failures" -c general -a chief    # pipe output into an agent
memdoor messages --channel general --limit 20 --include-threads                  # read the conversation
memdoor logs query --regex "route|rate" --since 5m                            # structured logs
memdoor logs errors --since 5m
memdoor sessions rewind --channel <name> --turns 1                               # undo a derailed turn
memdoor upgrade                                                                  # newer build via install.sh
```

## Troubleshooting

**Port already in use** (something else is bound to 18789): stop it, then
re-run any memdoor command — the gateway auto-starts when needed.

**`agent` returns "agent not found":** use a real agent ID (`chief`,
`coder`, `planner`); `memdoor agent list` shows them.

**A turn slows down mid-burst:** your provider is rate-limiting it; the turn
waits out the provider's Retry-After rather than failing
(`memdoor logs query --regex "rate|429" --since 5m`). A turn that stops with "the last steps made no progress" was
ended by the decision model, not a cap: say what to try instead.

## Next steps

- [README](README.md) — what Memdoor is, in one page
- [CLI Reference](docs/reference/CLI.md) — every command, every flag
- [Decisions](docs/features/DECIDE.md) — what Jev judges, what it saved, how to turn it off
- [Architecture](docs/reference/ARCHITECTURE.md) · [Skills](docs/reference/SKILLS.md)
- Served docs: [The TUI](https://memdoor.ai/docs/tui) · [Why](https://memdoor.ai/docs/why)
