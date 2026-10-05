# ADR-0017: One vendor path — every agent's chat through OpenRouter

Date: 2026-09-26 · Status: accepted (Greg: "100% OpenRouter behind the
scenes for every agent … this should simplify code and remove scope creep"),
on branch `decide-mlx`. Amends ADR-0011 (the API brain) and ADR-0016 (a
brain per agent); ADR-0014 (generated shots on DashScope) is unchanged.

## Context

Since ADR-0016 the broker served two vendor paths: the coder on DeepSeek
through OpenRouter, every other agent on Qwen3.8-Omni-Flash on DashScope
direct, each with its own key, base URL, request quirks and tests. The
per-session model routing plan (docs/roadmap/SHOULD.md) needs one catalogue
to move within, and the research found the omni model on OpenRouter, served
by Alibaba, at DashScope's own price ($0.15 in, $0.47 out, $0.016 per
cached million).

Probed through OpenRouter on 2026-09-26 with a six-second clip, using the
gateway's exact request shapes: the clip's sound track is heard (the words
quoted, "fridge" placed at about 6 s), audio input transcribes (bare base64
and data-URL forms), the text-tag tool call works, the partial-prefix
continuation continues the call, and both streaming and non-streaming
answer — DashScope direct refused non-streaming.

## Decision

- Every agent's chat goes through OpenRouter with the broker's one key. The
  omni brain is `qwen/qwen3.8-omni-flash` pinned to Alibaba, thinking off;
  the coder's is DeepSeek V4.1 Flash pinned to Fireworks then DeepInfra
  (phase 1). Every request says `data_collection: deny`.
- `OPEN_ROUTER_API_KEY` is the brain switch: set, the lease answers this
  broker; unset, no chat and the lease says so.
- `MEMDOOR_API_BRAIN_KEY` (DashScope) stays for generated shots only
  (ADR-0014). `MEMDOOR_API_BRAIN_URL` and `_MODEL` are gone.
- The gateway does not change: the clipper's client (text-tag protocol,
  eyes and ears) talks to the broker as before; it maps the model's
  OpenRouter id to the same client.

## Consequences

- One vendor path in the broker: one key, one request policy (pins, no
  training on inputs, session identity, cost per answer), one set of tests.
- The lease's model id changes to `qwen/qwen3.8-omni-flash`; the person
  sees that name in the footer and per turn.
- DashScope's free quota of 1M tokens per model is not used for chat.
- Rate limits on pinned hosts are shared with everyone on OpenRouter's key
  there; a later roadmap item.
- With one catalogue, a per-agent ladder is a list of models in the broker
  (brains.go, `agentLadders`), and a session's rung is one header: built
  the same day (SHOULD.md, phase 2).
