# The gateway

The gateway is a daemon on your machine. Every run is a conversation on it.
The window, `memdoor run` in a script or CI, a cron schedule, your editor
and your phone are doors onto the same gateway, so a run started by one can
be finished from another.

## What it is

A background process on the machine the run belongs to: your laptop or a
CI runner. It holds the conversations, runs the turns and their tools in
your project, keeps the schedules and workflow runs, and talks to the model
on your key. It listens on port 18789 on the machine and its network, so a
phone can reach it. Each door requires a token. The first account can only
be created on the machine itself. `MEMDOOR_HOST=localhost` restricts it to
the machine. Only the requests to the provider you chose leave the machine.

```
  window · memdoor run · cron · editor · phone ──► the gateway on your machine
                                                       │
                                                       ├─► your project: files, shell, git
                                                       ├─► conversations, schedules, workflow runs
                                                       └─► your provider key ──► the model
```

## Starting it

The first command that needs it starts one when none is running: `memdoor`,
`memdoor run`, `memdoor resume`, `workflow run`, `cron add` and `acp`. It
stays up, so the next command finds it. A key exported later is picked up
by the next command. A reboot, a logout or a closed lid ends it, and the
next command starts a fresh one. By hand:

```bash
memdoor gateway                 # on :18789, in the foreground
memdoor gateway --port 18799    # another port; --gateway :18799 on every command reaches it
memdoor gateway status          # is it up, which version, who manages it; exit 1 when it does not answer
memdoor doctor                  # is it healthy
memdoor logs errors --since 5m  # what went wrong
```

Its log is `~/.memdoor/gateway.log`. The pid of one Memdoor started is in
`~/.memdoor/gateway.pid`, and `kill $(cat ~/.memdoor/gateway.pid)` stops it.

## On a box that stays on

A laptop's gateway stops when the lid closes. A run that must outlive it
runs on a machine that does not sleep: a VPS, a Mac mini, a home server.
There the gateway is a service: up at boot, restarted if it stops. Your
laptop and phone are windows onto it.

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

The gateway listens on the box's network over plain HTTP. Trust that
network, or keep the box behind ssh, Tailscale or a reverse proxy. `--host
localhost` restricts it to the box; the ssh door still reaches it. On
Linux, `loginctl enable-linger $USER` keeps the user service running when
you are logged out; the install prints the line when it is needed. `memdoor
gateway uninstall` removes the service and keeps the data.

From your laptop:

```bash
memdoor --gateway ssh://you@<box> run "say hi"   # once: the ssh door signs in with the box's own login, and keeps it for <box> too
memdoor --gateway <box> --dir /srv/proj                                  # the window, on that box's checkout
memdoor --gateway <box> run --dir /srv/proj "run the tests"            # a headless turn there; without --dir: the same path as here if the box has it, else its home
memdoor --gateway <box> workflow run release                            # a workflow there; it runs with no one watching
memdoor --gateway <box> resume --last                                   # the window on that machine's last run, in its directory
```

The run happens on that machine, in its project, on its key. The box
generated a login on its first run (`~/.memdoor/local-login` there). The ssh
door reads it and signs you in, for `ssh://` and for the box's plain address,
so no password is typed. Without ssh, `memdoor auth login-direct --gateway
<box> --email … --password …` with that login does the same. The files
panel (ctrl+f), @-mentions and `/dir` read that machine's directory through
the gateway. A directory the gateway does not have is refused by name, not
replaced. Workflows and schedules there need no window.

Over ssh, nothing on the box is opened to the network. `--gateway
ssh://you@box` (or `MEMDOOR_GATEWAY=ssh://you@box`) opens a tunnel to the
gateway there for the length of the command, with the keys ssh already has.
It signs in with the login the box generated. `?port=18799`
names a gateway on another port there.

A box with nothing on it is set up by the same line. When no gateway
answers through the tunnel, the door inspects the box over the same ssh.
No memdoor there: it runs memdoor.ai's installer (binary in `~/.local/bin`,
no sudo). Gateway not running: it installs one as a service on the door's
port (systemd --user or launchd; without a service manager, a background
process). Each step is printed, then the command you typed runs. A running
gateway is left alone. The box has no model until a key is connected there
once: `memdoor --gateway ssh://you@box connect openrouter` (any provider;
`openrouter-login` or `chatgpt` sign in from the laptop's browser, no key).
Every turn on that box then runs on that key. Linux and macOS boxes are set
up this way. A Windows box is set up by hand with `irm
memdoor.ai/install.ps1 | iex`.

```bash
memdoor --gateway ssh://you@box                                   # the window, in the box's home
memdoor --gateway ssh://you@box --dir /srv/proj                   # the window, on a project there
memdoor --gateway ssh://you@box run --dir /srv/proj "run the tests"
memdoor --gateway ssh://you@box workflow run release
```

Your phone reaches the box the way your laptop does. `/remote` in a window
on a box prints a link on that gateway, `http://box:18789/r/…`. A phone on
the box's network, VPN or Tailscale opens the conversation there, with no
memdoor.ai account and no plan. `memdoor --gateway <box> remote --last`
prints the same link from a shell. The box holds the conversation. The key
after `#` never leaves the page and every frame is sealed with it, so the
link works over the box's plain http. Through the ssh door the link is on
the box's name with the gateway's port. A gateway kept to localhost has no
door for a phone; `/remote` then goes through memdoor.ai and says so.

Once a window turned `/remote` on, the gateway holds the link. Close the
window and the phone still opens the conversation and answers its gate. A
restart of the gateway brings the link back.

What is not there yet: an image pasted into a window whose project is
elsewhere.

## What is on it

| | Where |
|---|---|
| Conversations and their history | `~/.memdoor/memdoor.db`; `memdoor resume` lists them, `memdoor resume <id>` opens one |
| Providers and keys | its environment, or what `memdoor connect` kept in `~/.memdoor/providers.json` |
| Every model request, with its cost | `~/.memdoor/meter.jsonl`, summed by `/usage` and `memdoor savings` |
| Schedules | `memdoor cron list` |
| Workflow runs | `memdoor workflow status <run-id>` |

## If it restarts

Everything above is on disk. A window reconnects on its own. A turn that
was mid-run may have finished while the window was away, and its result is
in the saved messages. A workflow run that was cut off continues from its
last finished task with `memdoor workflow resume <run-id>`.

## Where to next

- **[Headless, then attached](/docs/headless)** — every door, with the commands.
- **[How it works](/docs/how-it-works)** — what happens inside a turn.
- **[Security](/docs/security)** — the hosts the binary names, and when each is contacted.
