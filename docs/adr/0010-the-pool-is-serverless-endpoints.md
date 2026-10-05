# ADR-0010 — The pool is serverless endpoints; the broker stops renting machines

Status: SUPERSEDED (2026-09-19) by
[ADR-0012](0012-the-api-brain-is-the-only-brain.md), which deletes the
Vast Serverless path entirely; the API brain of
[ADR-0011](0011-an-api-brain-through-the-broker.md) is the only brain.
The record below is kept for why renting was dropped — that reasoning
still holds and is the reason nothing rents today.

Status when accepted: ACCEPTED (2026-09-15). Greg: "can we try serverless?" →
"vast.ai has serverless" → "yes exactly what we want" → "it's for coder
and clipper, for the whole stack" → **"serverless is now our goto
offer"**.

## Context

ADR-0007 made the broker the only renter; ADR-0008 gave every workspace
one shared pool of on-demand 27B members, four seats per member. The
broker therefore owns the whole machine lifecycle: search offers, rent,
wait for vLLM, heartbeat, pause when idle, resume on a lease, sweep
orphans, settle. That code is where the failures live, measured on the
broker's own `/billing/v1/waste` on 2026-09-15:

- **36.8 % of rentals never served** (35 of 58), against a 6.4 %
  break-even — margin −32.5 %.
- A paused member nobody swept, still listed after its instance was gone.
- No client-side stop: the lease id lives only in gateway memory, so a
  killed gateway cannot release its brain (30 min of idle billing).
- A stopped member lost its GPU to another renter; the endpoint could
  not replace it and the brain was gone for 20 minutes.

Vast Serverless was measured the same day end to end: an endpoint, a
workergroup on our own template, our vLLM build and our price cap. It
rents, benchmarks, replaces and stops workers itself, at the same
per-second price as an instance with no markup. Receipts: 8 min to
routable on a warm host (23 min on a slow one), TTFT 0.84 s, ~47 tok/s
against the A100 member's ~27, six Hacker News shorts made on it,
$0.12 of GPU per film, 171 s from a stopped worker back to ready.

## Decision

**The pool is Vast Serverless endpoints. The broker stops renting
machines.** The division of labour:

| Job | Owner |
|---|---|
| Rent, benchmark, replace, stop workers; autoscale | Vast Serverless (endpoint + workergroup) |
| Which GPU class, which price ceiling, which vLLM build and args | our template (`search_params`, `env`), version-controlled in the Makefile |
| Identity, seats, plan, quota, billing, receipts | our broker |
| Which endpoint a workspace's brain class resolves to | our broker — the lease response returns an endpoint URL, not a machine |
| Turns, tools, media (ffmpeg, whisper, voice) | the gateway on the user's own machine |

Consequences:

1. The broker's `rentMember` / `setRunning` / pause / resume /
   orphan-sweep path is deleted. `brainLease` returns
   `https://openai.vast.ai/<endpoint>` and its token; the client already
   speaks that API (it is OpenAI-shaped; the path has no `/v1`).
2. The shared pool is ONE endpoint per brain class, `max_workers`
   following the load, `min_load 0`, a short idle tail. A dedicated
   endpoint per workspace is one `create endpoint` call — that is the
   enterprise tier, not a new architecture.
3. `poolPerMember = 4` becomes the autoscaler's concern (`target_util`,
   `max_queue_time`). Our concurrency figure stays a sizing fact: a
   48 GB card holds 2 clipper sessions at 65k, ~4 coder sessions.
4. Waste is re-measured after the cut. It should fall to the
   autoscaler's own failure rate; if it does not, the cause is our
   template, not our timing.
5. ADR-0008's pooling intent stands (one shared pool, seats, no bids,
   one good-enough model). Only the implementation of "who rents"
   changes.

## What this does not fix

- **Cold start, 8–23 min.** The autoscaler cannot make weights download
  faster. Capacity is a schedule (warm before the working day), never a
  reflex. A stopped worker's 171 s restart is a gamble on a busy host —
  its GPU can be taken.
- **One endpoint, one model.** A second brain class is a second
  endpoint.
- **The proxy is in the path.** It answers 502/503/504 under load; the
  client retries three times with growing pauses (2026-09-15).

## Alternatives rejected

- **Keep renting ourselves.** It is the code with the worst receipt in
  the project and it duplicates, badly, what the marketplace now sells.
- **Per-token hosting (OpenRouter et al.).** Cheapest under ~16 users
  and kept as the overflow lane, but it is someone else's
  infrastructure and someone else's trust boundary — the opposite of
  the product's claim (ADR-0001, the local-only pivot).
- **A vendor subscription as supply** (GLM Coding Plan, $18–84/mo).
  Cheapest on paper, converges with per-token once sized for a fleet's
  concurrency, and reselling one account's quota is outside its terms:
  the failure mode is every customer offline in the same hour. It
  belongs on the customer's side of the line, as a bring-your-own lane.

## Receipts

`docs/internal/SERVERLESS_MATH.md` — the day's measurements, the cost
per film by pattern, the per-user unit economics (81 → 91 % gross
margin), the market depth at each price cap, and the ten-user case.

## Built, 2026-09-17

The broker's half, which was the half that never existed. Greg, looking at
two GPUs billing for one app: "aren't we using serverless? to avoid having
orphan" — and he was right that we were not. The decision was two days old
and no line of broker code knew the word.

- `MEMDOOR_SERVERLESS_ENDPOINT` is the whole switch. Set it and no machine
  is rented for anybody: the lease names the endpoint, is READY at once
  (there is nothing to start), and still carries the seat and the held
  seconds, which is what the flat plan bills on.
- `POST /v1/brain/route` is how the key stays here. The worker protocol
  wants the account key, and that key is the whole account — so the broker
  makes the routing call and returns the autoscaler's answer verbatim,
  after checking the lease belongs to the workspace asking. The inference
  then goes STRAIGHT to the worker, which is ADR-0002 unchanged: we route,
  we do not proxy.
- The gateway holds no key on this path at all. A lease that names a route
  wires `providers.BrokerRoute`, and the patient retry that already knew
  "no worker yet, one is loading" applies to the brokered answer too.

The renting path is still here, and still the default when no endpoint is
named. Deleting it is the next step, and it is a bigger one: the pool
reconciler, the watchdog, the settlement and the waste ledger all hang off
it.
