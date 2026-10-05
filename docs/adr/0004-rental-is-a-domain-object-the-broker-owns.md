# ADR 0004 — A rental is a domain object, and the broker owns it

Status: PROPOSED (2026-08-28) · Refines ADR 0001 · Affects `pkg/provision`,
`pkg/pool`, `pkg/metering`, `gateway/warm_pool.go`, `gateway/rental_control.go`,
`gateway/deploy_watchdog.go`, `gateway/billingsvc`

## Context

The rental subsystem is ~8,700 lines across eleven packages and files, and it
is the least reliable part of the product. The 2026-08-26 receipts record seven
hosts rented and **zero served**; `GPU_RENTAL_LEARNING.md` counts seventeen
rental failures of which **thirteen were ours**, not the hardware's — only one
("a V100 cannot do FP8") was predictable from the provider's metadata.

That ratio is the finding. The thesis is "software that makes unreliable
hardware behave like a reliable endpoint"; today the software is the larger
source of unreliability. Reviewing the design rather than the individual bugs
turns up one cause underneath most of them.

### There is no `Rental`

A GPU-rental marketplace has no rental entity. Grepping every declared type
finds `rentalControl` (an interface), `rentalView` (a DTO) and `rentalRegistry`
(a map). What exists instead is **one thing represented six ways, each owned by
a different layer**:

| # | representation | where | what it knows |
|---|---|---|---|
| 1 | `provision.Instance` | `pkg/provision/vast.go:411` | the provider's JSON: status, intended, IP, DPH, start date |
| 2 | `pool.Member` | `pkg/pool/pool.go:326` | `{ID, Serving, Since}` — three fields |
| 3 | `brokerRental` in `rentalRegistry` | `gateway/billingsvc/broker.go:146` | the broker's persisted map, plus `superseded` and `blocked` |
| 4 | `rentalView` | `gateway/rental_control.go:66` | the client's DTO — twelve fields, overlapping (1) |
| 5 | a deploy-state file | the renter's disk | the client's own truth |
| 6 | `metering.Settlement` | `pkg/metering/metering.go:251` | the money record, written at death |

No single component answers *"what is the state of this rental?"* — so the
question is answered six times, and **the recorded failures are these
representations disagreeing**:

- **The pool cannot see a dead member.** `pool.Reconcile` counts what has a
  state file (`pkg/pool/pool.go:348`), not what serves — `Member.Serving`
  exists but only orders releases (`:361`). A host that cannot attach its GPU
  holds its slot, `act.Rent` stays 0, and the pool never replaces it. That is
  commit `a18ab75` verbatim: *"the waste mechanism works, the replacement does
  not… nine minutes later both were still sitting there billing, unreplaced."*
- **The client kept talking to a corpse.** `DEEPSEEK_FAILURES.md` #12: the
  broker replaced an instance, the client's local target file no longer matched,
  and an 8×GPU rental at $1.11/hr became invisible to `--status` and unstoppable
  by `--down`. Representation (5) disagreed with (3).
- **A healthy machine was destroyed at 14 minutes.** The reaper judged
  representation (1) — a Vast status message — while the operator watched the
  container log, and a registry timeout during a 24 GB pull read as a dead host
  (fixed in `a15dc48`, but the shape recurs: `DEEPSEEK_FAILURES.md` #13 is the
  same bug and is still open).

### Two watchdogs, two authorities, one machine

Lifecycle policy is implemented twice:

| | client — `gateway/deploy_watchdog.go` (870 loc) | broker — `gateway/billingsvc/watchdog.go` (695 loc) |
|---|---|---|
| tick | 45 s (`:39`) | 60 s (`:29`) |
| idle | `MEMDOOR_BURST_IDLE_MINUTES`, default **30** (`:32`) | `brokerIdleTimeout` = **30 min** (`:30`) |
| also decides | pausing, waking a paused rental, following a replacement | serve deadline, crash loop, status fault, hard cap, settlement |

The same 30-minute idle rule is written on both sides of a network boundary. The
consequence is already in the code as a comment: the client **sends heartbeats
for pool members specifically so the broker's reaper will not kill them**
(`deploy_watchdog.go:376–385`) — *"It is not a lie to the reaper: the pool IS
using the machine, by holding it ready."* A component arguing that its message
to another component is technically not a lie is a design telling you it has two
owners for one decision.

### Server and client code share a package

`package gateway` contains both sides with nothing marking which is which:
`rental_control.go` is a client calling `/v1/broker/*` over HTTP;
`deploy_watchdog.go` is a client-side watchdog; `warm_pool.go` reconciles a
pool. Only `gateway/billingsvc` — which runs on memdoor.ai alone — is separated,
and only by being a subpackage. A reader cannot tell from a file's location
whether it executes on a customer's laptop or on our VPS, which is exactly the
distinction that decides what it is allowed to know and to be trusted with.

## Decision

**1. A rental is a domain object.** Introduce `pkg/rental` holding one
aggregate: identity, provider, model, tier, price, timestamps, endpoint,
workspace — and an explicit state:

```
Requested → Launching → Serving → Paused → Serving
                │           │        │
                └───────────┴────────┴──────► Dead(reason)
```

`reason` is a value object, not a string: `NeverServed`, `Outbid`,
`IdleReaped`, `CapReached`, `HostFault(class)`, `ReplacedBy(id)`,
`UserStopped`. It is the thing settlement, waste accounting and the customer's
explanation all read, so it is written once, at the transition.

Every lifecycle rule becomes a transition guard on this object — pure, and
testable without renting a GPU. `provision.Instance` becomes what it should
always have been: a **provider reading** that the domain interprets, not a
competing truth.

**2. The broker owns the state; the client is thin.** memdoor.ai holds the
state machine and is the only authority that decides a rental's fate. The
renter's gateway keeps no lifecycle policy: it reports usage (a heartbeat is a
fact — "a turn ran at T" — never a request to stay alive), renders what the
broker reports, and routes turns to the endpoint the broker names. Its local
file becomes a **cache**, never a source of truth, and it recovers from the
broker on mismatch.

This deletes the duplicate 30-minute idle rule, deletes the reaper-gaming
heartbeat, and makes DEEPSEEK #12 structurally impossible: the client cannot be
authoritative about a rental it did not create and cannot see.

**3. Packages are split by who runs the code.**

| package | runs where | may hold |
|---|---|---|
| `pkg/rental` | both (pure domain) | the aggregate, states, transitions, reasons |
| `pkg/provision` | broker only | provider clients, selectors, bidding, diagnosis |
| `gateway/billingsvc` | memdoor.ai only | the state machine's owner, ledger, Stripe |
| renter-side (`gateway/…`) | the customer's machine | a thin follower: report, render, route |

A file's location answers "where does this run, and what may it be trusted
with?" — today it does not.

## The model

### Aggregates and how they relate

```
  User ──────N:1────► Workspace ──1:1──► BillingTerm
                          │                 (plan, tier entitlement,
                          │                  commitment, credit)
                          │
                          └──1:N──► Rental ──0:1──► Settlement
                                      ▲                  ▲
                                      │                  │
                    Pool ──selects/sizes         = f(Terms, Usage)
                  (a policy, not an owner)

  Rental holds two value objects that must not be confused:
      Terms  ── snapshot of a Product + the accepted quote   (immutable)
      Usage  ── the measured timeline                        (moves)

  Catalogue ──1:N──► Product ──quoted at Rent()──► Terms
   (operator data)   (id, description, price)      (frozen per rental)
```

**Three aggregate roots: `Workspace`, `Rental`, `Settlement`.** They reference
each other **by identity, never by pointer** — a `Rental` holds a `WorkspaceID`,
not a `*Workspace` — so each can be loaded, changed and persisted alone. That is
what makes a rental's state machine testable without dragging billing in.

Cardinalities, and why each is what it is:

| relation | card. | why |
|---|---|---|
| `User` → `Workspace` | **N : 1** | a token IS a workspace binding; the device grant refuses to complete without one (`PLANS_AND_CAPACITY.md §3`). One workspace, N people — that is the only thing that varies between solo and enterprise. |
| `Workspace` → `BillingTerm` | **1 : 1** | the workspace is the billing boundary, "never a seat". Today this is the bare `plan.Plan` string; it becomes an object because a term has more than a name (see below). |
| `Workspace` → `Rental` | **1 : N** | a pool is several rentals for one workspace (`Spec.Min = 3` on enterprise). |
| `Rental` → `Settlement` | **1 : 0..1** | exactly one money record, written at the terminal transition; none while alive. |
| `Pool` → `Rental` | selects | a pool is a **policy** (floor + schedule + load) that decides how many rentals should exist. It is not their owner; it counts them and asks for more. This is the fix for `Reconcile` counting state files. |

`BillingTerm` earns its place as an object rather than a `string`: it answers
*"may this workspace rent this, on which tier, and what happens when the auction
takes the machine"* — plan, allowed tiers, committed floor, and whether credit
covers the ask. That is the question `rentOne` and the pool reconciler both ask
today by reading a plan string and re-deriving the rest.

### Product, Terms, Usage — three things, not one

The single most important split inside the aggregate. Today a `brokerRental`
carries the catalogue key, a price, and the clock all in one struct, and
`metering.Contract` (`metering.go:111`) mixes the same three again. They change
at different rates, are owned by different people, and settlement is a *function
of two of them*:

```
        Settlement  =  f( Terms , Usage )
                          │       └── what actually happened (measured)
                          └────────── what was agreed (snapshotted, immutable)
```

**1. `Product` — the catalogue entry.** Operator-edited data
(`/var/lib/memdoor/catalog.json`), slow-changing, the same for everybody:

```go
type Product struct {          // value object, from the catalogue
    ID          ProductID      // "qwen3.8-27b" — the key a customer picks
    Description string         // the Blurb the picker shows
    Model       ModelRef       // HF id vLLM serves
    Requires    Requirements   // MinVRAM, GPU class, NeedsFP8, image
    MinPlan     Plan           // who may rent it at all
    ListPrice   PriceHr        // the ballpark fallback (catalogue EstHr)
}
```

**2. `Terms` — what this renter agreed to, at the moment they agreed.**
Immutable for the life of the rental, and **snapshotted, never looked up again**:

```go
type Terms struct {            // value object — frozen at Rent()
    Product   ProductID        // what was bought
    AgreedHr  PriceHr          // the quote the customer accepted, commission in
    Tier      Tier             // on-demand | interruptible | committed
    Commission Rate            // the cut, recorded so a later change can't rewrite history
    QuotedAt  time.Time        // which quote this was
}
```

This exists because **the customer's price is a live quote, not the catalogue
value**: `/v1/prices` returns "what the renter pays, commission included"
(`gateway/billingsvc/prices.go:38`), cached for twenty minutes and moving with
the market. Settling against "the current price" bills something the customer
never agreed to — a whole class of the "cost shown ≠ cost charged" bug
(2026-08-22), designed out rather than fixed again.

**3. `Usage` — what actually happened.** Measured, append-only, the only part
that changes while the rental lives:

```go
type Usage struct {            // value object, recomputed from the timeline
    RequestedAt time.Time
    ServingFrom time.Time      // zero until it first served — NeverServed is this being zero
    Paused      []Interval     // outbid or idle; excluded from billable time
    EndedAt     time.Time
    LastTurnAt  time.Time      // the fact the client reports; never a request to stay alive
}
func (u Usage) Billable() time.Duration   // uptime minus paused
func (u Usage) NeverServed() bool         // ServingFrom.IsZero()
```

`NeverServed()` becomes a property of measured usage rather than a flag someone
remembers to set — and it is the key to the waste rate and the 6.4 % break-even
(`pkg/billing/billing.go:170`). A rental that never served is *arithmetically*
waived, not waived by a code path that has to be reached.

### Entity — `Rental`

The one entity with a lifecycle: identity, terms, usage, state. Two rentals are
the same rental iff the ids match — which is what `superseded` papers over
today, and becomes an explicit `Dead(ReplacedBy(id))` transition instead.

```go
// pkg/rental
type Rental struct {          // aggregate root
    id        InstanceID      // identity (the provider's; theirs to name)
    workspace WorkspaceID     // by id, never a pointer
    provider  ProviderName
    terms     Terms           // AGREED — immutable for the rental's life
    usage     Usage           // MEASURED — the only part that moves
    state     State           // Requested|Launching|Serving|Paused|Dead
    reason    DeathReason     // set only on the terminal transition
    endpoint  Endpoint        // "" until it has one
}
```

The split pays off immediately in the three places that hurt today: the pool
asks `state`, billing asks `terms × usage`, and the customer's explanation asks
`reason` — three questions that currently all rummage in one flat struct.

State is **private**, mutated only through intention-revealing transitions that
return an error when the move is illegal — so an invalid state is unconstructible
rather than merely unlikely:

```go
func (r *Rental) Launched(at time.Time) error
func (r *Rental) Served(ep Endpoint, at time.Time) error
func (r *Rental) Paused(at time.Time) error          // Outbid or idle
func (r *Rental) Resumed(at time.Time) error
func (r *Rental) Died(reason DeathReason, at time.Time) error
func (r *Rental) Serving() bool                      // the pool's real question
func (r *Rental) BillableFor() time.Duration
```

Construction is a fail-fast factory returning the interface the callers use, per
the house rules (`coding-principles` §3/§4): `NewRental(...) (RentalSnapshot, error)`
refuses a rental with no workspace, no model, or a negative price.

### Value objects

Immutable, compared by value, validated at the boundary with `Parse*`
constructors (`coding-principles` §10) — replacing bare strings and floats:

| value object | replaces today | why it must be one |
|---|---|---|
| `InstanceID` | `metering.InstanceID` (already one — keep) | providers disagree: Vast numbers, RunPod names |
| `WorkspaceID` | a bare `string` on every struct | the billing boundary; must never be empty |
| `ModelRef` | `string` catalogue key | gates FP8/VRAM fit; a typo is a failed rental |
| `Tier` | `metering.Tier` (already one — keep) | decides `CanBeOutbid`, the whole plan difference |
| `PriceHr`, `Cents` | `float64` everywhere | money in floats is how "the cost shown and the cost charged used different arithmetic" happened (2026-08-22) |
| `DeathReason` | a free-text `reapReason` string | it drives waste accounting, the customer's explanation, and whether we waive — too load-bearing to be prose |
| `Endpoint` | `string` | empty vs present is a state distinction, not a formatting one |
| `HostFaultClass` | strings matched in `diagnose.go` | CDI / driver / OOM / image-pull — the classes we write off on sight |
| `ProductID` | the catalogue's map key, passed as `string` | what the customer picked; the join between catalogue, terms and settlement |
| `Terms` | fields spread over `brokerRental` + `metering.Contract` | the agreement, frozen — a live quote must not be re-read at settlement |
| `Usage` | timestamps spread over `brokerRental` | the measurement; `Billable()` and `NeverServed()` are computed, never flags |

`DeathReason` is the one to get right: `NeverServed` already keys the waste rate
and the 6.4 % break-even (`pkg/billing/billing.go:170`). As a value object it is
recorded at the transition by the only component entitled to say so, rather than
reconstructed later from a status string.

### Repositories — interfaces, in the consumer

Ports the domain declares and infrastructure satisfies (factories return
interfaces; the domain never imports the gateway):

```go
type RentalRepository interface {
    Save(context.Context, RentalSnapshot) error
    Get(context.Context, InstanceID) (RentalSnapshot, error)
    ByWorkspace(context.Context, WorkspaceID) ([]RentalSnapshot, error)
    Live(context.Context) ([]RentalSnapshot, error)   // what the watchdog sweeps
}

type SettlementLedger interface { ... }   // exists (pkg/metering) — keep
```

`rentalRegistry` (`broker.go:146` — a map plus `superseded` plus `blocked`
persisted by hand) becomes the SQLite/JSONL implementation of this interface.
`blocked` becomes a separate small port (`HostBlocklist`), because "this machine
failed to launch" is a fact about a *host*, not about a rental.

### Domain service — the rules that need no I/O

Rules spanning more than one entity, pure and clock-injected so they are
testable without a GPU:

```go
type LifecyclePolicy interface {
    // Verdict is what SHOULD happen to this rental now, given the clock and
    // the last provider reading. Pure: no HTTP, no destroy, no billing.
    Verdict(RentalSnapshot, ProviderReading, time.Time) (Action, DeathReason)
}
```

This is where the ladder currently spread across `billingsvc/watchdog.go:270-433`
(serve deadline, stall, crash-loop, status fault, idle, hard cap) moves — one
ordered decision, one place to calibrate, and the receipts' finding 2 (the
cold-start estimate is optimistic and drives the deadline) becomes a change to
one guard rather than to a rule ladder read across two files.

### Application services — the things with side effects

Thin orchestration: load, decide, act, persist. No rules of their own.

- **`RentalService`** — `Rent(workspace, model, tier)` / `Stop(id)` / `Report(usage)`.
  Loads the `BillingTerm`, asks whether the ask is allowed and covered, calls the
  `Provisioner`, saves the aggregate. This is `rentOne` with its policy taken out.
- **`Watchdog`** — an application service on a clock: `Live()` from the
  repository → `Provisioner.Read(id)` → `LifecyclePolicy.Verdict(...)` → apply
  the transition → on `Dead`, `Settlement` then destroy. It becomes a **loop with
  no policy in it**, which is why there can then be only one of it.
- **`PoolService`** — reconciles a workspace's pool: counts rentals where
  `Serving()`, compares to floor/schedule/load, rents or releases the difference.
  A `Dead(HostFault)` member stops counting **by construction**, which is the
  replacement gap closed at its mechanism rather than by a second reaper.
- **`SettlementService`** — turns a terminal transition into money as a pure
  function of the two value objects: `Settle(Terms, Usage) Settlement`. Bills
  `Terms.AgreedHr × Usage.Billable()`, waives when `Usage.NeverServed()`, and
  records the waste either way. Because terms are frozen and usage is measured,
  the same rental settles to the same number however long afterwards it is
  recomputed — which is what makes the ledger auditable and the "cost shown vs
  cost charged" divergence structurally impossible.
- **`CatalogService`** — serves `Product`s and quotes them: `Quote(ProductID,
  Tier, at time.Time) (Terms, error)`. The only place a live price becomes an
  agreed price, and the only writer of `Terms`.

### Ports (interfaces) out to the world

```go
type Provisioner interface {         // pkg/provision.Provider, essentially as-is
    SearchFit(Model, Selectors) ([]Offer, error)
    Rent(...) (InstanceID, error)
    Read(InstanceID) (ProviderReading, error)   // was Instance — now clearly A READING
    Destroy(InstanceID) error
}
type HealthProbe interface { Serving(Endpoint) bool }   // the model answers /v1/models
type Clock interface { Now() time.Time }
```

Renaming `Instance` → `ProviderReading` is deliberate and is half the fix: it
stops being a competing truth about the rental and becomes *what the provider
said at a moment*, which the domain interprets. The rental's state is ours; the
reading is theirs.

### Where each piece runs

| package | runs where | holds |
|---|---|---|
| `pkg/rental` | both (pure) | `Rental`, value objects, `LifecyclePolicy`, repository + port interfaces |
| `pkg/provision` | broker only | Vast client, selectors, bidding, diagnosis → `ProviderReading` |
| `gateway/billingsvc` | memdoor.ai only | `RentalService`, `Watchdog`, `PoolService`, `SettlementService`, the repository impl |
| renter-side `gateway/…` | customer's machine | report usage, render, route turns — **no policy, no watchdog** |

## Consequences

- **The pool asks the domain, not the filesystem.** `Reconcile` counts rentals
  in `Serving`, so a member in `Dead(HostFault)` frees its slot and is replaced
  by construction rather than by a second mechanism
  (`brokerMaxAttempts = 2` today, one replacement then it stops).
- **One place to calibrate.** Serve deadline, stall, crash-loop and status-fault
  rules become guards on one object with one clock. Receipts finding 2 (the
  cold-start estimate drives the serve deadline and is materially optimistic)
  becomes a change to one transition, not to a rule ladder read across two
  files.
- **Waste becomes a property of the model.** `NeverServed` is already the key to
  the waste rate (`pkg/billing/billing.go:170`) and the 6.4 % break-even. As a
  transition reason it is recorded at the moment it becomes true, by the only
  component entitled to say so.
- **The client gets simpler and less trusted.** ~870 lines of client watchdog
  mostly go; what remains is display and routing. This also fits ADR 0001's
  trust placement: anything on a user's machine is extractable by that user, so
  it should not be deciding what we bill.
- **Cost.** This is a refactor of the least-reliable, most-load-bearing
  subsystem, with no new customer-visible feature at the end of it. It is
  justified only by the 13-of-17 number: the failures are ours, and they are
  concentrated where these representations meet.
- **Migration risk.** Rentals are live money. The move must be incremental —
  the aggregate introduced beside the existing code and made authoritative one
  rule at a time, each with a receipt — never a big-bang rewrite of a system
  that is currently spending real dollars.

## Alternatives considered

- **Keep patching symptoms.** This is the status quo and it has a track record:
  the CDI class was classified (`cb1447c`), the 14-minute reap was explained
  (`a15dc48`) — both good fixes, both leaving the mechanism (a pool blind to
  non-serving members, two authorities over one machine) untouched.
- **Make the client authoritative** and let the broker follow. Rejected: the
  client is on the customer's machine, is extractable, and cannot be trusted
  with what we bill (ADR 0001), and it cannot see the pool.
- **Do multi-provider (MUST #0.3a) first.** It is nearly free — the `Provider`
  interface, the registry and cross-provider ranking already exist
  (`pkg/provision/provider.go:143`, `:220`). But it improves *price*, and the
  current failure rate is 7-for-7; a second supplier would double the surface
  over which no component owns rental state.

## Open questions

1. **Does the renter's gateway keep a local watchdog for the case where the
   broker is unreachable?** A thin client that cannot reach the broker still
   holds a paid-for endpoint. Proposal: it may report and render, and it may
   refuse to route, but it never destroys — the hard cap already bounds the
   worst case.
2. **Does `pkg/rental` own the pool, or does the pool stay separate?** A pool is
   a set of rentals with a floor; either it becomes a collection of aggregates
   or it keeps its own reconcile with the domain underneath it.
3. **What is the migration's first cut?** The most valuable single slice is
   probably "the pool counts `Serving` rentals" — it closes the replacement gap
   that MUST.md lists as priority #3 — but it depends on the aggregate existing
   first.
