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

The gateway is its own relay whenever a phone could reach it (2026-10-23,
Greg: "a phone can connect to daemon", "no pro story", "/remote should not go
through memdoor.ai anymore unless it's needed"). `remoteSession.resolved()`
settles it once per `/remote`: a gateway on another machine is the relay as
the window reaches it; a gateway reached through loopback (the laptop's own,
or the end of an ssh tunnel, `--gateway ssh://box` (which sets the box up on
first use when nothing answers there: `sshBoxSetup`, 2026-10-24),
`cmd/cli/cmd/gateway_ssh.go`) is asked where it listens (`/health`'s
`listen`: host, port, and the machine's addresses when it binds every
interface, private network first, `gateway/listen.go`), and the link goes on
the first address it names, or on the box's name through the ssh door; only
a gateway on localhost alone goes through memdoor.ai, with the reason in the
window (`RelayWhy`). On the gateway's relay the terminal dials `/api/relay`
with the gateway's own token (`relayAuth.local`, never billing) and no
account or plan is asked. A plain-http page away from localhost has no
`crypto.subtle` (an insecure context), which is how a phone reaches
`http://box:18789/r/…`: `remoteCrypto.ts` then runs the same HKDF and
AES-GCM in JavaScript (noble); `TestRemoteCryptoMatchesTheBrowser` checks
both paths against the Go vectors.

## Who does what

| Part | Where | What it does |
|---|---|---|
| The link, the key, the terminal side | `cmd/cli/cmd/tui_remote.go` | `/remote` creates `{id, key}` per conversation (`~/.memdoor/`, mode 0600), dials the relay, seals and opens frames |
| The gateway hosts the link | `gateway/remote_host.go`, `cmd/cli/cmd/remote_host.go` | `PUT/GET/DELETE /api/remote/<channel>`: a window hands its link (id, key, its session token, where the link lives) to the gateway; the gateway's own process runs the terminal side with the same wiring, keeps the links in `~/.memdoor/remote-hosted.json` (0600) and revives them at start. A gateway without a host (404) leaves the relay to the window |
| The phone's TUI | `cmd/cli/cmd/tui_remote_tty.go` | asked by the page, runs a second bubbletea program on the same conversation, at the page's size; keys in, screen out |
| The relay | `gateway/remote_relay.go` | pairs one terminal with one browser per id and forwards frames verbatim |
| Who may attach a terminal | `gateway/remote_relay_auth.go` | a gateway token, or a memdoor.ai account token checked with billing's `GET /v1/me` |
| The page | `web/src/components/RemotePage.tsx`, `RemoteTerminal.tsx` | the terminal view (default) and the chat view |
| The page's crypto and transcript | `web/src/remoteCrypto.ts`, `remoteTranscript.ts` | WebCrypto derive/seal/open; folding events into chat lines |

## The gateway holds the link (2026-10-23)

Until the command review the terminal side lived in the process that typed
`/remote` (the window, the old `memdoor remote`), so a link died with it: closing
the window on a box closed the phone's door. Now `tuiRemote` asks the
gateway's `/api/remote/<channel>` first (`hostedLink`: the link it already
holds, from any window), hands it a new one (`hostRemoteOnGateway`, the
`RemoteHostSpec`), and runs its own relay only when the gateway cannot host
(an older gateway, 404). The host (`gatewayRemoteHost`, registered by
`memdoor gateway` through `gateway.SetRemoteHost` before `Start`: the cmd
package's half of the gateway's duck-typed `RemoteHost`) builds the same
`remoteSession` a window builds, with a client on the spec's token, and
calls `startRemote`. `resumeRemote` in a relaunched window checks the gateway
before starting anything. In the gateway's process `remoteHostedHere` is
set: the phone's own `/remote` turns nothing on, the host never hands a link
to itself. A relay that answers 401 (`errRemoteSessionGone`) stops the loop
instead of retrying every five seconds. Receipt (scratch gateway on
0.0.0.0:18799, 2026-10-23): window closed, a terminal on the link (`memdoor join`,
since removed) drove a turn; gateway restarted, the same link drove another; `/remote off` made the
gateway forget it and `join` said nothing serves it.

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

## `memdoor remote` holds nothing (2026-10-24)

`cmd/cli/cmd/remote.go`: the link from a script. The gateway holds it, and
`--gateway <box>` names whose (Greg: "memdoor remote does not make sense, we
should use gateway instead", "we need to pass the --gateway <host> so memdoor
--gateway <host> remote"). It settles the
conversation (`remoteTarget`: an id prefix, `--last`, or the list and a
question at a terminal), calls `tuiRemote` exactly as the window does, and
returns: the link on stdout, the title, the watch-only link and the QR code
on stderr (`remoteLinkText`). It never holds anything; when the gateway could
not take the link (`remoteHeldHere`: an older gateway, 404) it stops the
relay it would have run and fails with `errGatewayHoldsNoLink`. The former
hold (`memdoor remote` stayed up until ctrl+c) and `memdoor join` (a relay
client in a terminal, "redundant with gateway": `--gateway <box>` is the
terminal's door onto a box) were removed the same day. A hand-over the
gateway refuses (403: another user holds the conversation's link) is an
error in the gateway's words, in the window too, instead of a second relay
started beside the gateway's. A CI job that waits for
the phone starts its gateway in the step and `wait`s on it.

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
