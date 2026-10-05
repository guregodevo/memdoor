# Remote control

`/remote` in the TUI opens the conversation on another device. The page at
`https://memdoor.ai/r/<id>#k=<key>` is the TUI itself: a second program running
on the developer's computer, drawn by xterm.js in the browser. Live since
2026-09-28.

Pro on the hosted relay (Greg, 2026-10-04: "Remote should be pro only"; it
was free on every plan from 2026-09-29). The memdoor.ai relay asks billing who
the terminal belongs to and which plan: an account without a seat gets
402 and the sentence the window prints (`gateway/remote_relay_auth.go`). A
gateway relaying for its own users (`MEMDOOR_REMOTE_URL` at it, someone
hosting Memdoor themselves) never asks for a plan.

## Who does what

| Part | Where | What it does |
|---|---|---|
| The link, the key, the terminal side | `cmd/cli/cmd/tui_remote.go` | `/remote` creates `{id, key}` per conversation (`~/.memdoor/`, mode 0600), dials the relay, seals and opens frames |
| The phone's TUI | `cmd/cli/cmd/tui_remote_tty.go` | asked by the page, runs a second bubbletea program on the same conversation, at the page's size; keys in, screen out |
| The relay | `gateway/remote_relay.go` | pairs one terminal with one browser per id and forwards frames verbatim |
| Who may attach a terminal | `gateway/remote_relay_auth.go` | a gateway token, or a memdoor.ai account token checked with billing's `GET /v1/me` |
| The page | `web/src/components/RemotePage.tsx`, `RemoteTerminal.tsx` | the terminal view (default) and the chat view |
| The page's crypto and transcript | `web/src/remoteCrypto.ts`, `remoteTranscript.ts` | WebCrypto derive/seal/open; folding events into chat lines |

## Two hosts

The link and the relay socket go to `remoteURL()`: `https://memdoor.ai`, or
`MEMDOOR_REMOTE_URL`. The terminal authenticates there with the account token
(`memdoor login`). The conversation's events and posted turns stay on
the developer's own gateway, with its own token. Neither token opens the other
host.

## What the relay can and cannot see

Every payload is `base64url(nonce ‖ AES-256-GCM ciphertext)`, with the direction
(`t2b` or `b2t`) as additional data, so a frame captured one way does not open
the other way. Keys come from the link key by HKDF-SHA256, salt
`memdoor/remote-control/v1`: info `session` is the frame key, info
`browser-auth` is the proof the browser shows to join. The relay holds the
pairing id, the owner, and `sha256(proof)`.

One frame is at most 4 MiB (`remoteRelayMaxFrame`); a larger one closes the
socket that sent it (1009). The terminal holds the relay to the same limit.

It can see: frame sizes and timing, pairing ids, and the account token on the
terminal's socket. It cannot see: the screen, the keystrokes, the turns.

The link is the key. Whoever has it can drive the agent in that conversation,
and the agent runs commands on the developer's computer. `/remote off` revokes
it.

## Frames

Browser to terminal (`kind: "turn"`), one of:

| Field | Meaning |
|---|---|
| `text` | post this as the next turn |
| `question_id`, `answer` | answer a pending question |
| `cancel` | interrupt the running turn |
| `history` | send the conversation so far |
| `tty: {cols, rows, fresh}` | start the phone's TUI if none runs, else resize it |
| `tty_in` | keys typed into it |
| `watch` | the screen so far, addressed to this page's random id |

A frame from the full link is sealed with the **write key** (HKDF of the link
key under `write`); a watch-only link (`#v=`, base64url of the session key and
the relay proof) cannot derive it. The terminal acts on a write-sealed frame;
a frame that opens only with the session key may ask for `history` or `watch`,
and everything else it sends is dropped. The relay admits up to 8 pages per
session and gives every one the same frames, so there is one screen: a page
never restarts a program another page is using, and a late page asks to be
replayed what it has drawn (up to 1 MiB, then a repaint).

Terminal to browser (`kind: "event"`): the gateway's agent events as they are,
plus two streams of its own, `history` (the last 20 turns, long replies cut at
8,000 characters) and `tty` (screen bytes, base64, or `ended`).

## `memdoor join`

`cmd/cli/cmd/remote_join.go`: a relay client that is the page without the
browser. It parses `#k=` (driver: session, write and proof keys) or `#v=`
(watcher: session key and proof), joins `/api/relay/browser`, asks for the
screen with `tty` (drivers) and `watch`, writes `tty` frames to the terminal in
raw mode, sends keys as `tty_in`, sends its size on SIGWINCH, and leaves on
Ctrl+]. The host's TUI runs the agent in the host's folder, which is what
`memdoor tui --gateway <url> --channel <id>` cannot promise.

## `/share`

`cmd/cli/cmd/tui_share.go` builds the transcript (user and assistant text, no
tool output), runs `pkg/secrets.RedactText` over every message, seals the JSON
under a fresh 32-byte key (AES-256-GCM, additional data `memdoor/share/v1`),
drops the oldest messages if it exceeds 1 MiB, and uploads the ciphertext to
`POST /api/share` with the account token. `gateway/share_store.go` keeps
`<id>.bin` and `<id>.owner` under the data dir; `GET /api/share/<id>` is public
(the key is in the link), `DELETE` is the owner's only, 200 shares per account.
The page is `web/src/components/SharePage.tsx` with `web/src/shareCrypto.ts`.
Shares are recorded per conversation in `~/.memdoor/shares.json` (0600) so
`/unshare` can delete them.

## Production

nginx needs a `location /api/relay` block with the websocket upgrade headers
(`scripts/deploy.sh`). Without it the catch-all strips them and every socket
fails its handshake with a 400.

## Testing it locally

```bash
cd ~/memdoor-coder
MEMDOOR_REMOTE_URL=http://localhost:18789 \
MEMDOOR_BILLING_TOKEN=<your local gateway token> memdoor tui
```

Then `/remote`, and open the link in a browser on the same machine (WebCrypto
needs https or localhost). For the page's own code, `cd web && npx vite` proxies
`/api/relay` to the gateway.

## The landing page's recording

`web/public/remote.cast` is a real session recorded on the browser's side of the
relay at 50x34: see the comment in `web/src/components/RemoteDemo.tsx`.
