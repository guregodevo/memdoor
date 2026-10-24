# Model routing: how a request reaches a model

This page follows one LLM request from the agent runtime to the wire, as the
code does it on 2026-10-24. The feature pages say what each part is for
([PROVIDERS](../features/PROVIDERS.md), [DECIDE](../features/DECIDE.md),
[WEB_SEARCH](../features/WEB_SEARCH.md)) and the ADRs why
([0020, every request through a provider](../adr/0020-every-request-through-a-provider.md),
[0015, decisions on the person's own key](../adr/0015-decisions-through-jev-on-openrouter.md));
this one says how they fit, with the file and function that does each step.

## The rule

Every LLM request goes through a provider. The agent runtime never builds an
HTTP client of its own: it asks `providers.ClientFactory.GetClientFor`
(`gateway/providers/factory.go`) for the client of one agent's turn, and that
factory only ever answers with a registry provider's client
(`Provider.Client`, `gateway/providers/registry.go`). Its callers are the turn
(`gateway/agent_runtime_inference.go`), the reflection pass and the fit check,
`memdoor model check` (`gateway/model_check.go`) and a workflow's `llm` task
(`gateway/workflow_runs.go`).

Two things follow. No gateway leases a model: the engine is decided from the
gateway's own environment at every start (`selectModelEngine`,
`gateway/model_engine.go`), nothing from memdoor.ai. No seat supplies a key: a
seat changes what memdoor.ai hosts, never which key a model call goes on. With
no key and no connected provider, `GetClientFor` returns an error ("no model
is configured: set a provider key … or run memdoor connect"), `BrainStatus`
reports `none` and the window says how to connect one.

```
                 turn (agent, session, pin, tier, effort in ctx)
                                   │
                                   ▼
                 ClientFactory.GetClientFor(ctx, agent)       factory.go
                   1. pin by id  → FindModel → its provider
                   2. active engine's provider, AnsweringModel(agent)
                   3. first connected provider, first model
                                   │
                                   ▼
                 Provider.Client(ctx, model)                  registry.go
                   anthropic  → anthropicClient      anthropic.go
                   responses  → responsesClient      responses.go
                   chat       → oaiClient            oai_native.go
                   openrouter → oaiClient + policy   openrouter.go, byok.go
                   chatgpt    → chatGPT client       chatgpt.go
                                   │
                  llm.MessageNewParams in, *llm.Message out (pkg/llm)
                                   │
                                   ▼
                 upstream: the vendor, the company gateway, OpenRouter

   on the side, before and during the turn:
                 decision.Service (pkg/decision)  ── System One client
                   tool routing · message gate · jgrep/jread · effort
                   key: MEMDOOR_SYSTEMONE_API_KEY, else OPEN_ROUTER_API_KEY
```

## The provider registry

`Providers()` in `gateway/providers/registry.go` is every provider a gateway
knows, in the order `/model` shows them: the built-ins from the environment,
then the ones in `~/.memdoor/providers.json`. A `Provider` is an id, a name,
a wire (`API`: `openrouter`, `chat`, `responses`, `anthropic`, plus
`decisions` for a judge that answers no turn), a base URL, a credential, headers
and a model list. `Connected()` is true when it has a credential.

`builtInProviders()` reads the same variables the engines read:

- A company's AI gateway first: `AI_GATEWAY_BASE_URL`, a token from
  `AI_GATEWAY_TOKEN` or any variable ending in `_AI_GATEWAY_TOKEN`
  (`gatewayTokenVar`, `gateway/providers/vendor.go`), `MEMDOOR_MODEL` as the
  one declared model (the gateway's provider slug). The wire is `responses`,
  `POST <base>/v1/responses` (`gateway/providers/responses.go`), or `chat`
  when `AI_GATEWAY_API=chat`. `MEMDOOR_VENDOR_HEADERS` adds headers to every
  request (`VendorHeaders`).
- Then a vendor key: `ANTHROPIC_API_KEY` on the `anthropic` wire
  (`gateway/providers/anthropic.go`), `OPENAI_API_KEY` on `responses`,
  `GEMINI_API_KEY`, `XAI_API_KEY` (or `GROK_API_KEY`), `BASETEN_API_KEY`,
  `GROQ_API_KEY` and `DEEPSEEK_API_KEY` on `chat`, each with a `*_BASE_URL`
  that replaces the vendor's host with a proxy in front of it.
- Then OpenRouter on `OPEN_ROUTER_API_KEY` (`ByokKey`,
  `gateway/providers/byok.go`), and a signed-in ChatGPT plan
  (`chatgptProvider`, `gateway/providers/chatgpt.go`).
- Last the decision provider `typesafe` on `MEMDOOR_SYSTEMONE_API_KEY` or
  `TYPESAFE_API_KEY`, listed in `memdoor providers`, never in `/model`.

Every built-in is listed whether or not its key is set, so a locked one can
say `memdoor connect <id>`; one connected that way is kept in the file under
the same id and fills the built-in at the next start, the environment winning
when it has a key.

The engine chosen at start follows the same order (`VendorEngine` in
`vendor.go`: the company gateway, Anthropic, then the OpenAI-shaped vendors;
`MEMDOOR_VENDOR=off` skips them all; else `ByokEngine` in `byok.go`). It is
held in memory as a `RemoteEngine` (`gateway/providers/remote.go`) and names
the provider, the model per agent and the per-agent ladders. A connected
provider alone sets no engine; `connectedClient` in `factory.go` answers then.

### `memdoor connect` and the file

`memdoor connect <kind>` posts to `/api/providers/connect`
(`handleProvidersConnect`, `gateway/providers_connect.go`). `connectProvider`
probes before it keeps anything: `ModelsOf` reads the provider's list, then
`probeCall` makes one small call on the first model. With no `api` given,
`responses` is tried before `chat`. A first model the provider will not serve
the way it is asked (a quantization floor with no host, an id the key may not
use) proves the URL and the key all the same (`refusesTheModelNotTheKey`): up
to three more models from the list are tried, and if all refuse, the provider
is kept with the refusal as the answer. `--probe` keeps nothing; a success
calls `SaveProvider`, which writes `~/.memdoor/providers.json` at 0600.

The credential is written as given and resolved at read time by `resolveKey`:
a literal, the NAME of an environment variable (unset in the gateway's
environment is an error in words, never sent as the key), or `!command` whose
stdout is the key. `KeySource` says which; `memdoor providers` prints it.

## Choosing the model

The turn's context, set in `gateway/agent_runtime_process.go` from session
metadata, carries the model pinned by id (`sharedctx.ModelKey`, written by
`/model <id>` in `gateway/route_handler.go`), the tier (`sharedctx.TierKey`,
`/model 2` pins a rung) and the effort (`sharedctx.EffortKey`, below).

1. A pin by id goes to the provider that lists it: `FindModel` in
   `registry.go` reads `provider:model` first (`SplitPin`), else the active
   engine's provider when it lists the id, else the first connected provider
   that does; a signed-in ChatGPT plan wins over an `OPENAI_API_KEY` for an id
   both list. The model id keeps the provider's spelling.
2. With no pin, the active engine's provider answers on the agent's rung:
   `RemoteEngine.AnsweringModel` returns `ModelForTier(agent, tier)`, the
   agent's ladder at that rung when it has one (`byokLadders` in `byok.go`,
   coder: `z-ai/glm-5.3-flash` → `deepseek/deepseek-v4.1-flash` → `z-ai/glm-5.3`),
   else the agent's model, else the engine's. The window a turn is measured
   against and the request log read the same function, so they cannot name
   different models.
3. With no engine, the first connected chat provider on the first model it
   lists (`connectedModel`).

`memdoor model` prints each agent's ladder from `/api/llm/burst`
(`cmd/cli/cmd/model.go`). `memdoor model search <name>` and the window's
`/model-search` read `/api/models?q=` (`handleModels`,
`gateway/models_handler.go`): every connected provider's list with the prices
the provider states; OpenRouter's come from its own catalogue
(`gateway/byok_catalog.go`, wired into `providers.OpenRouterModels`), a vendor
that states none shows the reference's list price. With nothing connected the
route answers 409: there is no catalogue without a key.

## The internal shape

The agent runtime speaks one shape, Anthropic's Messages API, in Go types:
`llm.MessageNewParams` in (model, messages of content blocks, tools, system,
max_tokens, temperature, stop sequences) and `*llm.Message` out (content
blocks, `StopReason`, usage, thinking kept for display), both in
`pkg/llm/types.go`. The interface is `LLMClient.Messages().New(ctx, params)`
(`gateway/providers/interface.go`). Each wire converts at its edge:

- `anthropic.go`: `anthropicMessagesOf` and `anthropicTools` build the
  Messages request near one to one; `consumeAnthropicStream` reads the event
  stream back into a `Message`.
- `responses.go`: `responsesInput` turns the conversation into Responses
  items (`input_text`/`output_text` parts, `function_call`,
  `function_call_output`); `consumeResponsesStream` reads it back.
- `oai_native.go` with `oai_native_types.go`: `convertAnthropicToOAI` and
  `convertToolsToOAI` build a chat/completions request,
  `convertOAIToAnthropicWithTools` turns the choice back; `oai_stream.go`
  reads the SSE. This client serves the `chat` vendors and OpenRouter.

On OpenRouter the same client carries a `provider` block per request
(`byokProviderPolicy`, `byok.go`): `data_collection: deny` except on a `:free`
model, `require_parameters: true`, a precision floor, and `sort: price` unless
the person set a sort or a host order with `/model`. A vendor client sends
none of it (`newVendorClient`, `vendor.go`).

## The decision model

Decisions are typed questions answered with a probability per option
(`pkg/decision`), never a chat completion. `newDecisionService` in
`gateway/decisions.go` builds one `decision.Service` over the System One
client (`pkg/decision/systemone`), and `autoDecisionModel`
(`gateway/decision_model.go`) resolves which provider judges, per call:

- `systemOneFromEnv` takes a decision provider first, the one
  `memdoor connect typesafe` kept or the built-in on `MEMDOOR_SYSTEMONE_API_KEY`
  (`DecisionProvider`, `registry.go`); an `sk-or-` key defaults to
  `https://openrouter.ai/api/alpha/decisions` and `~typesafe/jev-latest`.
- Else the person's own OpenRouter key, `ByokKey()`, on by default.
- Never a seat or the broker. On a company vendor key (`VendorMode()`) the
  coding key is not used: only an explicit decision key judges there, else
  decisions are off and every consumer behaves as if the feature did not
  exist (`Unavailable`, `pkg/decision/service.go`).

What it decides, each consumer reading its setting per call through
`newWorkspaceSettingReader` (one query on `workspace_settings`, no restart):

- Tool routing (`gateway/tool_routing.go`, `decision_tool_routing`): one
  choice question per turn, which tool family the request needs; the family
  plus the agent's always-allowed core becomes the turn's submitted tools. It
  fails open.
- The message gate (`decisionMessageGate.ShouldRespond`, `decisions.go`,
  `decision_message_gate` on/shadow/off, `decision_gate_threshold`): should
  this agent answer a message nobody mentioned.
- Judged reads: `jgrep` (`tools/jgrep.go`) and `jread` (`tools/jev_tools.go`)
  judge each hunk or section against the task and return the few that matter;
  without a decision model they degrade to plain grep and read and say so;
  `memdoor savings` counts them (`pkg/savings`).
- The reasoning effort (`turnEffort`, `gateway/turn_effort.go`): the person's
  Shift+Tab choice wins; on auto, one choice question (low, medium, high)
  with a four-second timeout, else `high`. The effort is sent as
  `reasoning.effort` on the chat/completions wire only, and only to a model
  whose catalogue entry lists reasoning (`reasoningFor`,
  `gateway/providers/reasoning.go`; the support check is wired in
  `gateway/agent_adapter.go`).

The turn verdict, the done check, the ask gate and result acceptance consume
the same service; DECIDE.md lists their settings.

## Context windows and output caps are data

A model's window and reply cap are never constants in code. `ModelsOf`
(`registry.go`) fills them in order and records the source in
`Model.ContextSource`: the provider's own list (`provider`; each wire has a
`ModelReader` in `gateway/providers/catalogue.go`, Anthropic's `/v1/models`,
Groq's `context_window`, Gemini's native list, and so on), then the same model
in OpenRouter's catalogue (`CatalogueLimits`, `reference.go`; read on the
person's key), then models.dev (`ReferenceLimits`: exact id, then without a
date suffix, then without a vendor prefix; no family guessing), then the
provider's default, marked `default` and shown as `~`. `referenceDoc` reads
`https://models.dev/api.json` at most once a day into
`~/.memdoor/models-dev.json`; in vendor mode it reads the disk copy and never
contacts the host. `warnStaleReference` (`model_engine.go`) says at start when
the copy is a week old. `memdoor providers --models <id>` prints one
provider's list with the source column, `--audit` counts the guesses,
`--refresh` calls `ForgetAllModels` and reads everything again.

## Egress

Every host literal in non-test Go must appear in `docs/SECURITY.md`:
`TestEveryHostTheBinaryNamesIsDocumented` in `gateway/egress_test.go` walks
`gateway`, `cmd`, `pkg` and `tools` and fails on a host the page does not
name. On a company key nothing but that vendor or gateway is contacted: no
OpenRouter (`CatalogueLimits` needs `ByokKey`), no models.dev (`referenceDoc`
under `VendorMode`), no web search, no decision call without an explicit
decision key.

## Web search

`web_search` (`tools/web_search.go`) is one request to OpenRouter with its
`openrouter:web_search` server tool, on the person's key, answered by the first
rung of the coder's ladder (`providers.WebSearch`, `byok.go`, wired by
`tools.SetWebSearchBackend` in `gateway/agent_adapter.go`); no OpenRouter key, no search.
