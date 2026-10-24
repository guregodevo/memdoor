# ADR-0021: The gateway owns the work; every door onto it is free; it binds the network by default

Date: 2026-10-23 (Greg: "default to be the network", "the pro story will be
the self host") · 2026-10-24 (the ssh door: "accessible from http and ssh
like github") · Status: accepted, in force. Record:
`docs/architecture/GATEWAY_AND_DOORS.md`, `docs/features/REMOTE.md`,
`web/public/docs/gateway.md`.

## Context

A coding agent that lives in a terminal window dies with the lid. The work
(a long turn, a scheduled check, a workflow run, a conversation a phone is
watching) needs a process that stays: the gateway. Once the gateway is the
thing that works, the window is one way in among several, and the question
becomes which ways in are free and where the gateway should listen.

## Decision

- **The gateway is the product.** It runs the turns, holds the sessions,
  the log, the cron and workflow state, and the remote links
  (`gateway/remote_host.go`, `~/.memdoor/remote-hosted.json`): a `/remote`
  link outlives the window and a restart revives every link. `memdoor
  gateway install` makes it a service on a box.
- **Every door onto a gateway of your own is free.** The window on the same
  machine (`memdoor tui`); the window on a laptop against a box (`--gateway
  <box> --dir`, files over `/api/fs`); ssh (`--gateway ssh://you@box`,
  `cmd/cli/cmd/gateway_ssh.go`: a tunnel, and on a bare box the installer
  and a gateway started over that ssh on first use); headless (`memdoor
  run`, `memdoor resume`); the phone (`/remote` on the gateway's own
  address, no account); cron; peers (ADR-0023).
- **The gateway binds the network by default** (0.0.0.0, port 18789).
  `/remote` links live on the gateway's own address, which is right on a
  LAN, a VPN, Tailscale or a VPS. `MEMDOOR_HOST=localhost` closes it; only
  then does `/remote` go through the memdoor.ai relay. Open with a gate,
  never closed by default.
- **The gate is a token on every door, and the first account is made from
  the machine itself** (`gateway/setup_handlers.go`). An ssh sign-in carries
  to the box's http address; a remote link's key sits after the `#` and
  never reaches a server.
- **Pro is what memdoor.ai hosts**: the relay for a gateway kept to
  localhost (a laptop behind NAT), the hosted workflow state across
  machines, and next the box itself. Never a model key (ADR-0020).

## Consequences

- A window may close mid-turn; the turn finishes in the gateway and the
  answer is there when a window, a phone or `memdoor resume` opens it.
- The CODER never restarts the gateway from its own turn (it would kill the
  turn running it); the person does, with `make up`.
- A new websocket path needs its own nginx location in `scripts/deploy.sh`
  for the memdoor.ai relay.
- The two account systems stay distinct: a gateway account (on the box) and
  a memdoor.ai account (for what memdoor.ai hosts).
