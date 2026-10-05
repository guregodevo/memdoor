# ADR-0012 — The API brain is the only brain; the Vast path is deleted

Status: ACCEPTED (2026-09-19). Greg: **"remove vast stuff"** →
"less is more" → "less friction as well". Supersedes ADR-0010 (the pool is
serverless endpoints) and closes the renting line that began with ADR-0004.
Amended by [ADR-0017](0017-one-vendor-path-openrouter-for-every-agent.md)
(2026-09-26): the switch is now `OPEN_ROUTER_API_KEY`; the DashScope key serves
generated shots only.

## Context

ADR-0011 put an API brain behind the broker and it has served every turn
since: a film cut end to end, eyes and ears on the same model, cents a
brief, nothing to start and nothing to sweep. The Vast Serverless path it
was meant to amend went on existing beside it — an endpoint name, a
worker router, an account key, a provisioning package, four make targets
and a Python helper — and served nothing.

Two brains is not redundancy when only one of them is ever asked. It is a
second contract to keep true: a `Route` field threaded through the lease,
the engine and the vision wiring; a proxy host that could not carry a
picture, and a predicate to work around it; a lease state ("stopped")
that only a hand on the Vast console could produce; a market reader for
prices nobody pays. Every one of those was a place for a bug the live
path would never catch.

## Decision

Delete it. The API brain is the only brain this broker serves.

- `pkg/provision` (search, rent, wait, pause, resume, sweep, settle),
  `gateway/providers/vast_worker.go`, `gateway/billingsvc/serverless.go`,
  `scripts/vast_serverless.py`, `scripts/vast-summarize.sh`,
  `cmd/cli/cmd/llm_search.go` and `cmd/cli/cmd/rent_state.go` are gone.
- `Route` is gone from `leaseResponse`, `brainLeaseResponse` and
  `RemoteEngine`. A lease names this broker or it names nothing.
- `remoteCanSee` is gone: the API brain is reached through our own
  broker, which passes the body through untouched, so it can always see.
  The eyes now follow one condition — the local model is off.
- The `serverless-up|status|use|down` make targets and the
  `SERVERLESS_*`/`VASTAI` variables are gone, with the Vast key they read.
- `MEMDOOR_API_BRAIN_KEY` is the ONLY switch. With no key the lease
  answers `local` with reason `no endpoint` and says so plainly; there is
  no second path to fall back to, silently or otherwise.

## Consequences

**There is no fallback.** If Alibaba is down or the key lapses, there is
no GPU path behind it: the app says it has no brain and uses the local
model where there is one. That is the cost, stated once and accepted —
the fallback we deleted had not served a request in the life of the API
brain, and an untested fallback is not one.

Getting a GPU path back means renting again, which is ADR-0004 through
ADR-0010 in reverse, or pointing `MEMDOOR_API_BRAIN_URL` at a second
vendor with the same OpenAI shape. The second is a line of env; the first
is the thing this ADR deletes.

**What stays.** The lease, the seat, the held seconds the flat plan bills
on, the Raft replication of the lease book, metering per workspace: all
untouched. The pool's shape was never the money — the seat is.
