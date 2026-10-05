# MCP servers

The coder can use tools from MCP servers the person connects: their issue
tracker, database, docs, any service with an MCP server. Added in the
TUI with `/mcp`, or from a shell with `memdoor mcp`.

Decided 2026-10-01 (Greg), after reading omp's MCP support
(`~/Dev/oh-my-pi/packages/coding-agent/src/mcp`): its auth detection and
sign-in screen are copied, its rough edges are not (transport asked by
hand, failures saved silently, a thin status list, a repo's servers
started without asking, a fixed callback port).

## Adding one

`/mcp` opens the panel; `a` (or `/mcp add`) opens one box. Paste:

- a URL — an HTTP server: `https://mcp.linear.app/mcp`
- a command line — a local server: `npx -y @modelcontextprotocol/server-github`
  (quotes respected; leading `NAME=value` pairs are its environment)
- a `.mcp.json` snippet — one server or several, as a README shows them

Or search the official MCP registry (`registry.modelcontextprotocol.io`,
public, no sign-in): `s` in the panel, `/mcp search <words>`, or
`memdoor mcp search <words>`. A result becomes the entry a person would
write — a hosted server's URL, else `npx -y pkg@version`,
`uvx pkg==version` or `docker run -i --rm image` — with everything it needs
from the person as `${VAR}` (a required environment variable, a header
such as `Bearer ${API_KEY}`), so nothing secret is written and a missing
value is named when it connects. Only each server's latest version is
shown.

The transport follows from what was pasted; nobody is asked. A name is
suggested (`mcp.linear.app` → `linear`, `@modelcontextprotocol/server-everything`
→ `everything`). Then: save for **this project** (`.mcp.json`, shared
with the repo) or **every project** (`~/.memdoor/mcp.json`, just you).
It connects at once and says what it got: its tools, or the reason it
failed and what to do. A server that asks for sign-in starts the sign-in.

**A server that cannot run is not kept.** A command that is not on the
machine, a URL that answers something other than MCP — the box opens again
with your text and the reason, and nothing is written, so no repository ends
up carrying a server that fails for everyone who clones it. The exception is
a server waiting for a variable (`${API_KEY}` unset): there the entry is
right and the environment is not, so it is saved and the panel names what to
set. A file left holding no servers is deleted with the last one, unless it
holds something else of yours.

## Signing in

An HTTP server that answers 401 needs sign-in. Memdoor follows the MCP
authorization spec: the server's challenge names its protected-resource
metadata (RFC 9728), that names its authorization server (RFC 8414 or
OpenID; a server from before RFC 9728 is its own), Memdoor registers
itself as a public client (RFC 7591), and the person approves in a
browser with PKCE (S256) and the resource indicator (RFC 8707).

On screen: the browser opens, the link is in the transcript (clickable)
and copied to the clipboard, a timer runs. The code comes back to a
listener on 127.0.0.1 (a random port). When the browser is on another
machine, paste the address the browser ends on; Esc cancels.

Tokens live in `~/.memdoor/mcp-tokens.json`, each encrypted with the
master key, keyed by the server's URL — never in a config file, so a
committed `.mcp.json` carries no credential. A token is refreshed five
minutes before it expires. `/mcp logout <name>` forgets it. A server whose
config sends its own `Authorization` header (an API key, `${VAR}` from
the environment) is not signed in to.

## The files

`.mcp.json` is Claude Code's format, so an existing one works as is:

```json
{"mcpServers": {
  "linear": {"type": "http", "url": "https://mcp.linear.app/mcp"},
  "github": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"],
             "env": {"GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}"}}
}}
```

`${VAR}` and `${VAR:-default}` expand from the gateway's environment; a
missing one is named. A name in both files is the project's. Memdoor's
writes keep every key and field it does not model. `"disabled": true`
turns a server off. A server on the old HTTP+SSE transport (MCP
2024-11-05) works too: a URL whose POST is refused with 404 or 405 falls
back to it, as the spec says, and `"type": "sse"` goes straight to it.

## Trust

A project's `.mcp.json` can start commands, and a repo may have just been
cloned: its servers start only after the person's yes. At start, the TUI
opens the panel on what the file would run; `y` (or `memdoor mcp trust`)
allows it. The trust covers the servers as they are — a changed command
asks again; turning one on or off does not. Adding a server to a project
through Memdoor trusts the file only if it was trusted or held nothing
else, so it never approves a repo's own servers by the way.

## How the coder gets the tools

- The gateway runs the servers (`pkg/mcp` Manager): started on a turn that
  needs them (an 8 s window; a slow one joins on a later turn), kept while
  used, closed after 30 idle minutes, reconnected when their config
  changes, a failed one left alone for a minute.
- Tools are named `mcp__<server>__<tool>` (Claude Code's convention), kept
  within 64 characters. Results come back as text: images described,
  resources inlined, 50,000 characters at most; an error result is an
  error.
- Resources: when a connected server offers them, the coder gets
  `mcp__list_resources` and `mcp__read_resource` (Claude Code's two
  tools), not one tool per resource.
- An agent whose palette carries `mcp` gets them; the coder's does.
- Cost: every tool's schema travels on every request. With the decision
  model, tool routing offers an `mcp` family only on a turn whose project
  has connected servers, its description naming them, so the schemas go
  only on turns that need them. Without it, they go on every turn.

## Prompts

A server's prompts are slash commands in the TUI: `/<server>:<prompt>`,
listed in the `/` dropdown. They are re-read whenever the panel re-reads the
servers, so a server turned on, tested, added or signed in to brings its
prompts with it. A prompt whose server is not connected says which server,
rather than "unknown command". Arguments go in the order the prompt declares
them or as `name=value`; a missing required one shows the usage. The
filled prompt is sent as your message.

## Commands

| TUI | Shell | |
|---|---|---|
| `/mcp` | `memdoor mcp` | every server and its state |
| `/mcp add <…>` | `memdoor mcp add <…> [--name n] [--everywhere]` | add, test, sign in if asked |
| `/mcp login <name>` | `memdoor mcp login <name>` | sign in |
| `/mcp logout <name>` | `memdoor mcp logout <name>` | forget the sign-in |
| `/mcp test <name>` | `memdoor mcp test <name>` | connect now, list the tools |
| `/mcp on\|off <name>` | `memdoor mcp on\|off <name>` | turn on or off |
| `/mcp remove <name>` | `memdoor mcp remove <name>` | delete it from its file |
| `/mcp trust` | `memdoor mcp trust` | let this project's servers start |
| `/mcp search <words>` | `memdoor mcp search <words>` | find servers in the MCP registry |
| (pick a result) | `memdoor mcp add --registry <name>` | add one from the registry |
| `/<server>:<prompt> [args]` | | run a server's prompt as your message |

In the panel: ↑↓ move, enter signs in or tests, `a` add, `s` search the
registry, space on/off, `t` test, `o` sign out, `x` twice removes, `y`
trusts, esc closes.

## Not yet

Resource subscriptions and resource templates; prompt argument
completion; a tool list that changes while connected
(`notifications/tools/list_changed`) is picked up on the next connection.

## Receipts (2026-10-01)

- DeepWiki over HTTP: the coder called `mcp__deepwiki__read_wiki_structure`
  and listed a repository's 14 documentation pages, in 17 s.
- `@modelcontextprotocol/server-everything` over stdio: 13 tools, `echo`.
- Linear and Notion: needs sign-in detected, discovery, client
  registration and the authorize URL; the panel's sign-in screen.
- A real sign-in end to end, approved in the browser on Greg's account:
  "It works" (2026-10-01).
- A refused add: `nosuchbinary-xyz`, `cat` ("did not answer the MCP
  handshake — is it an MCP server?"), `https://example.com/mcp` ("connection
  reset by peer"), a truncated snippet ("that is not valid JSON") — each one
  reopened the box with the reason and wrote no file; a server whose
  `${NO_SUCH_VAR_XYZ}` is unset was kept, as it should be.
- server-everything in SSE mode, its URL given as plain HTTP: fell back,
  13 tools, 7 resources, 4 prompts. In the TUI, `/everything:args-prompt
  Paris TX` sent "What's weather in Paris, TX?"; the coder listed and read
  a resource with the two resource tools.
- Registry: `search github` in the panel, a hosted result added with
  `Bearer ${SMITHERY_API_KEY}` and the missing variable named.

Code: `pkg/mcp` (client, config, manager, OAuth, parse), `gateway/mcp_tools.go`,
`gateway/mcp_handlers.go` (`/api/mcp`), `cmd/cli/cmd/mcp.go`, `cmd/tui/ui/mcp_panel.go`.
