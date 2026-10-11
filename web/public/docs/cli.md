# CLI

Every command is `memdoor <group> <command>`. `--help` works at every level and is the authority — this page is the map.

## Getting going

```
export OPEN_ROUTER_API_KEY=sk-or-...   # or ANTHROPIC_API_KEY, OPENAI_API_KEY, … — or memdoor connect
memdoor setup                     # workspace and account on this machine
memdoor tui                       # the coding agent, on the directory you launch from
memdoor resume                    # pick up a previous conversation where you left off
memdoor login you@example.com     # sign in; Pro turns on remote control
memdoor account status            # this Mac's engine user, and the memdoor.ai account
memdoor logout                    # sign out of memdoor.ai; the agent keeps working on your key
memdoor doctor                    # diagnose an install
memdoor upgrade                   # install a newer build (--check only compares)
memdoor version                   # this build's version, commit and date
memdoor uninstall                 # remove the binary (and ~/.memdoor unless --keep-data)
```

`setup` takes flags for scripts: `--workspace-name`, `--admin-email`, `--admin-password`.

## Providers

```
memdoor providers                 # each provider, connected or not; ● the one answering
memdoor providers --models groq   # one provider's own list: window · max out · $ per million
memdoor providers --audit         # where every window came from, and which are guesses
memdoor providers --refresh       # read every list again (a vendor shipped a model)
memdoor connect [kind]            # add one: gateway, anthropic, openai, gemini, xai, baseten, groq, deepseek, custom
memdoor connect groq --probe      # test the key, keep nothing
memdoor connect --remove <id>     # forget one you added
```

A key can be the value, the NAME of an environment variable, or `!command`;
it is probed (the list, then one tiny call) before anything is kept in
`~/.memdoor/providers.json`. See [Providers](/docs/providers).

## Models and what they cost

```
memdoor model                     # each agent's ladder, first rung first
memdoor model search <name>       # every connected provider's models, with real prices
memdoor model providers <id>      # an OpenRouter model's hosts: precision, context, uptime, price
memdoor model check [<id>]        # probe a model for what a coder turn needs (no argument: the ladder)
memdoor model check --saved       # what was probed already, spending nothing
memdoor meter                     # every model request: the model, its tokens and cost
memdoor meter --tail 20           # the last 20 requests instead of the summary
memdoor savings                   # what the decision model kept out of your bill this month
memdoor savings --month 2026-08   # a month you have already paid for
```


## Workflows

```
memdoor workflow                  # the project's workflows, and the runs here
memdoor workflow run <name>       # start .memdoor/workflows/<name>/ — a fresh run
memdoor workflow status <run-id>  # every task's state
memdoor workflow approve <run-id> <task>   # complete a gate (an external task)
memdoor workflow resume <run-id>  # continue a failed or stopped run; finished tasks are skipped
memdoor workflow stop <run-id>    # cancel a run
memdoor workflow history <name>   # every run of a workflow, newest first (survives a restart)
```

A workflow is a directory of task files; the coder writes it from a sentence.
In the window, `/workflow` draws the graph live. See [Workflows](/docs/workflows).

## MCP servers

```
memdoor mcp                       # every server and its state, for this project
memdoor mcp add <url|command|snippet>   # tested at once, signed in to if it asks
memdoor mcp search <words>        # the official MCP registry
memdoor mcp login|logout <name>   # OAuth in your browser, or forget it
memdoor mcp test <name>           # connect now and list its tools
memdoor mcp on|off|remove <name>
memdoor mcp trust                 # let this project's .mcp.json servers start
```

`/mcp` in the window does the same. See [MCP](/docs/mcp).

## Conversations

Each `memdoor tui` starts a fresh conversation; `resume` is how you deliberately go back to an old one.

```
memdoor conversations             # what you've worked on here, newest first
memdoor conversations --all       # every directory, not just this one
memdoor resume                    # list and choose
memdoor resume --last             # the one you were just in
memdoor resume 2                  # straight to number 2 from the list
memdoor resume 8488c86b           # the id the window printed when you left it
```

The window's last line, once you have said something in it, is the way back:
`Resume this conversation with: memdoor resume 8488c86b`.

A resumed session opens with its title in the header and its last exchanges in view, continuing the same agent session. Conversations are listed per directory; `--all` widens it.

```
memdoor sessions list             # the gateway's live sessions (a different question)
memdoor sessions rewind --turns 1 # undo a derailed turn
memdoor sessions export           # sessions → JSONL
```

`sessions` is not listed in `memdoor --help`; it runs when typed.

## Joining from another terminal

```
memdoor join 'https://memdoor.ai/r/<id>#k=…'   # a /remote link, in this terminal
```

The host's computer runs the agent, in the host's folder; this terminal is its
screen and keyboard. A watch-only link (`#v=`) shows the screen and sends
nothing. `Ctrl+]` leaves. See [Remote control](/docs/remote).

## Workspace and channels

```
memdoor workspace use <slug>      # pin this directory to a workspace
memdoor workspace which           # what would resolve here
memdoor channels list|create
memdoor messages -c <channel> [--include-threads]
memdoor run "<prompt>" [--json] [--yes] [--dir <path>]   # a headless turn: answer on stdout, exit code = the receipt
memdoor resume <id> --headless [--follow] | head          # the conversation as text, for a pipe
git diff | memdoor run "review this"                    # stdin is the input the prompt is about
memdoor workflow run <name> --wait                      # progress on stderr, results on stdout, exit = outcome
memdoor agent -m "<text>" -c <channel> -a <agent>   # message an agent from a script
memdoor agent list|show|add|update|delete
```

## Running the gateway

```
memdoor gateway                   # API + web UI on :18789
memdoor doctor                    # is it healthy
memdoor logs query --regex "<re>" --since 10m
memdoor logs errors --since 5m
memdoor logs stats                # how much is logged, and since when
memdoor users create --email <e> --password <p> [--admin]
memdoor auth login-direct --email <e> --password <p>   # the engine's sign-in, by hand
memdoor auth token                # the session token, for curl
```

`users`, `channels`, `messages`, `sessions`, `auth`, `cron` and the other
operator commands are not listed in `memdoor --help`; every one runs when
typed, and `docs/reference/CLI.md` lists them.

## Global flags

| Flag | Meaning |
|---|---|
| `-w, --workspace <slug>` | Override workspace discovery |
| `--gateway <url>` | Point at a different gateway (default `http://localhost:18789`) |
| `-v, --verbose` | Verbose output |

Workspace resolution order: `-w` → `$MEMDOOR_WORKSPACE` → a `.memdoor/workspace` file walked up from the current directory → `workspace_id` in `~/.memdoor/config.json`.

## Next

- **[Configuration](/docs/configuration)** — environment variables and file locations.
- **[Slash commands](/docs/slash-commands)** — the TUI equivalents.
