# Peers — @<peer>:<agent>, a gateway's agent asked from another gateway (experimental, 2026-10-24)

Greg, 2026-10-24: "we need A2A from one gateway to another", "we already have
@mention mechanism", "we can do @<host>:agent_name", "we have a quorum of
gateway; from quorum we can see which gateway are available", "this is
experimental". The peer-to-peer problems it is for: a laptop that sleeps hands
the run to a box that stays on; a step runs on the machine that has the repo;
each peer has its own key and its own project; a company's data answers
from the machine that holds it, only the answer crossing.

## What it is

A **peer** is a gateway this person has signed in to once and named:

    memdoor --gateway http://box:18789 auth login-direct --email … --password …   # or the ssh door, which signs you in on the way
    memdoor peers add box http://box:18789 --dir /srv/app
    memdoor peers        # the quorum: each peer, answering or not, its version, its agents, its dir

`~/.memdoor/peers.json` (0600): `{"box": {"gateway": "http://box:18789", "dir": "/srv/app"}}`.
The sign-in is the one `--gateway <box>` already keeps per gateway in
`~/.memdoor/credentials.json`; `peers add` refuses without it and says how.

From then on **`@box:coder <what>`** in any conversation on this gateway — the
window, `memdoor run "@box:coder …"`, `memdoor agent -m "@box:coder …"` — is a
mention of box's coder: the turn runs on box, in `/srv/app`, as `memdoor
--gateway box run` would run it, and the answer is this thread's reply from
`box:coder`. The run on the peer is a conversation there (`memdoor --gateway
box resume <id>` opens it; the reply ends with that line).

## Recorded

The landing's "Your own model" tab and the gallery (web/public/local-model.cast,
2026-10-26): the window on a laptop with no GPU asks `@box:coder` to add a flag
to a Go program and run it; the box, a g2-standard-16 with one L4 on Google
Cloud serving Qwen3.8-27B through janis, reads, patches, builds and runs it,
and the receipt lands in the laptop's window. The take needed janis to keep the
stream alive while a tool call is generated (the gateway's idle watchdog is a
minute) and the gateway to keep a local runtime's model order (the loaded model
first, v1.697.0).

## How it works

- `pkg/message/mention.go`: `ParseMentions` accepts `@<peer>:<agent>`
  (`[a-zA-Z0-9_.-]+:[a-zA-Z0-9_-]+`); `PeerAgent` splits it. `resolveMentions`
  makes it an actor `agent:box:coder` when the service's `PeerGateways` says
  `box` is a peer; otherwise the part before the colon is tried as a local
  name ("@coder: outline" is the coder).
- `pkg/message/peers.go`: `executeAgentTurn` branches before the buddy
  lookup: `finishPeerTurn` strips the mention from the text, asks the peer,
  saves the answer as the thread's reply from that actor and broadcasts it.
- **The reply is A2A across the gateways** (Greg, 2026-10-24: "it should be
  a2a with reply across gateway"). The peer's reply is an agent's message in
  this thread, so the names in it are resolved here as a local agent's
  would be (`peerReplyMentions`: agents of this gateway, people, this
  gateway's peers, never the peer agent itself) and `HandleAgentMentions`
  runs them with the mention depth incremented. `@coder` in box's reply
  wakes this gateway's coder; `@vps:coder` asks vps, a peer of this gateway;
  the coder's reply may ask box again. The depth bound that ends any A2A
  chain (two agent hops after the person) ends this one.
- The turn is posted on the peer with `asked_by_peer` (`gatewayRunner.asPeer`,
  `PostMessageRequest.AskedByPeer`, `sharedctx.AskedByPeerKey`): there, the
  reply's mentions wake nobody, since the thread the reply lands in is the
  asker's. The ask ends with a line saying so to the peer's agent: who asked,
  as what, where the reply goes, and that `@name` in it addresses an agent of
  the asking gateway (`peerAskText`).
- A turn an agent's reply wakes gets the turn's context whole
  (`asyncTurnContext`): the execution context with its depth, the project
  and the mode. It used to get the execution context alone, so the coder a
  reply woke ran in the scratch folder instead of the thread's project.
- A local agent's reply may name a peer's agent too (`@box:coder` in the
  coder's answer recruits box's coder); a peer is a name this gateway was
  given, so it needs no prior membership of the channel.
- `gateway/peers.go`: `peerSet` is the service's `PeerGateways`: the file
  above, and `Ask`, which hands the call to the binary (`SetPeerCaller`,
  `cmd/cli/cmd/peers.go` `peerAsk`: a `gatewayRunner` against the peer with
  the stored session, the peer's one workspace from `/api/setup/status`,
  the agent named, the text streamed back). While the peer answers, the
  asking session gets the events a local turn emits — lifecycle start, the
  text as it comes, lifecycle complete, model `coder@box` — so a window shows
  it live and ends, and `memdoor run` streams it to stdout and exits with its
  code.
- `cmd/tui/ui/model_ops.go` `RouteByMention`: a turn that opens with
  `@name` runs as that agent; the name may now be `box:coder` or
  `box.lan:chief`; a trailing colon is prose. `memdoor run` routes the same
  way (it used to prefix `@coder` to everything, so a peer mention from a
  run made both the local coder and the peer answer).
- A question the peer's agent asks has nobody there to answer: the first
  option is taken and the reply says so.

## Nobody answers (found while testing, fixed 2026-10-24)

A turn addressed to a name nobody answers to ("@vps:coder …", "@nobody …")
hung the window and the run for good: no agent started, so no turn ended.
Now `HandleAgentMentions`, when a person's turn opens with a mention that
resolved to nobody and no agent will run, broadcasts `execution.failed` to
the channel (the web chat) and, through `SetTurnFailed`, emits a lifecycle
error on the session's event stream (the window and `run` end with the
reason: "nobody named vps:coder answers here: not an agent on this gateway
(memdoor agent list), not a peer's (memdoor peers)").

## Receipts (two scratch gateways on one machine, 2026-10-24)

- `peers add box http://127.0.0.1:18798 --dir …/ciw/proj`: refused until the
  sign-in existed; then listed box's six agents. `peers` showed it answering.
- `memdoor run "@box:coder print the absolute path of the directory you are
  working in"`: stdout was box's project path, exit 0, only the peer
  answered; on A the thread holds the reply from `box:coder` with the
  resume line; on B the conversation `acp-…` holds the turn.
- The window: `@box:coder reply with the single word quokka` streamed
  `quokka`, footer `answered by coder@box`, the turn ended.
- `run "@vps:coder say hi"` (no such peer): exit 1 in under a second with
  the reason; the window showed the same line.

Reply across gateways (same two gateways, B named twice on A, as `box` and
as `vps`; the asks say "replacing WHO with …" so that the ask itself names
nobody):

- `run "@box:coder … @WHO say the single word quokka"` (WHO = coder): box's
  reply was `@coder say the single word quokka`; three seconds later A's
  coder answered `quokka` in the same thread. The run printed box's line
  and exited at the end of the turn it asked for; the thread went on.
- WHO:WHAT = vps:coder, `wombat`: box's reply named `@vps:coder`; A asked
  vps and `vps:coder: wombat` is the next reply in the thread.
- The round trip, WHO = coder, WHO2 = box:coder, `emu`: `box:coder` →
  `coder: @box:coder say the single word emu` → `box:coder: emu`, 16 s,
  three agent replies, the depth log incremented twice.
- The coder's own reply naming `@box:coder` asked box: `box:coder` answered
  with its project's path in the thread.
- On B each asked conversation holds exactly one agent reply: the `@coder`
  in its own reply became `@me` there, `@vps:coder` was skipped as not in
  the channel; nothing ran on B that A did not ask for.
- The window: box's line with `answered by coder@box`, then `kiwi` with
  `answered by z-ai/glm-5.3-flash`, no spinner left.

## Not yet

- The chain is a star around the thread's gateway: a peer's reply names
  this gateway's agents and peers, never its own peers (its gateway wakes
  nobody for an asked turn). A peer of a peer is reached by naming it here.
- `memdoor run` exits when the turn it asked for ends; the replies the
  chain adds after that are in the thread (`memdoor resume <id>`), as for a
  local A2A chain.
- The peer's agent cannot ask the person back; a gate there takes the default.
- `memdoor peers` probes from the shell; the gateway does not watch its peers.
- Nothing on the public site: experimental.
