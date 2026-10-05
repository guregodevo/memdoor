# ADR-0011: an API brain, served through the broker

Date: 2026-09-18. Amends ADR-0010 (the pool is serverless endpoints),
which [ADR-0012](0012-the-api-brain-is-the-only-brain.md) then
superseded: this is now the only brain, with no GPU path behind it.
Amended by [ADR-0017](0017-one-vendor-path-openrouter-for-every-agent.md)
(2026-09-26): the same model, served through OpenRouter, one key for every agent.

## Context

Qwen3.8-Omni-Flash (released 2026-09-18) does the editor's whole job —
hears audio, watches video up to two hours, plans, calls tools — and has
no open weights: it exists behind Alibaba Model Studio's OpenAI-compatible
endpoint at $0.15/M input, $0.47/M output. On today's traffic (8M input
tokens a day for five films) that is about a dollar a day, cents a brief,
with no cold start and nothing to sweep.

The key is the account, like the Vast key (ADR-0010), and must never
reach a client. ADR-0002 put inference on the worker directly; with a
vendor's endpoint there is no worker to route to, only a key to hide.

## Decision

The broker is the endpoint. `MEMDOOR_API_BRAIN_KEY` set on memdoor.ai is
the switch: the lease answers `endpoint: <broker>/v1/brain/chat/completions`,
`brokered: true`, the model and a window, and **no token**. The gateway
sends its chat completions there with the sign-in it already holds. The
broker checks the seat and the plan, rewrites the request for the vendor
(its model, `reasoning_effort: none`, vLLM-only fields dropped, usage
asked for), forwards with the key, streams the answer back verbatim, and
meters input/output tokens per workspace in the registry (`api_tokens`).

nginx streams that location unbuffered with a turn's worth of patience.
`/v1/health` says which brain the lease answers (`api_brain`,
`serverless`). Unset the key and the serverless endpoint serves again.

## Consequences

- memdoor.ai is in a turn's data path (it is not for the serverless
  route): a request is the turn's context, a few hundred KB; the reply
  streams. The 4 GB box carries that.
- The seat's cost model is per token at the vendor, flat to the customer;
  `api_tokens` is what a cap or the usage page reads.
- The window told to the app is 256k, not the model's 1M: every token is
  paid for, and the compaction ladder is what keeps a session affordable.
- The tool-call format is the open question: the app renders tools as
  Qwen XML with a vLLM grammar; the vendor rejects the grammar (the app
  already retries without it) and the model must honour the XML unprompted.
  If it does not, the fix is sending OpenAI `tools` on brokered engines.
