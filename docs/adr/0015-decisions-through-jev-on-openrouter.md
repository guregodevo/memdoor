# ADR-0015: Decisions through Jev on OpenRouter

Date: 2026-09-25 · Status: accepted (Greg: "we can now use OPEN_ROUTER_API_KEY"),
on branch `decide-mlx`

## Context

The harness makes dozens of small judgments per turn — which grep hits
matter, which log lines, which tools a request needs, whether an agent
should answer a message — and today they are regexes, phrase lists, or a
full chat turn of the brain reading everything. A decision model answers
them as typed questions with a probability per option, no text to parse,
in under half a second.

Three ways to get one were tried the same day:

- **The engine in the binary** (`mlx`): works (Qwen3.5-9B answered the test
  tickets correctly), but needs the local model in memory, which the 16 GB
  policy keeps out while the rented brain serves; raw probabilities need a
  per-task temperature to be honest.
- **A local Kev server**: open weights, no key, but Kev-4B wants 32 GB in
  bf16 and scores below Jev on unseen tasks.
- **Jev through OpenRouter**: 0.3–0.5 s a call, $0.042 per million input
  tokens, calibrated by its vendor; the OpenRouter key was already in the
  developer's environment.

Every other vendor call goes through the broker, which holds the key
(ADR-0011, ADR-0012, ADR-0014). The retired-variables list named
`OPEN_ROUTER_API_KEY` as ignored.

## Decision

- **Decisions go to Jev through OpenRouter, from the gateway, with
  `OPEN_ROUTER_API_KEY`** (or `OPENROUTER_API_KEY`). With only that key set,
  the System One provider defaults to `https://openrouter.ai/api/alpha/decisions`
  and `~typesafe/jev-latest`. `MEMDOOR_SYSTEMONE_URL` / `_API_KEY` / `_MODEL`
  point it elsewhere (TypeSafe directly, a local Kev).
- **Decisions only.** Chat, eyes, ears and generated video stay on the
  broker's lease. The key is never used for a chat completion.
- **The OpenClaw contract.** `pkg/decision` mirrors OpenClaw's decision
  slot: the service picks the provider (amended 2026-09-29: no setting — a
  seat, else a decision key of the gateway's own, else off); an unavailable
  decision is a reason, never a fallback to chat.
- **Optional.** Without a seat or a decision key no decision leaves
  the machine; every consumer then behaves as before it existed (tools
  return their unjudged output, routing and the gate do nothing).

## Consequences

- **A per-token vendor bill the operator pays.** Small (a 3-question call ≈
  $0.00002, a judged grep ≈ $0.002) and not a customer price: the flat seat
  (ADR-0013) is unchanged.
- **Text leaves the machine** — the state judged: ticket text, code hunks,
  log lines, the agent's task — to OpenRouter and TypeSafe. Documented in
  `docs/features/DECIDE.md`; no seat and no decision key keeps it local.
- **Two routes, one contract.** A gateway with its own key (a developer
  machine, a self-host) calls OpenRouter directly. An app seat, which has no
  key, goes through the broker like the brain and video (ADR-0011, ADR-0014):
  `POST /v1/brain/decide?workspace=…` with the seat token; the broker holds
  `OPEN_ROUTER_API_KEY` (or `MEMDOOR_DECISIONS_KEY`), sets the model
  (`MEMDOOR_DECISIONS_MODEL`, default `~typesafe/jev-latest`), passes the
  vendor's answer and status back untouched, refuses a free workspace (402)
  and a keyless broker (501), and tallies calls and input tokens per
  workspace per month, shown in `/v1/usage` as `decisions`. The tally is
  committed every 50 calls, not on each: it is shown, not billed.
  The route: the broker when signed in, else a decision key of the
  gateway's own, else off (no setting since 2026-09-29). Built 2026-09-25, tested against a local broker.
  **To serve customers, the production broker needs the key in its
  environment and a deploy.**
- **A second vendor dependency** (TypeSafe, via OpenRouter), with no
  fallback by design: when it is down, decisions answer `unavailable` and
  the harness runs as it did before.
