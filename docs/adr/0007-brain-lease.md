# ADR-0007 — The brain is leased from the broker; the client never rents

Status: ACCEPTED (2026-09-04). Broker side shipped the same day: `gateway/billingsvc/lease.go` (lease, heartbeat, release; `lease_seconds` metered from heartbeats, capped at two minutes per beat). Greg: "the broker should rent the machine…
the broker is the server now… the client is thin and has no permission to
rent a machine… let's rethink the renting with this app product and new
pricing strategy" → "yes exactly."

**AMENDED 2026-09-15 by ADR-0010.** The lease stands — the client still
never rents, and the broker still owns identity, seats, plan, billing and
the meter. What changed is WHAT a lease returns: a serverless ENDPOINT
(`openai.vast.ai/<name>`), not a machine the broker rented. The
`rentMember` / `setRunning` / pause / resume / orphan-sweep path is
deleted; Vast's autoscaler does the machine work under our template.

## Context

The developer product rents by the hour from credits: the client picks a
target, a tier and a price cap (`memdoor llm deploy`), the broker on
memdoor.ai rents it (it alone holds provider keys), the client waits,
activates and pays per hour. The creator app (ADR-0006's Mac window) sells
a flat plan with the good brain always on (docs/strategy/PRICING_APP_2026_09.md).
A creator must never choose a machine, see a price, or hold a rental.

## Decision

The client asks for a BRAIN, never a machine. One contract on the broker:

    POST   /v1/brain/lease        {workspace, class: "pro"}
      →    {lease_id, state: ready | starting | local, eta_s, endpoint, token, model}
    POST   /v1/brain/heartbeat    {lease_id}        while a turn runs
    DELETE /v1/brain/lease/{id}                      the app quits (optional)

- The broker decides all hardware: a slot on a warm shared machine when one
  exists, a rental when not, the interruptible tier when it can absorb a
  pause, the pool size from active leases; it stops what nobody leases.
  Price, provider and GPU never cross to the client.
- Entitlement is the plan (`/v1/plan`): pro leases; free is answered with
  `state: local`, not an error.
- The client is thin: the gateway leases when a turn needs the pro brain
  (or at app start on a pro plan), sets the remote engine from endpoint +
  token as `llm use` does, heartbeats while turns run, falls back to the
  local brain when refused or failed, and shows one status line ("Starting
  the good brain, about two minutes"). It holds no provider credential and
  no rental control. `memdoor llm deploy` stays an ADMIN tool; the burst
  endpoint already refuses non-admins.
- Metering lives at the broker: lease minutes per workspace/user from
  heartbeats, requests from the endpoint when the broker fronts it — the
  flat plan's cost line. The gateway's per-user meter stays as a mirror.
- Billing is the plan, not the hour. Credits and top-ups remain for the
  developer product.
- A lease token is scoped to its endpoint: a rented endpoint is reachable
  only by the workspace that leased it.

## Consequences

- One place can pool; the idle tail of per-session rentals disappears
  behind it (PRICING_APP: pool past ~10 paying users).
- The app's footer carries no money; the TUI's `/meter` and rental lines
  stay for developers.
- Order of work: broker lease endpoints + pooling → gateway lease client
  and fallback → app status line. Related: ADR-0003 (broker-authoritative),
  ADR-0006 (Python never ships).
