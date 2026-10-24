# ADR-0020: Every request goes through a provider; a seat never supplies a key

Date: 2026-10-02 (providers, each with a list) · 2026-10-04 (Greg: "no
exception. all request for llm goes through provider"; "a seat never
supplies a model key") · Status: accepted, in force. Record:
`docs/features/PROVIDERS.md`, `gateway/providers/`.

## Context

Until September the gateway could reach a model four ways: a brain leased
from memdoor.ai's broker, OpenRouter on the person's key, a vendor key, a
local engine. Each had its own client, its own catalogue and its own idea of
a context window, and the answer to "which model is answering?" depended on
which path won at start. The broker's brain also meant memdoor.ai paid for
inference, which no plan could carry.

## Decision

- **One path.** `gateway/providers/factory.go` (`ClientFactory`) is the only
  way a chat request reaches a model. It resolves a pinned model to the
  provider that lists it, else the engine chosen at start to its provider,
  else the first provider added with `memdoor connect`.
- **A provider is a key and a list.** `gateway/providers/registry.go` builds
  the built-ins from the environment, in this order: a company's own AI
  gateway (`AI_GATEWAY_BASE_URL` + `*_AI_GATEWAY_TOKEN` + `MEMDOOR_MODEL`,
  OpenAI Responses shape, `providers/responses.go`); a vendor key
  (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, with `*_BASE_URL`
  for a company gateway in front of the vendor); OpenRouter on
  `OPEN_ROUTER_API_KEY` (`providers/byok.go`). Then every provider in
  `~/.memdoor/providers.json`, written by `memdoor connect` after a probe,
  the key kept as a value, an `ENV_NAME` or a `!command`.
- **A vendor key is that vendor and nothing else.** No OpenRouter beside
  it, no catalogue, no web search, decisions off unless a decision key is
  set (ADR-0015). `docs/SECURITY.md` lists every host the binary names and
  `gateway/egress_test.go` fails the build on a new one.
- **A seat never supplies a key.** No gateway leases a model; the broker's
  brain, its lease, its pool and its catalogue are deleted. Pro is what
  memdoor.ai hosts around the person's own key (ADR-0021), never inference.
- **`/model` is unchanged.** It answers across providers; a pin by id takes
  that provider's client. `memdoor providers` shows each provider and marks
  the one answering.
- **Context windows and output caps are data, never constants.** The
  provider's own list first, then models.dev (`providers/reference.go`,
  daily, on disk, off the network on a company key), then the provider
  default marked `~`. `memdoor providers --models <id>` says where each
  figure came from.
- **Every request is Anthropic-shaped inside.** One internal request and
  response type; each provider converts to its wire shape (Responses,
  chat/completions, Messages).

## Consequences

- A gateway with no key has no model until one is connected, and says so.
- Adding a provider is a registry entry and a converter, not a new client.
- The bill is the person's, on the key they chose; `memdoor savings` and
  `/model`'s real list prices are how Memdoor shows what it saved them.
- Nothing in the repo may add a model's window to code.
