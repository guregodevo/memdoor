# The gateway

A daemon on your machine. Every run is a conversation on it. The window,
`memdoor run`, cron, the editor and the phone are doors onto the same
gateway: a run started by one can be finished from another.

## What it is

A background process on the machine the run belongs to. Holds the
conversations, runs turns and tools in the project, keeps schedules and
workflow runs, talks to the model on your key. Port 18789, on the machine
and its network. Each door requires a token. The first account is created
on the machine itself only. `MEMDOOR_HOST=localhost` restricts it to the
machine. Only requests to your provider leave the machine.

```
  window · memdoor run · cron · editor · phone ──► the gateway on your machine
                                                       │
                                                       ├─► your project: files, shell, git
                                                       ├─► conversations, schedules, workflow runs
                                                       └─► your provider key ──► the model
```

## Starting it

Started by the first command that needs one: `memdoor`, `memdoor run`,
`memdoor resume`, `workflow run`, `cron add`, `acp`. Stays up. A key
exported later is picked up by the next command. Reboot, logout or closed
lid ends it; the next command starts a fresh one. By hand:

```bash
memdoor gateway                 # on :18789, in the foreground
memdoor gateway --port 18799    # another port; --gateway :18799 on every command reaches it
memdoor gateway status          # is it up, which version, who manages it; exit 1 when it does not answer
memdoor doctor                  # is it healthy
memdoor logs errors --since 5m  # what went wrong
```

Log: `~/.memdoor/gateway.log`. Pid: `~/.memdoor/gateway.pid`;
`kill $(cat ~/.memdoor/gateway.pid)` stops it.

## On a box that stays on

A laptop's gateway stops with the lid. A run that must outlive it runs on a
VPS, a Mac mini, a home server, as a service: up at boot, restarted if it
stops. Laptop and phone are windows onto it.

```bash
# from your laptop: ssh to the box, nothing installed there
memdoor --gateway ssh://you@box run "say hi"         # installs, starts the gateway, signs in, runs
memdoor --gateway ssh://you@box connect openrouter   # your key, asked unseen, kept on the box
memdoor --gateway ssh://you@box connect openrouter-login   # or no key: sign in from this laptop's browser (chatgpt: a plan)
```

Or on the box, once:

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
cd <your-project> && memdoor run "say hi"   # the first run creates the first account there
memdoor connect openrouter                   # a key kept on disk; the service reads it
memdoor gateway install                      # systemd --user on Linux, launchd on macOS; no root
memdoor gateway status
```

Network: plain HTTP on the box's network. Trust the network, or keep the
box behind ssh, Tailscale or a reverse proxy. `--host localhost` restricts
it to the box; the ssh door still reaches it. Linux: `loginctl
enable-linger $USER` keeps the user service up after logout; the install
prints the line when needed. `memdoor gateway uninstall` removes the
service, keeps the data.

From the laptop:

```bash
memdoor --gateway ssh://you@<box> run "say hi"   # once: the ssh door signs in with the box's own login, and keeps it for <box> too
memdoor --gateway <box> --dir /srv/proj                                  # the window, on that box's checkout
memdoor --gateway <box> run --dir /srv/proj "run the tests"            # a headless turn there; without --dir: the same path as here if the box has it, else its home
memdoor --gateway <box> workflow run release                            # a workflow there; it runs with no one watching
memdoor --gateway <box> resume --last                                   # the window on that machine's last run, in its directory
```

The run happens on that machine, in its project, on its key. Login: the
box generated one on its first run (`~/.memdoor/local-login`). The ssh door
reads it, for `ssh://` and for the box's plain address. Without ssh:
`memdoor auth login-direct --gateway <box> --email … --password …` with
that login. The files panel (ctrl+f), @-mentions and `/dir` read that
machine's directory through the gateway. A directory the gateway does not
have is refused by name. Workflows and schedules there need no window.

Ssh door: `--gateway ssh://you@box` (or `MEMDOOR_GATEWAY=ssh://you@box`)
tunnels to the gateway for the length of the command, with the keys ssh
already has, and signs in with the box's login. Nothing on the box is
opened to the network. `?port=18799` names another port there.

Empty box, same line. No gateway through the tunnel: the door inspects the
box over ssh. No memdoor: runs memdoor.ai's installer (binary in
`~/.local/bin`, no sudo). Gateway not running: installs one as a service on
the door's port (systemd --user or launchd; else a background process).
Each step printed, then the command runs. A running gateway is left alone.
No model until a key is connected there once: `memdoor --gateway
ssh://you@box connect openrouter` (any provider; `openrouter-login` or
`chatgpt` sign in from the laptop's browser). Linux and macOS boxes.
Windows: by hand, `irm memdoor.ai/install.ps1 | iex`.

```bash
memdoor --gateway ssh://you@box                                   # the window, in the box's home
memdoor --gateway ssh://you@box --dir /srv/proj                   # the window, on a project there
memdoor --gateway ssh://you@box run --dir /srv/proj "run the tests"
memdoor --gateway ssh://you@box workflow run release
```

Phone: `/remote` in a window on the box prints a link on that gateway,
`http://box:18789/r/…`. A phone on the box's network, VPN or Tailscale
opens the conversation there; no memdoor.ai account, no plan. `memdoor
--gateway <box> remote --last` prints the same link from a shell. The key
after `#` never leaves the page; every frame is sealed with it, so plain
http is fine. Through the ssh door the link is on the box's name with the
gateway's port. A gateway kept to localhost is not reachable from a phone;
`/remote` then goes through memdoor.ai and says so.

The gateway holds the link once a window turned `/remote` on. Window
closed: the phone still opens the conversation and answers its gate. A
gateway restart brings the link back.

Not there yet: an image pasted into a window whose project is elsewhere.

## What is on it

| | Where |
|---|---|
| Conversations and their history | `~/.memdoor/memdoor.db`; `memdoor resume` lists them, `memdoor resume <id>` opens one |
| Providers and keys | its environment, or what `memdoor connect` kept in `~/.memdoor/providers.json` |
| Every model request, with its cost | `~/.memdoor/meter.jsonl`, summed by `/usage` and `memdoor savings` |
| Schedules | `memdoor cron list` |
| Workflow runs | `memdoor workflow status <run-id>` |

## If it restarts

All of the above is on disk. A window reconnects. A turn mid-run may have
finished while the window was away; its result is in the saved messages. A
cut-off workflow run continues from its last finished task with `memdoor
workflow resume <run-id>`.

## Where to next

- **[Headless, then attached](/docs/headless)** — every door, with the commands.
- **[How it works](/docs/how-it-works)** — what happens inside a turn.
- **[Security](/docs/security)** — the hosts the binary names, and when each is contacted.
