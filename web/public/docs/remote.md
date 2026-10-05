# Remote control

![/remote: the same terminal on a phone, sealed end to end](/remote.cast)

The same terminal, on your phone. Type `/remote` in the TUI, scan the code, and
the page that opens is the TUI itself — running on your computer, drawn on your
phone.

Part of Pro on the memdoor.ai relay. If you run your own gateway and point `MEMDOOR_REMOTE_URL` at it, it is yours and free.

## Turn it on

Sign in once, so the relay knows whose terminal it is:

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

While the TUI on your computer is running, with that conversation open. Close
the TUI and the page waits. Open the conversation again (`memdoor resume`) and
the same link picks up where it was — there is nothing to scan twice.

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
keystrokes, through a relay on memdoor.ai, end-to-end encrypted: the key is only
in the link, after the `#` — the part of a link a browser never sends to a
server. The relay forwards what it has no key for.

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
| `memdoor join <link>` | Open a link in a terminal instead of a browser (`Ctrl+]` leaves) |
| `memdoor login you@example.com` | Sign in, once per computer |

## Where to next

- **[The TUI](/docs/tui)** — the screens and the keys.
- **[Slash commands](/docs/slash-commands)** — the full list.
