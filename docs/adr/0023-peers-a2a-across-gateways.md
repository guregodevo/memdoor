# ADR-0023: Peers: an agent on another gateway, asked by name, answering into the thread

Date: 2026-10-24 (Greg: "it should be a2a with reply across gateway") ·
Status: accepted, experimental. Record: `docs/features/PEERS.md`,
`pkg/message/peers.go`, `gateway/peers.go`, `cmd/cli/cmd/peers.go`.

## Context

Agent-to-agent within one gateway was an `@mention`: the mentioned agent
runs a turn and its reply is a thread reply, and mentions in that reply are
resolved in turn, bounded by depth. A second gateway (a box that stays on,
a VPS, a colleague's machine) had no way in but a person copying text. The
mention mechanism already had the shape; it lacked a name for "an agent
over there".

## Decision

- **`@<peer>:<agent>` is a mention like any other.** `memdoor peers add box
  <gateway> --dir <path there>` names a gateway you are signed in to;
  `@box:coder …` in any conversation (window, run, `memdoor agent -m`) runs
  the turn on box, in that directory, and brings the answer back as the
  thread's reply. `memdoor peers` lists the quorum.
- **The reply is A2A where the thread is.** The names in a peer's reply are
  resolved on the asking gateway (`peerReplyMentions`): `@coder` wakes this
  coder, `@vps:coder` asks another peer, and the coder may ask box again.
  The mention depth ends it, as for local mentions. A star around the
  thread's gateway, never a chain of gateways.
- **A turn asked by a peer wakes nobody on the peer** (`asked_by_peer` on
  the post, `AskedByPeerKey` in the context): the peer answers and is done;
  its own reply-mention loop does not run.
- **A woken turn gets the project and the mode of the turn before it**
  (`asyncTurnContext`): the peer's reply lands in the asker's project, not
  the scratch directory.
- **A mention nobody answers fails the turn** instead of hanging the window
  and the run.
- **Transport is the gateway's own API**, with the asker's token for that
  gateway; peers are a door (ADR-0021), gated like the others.

## Consequences

- Experimental: the command is marked so in `--help`; the shape may change.
- A peer's model, key and bill are the peer's; nothing of the asker's key
  travels.
- Depth is the only loop guard; two gateways mentioning each other stop at
  the bound, with the partial thread visible.
