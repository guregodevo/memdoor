# Remote control

![/remote: the same terminal on a phone, sealed end to end](/remote.cast)

The same terminal, on your phone. Type `/remote` in the TUI, scan the code, and
the page that opens is the TUI itself — running on your computer, drawn on your
phone.

The link lives on your gateway whenever a phone could reach it, which is
the usual case: the gateway listens on this machine's network, so the link
is `http://<its address>:18789/r/…`, the gateway is its own relay, and a
phone on the same Wi-Fi, VPN or Tailscale opens the conversation there with
no account and no plan. The same holds for [a box that stays
on](/docs/gateway). Only a gateway kept to localhost (`MEMDOOR_HOST=localhost`)
goes through the memdoor.ai relay; `/remote` says so, and that relay is
part of Pro. A relay of your own at `MEMDOOR_REMOTE_URL` is yours and free.

## Turn it on

For the memdoor.ai relay, sign in once, so it knows whose terminal it is (a
gateway on a box asks for nothing):

```
$ memdoor login you@example.com
```

Then, in the conversation you want to take with you:

```
> /remote

  Remote control for this conversation — open the link or scan the code:

  https://memdoor.ai/r/0L17wXRvEC5IzW_D#k=…

  The key after # never leaves your devices; the relay only carries what it
  cannot read. /remote off revokes the link.
```

Scan the code with your phone, or open the link in any browser.

Your gateway holds the link. Once `/remote` is on, the gateway's own process
is the terminal end of the relay, not the window: close the window, put the
laptop to sleep when the gateway is on a box, the link still opens the
conversation and a gate is still answered from the phone. A gateway restart
brings every link back. The same link comes from any window on the
conversation, on any machine, and from `memdoor remote` (below); `/remote
off` revokes it everywhere.

From a script, with no window, the same link comes from `memdoor remote`
(below).

## What you can do from the page

Everything you can do at the keyboard, because it is the same program:

- watch a turn as it runs, tool frame by tool frame;
- answer a question the agent asks;
- stop a turn that is going the wrong way;
- type the next request, or any slash command.

A bar under the screen has the keys a phone's keyboard lacks: `Esc`, `Tab`, the
arrows, `Enter`, `Ctrl+C` and `Ctrl+O`.

**Chat**, in the corner, switches to a plain reading view of the same
conversation, with a box to send a turn. **Stop** interrupts the running turn
from either view.

## When it works

While your gateway is up: it holds the link once `/remote` turned it on, so
the window may close and the page keeps working. If the gateway is down (a
laptop asleep, say) the page waits; when the gateway is back, every link is
back with it, and there is nothing to scan twice.

Several pages can be open at once — your phone and your laptop, say — and
they all show the same screen. A page that opens later is shown the screen so
far. Up to eight pages per conversation.

## A link to watch, not to drive

`/remote view` prints a second link that sees everything and cannot type:

```
> /remote view

  A watch-only link to this conversation — whoever opens it sees the terminal
  and cannot type:

  https://memdoor.ai/r/0L17wXRvEC5IzW_D#v=…
```

Send it to someone who should follow along. Their page says **watching**, has
no key bar, no Stop and no box to send a turn, and your terminal drops anything
it sends except a request to see. The watch-only link does not contain your
link's key and cannot be turned into it. `/remote off` revokes both links.

## From another terminal

The same link opens in a terminal on any machine with Memdoor installed:

```
$ memdoor join 'https://memdoor.ai/r/0L17wXRvEC5IzW_D#k=…'
Joining 0L17wXRvEC5IzW_D — you can type; Ctrl+] leaves.
```

It is the page without the browser: the same screen the others see, drawn in
your terminal at its size. The agent runs on the host's computer, in the
host's folder. A watch-only link joins the same way and sends nothing.
`Ctrl+]` leaves; every other key goes to the TUI.

## From a script, no window

The phone door is a command like every other door (2026-10-12): what
`/remote` does in the window, headless, for the conversation a run printed.

```bash
memdoor remote <id>        # the link on stdout, one line; then it holds the conversation on the relay
memdoor remote --last      # this directory's most recent conversation
memdoor remote             # at a terminal: the list, and a question (which one), as memdoor resume asks
memdoor remote --view <id> # the watch-only link instead
memdoor remote off <id>    # revoke
```

The id is the one `memdoor run` and `memdoor resume` print; any unique
prefix works. Stdout is the link and nothing else, so `memdoor remote --last
| pbcopy` works; the title, the watch-only link, the QR code and the hold line
go to stderr, so a job's log shows a code your phone scans off the screen.
The command stays up until `ctrl+c` or the job's timeout, and nothing else
ends it: a closed pipe does not. The gateway holds the link, so the hold is
for the job that started the run: it waits there for you. On a laptop,
`ctrl+c` right away is fine. Ending the hold does not revoke the link: the
same link opens the conversation the next time, from the window or from the
command.

```
$ memdoor remote --last
https://memdoor.ai/r/6i3nQ7d8GL02TvGT#k=…
on your phone · "fix the failing test"
watch only: https://memdoor.ai/r/6i3nQ7d8GL02TvGT#v=…
█▀▀▀▀▀█ ▄▀ ▄█▄ █▀▀▀▀▀█
…
holding this conversation on the relay · ctrl+c ends it, the link stays
```

### Finishing a CI run from your phone

A job that started a run with `memdoor run` hands it to you with one more
line: the link and its code are in the job's log and the job waits there,
the gateway alive, until you are done or the job's timeout ends it. A
runner's own address is no door for a phone, so the job keeps its gateway to
localhost (`MEMDOOR_HOST: localhost`) and the link goes through the memdoor.ai
relay: the runner needs the account's token in `MEMDOOR_BILLING_TOKEN`, a Pro
seat; on your own machine `memdoor login` keeps it in `~/.memdoor/billing-token`.

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
          git diff origin/${{ github.base_ref }}...HEAD \
            | memdoor run "review this diff; one finding per line; change nothing"
          memdoor remote --last
```

The run ends with the job; a run that must outlive it belongs on [a box
that stays on](/docs/gateway).

### The receipt

Every step below ran on 12 October 2026 against a gateway on a scratch
machine, in this order:

| Step | What happened |
|---|---|
| `memdoor run "reply with the single word ok; change nothing"` | started the gateway, answered `ok`, printed the conversation id |
| `memdoor remote --last > link.txt` | one line on stdout, the link; stderr the title, the watch-only link, the code, the hold line; the process stayed up |
| `memdoor join "$(cat link.txt)"` from another terminal | the run's conversation, its `ok` in view |
| a turn typed in that terminal | answered on the gateway (`pong`, the model named in the footer) |
| `memdoor resume --last`, then `/remote` in the window | the same link, same id |
| `memdoor remote off <id>` | "Remote control is off: the old link and its key no longer open this conversation." |

## Sharing a conversation

`/share` is the other kind of link: not the live terminal but the conversation
as it is now, read-only, for someone who should read it later.

```
> /share

  A read-only link to this conversation as it is now — 12 messages, secrets removed.

  https://memdoor.ai/s/z8eKiSvRuHMsIGES#k=…
```

What was asked and what was answered — not tool output — with anything that
looks like a key, a token or a password replaced by `[redacted]` before it
leaves your computer. It is encrypted there with a key that is only in the link;
memdoor.ai stores bytes it cannot read. `/unshare` deletes every shared link to
the conversation. It needs the same sign-in as `/remote`.

## What travels, and who can read it

Your files stay on your computer. What travels is the screen and your
keystrokes, through your gateway, or through the memdoor.ai relay when the
gateway is kept to localhost, end-to-end encrypted: the key is only in the
link, after the `#` — the part of a link a browser never sends to a server.
The relay forwards what it has no key for.

## The link is the key

Anyone who has the link can drive the agent in that conversation, and the agent
runs commands on your computer. Treat it like a password:

- open it on your own devices;
- run `/remote off` if it went anywhere else. The old link then opens nothing,
  and the next `/remote` makes a new one.

## Commands

| Command | What it does |
|---|---|
| `/remote` | Print the link and the QR code for this conversation, and the watch-only link |
| `/remote view` | Print the watch-only link and its QR code |
| `/remote off` | Revoke both links |
| `memdoor remote <id>` | The link on stdout, from a script; holds the conversation on the relay until `ctrl+c` (`--last`, `--view`, `off <id>`) |
| `memdoor join <link>` | Open a link in a terminal instead of a browser (`Ctrl+]` leaves) |
| `memdoor login you@example.com` | Sign in, once per computer |

## Where to next

- **[The TUI](/docs/tui)** — the screens and the keys.
- **[Slash commands](/docs/slash-commands)** — the full list.
