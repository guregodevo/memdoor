# ADR-0008 — One pool for every workspace, on one model, under a Raft broker

Status: ACCEPTED (2026-09-04). Greg: "apply the pooling for different
workspace… one pool of servers for every workspaces… the broker and server
fault tolerant with go raft… 3 instances run and there is always one
instance up… same machine, high availability, high consistency… seats per
workspace, billing for 1 user first then Enterprise (contact us)… pick one
good enough model, 27B, on demand only, no bid… let's trim our jobs."

**AMENDED 2026-09-15 by ADR-0010.** The pooling intent stands — one
shared pool for every workspace, seats as slots, no bids, one
good-enough model, a dedicated pool as the enterprise tier. Only the
implementation of "who rents" changes: the pool IS a Vast Serverless
endpoint (one per brain class; a dedicated endpoint per workspace for
enterprise), so `poolPerMember`, the idle pause and the orphan sweep
become the autoscaler's `target_util`, `inactivity_timeout` and worker
replacement. Reason: the broker's own renting carried 36.8 % of rentals
never served against a 6.4 % break-even.

## Context

ADR-0007 made the broker the only renter and the client thin. Its first
lease was a rental per workspace: the idle tail every workspace pays for,
and the money a flat plan cannot afford past a handful of users
(docs/strategy/PRICING_APP_2026_09.md). The broker was also one process
with its registry in a JSON file: a crash or a deploy is a window in which
no one watches the meter.

## Decision

1. **One model, on demand.** The pool serves one catalogue target,
   `qwen3.8-27b`, on the on-demand tier. No bidding, so no preemption: a
   member is ours until we stop it. "Good enough" beats "cheapest".

2. **One pool, every workspace.** A lease is a SLOT on a shared pool member,
   not a machine. The broker places a lease on the serving member with the
   fewest leases below the per-member cap; when every member is full it
   rents another; a member nobody leases for the idle window is paused,
   then released. Members belong to the pool (`workspace: pool`), never to
   a customer. The token a lease carries is the member's; it reaches only
   workspaces the broker leased there, and changes when the member does.

3. **Seats.** A workspace has `seats` (default 1). A lease names its seat
   (the user the gateway is serving); a seat beyond the count is answered
   `state: local` with `reason: seats`, never an error. Pro is one seat;
   enterprise sets seats by hand ("contact us"). Held time is metered per
   lease, so per seat and per workspace.

4. **The pro plan.** `pro` is what a customer buys: **$149 per SEAT per
   month, unlimited use**, monthly or yearly (Greg, 2026-09-05: "there is
   no balance… it's now subscription… Free or subscription… we have a seat
   per month that costs 149$ and has unlimited access"). It leases the
   pool and prepays nothing — the SUBSCRIPTION is the entitlement, and no
   balance stands behind it. The prepaid tier (`solo`) is retired: its
   rows read as `pro`, and the credit gate is gone from the rent path. It is set by the operator today
   (`POST /v1/plan`) against a payment link, or by the Stripe subscription
   webhook. `free` stays local — with the Memdoor name on what it makes —
   and `enterprise` is still a conversation.

5. **A Raft broker.** The billing service runs as three instances on the
   VPS under hashicorp/raft; the rental registry (rentals, leases, blocked
   hosts) is the replicated state machine, the JSON file becomes its
   snapshot. Only the leader rents, watches and settles; a follower forwards
   every request to the leader. nginx spreads `/billing/` over the three.
   Same machine, so this survives a process crash or a rolling deploy with
   no gap and no split state — not the loss of the machine; that stays a
   backup-and-restore matter. The credits database and the auth store stay
   files with a single writer, the leader.

## Consequences

- The gateway's own pool (`pkg/pool` on the client, `warm_pool.go`) is the
  developer product's and is no longer the app's path; it goes when the
  developer product does.
- `lease_id` is a lease, not an instance: a client that heartbeats a lease
  the broker dropped is told 404 and asks again (its client already does).
- Trimmed out on purpose: Stripe subscriptions, per-seat usage reports,
  scheduled pool windows, a floor of warm members. Each is a day when a
  paying user asks.
