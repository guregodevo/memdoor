# ADR-0016: A brain per agent

Date: 2026-09-25 · Status: accepted (Greg: "coder with deepseek flash and
clipper with omni (no change), through broker"), on branch `decide-mlx`

## Context

One brain served every agent (ADR-0011, ADR-0012): Qwen omni on DashScope.
The clipper needs omni's eyes and ears; the coder needs none of that and
pays omni's price on ~10 000-token turns. The per-provider design that could
route an agent elsewhere existed and was collapsed when MLX became the only
path (37fe96e4, 7ff51069).

## Decision

- **The broker picks the brain per agent** (`billingsvc/brains.go`): a
  `brain` interface (omni, OpenRouter) and `agentBrains`, the agent → brain
  map, in code. `coder` → `deepseek/deepseek-v4.1-flash` on OpenRouter with
  the broker's `OPEN_ROUTER_API_KEY` (ADR-0015); `clipper` and every other
  agent → omni, unchanged; the vision/hearing helpers name themselves
  `eyes` → omni. A request that names **no** agent — every client built
  before the header — gets DeepSeek (Greg: "deepseek by default"); a
  **named** agent missing from the map, or a brain whose key the broker
  lacks, is refused (400) with the reason.
- **The gateway names the agent** on each brokered request
  (`X-Memdoor-Agent`) and learns each agent's model from the lease
  (`agent_models`).
- **The provider is the per-model implementation** (`LLMClient`), built by
  `ClientFactory.GetClientFor(agent)` from the model map in
  `providers/model_providers.go`, no default: omni → the brain client
  (Hermes tool protocol), DeepSeek → the OpenAI-compatible provider restored
  from before 37fe96e4 (native tool calls, OpenRouter's upstream hook),
  pointed at the broker. Token usage is logged and metered as before.

## Consequences

- Clients installed before this change send no agent header, so on this
  broker their clipper runs on DeepSeek: text only (no eyes or ears), and
  their forced tool-call prefill is the path that produced DeepSeek's own
  markup in testing. Upgrading the app restores omni for the clipper.
- Adding an agent means a line in `agentBrains`; adding a model means a line
  in `modelProviders` (and, for a new vendor, a `brain` implementation).
- The OpenAI-compatible provider streams (restored from before 13e5cd4f):
  the coder's replies type live, the prose breaker applies, a cut reply
  reads as max_tokens. First token ~1.1 s on DeepSeek Flash.
- Live through a local broker: the coder on DeepSeek made native tool calls
  and answered correctly (27 s, ~10k tokens a call, metered); the clipper
  went to omni. **The production broker needs the OpenRouter key and a
  deploy** before any seat sees this.
