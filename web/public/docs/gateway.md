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

## You never start it

The first command that needs it starts one when none is running: `memdoor
tui`, `memdoor run`, `memdoor resume`, `workflow run`, `cron add` and
`acp`. It stays up afterwards, so the next command finds it, and a key you
export later reaches it the next time you run one. To start it by hand:

```bash
memdoor gateway                 # on :18789, in the foreground
memdoor gateway --port 18799    # another port; --gateway :18799 on every command reaches it
memdoor doctor                  # is it healthy
memdoor logs errors --since 5m  # what went wrong
```

Its log is `~/.memdoor/gateway.log`; the pid of one Memdoor started is in
`~/.memdoor/gateway.pid`, and `kill $(cat ~/.memdoor/gateway.pid)` stops it.

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

## Another machine

Every command takes `--gateway`: a URL, a host:port, or a bare host.

```bash
memdoor --gateway 192.168.1.10 resume --last      # the window, on that machine's last run
memdoor --gateway 192.168.1.10 run "run the tests" # a headless turn there, in its project
export MEMDOOR_GATEWAY=http://192.168.1.10:18789   # every command from now on
```

The run happens on that machine, in that machine's project, on that
gateway's key; your terminal is the door. A gateway listens on localhost
unless started with `--host`; a gateway you open to your network is yours
to keep behind it.

## Where it is not

A CI runner's gateway lives as long as the job: `memdoor remote --last` as
the job's last line keeps it up for your phone until the job's timeout. A
run with the laptop closed is not here yet; it is what the hosted workflow
state on memdoor.ai is for, and it is coming.

## Where to next

- **[Headless, then attached](/docs/headless)** — every door, with the commands.
- **[How it works](/docs/how-it-works)** — what happens inside a turn.
- **[Security](/docs/security)** — every host the binary names, and when it is contacted.
