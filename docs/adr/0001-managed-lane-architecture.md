# ADR 0001 — The managed lane: broker, billing service, two trust placements

Status: ACCEPTED (2026-08-18) · Supersedes nothing · Implemented in
`pkg/billing`, `pkg/provision`, `gateway/billingsvc`, `cmd/cli/cmd/broker_client.go`

> **Amended 2026-09-15 by [ADR-0010](0010-the-pool-is-serverless-endpoints.md).**
> The managed lane stands and so do the two trust placements. What the
> broker manages is no longer a machine it rented: it resolves a brain
> class to a **Vast Serverless endpoint** and returns that, while the
> marketplace autoscales the workers under our template (our GPU class,
> our price cap, our vLLM build, our account). `pkg/provision`'s renting
> path and the broker's pause/resume/orphan sweep go with it. The
> reason is in the receipt: 36.8 % of the broker's own rentals never
> served, against a 6.4 % break-even.

> **Amended 2026-08-20.** The BYO lane described below — a marketplace key on
> the user's machine, `hasMarketplaceKey()` choosing the lane, the client
> watchdog policing BYO rentals, and the "BYO users are unaffected" consequence —
> was **removed** on 2026-08-20. The managed lane is the only lane: renting
> runs entirely through memdoor.ai's broker against prepaid credit, and there
> is no bring-your-own-key path. See
> [ADR 0003](0003-supplier-risk-and-multi-provider.md) ("Removing the
> bring-your-own-key lane (2026-08-20) increased the exposure") and the epoch
> note at the top of [`../roadmap/MUST.md`](../roadmap/MUST.md). The body is
> left as written for the record; the "known debt" about `pkg/provision`
> duplicating the CLI's BYO flow is moot for the same reason.

## Context

Memdoor rents GPUs so a coding agent can burst beyond the local model.
The first working version (2026-08-16/17) required each user to hold a
marketplace API key. Live onboarding proved that unacceptable for a
paying product: 2FA, scoped-vs-classic keys, prepaid provider credit —
an hour of friction before the first token. We also decided the business
model is TOP-UP (prepaid credits, commission on settled spend), and that
the distributed binary is closed-source.

Three constraints follow:

1. Users must be able to rent WITHOUT a marketplace account.
2. Pool keys and Stripe secrets can never ship in a distributed binary —
   anything on a user's machine is extractable by that user.
3. Money code must not share a process with agent/tool execution
   (prompt-injection blast radius).

## Decision

**Two trust placements, one product, one binary.**

- **BYO lane** (unchanged): a marketplace key on the user's machine →
  the CLI rents on the user's own account. Free, sovereign, no memdoor
  involvement. This lane stays forever as the anti-lock-in proof.
- **MANAGED lane** (new): no key → the CLI calls the BROKER, which rents
  from memdoor's pool account against the user's prepaid credits.

**Lane selection is automatic.** `hasMarketplaceKey()` decides; the user
never picks a "mode".

**The billing service is a separate PROCESS in the SAME binary**
(`memdoor billing`), running only on memdoor.ai. It refuses to start
without `MEMDOOR_BILLING_TOKEN` and holds `STRIPE_SECRET_KEY`,
`STRIPE_WEBHOOK_SECRET`, `MEMDOOR_POOL_VAST_KEY`. A self-hosted gateway
never starts it and therefore carries no billing code and no secrets.

**Domain packages are HTTP-free**: `pkg/billing` (credits, commission,
runway gate, settlement debits), `pkg/metering` (tiers, contracts,
settlements), `pkg/scheduling` (fit rule), `pkg/provision` (server-side
rental engine). `gateway/billingsvc` is transport only.

## Invariants (each enforced in code, most covered by tests)

- **Credits are per-WORKSPACE**, never per-user — the workspace is the
  billing boundary, which is what makes a plan change (free → solo by
  crediting, solo → enterprise by an operator's `POST /v1/plan`) a flag on
  the workspace, not a migration — everyone in the workspace moves with it,
  never a seat (see docs/internal/PLANS_AND_CAPACITY.md; the team tier this
  line once named was retired 2026-08-26).
- **Overdraft is impossible**: balance check and debit happen in one
  sqlite transaction (`DebitIfSufficient`), proven under concurrency.
- **Everything is idempotent by reference**: Stripe event id for
  deposits, `provider:instance` for settlements. Replays are no-ops.
- **We bill the provider's REAL charge**, never the estimate:
  credit-delta settlement (balance before − after + deposits in window).
  Estimate-fallback is allowed only when the balance is unreadable, and
  is FLAGGED in the record.
- **Shared-window settlements never auto-bill** — concurrent pool
  rentals make the delta a bound, not an isolation; a human resolves.
- **A deploy must be affordable for 30 minutes** or it is refused before
  a cent of pool money is spent.
- **The auto-stop promise is kept by whoever owns the money**: BYO
  rentals by the client watchdog; managed rentals by the broker
  watchdog (client heartbeats + 12-hour hard cap + disk-persisted
  rentals so a restart keeps policing).

## Consequences

- A paying user needs no marketplace account, no keys, no provider
  credit — top up, deploy, done.
- memdoor carries float risk and treasury duty (pool funding, Vast
  auto-recharge) — accepted; the solvency invariant is written down.
- **Known debt**: `pkg/provision` duplicates the CLI's BYO deploy flow.
  They must stay behaviorally identical (same product promise) until
  consolidated into one engine used by both lanes.
- The broker is a single point of failure for managed users; BYO users
  are unaffected by its downtime, and the local model floor means
  neither lane's outage is a work stoppage.

## Alternatives rejected

- **Ship pool keys to clients** — impossible with a distributed binary.
- **Mount billing inside the chat gateway** — rejected: money endpoints
  in the same process as agent/tool execution.
- **A separate billing repo/service binary** — rejected: a second build
  and deploy story for a solo maintainer; the subcommand pattern gives
  process isolation without it.
- **Bill from dph×uptime estimates** — rejected on evidence: estimates
  ran ~$1.60 off in a single evening.
