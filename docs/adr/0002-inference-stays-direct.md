# ADR 0002 — Inference goes straight to the rented GPU, not through memdoor.ai

Status: ACCEPTED (2026-08-20) · Extends ADR 0001 · Implemented in
`gateway/providers/remote.go`, `gateway/billingsvc/broker.go`

## Context

After ADR 0001 the client's transactional surface is entirely memdoor.ai:
catalog, prices, deploy, status, logs, teardown, credits, login. That
raised the obvious next question — should inference go through
memdoor.ai too, making it an OpenRouter-shaped gateway with one stable
URL in front of every provider?

Today the gateway talks straight to the rented instance
(`http://<ip>:<port>`) with a token minted for that rental alone. Only
the transactional calls go to memdoor.ai.

## Decision

**Inference stays direct.** memdoor.ai brokers the rental; it does not
carry the traffic.

## Why

- **The privacy claim is load-bearing and true today.** The site and
  docs say prompts go to the machine you rented rather than into a
  vendor's logs. Proxying would make that false, and it is one of the
  few claims a buyer can verify (they can watch the connection).
- **Bandwidth lands on the wrong box.** The VPS is a 4 GB web host, not
  a token conduit. Every prompt and completion of every customer would
  cross it, at our cost, sized for a marketing site.
- **It would add a single point of failure** in front of rentals that
  currently keep working whatever memdoor.ai is doing — and an outage
  would look like every GPU dying at once.
- **Latency on every call**, on a product whose pitch is that an agent
  loop is cheap: the loop is exactly what a proxy hop taxes.

## What we give up

A proxy would work on locked-down corporate networks that block
outbound connections to arbitrary IP:port — which is precisely the
"locked corporate laptop" wedge. Direct inference may simply fail
there, and we will not know until a real user hits it.

## Revisit when

A real user is blocked by their network. That is the signal that buys
the bandwidth cost and the changed claim — not a hypothetical. If it
happens, build it as opt-in (`--via-memdoor`) rather than a flip, so
the privacy property survives for everyone who does not need the
workaround.
