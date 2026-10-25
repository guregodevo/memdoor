# CLI

Every command is `memdoor <group> <command>`. `--help` works at every level and is the authority — this page is the map.

## Getting going

```
export OPEN_ROUTER_API_KEY=sk-or-...   # or ANTHROPIC_API_KEY, OPENAI_API_KEY, … — or memdoor connect
cd your-project && memdoor        # the first run sets the machine up: gateway, workspace, account, then the window
memdoor tui                       # the coding agent, on the directory you launch from (the bare `memdoor` opens it too)
memdoor resume                    # pick up a previous conversation where you left off
memdoor login you@example.com     # sign in to memdoor.ai: workflow state there, and the relay for a gateway kept to localhost
memdoor account status            # this Mac's engine user, and the memdoor.ai account
memdoor logout                    # sign out of memdoor.ai; the agent keeps working on your key
memdoor doctor                    # diagnose an install
memdoor upgrade                   # install a newer build (--check only compares)
memdoor version                   # this build's version, commit and date
memdoor uninstall                 # remove the binary (and ~/.memdoor unless --keep-data)
```

`memdoor setup` does the first run's setup from a script: `--workspace-name`, `--admin-email`, `--admin-password`, `--workspace-dir`, `--non-interactive`; `doctor` takes `--workspace-dir` too.

## Providers

```
memdoor providers                 # each provider, connected or not; ● the one answering
memdoor providers --models groq   # one provider's own list: window · max out · $ per million
memdoor providers --audit         # where every window came from, and which are guesses
memdoor providers --refresh       # read every list again (a vendor shipped a model)
memdoor connect [kind]            # add one: gateway, chatgpt, openrouter, anthropic, openai, gemini, xai, baseten, groq, deepseek, typesafe, custom
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
memdoor meter --models            # this month per model, largest first (what /usage shows in the window)
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

## Guards

```
memdoor guards                    # the rules
memdoor guards add --tool bash --pattern '\bgit push\b' --message 'pushing is mine'
memdoor guards add --tool '*' --pattern '\.env\b' --message 'secrets stay out of reach'
memdoor guards remove 2           # the number `memdoor guards` shows
memdoor guards clear
```

A rule names a tool (`*` for any) and a regex matched against the call's input; a matching call is refused with the message, on every agent and every turn. Setting them takes an admin's sign-in. See [Boundaries](/docs/boundaries).

## Your editor

```
memdoor acp                       # the coder over the Agent Client Protocol, on stdin/stdout, for Zed, JetBrains or VS Code's ACP client to spawn
```

Each editor session is a conversation on this machine's gateway, in the folder the editor has open. See [Memdoor in your editor](/docs/editor).

## Peers (experimental)

```
memdoor peers                                   # each peer: answering or not, its agents
memdoor peers add box <gateway> --dir <path>    # name a gateway you are signed in to; @box:coder reaches it
memdoor peers remove box
```

`@<peer>:<agent> …` in a window, a run or a message sends that turn to the agent on the peer, in the directory named there; the answer comes back as the reply.

## Conversations

Each `memdoor tui` starts a fresh conversation; `resume` is how you deliberately go back to an old one.

```
memdoor resume                    # what you've worked on here, newest first, and which one to open
memdoor resume --all              # every directory, not just this one
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

## The phone door from a script

```bash
memdoor remote <id>                    # the link on stdout; the gateway holds the conversation on it
memdoor remote --last                  # this directory's most recent conversation
memdoor remote                         # at a terminal: the list, and a question (which one)
memdoor remote --view <id>             # the watch-only link
memdoor remote off <id>                # revoke
memdoor --gateway <box> remote <id>    # a conversation on that box: the box holds the link
```

What `/remote` does in the window, as a command: the id is the one `memdoor
run` and `memdoor resume` print; stdout is the link alone. See
[Remote control](/docs/remote).

## Workspace and scripts

```
memdoor workspace use <slug>      # pin this directory to a workspace
memdoor workspace which           # what would resolve here
memdoor workspace list            # every workspace you have
memdoor run "<prompt>" [--json] [--yes] [--dir <path>]   # a headless turn: answer on stdout, exit code = the receipt
memdoor resume <id> | head                                # into a pipe: the conversation as text (a window at a terminal)
git diff | memdoor run "review this"                    # stdin is the input the prompt is about
memdoor workflow run <name> --wait                      # progress on stderr, results on stdout, exit = outcome
memdoor agent -m "<text>" -c <channel> -a <agent>   # message an agent from a script
memdoor agent list|show|add|update|delete
```

## Running the gateway

```
memdoor gateway                   # the gateway in the foreground on :18789 (the API, the /r/ remote pages, the docs); install|status|uninstall run it as a service
memdoor doctor                    # is it healthy
memdoor logs query --regex "<re>" --since 10m
memdoor logs errors --since 5m
memdoor logs stats                # how much is logged, and since when
memdoor users create --email <e> --password <p> [--admin]
memdoor auth login-direct --email <e> --password <p>   # the engine's sign-in, by hand
memdoor auth token                # the session token, for curl
```

`users`, `channels`, `messages`, `sessions`, `auth` and the other operator
commands are not listed in `memdoor --help`; every one runs when typed, and
`docs/reference/CLI.md` lists them. `cron` is listed: see [Scheduled checks](/docs/cron).

## Global flags

| Flag | Meaning |
|---|---|
| `-w, --workspace <slug>` | Override workspace discovery |
| `--gateway <address>` | Point at a different gateway (or `MEMDOOR_GATEWAY`): a URL, `host:port`, a bare host or IP, or `ssh://[user@]box[:port]` for a gateway reached through ssh (default `http://localhost:18789`) |
| `--dir <path>` | The project directory: here by default; with `--gateway`, the directory on that machine |
| `-v, --verbose` | Verbose output |

Workspace resolution order: `-w` → `$MEMDOOR_WORKSPACE` → a `.memdoor/workspace` file walked up from the current directory → `workspace_id` in `~/.memdoor/config.json`.

## Next

- **[Configuration](/docs/configuration)** — environment variables and file locations.
- **[Slash commands](/docs/slash-commands)** — the TUI equivalents.
