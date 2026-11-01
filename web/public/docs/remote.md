# Remote control

![/remote: the same terminal on a phone, sealed end to end](/remote.cast)

`/remote` in the TUI prints a link and a QR code. The page is the TUI,
running on your computer and drawn on your phone.

The gateway listens on this machine's network. The link is `http://<its
address>:18789/r/…` and the gateway is its own relay. A phone on the same
Wi-Fi, VPN or Tailscale opens the conversation with no account and no plan.
Same on [a box that stays on](/docs/gateway). A gateway kept to localhost
(`MEMDOOR_HOST=localhost`) goes through the memdoor.ai relay, part of the
hosted gateway; `/remote` says so. A relay of yours at `MEMDOOR_REMOTE_URL`
is free.

## Turn it on

In the conversation:

```
> /remote

  Remote control for this conversation, on your gateway at 192.168.1.20:18789 —
  open the link or scan the code from a phone that reaches that machine:

  http://192.168.1.20:18789/r/0L17wXRvEC5IzW_D#k=…

  The key after # never leaves your devices; the gateway only carries what it
  cannot read. /remote off revokes both links.
```

Scan the code or open the link in any browser. No account, no plan: the
gateway serves the page on its own address.

A gateway kept to localhost (`MEMDOOR_HOST=localhost`) goes through the
memdoor.ai relay; its link reads `https://memdoor.ai/r/<id>#k=…`. That one
needs a sign-in, once:

```
$ memdoor login you@example.com
```

Once on, the gateway's process holds the link as the terminal end of the
relay. The window may close; a laptop may sleep when the gateway is on a box;
the link still opens the conversation and a gate is still answered from the
phone. A gateway restart brings every link back. The same link comes from any
window on the conversation, on any machine, and from `memdoor remote`
(below). `/remote off` revokes it everywhere.

## What you can do from the page

The same program as the keyboard: watch a turn as it runs, tool frame by tool
frame; answer a question; stop a turn; type the next request or any slash
command.

A bar under the screen has the keys a phone's keyboard lacks: `Esc`, `Tab`,
the arrows, `Enter`, `Ctrl+C` and `Ctrl+O`. Chat, in the corner, switches to
a plain reading view with a box to send a turn. Stop interrupts the running
turn from either view.

## When it works

While your gateway is up. The window may close. If the gateway is down (a
laptop asleep) the page waits; when it is back, every link is back, nothing
to scan again.

Several pages at once (phone and laptop) show the same screen; a page that
opens later is shown the screen so far. Up to eight pages per conversation,
watch-only pages apart (below).

Where the page connects depends on where the gateway listens:

```
gateway on the network (default)      gateway kept to localhost (MEMDOOR_HOST=localhost)
http://<box>:18789/r/<id>#…           https://memdoor.ai/r/<id>#…
gateway ──http──▶ viewer              gateway ──dials out──▶ memdoor.ai ◀── viewers
```

On its own address the gateway serves the page and sends every frame to every
viewer: a LAN, a VPN, Tailscale or a VPS; a laptop behind NAT cannot be
reached. Kept to localhost, the gateway dials out to memdoor.ai once with
your account token, each viewer dials memdoor.ai with a proof from the link,
and the relay fans out sealed frames it cannot read; the gateway sends each
frame once. That is the path for an audience anywhere.

## A link to watch, not to drive

`/remote view` prints a second link that sees everything and cannot type:

```
> /remote view

  A watch-only link to this conversation — whoever opens it sees the terminal
  and cannot type:

  http://192.168.1.20:18789/r/0L17wXRvEC5IzW_D#v=…
```

Their page says watching: no key bar, no Stop, no box to send a turn; your
terminal drops anything it sends except a request to see. The watch-only
link does not contain your link's key and cannot be turned into it. `/remote
off` revokes both links.

## An audience

Up to 200 pages may watch at once, apart from the eight the full link allows.
Each sees the terminal as you see it, sealed the same way, no account,
nothing uploaded. `/remote view` also prints the link with `?live=1` before
the `#`: the terminal alone, no header, for a stream, a projector or an OBS
browser source. The watch link goes in the chat, the live link in the scene.
A watcher's opening starts the terminal, so the stream shows before you open
the full link; a page of yours that opens later resizes it. Pair
programming: the full link in a second pair of hands, both typing into the
same window.

## From a script, no window

`memdoor remote` is `/remote` headless, for the conversation a run printed.
The gateway holds the link; `--gateway <box>` names whose.

```bash
memdoor remote <id>                    # the link on stdout, one line; the gateway holds the conversation on it
memdoor remote --last                  # this directory's most recent conversation
memdoor remote                         # at a terminal: the list, and a question (which one), as memdoor resume asks
memdoor remote --view <id>             # the watch-only link instead
memdoor remote off <id>                # revoke
memdoor --gateway <box> remote <id>    # a conversation on that box: the box holds the link
```

The id is the one `memdoor run` and `memdoor resume` print; any unique prefix
works. Stdout is the link alone, so `memdoor remote --last | pbcopy` works.
Title, watch-only link and QR code go to stderr, so a job's log shows a code
a phone scans off the screen. The command returns at once: the gateway holds
the link with no window and no command open and brings it back on restart.
The same link opens the conversation next time, from the window or the
command. A gateway older than this command cannot hold a link, and the
command says so.

```
$ memdoor remote --last
http://192.168.1.20:18789/r/6i3nQ7d8GL02TvGT#k=…
on your phone · "fix the failing test"
watch only: http://192.168.1.20:18789/r/6i3nQ7d8GL02TvGT#v=…
█▀▀▀▀▀█ ▄▀ ▄█▄ █▀▀▀▀▀█
…
held by your gateway, on 192.168.1.20:18789: it works with nothing open here, and comes back when the gateway restarts
```

### Finishing a CI run from your phone

The job starts the gateway in the step, runs `memdoor run`, prints the link,
and `wait`s on the gateway until you are done or the job's timeout ends it. A
runner's address is not reachable from a phone, so the job keeps its gateway
to localhost (`MEMDOOR_HOST: localhost`) and the link goes through the
memdoor.ai relay. The runner needs the hosted gateway's account token in
`MEMDOOR_BILLING_TOKEN`; on your machine `memdoor login` keeps it in
`~/.memdoor/billing-token`.

```yaml
      - name: Review, then wait for me on the phone
        env:
          OPEN_ROUTER_API_KEY: ${{ secrets.OPEN_ROUTER_API_KEY }}
          MEMDOOR_BILLING_TOKEN: ${{ secrets.MEMDOOR_BILLING_TOKEN }}
          MEMDOOR_HOST: localhost            # a runner's address is no door for a phone: the link goes through memdoor.ai
        timeout-minutes: 30
        run: |
          curl -fsSL "https://memdoor.ai/install.sh?ci=1" | MEMDOOR_CI=1 bash
          export PATH="$HOME/.local/bin:$PATH"
          memdoor gateway &                  # the job's gateway: the run lives in it
          until memdoor gateway status >/dev/null 2>&1; do sleep 1; done
          git diff origin/${{ github.base_ref }}...HEAD \
            | memdoor run "review this diff; one finding per line; change nothing"
          memdoor remote --last              # the link and its code, in the job's log
          wait                               # the job stays on its gateway while you finish the run from the phone
```

The run ends with the job. A run that must outlive it belongs on [a box that
stays on](/docs/gateway).

### The receipt

Run on 12 October 2026 against a gateway on a scratch machine, in this
order:

| Step | What happened |
|---|---|
| `memdoor run "reply with the single word ok; change nothing"` | started the gateway, answered `ok`, printed the conversation id |
| `memdoor remote --last > link.txt` (since 24 October the command returns at once: the gateway holds the link) | one line on stdout, the link; stderr the title, the watch-only link, the code |
| the link opened in another terminal | the run's conversation, its `ok` in view |
| a turn typed there | answered on the gateway (`pong`, the model named in the footer) |
| `memdoor resume --last`, then `/remote` in the window | the same link, same id |
| `memdoor remote off <id>` | "Remote control is off: the old link and its key no longer open this conversation." |

## Sharing a conversation

`/share`: the conversation as it is now, read-only, for later reading.

```
> /share

  A read-only link to this conversation as it is now — 12 messages, secrets removed.

  https://memdoor.ai/s/z8eKiSvRuHMsIGES#k=…
```

What was asked and answered, without tool output. Anything that looks like a
key, a token or a password is replaced by `[redacted]` before it leaves your
computer; the rest is encrypted there with a key that is only in the link, so
memdoor.ai stores bytes it cannot read. `/unshare` deletes every shared link
to the conversation. Sharing needs a memdoor.ai sign-in (`memdoor login
you@example.com`); `/remote` on your gateway does not.

## What travels, and who can read it

Files stay on your computer. What travels: the screen and your keystrokes,
end-to-end encrypted, through your gateway or through the memdoor.ai relay
when the gateway is kept to localhost. The key is only in the link, after
the `#`, the part a browser never sends to a server. The relay forwards what
it cannot read.

## The link is the key

Anyone with the link can drive the agent in that conversation, and the agent
runs commands on your computer. Treat it like a password: open it on your
devices; `/remote off` if it went anywhere else. The old link then opens
nothing and the next `/remote` makes a new one.

## Commands

| Command | What it does |
|---|---|
| `/remote` | Print the link and the QR code for this conversation, and the watch-only link |
| `/remote view` | Print the watch-only link and its QR code |
| `/remote off` | Revoke both links |
| `memdoor remote <id>` | The link on stdout, from a script; the gateway holds it (`--last`, `--view`, `off <id>`) |
| `memdoor login you@example.com` | Sign in, once per computer: for the memdoor.ai relay (a gateway kept to localhost) and `/share`; `/remote` on your own gateway needs none |

## Where to next

- **[The TUI](/docs/tui)** — the screens and the keys.
- **[Slash commands](/docs/slash-commands)** — the full list.
