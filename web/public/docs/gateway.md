# The gateway

One gateway, every door. The gateway is a small server on your machine, and
every run is a conversation on it. The window, `memdoor run` in a script or
CI, a cron schedule, your editor and your phone are doors onto the same
gateway, which is why a run started by one can be finished from another.

## What it is

A background process on the machine the run belongs to: your laptop, or a
CI runner. It holds the conversations, runs the turns and their tools in
your project, keeps the schedules and the workflow runs, and talks to the
model on your key. It listens on `localhost:18789` and nothing else reaches
it. Your files never leave the machine; what goes out is the request to the
provider you chose.

```
  window · memdoor run · cron · editor · phone ──► the gateway on your machine
                                                       │
                                                       ├─► your project: files, shell, git
                                                       ├─► conversations, schedules, workflow runs
                                                       └─► your provider key ──► the model
```

## You never start it on your laptop

The first command that needs it starts one when none is running: `memdoor`,
`memdoor run`, `memdoor resume`, `workflow run`, `cron add` and `acp`. It
stays up afterwards, so the next command finds it, and a key you export
later reaches it the next time you run one. It ends with your login session:
a reboot, a logout or a closed lid ends it too, and the next command starts a
fresh one. To start it by hand:

```bash
memdoor gateway                 # on :18789, in the foreground
memdoor gateway --port 18799    # another port; --gateway :18799 on every command reaches it
memdoor gateway status          # is it up, which version, who manages it
memdoor doctor                  # is it healthy
memdoor logs errors --since 5m  # what went wrong
```

Its log is `~/.memdoor/gateway.log`; the pid of one Memdoor started is in
`~/.memdoor/gateway.pid`, and `kill $(cat ~/.memdoor/gateway.pid)` stops it.

## On a box that stays on

A laptop sleeps when its lid closes, and everything on it stops, the gateway
included. A run that must outlive the lid runs on a machine that does not
sleep: a VPS, a Mac mini, a home server. There the gateway belongs to the
service manager, so it is up at boot and comes back if it stops, and your
laptop and your phone are windows onto it.

```bash
# on the box, once
curl -fsSL https://memdoor.ai/install.sh | bash
cd <your-project> && memdoor run "say hi"   # the first run creates the first account there
memdoor connect openrouter                   # a key kept on disk; the service reads it
memdoor gateway install                      # systemd --user on Linux, launchd on macOS; no root
memdoor gateway status
```

`gateway install --host 0.0.0.0` listens for your laptop. The gateway speaks
plain HTTP, so keep that box behind ssh, Tailscale or a reverse proxy; it
says so when it starts. On Linux, `loginctl enable-linger $USER` keeps the
user service running when you are not logged in; the install prints the line
when it is needed. Removing it is `memdoor gateway uninstall`; the data stays.

From your laptop:

```bash
memdoor auth login-direct --gateway <box> --email you@… --password …   # once; the credentials are kept per gateway
memdoor --gateway <box> --dir /srv/proj                                  # the window, on that box's checkout
memdoor --gateway <box> run --dir /srv/proj "run the tests"            # a headless turn there
memdoor --gateway <box> workflow run release                            # a workflow there; it runs with no one watching
memdoor --gateway <box> resume --last                                   # the window on that machine's last run, in its directory
```

The run happens on that machine, in that machine's project, on that
gateway's key; your terminal is the door. The window's files panel (ctrl+f),
the @-mentions and `/dir` read that machine's directory through the gateway,
so what you see is what the agent sees. A directory the gateway does not have
is refused by name, never quietly replaced. Workflows and schedules on that
gateway need no window at all.

Over ssh, nothing on the box is opened to the network: `--gateway
ssh://you@box` (or `MEMDOOR_GATEWAY=ssh://you@box`) opens a tunnel to the
gateway there for the length of the command, with the keys ssh already has,
and signs in with the login the box generated for itself. `?port=18799`
names a gateway on another port there.

```bash
memdoor --gateway ssh://you@box --dir /srv/proj                   # the window
memdoor --gateway ssh://you@box run --dir /srv/proj "run the tests"
memdoor --gateway ssh://you@box workflow run release
```

Your phone reaches the box the way your laptop does. `/remote` in a window
on a box prints a link on that gateway, `http://box:18789/r/…`: a phone on
the box's network, VPN or Tailscale opens the conversation there, with no
memdoor.ai account and no plan. `memdoor --gateway <box> remote --last`
prints the same link from a shell and holds the conversation on it;
`memdoor join <link>` opens it in another terminal. The key after `#` never
leaves the page and every frame is sealed with it, so the link works on the
box's plain http. Through the ssh door the link is on the box's name, with
the gateway's port, as long as that gateway listens on the network
(`gateway install --host 0.0.0.0`); a gateway that listens on localhost
alone has no door for a phone, and `/remote` then goes through memdoor.ai
and says so.

What is not there yet: a gate answered from the phone when no window is open
on the box, and an image pasted into a window whose project is elsewhere.

## What is on it

| | Where |
|---|---|
| Conversations and their history | `~/.memdoor/memdoor.db`; `memdoor conversations` lists them, `memdoor resume <id>` opens one |
| Providers and keys | its environment, or what `memdoor connect` kept in `~/.memdoor/providers.json` |
| Every model request, with its cost | `~/.memdoor/meter.jsonl`, summed by `/usage` and `memdoor savings` |
| Schedules | `memdoor cron list` |
| Workflow runs | `memdoor workflow status <run-id>` |

## If it restarts

Everything above is on disk. A window reconnects on its own; a turn that
was mid-run may have finished while the window was away, and its result is
in the saved messages. A workflow run that was cut off continues from its
last finished task with `memdoor workflow resume <run-id>`.

## Where to next

- **[Headless, then attached](/docs/headless)** — every door, with the commands.
- **[How it works](/docs/how-it-works)** — what happens inside a turn.
- **[Security](/docs/security)** — every host the binary names, and when it is contacted.
