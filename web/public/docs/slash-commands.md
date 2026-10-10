# Slash commands

Everything you can type in the TUI. The `/` key opens a dropdown that filters as you type; `/help` prints the list in the session. Screens for most of these are in **[The TUI](/docs/tui)**; `/remote` has its own page, **[Remote control](/docs/remote)**.

## Models and cost

| Command | What it does |
|---|---|
| `/model` | Which model is answering this conversation, its rung on the agent's ladder, and why |
| `/model <rung>` | Pin a rung — `/model 3` for the top one. A pin stays: every new conversation starts on it until `/model auto` |
| `/model <vendor/name> [price\|throughput\|latency\|default] [order a,b]` | Pin any catalogue model, and say how to order its hosts; it stays the same way |
| `/model auto` | Hand the choice back to the ladder |
| `/model-search [text]` | Alone: a picker of your providers — enter opens one's models, enter again pins. With text: every connected provider's matches, with prices; enter pins (an OpenRouter row opens its hosts first) |
| `/connect [kind]` | Add a provider — the company AI gateway, Anthropic, OpenAI, Gemini, any OpenAI-compatible endpoint: pick the kind, the base URL prefilled, the token typed but never shown, probed before it is kept |
| `/usage` | What this workspace used this month: turns, tokens, cost on your key |

## Working

| Command | What it does |
|---|---|
| `/dir <path>` | Re-root the session on another folder. Bare `/dir` shows the current one |
| `/copy [N\|all]` | Copy the last exchange (or the last N, or all) to the clipboard |
| `/remote` | Open this conversation on your phone: prints a link and a QR code. The page is the TUI itself, end-to-end encrypted |
| `/remote view` | A watch-only link: it sees the same screen and cannot type |
| `/share` | A read-only link to this conversation as it is now: end-to-end encrypted, secrets removed |
| `/unshare` | Delete this conversation's shared links. They open nothing |
| `/remote off` | Revoke both links. The old ones open nothing |
| `/update` | Install the published build and reopen this conversation on it |
| `/skill:<name> [args]` | Run a named skill as this turn's task. Tab-completes from disk |
| `/init` | Write the project's `AGENTS.md` — build, test, conventions — so every turn starts from it |
| `/handoff [request]` | The model summarizes goals, decisions, progress and next steps, then a clean session starts from the summary. With a request, it is sent to the new session |
| `/go` | Hand the last plan to the coder to execute. `/run` and `/approve` do the same |
| `/context` | How full the context window is, and when compaction triggers |
| `/compact [focus]` | Summarize the conversation now, rather than waiting for the budget. The focus says what to keep in mind |
| `/agents` | The agents in this workspace and how many tools each holds |
| `/files [filter]` | The project's files with the ones this conversation read or changed first, newest first, each with what happened to it; `ctrl+f` opens it too. Type to filter, enter previews, `ctrl+d` shows the diff, `ctrl+e` opens the file in `$EDITOR`, tab puts `@path` in the prompt, esc closes |
| `/mcp` | Every connected MCP server and its state; `a` adds one, arrows move, enter signs in or tests |
| `/mcp add <…>` | Add a server from a URL, a command line or a `.mcp.json` snippet; connects at once and signs in if asked |
| `/mcp search <words>` | Search the official MCP registry; a result is added as the entry it describes |
| `/mcp login <name>` / `/mcp logout <name>` | Sign in to (or forget the sign-in for) an HTTP server that asks for it |
| `/mcp test <name>` | Connect now and list the server's tools |
| `/mcp on\|off <name>` | Turn a server on or off |
| `/workflow` | The project's workflows and runs; enter runs one or opens a run's graph; `a` approves, `c` continues, `s s` stops |
| `/workflow:<name>` | Run a workflow straight from the dropdown, which completes it with what the project has |
| `/workflow run <name>` / `resume <run-id>` / `approve <task>` / `stop <run-id>` | The same from the line |
| `/mcp remove <name>` | Delete a server from its file |
| `/mcp trust` | Let this project's `.mcp.json` servers start |
| `/<server>:<prompt> [args]` | Run a connected server's prompt as your message; arguments in the declared order or as `name=value` |
| `/doctor` | Health checks on the session and its connection |

## Session

| Command | What it does |
|---|---|
| `/fresh` | Start your last request over in a clean session: the agent forgets what it read, keeps the task. Stops a running turn first |
| `/clear` | Wipe the agent's memory of this conversation; its next turn starts from nothing. The messages stay for `memdoor resume` |
| `/new` | Start a fresh session |
| `/exit` | Leave the TUI. Same as `Ctrl+C` |
| `/help` | Print the whole list in the session |

Your own commands: any `.md` file in `<project>/.agents/commands/` becomes a slash command named after the file (`review-pr.md` → `/review-pr`); `.claude/commands/` still works as a legacy fallback. A name in both is offered once. They appear in the `/` dropdown and `/help`.

## Keys

| Key | What it does |
|---|---|
| `/` | Open the command list (filters as you type) |
| `Esc` | Interrupt the running turn |
| `Enter` | Send. Typing during a turn queues instead |
| `Ctrl+O` | Expand tool frames to full output |
| `PgUp` / `PgDn`, wheel | Scroll the transcript |
| `End` | Jump back to the latest message |
| `1`–`9`, arrows | Answer the question picker |
| `Ctrl+C` | Quit |
| `Shift+Tab` | Cycle the reasoning effort (auto, low, medium, high) |
| `@` | Mention a file or folder in the prompt — completes as you type, and the file is read before anything else |
| `Ctrl+V` | Paste an image from the clipboard: it is saved and inserted as an `@` mention |
| `Alt+Enter` (or `Ctrl+J`) | Insert a newline in the prompt |

The footer shows only what applies right now: `esc interrupt · ctrl+o expand` while a turn runs, `/ commands · ctrl+o expand · ctrl+c quit` when it doesn't.
