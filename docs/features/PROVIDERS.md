# Providers — each with its list of models

The maintainer, 2026-10-02: "we had an OpenRouter integration … we can continue to
extend it with different providers including the company provider … each
provider has a list of models actually … anthropic as well … openai as
well etc. … let's not change the slash commands /model and /model-search,
let's make it consistent." And, as the brief: "you are a UX designer, check
the best experience — omp, also Cursor."

## The verdict (what the best experience is)

Four products were read for how a developer connects a model provider,
especially a company's own AI gateway:

| | Setup | Verify | Models from the gateway | Where the credential came from | Custom provider |
|---|---|---|---|---|---|
| **Cursor** | Settings → Models, paste a key, Save | none; requests fail later | no (built-in models only) | not shown | no base URL in the docs |
| **Claude Code** | env vars or a settings `env` block; managed settings for an org | a curl the person runs; `/status` shows base URL + credential source | `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1` adds the gateway's list to `/model` | `/status` names the variable | `ANTHROPIC_BASE_URL` + token; custom headers; `apiKeyHelper` for rotating tokens; a flag kills non-gateway traffic |
| **Codex** | `config.toml` `[model_providers.<id>]`: `base_url`, `env_key`, `wire_api = "responses"`, headers | none | no | no | yes, file only |
| **omp** | `/login` picker; `models.yml` providers each with `api` and `models`; discovery types | a probe per built-in provider (1-token call or `GET /models`); **none for custom providers** | `discovery: openai-models-list` | the picker shows `env: X` / `login` | hand-edit YAML; no command, no test |

What the best of them has, and nobody has all of: **providers each with a
list** (omp, Codex), **a credential whose source is shown** (omp, Claude
Code), **discovery from the gateway's own list** (Claude Code, omp), **a
verify step before anything persists** (Claude Code, by hand), and
**secrets from the environment or a command** (omp/pi's `!cmd`, Claude
Code's `apiKeyHelper`). What nobody has: one command that adds a custom
provider *and* tests it.

## What Memdoor does (shipped 2026-10-02)

- **A registry** (`gateway/providers/registry.go`): a provider is an id, a
  name, a wire (`openrouter` | `chat` | `responses` | `anthropic`), a base
  URL, a credential, headers, and its models. The built-in entries come from
  the environment exactly as the engines already read it — the company
  gateway (`AI_GATEWAY_BASE_URL`), Anthropic, OpenAI, Gemini, OpenRouter —
  listed whether or not a key is set, so a locked one can say `memdoor
  connect anthropic`. The ones a person adds live in
  `~/.memdoor/providers.json` (0600; the key is written as given — a
  literal, an env var's NAME, or `!command` — never resolved onto disk).
- **Each provider's list from its own endpoint**: OpenRouter's catalogue
  with real prices (unchanged), Anthropic's `GET /v1/models`, OpenAI's and
  Gemini's and a gateway's OpenAI-shaped `/models`; cached an hour; a model
  declared by hand only when a gateway returns none (the company gateway's
  own rule). `/api/models` answers `{"providers":[…each with models…],
  "models":[flat]}` — the flat list is what `/model search` and the picker
  already read, so **`/model` and `/model search` are unchanged** and now
  answer across every connected provider.
- **A pin lands on its provider**: `/model <id>` with an id another
  provider lists takes that provider's client for the conversation
  (`gateway/providers/factory.go: GetClientFor`), the active provider winning a tie.
- **`memdoor connect`** (`gateway/providers_connect.go`,
  `cmd/cli/cmd/providers.go`): pick the kind — company AI gateway,
  Anthropic, OpenAI, Gemini, any OpenAI-compatible endpoint — base URL
  prefilled, the token read without echo (or `--key`, as a value, an env
  var NAME, or `!command`), then the gateway **probes before it keeps**:
  the provider's model list, then one small call; `responses` is tried
  before `chat` when the kind does not say. A failure is an answer in
  words with advice (a refused token, a wrong path, a page instead of an
  API). `--probe` tests and keeps nothing; `--remove` forgets one.
  `memdoor providers` lists them with ● on the one answering and where
  each credential came from.

## Receipts (the maintainer's gateway, 2026-10-02)

- `memdoor providers`: anthropic / openai / gemini *not connected · memdoor
  connect <id>*, `● openrouter connected (env OPEN_ROUTER_API_KEY)`.
- `memdoor model search glm`: 18 models with prices — the same answer as
  before the registry.
- `memdoor connect custom --probe --base https://openrouter.ai/api/v1 --key
  OPEN_ROUTER_API_KEY --model z-ai/glm-5.3-flash`: `✓ orx connected ·
  responses API · 465 models · tested z-ai/glm-5.3-flash → "(a reply with
  no text)" · probe only, nothing kept` — the key given as a variable's
  NAME, never printed. (GLM spent the probe's cap reasoning; a 200 with
  tokens counted still proves URL, token and shape.)
- A wrong token: `✗ not connected: OpenRouter refused the key (HTTP 401)
  … the URL answers but refused the token: check it is the gateway's own
  (a personal or service-account token), not a vendor key`.
- A wrong path: `✗ … HTTP 404 … nothing at that path: the base is usually
  the part before /v1`.
- Tests: `TestProvidersAreBuiltInsThenTheFile`,
  `TestModelsOfReadsEachProvidersList`, `TestSearchModelsAndFindModel`,
  `TestConnectProbesThenKeeps`.

- **`/connect [kind]` in the window** (`cmd/tui/ui/connect_flow.go`): the
  kind picker (1-5, the ask_user_question picker's keys), the base URL
  prefilled in the input line, the token typed into it but drawn as dots,
  a model asked only for a gateway or a custom endpoint, "Probing …", then
  the ✓ line in the conversation with the sample of models and what to do
  next; a refusal with its advice; Esc cancels at any step. Connecting is
  the gateway operator's alone, on the gateway's machine (`gatewayOperator`,
  2026-10-30: the route answered anyone, and a `!command` key runs there);
  a window on a laptop connects through the ssh door. The maintainer: "omp
  does it with a slash command right?" — omp's `/login`, with the probe
  omp lacks. Receipt in a tmux window on the maintainer's gateway: `/connect` → 5 →
  `orx` → OpenRouter's base → the key as `OPEN_ROUTER_API_KEY` (shown as
  19 dots) → `z-ai/glm-5.3-flash` → `✓ orx connected · responses API ·
  465 models … kept`. `TestConnectFlowPicksTypesProbesAndSays`.

- **The first real Anthropic turn** (the maintainer's key, 2026-10-02): `memdoor
  connect anthropic --probe … --model claude-haiku-4-5-20251001` → `✓ 13
  models · tested claude-haiku-4-5-20251001 → "ok"`; `memdoor providers`
  → `anthropic connected (env ANTHROPIC_API_KEY)` beside `● openrouter`
  (`MEMDOOR_VENDOR=off` keeps OpenRouter the active engine); in a window
  `/model claude-haiku-4-5-20251001` → *pinned by you (off the ladder)*,
  the footer names it, a turn answers `via anthropic`, `answered by
  claude-haiku-4-5-20251001`, the log line `agent=coder
  model=claude-haiku-4-5-20251001 finish=end_turn`. The pin had been
  refused first — `/model` only took `vendor/name` ids — so the id rule is
  now "anything a connected provider lists" (`route_handler.go
  modelIDRe`, `tui_ops.go looksLikeModelID`), the catalogue check
  deciding whether one does.

- **The footer names the provider** (`provider · model`): `openrouter ·
  z-ai/glm-5.3-flash · rung 1/3 · first rung`, after `/model
  claude-haiku-4-5-20251001` → `anthropic · claude-haiku-4-5-20251001 ·
  pinned by you`, after `/model auto` back. The route view carries
  `provider` (the active engine's, or the one that lists a pinned id);
  `TestRouteViewNamesTheProvider`.

- **Streaming on the `anthropic` and `responses` wires**
  (`gateway/providers/sse.go`, `consumeAnthropicStream`, `consumeResponsesStream`):
  text deltas reach the window as they are written, tool-call JSON is
  assembled from its fragments, thinking is kept for display, the prose
  breaker (`llm.StreamGuard`) can stop a reply, a mid-stream error comes
  back in the vendor's words, and a proxy that answers one JSON object
  although a stream was asked is read as that object. Tests serve the
  real event streams.
- **Groq, xAI Grok and Baseten** as built-in providers and `/connect`
  kinds (`GROQ_API_KEY`, `XAI_API_KEY`, `BASETEN_API_KEY`, each with a
  `*_BASE_URL`), all on the OpenAI shape; `memdoor providers --models
  groq` prints one provider's own list (Groq's `context_window` read).
- **`provider:model`**, the explicit pin (`/model groq:qwen/qwen3.8-27b`)
  for an id two providers list — a bare id goes to the active provider,
  the prefix names one; the footer and the route view show the bare id
  under its provider.

- **A workflow's tasks can name their model** (`model:` on `agent` and
  `llm` tasks, as `/model` names them; the runner pins the task's session).
  A three-step digest on `groq:qwen/qwen3.8-27b`: the `llm` summary
  answered on Qwen, the `agent` write produced DIGEST.md, then Groq's free
  tier refused the next call with `HTTP 429 … service tier on_demand` after
  two retries at 2 s and 5 s. So: **`Retry-After` is honoured** on every
  wire (seconds or a date, capped at a minute), and a **pinned model's
  context window comes from its provider** (a Groq pin at 131k had been
  measured against OpenRouter's 262k).
- **Turns on Groq building a workflow** (the hero sentence): Qwen 27B
  loaded the skill and the tool, then wandered (`find`, `git log`) and ended
  in a picker offering to skip; `openai/gpt-oss-120b` did the digest inline
  (`git log` → `Edit(DIGEST.md)`) and never called the skill. Both are model
  compliance with the coder prompt, not provider faults; GLM 5.3 Flash and
  Haiku follow it. Worth a conformance probe line ("builds a workflow when
  asked") in `memdoor model check`.

- **`memdoor model check` has a `follows` column**: told, in a short
  system prompt, to load the workflow skill first when asked for a
  workflow, with `skill`, `bash` and `read_file` offered — did it? The pin
  warning says so (*"skipped an instruction to load the workflow skill
  first … a workflow ask may run inline on it"*); a report from before the
  probe shows `?`. Probing the four: gpt-oss-120b ✓, Qwen 27B ✓, Haiku ✓,
  GLM 5.3 Flash ✓ — so gpt-oss follows the short instruction and skipped
  it inside the 9.7k-char coder prompt with 16 tools: salience in the
  full prompt, not capability. The probe also caught a real Anthropic
  client bug: a tool declared with properties alone (no `type: object`)
  was refused by the Messages API (`tools.0.custom.input_schema.type:
  Input should be 'object'`); every schema is typed now.

- **Context windows, dynamically** (the maintainer: "make sure you have the correct
  context size; if we can get it dynamically that's better"): a provider's
  own figure first (OpenRouter's `context_length`, Groq's
  `context_window`), then **models.dev** — the public catalogue, read at
  most once a day, kept in `~/.memdoor/models-dev.json`, never contacted in
  company-key mode (the disk copy answers there) — then the provider's
  default. Anthropic's list states no window: the reference corrected
  Sonnet 5, Opus 4.6 and Fable 5.1 from the 200k default to 1M, Haiku
  stays 200k. Footer receipts: `claude-haiku-4-5-20251001 → 6.1k/183.6k`,
  `claude-sonnet-5 → 7.0k/983.6k`, `groq:qwen/qwen3.8-27b → 114.7k`,
  `z-ai/glm-5.3-flash → 1032.2k` (each = the window minus the 16k reply
  reserve). The reference's output cap is wired too: the per-call cap
  never exceeds the model's own (`qwen/qwen3.8-27b` 16k, Haiku 64k, Sonnet
  5 128k, gpt-oss-120b 65k — `memdoor providers --models <id>` shows the
  column), and the cut-off escalation stops at it
  (`ctxmgmt.SetRemoteOutput`, `ModelLimits.MaxOutputCeiling`).

- **Each provider's own metadata API, behind one interface** (the maintainer:
  "check if each provider has an API that provides model metadata … add
  this in our provider interface, getModel"). `ModelReader`
  (`gateway/providers/catalogue.go`): `ListModels` and `GetModel`, one reader per
  API read live with keys on 2026-10-02 — Anthropic `/v1/models`
  (`max_input_tokens`, `max_tokens`, capabilities: image, thinking,
  effort; a per-model endpoint), Groq (`context_window`,
  `max_completion_tokens`, prices, modalities), Gemini's native
  `/v1beta/models` (`inputTokenLimit`, `outputTokenLimit`, `thinking`;
  embeddings left out), xAI (`context_length` on `/v1/models`, modalities
  from `/v1/language-models`), OpenRouter (the catalogue, now with
  `top_provider.max_completion_tokens`). **OpenAI is the one vendor whose
  API states nothing** (`id`, `owned_by`, `created`, `shutdown_date`), so
  its windows come from the same model in OpenRouter's catalogue (an API:
  `openai/gpt-5-codex`, matched by normalized id), then models.dev, then a
  marked guess — no family heuristic. Every figure says where it came from
  (`context_source`: provider | catalogue | reference | declared |
  default), a guess is warned once per model in the log and shown `~`, a
  week-old reference is warned at startup, `memdoor providers --refresh`
  re-reads, `--audit` counts: **596 models across six providers, 94%
  known** — Anthropic 13/13, Gemini 44/44, Groq 12/12, OpenRouter 380/380
  from the provider; xAI 11 + 3; OpenAI 68 catalogue + 31 reference + 34
  guesses, the 34 being audio, image, realtime, embedding and 3.5-era ids.
- **OpenAI and xAI proven live**: gpt-5 refused the probe with
  `Unsupported parameter: 'max_tokens'` — OpenAI's newer models want
  `max_completion_tokens`, now sent on OpenAI's host and retried under
  that name when any host asks; then `openai · gpt-5 · pinned by you` did
  the small fix (+2 −2, tests green), and `xai · grok-4.3` did it too (the
  key is `GROK_API_KEY` in the maintainer's .envrc, accepted as the alias of
  `XAI_API_KEY`). Grok and Groq are different companies; both are in.

## The fast-model round (the maintainer, 2026-10-02: "test each provider with haiku equivalent model")

The same turn on each provider's fast model — a planted off-by-one in
`sum.go` with a failing test, "fix the bug (not the test)" — ground truth
`go test` and `git diff`:

| Provider · model | Steps | Time | Result |
|---|---|---|---|
| OpenRouter · `z-ai/glm-5.3-flash` (ladder) | Read, Edit, Bash | ~30 s | ok, +1 −1 |
| Anthropic · `claude-haiku-4-5-20251001` | notes, Bash, Read ×2, Edit, Bash | ~45 s | ok, +1 −1 |
| OpenAI · `gpt-5-mini` | notes, Bash, glob, Read ×3, Edit, Bash | 16 s | ok, +1 −1 |
| Groq · `openai/gpt-oss-20b` | Bash, Read ×2, Edit, Bash | **8 s** | ok, +3 −1 |
| xAI · `grok-4.20-0309-non-reasoning` | Read ×2, Bash ×2, Edit (one miss, then applied), Bash | 16 s | ok, +2 −2 |
| Gemini · `gemini-2.5-flash` | — | — | **HTTP 404**: "no longer available to new users" (the list still names it) |
| Gemini · `gemini-3-flash-preview` | Bash ×2, Read ×2, then — | 72 s | **HTTP 429**: "exceeded your current quota, check your plan and billing" — the key's account, after three retries |
| Gemini · `gemini-2.5-flash-lite` | — | 4 s | HTTP 404, as above |
| Gemini · `gemini-3-flash-preview`, billing on | Bash ×2, Read ×2, Edit, Bash ×2 | 16 s | ok, +2 −2 |
| DeepSeek · `deepseek-flash` | notes, glob, Read ×2, Bash, Edit, Bash ×2, notes | 16 s | ok, +2 −2 |
| Baseten · `zai-org/GLM-5.3-Flash` | notes, Read, Bash, Edit ×2, Bash ×2 | 12 s | ok, +2 −2 |

Two harness fixes came out of the round: OpenAI's newer models want
`max_completion_tokens` (sent on OpenAI's host, retried under that name
when any host asks), and Gemini 3's thinking models return a
`thought_signature` with each function call under `extra_content.google`
and refuse the follow-up without it — kept by call id and replayed
(`TestToolCallExtraContentIsReplayed`); with it, Gemini 3 Flash got four
steps in before its quota ran out. The maintainer enabled billing on the key and Gemini 3 Flash then did the fix in
16 s: all six providers have the same receipt.

- **DeepSeek and Baseten** (the maintainer: "DEEPSEEK and BASETEN providers as
  well"), read live with keys: DeepSeek's `/v1/models` states
  `context_window`, `max_output_tokens`, `effort`, modalities (two models,
  1M windows); Baseten's states `context_length`, `max_completion_tokens`,
  prices, modalities and `supported_features` (eleven tool-capable models,
  GLM 5.3 Flash at $0.15/$0.50). Readers for both; DeepSeek is a built-in
  provider and `/connect` kind (`DEEPSEEK_API_KEY`). The Baseten turn
  caught a bug: the window lower-cased every `/model` argument and
  Baseten's ids carry case (`zai-org/GLM-5.3-Flash` → refused by its
  host); keywords still read lower-cased, the id keeps its case and takes
  the catalogue's own spelling. Audit after: **609 models across eight
  providers, 94% known**, every window on seven of them from the provider
  itself.

- **EUrouter** (2026-11-08, #54): a built-in provider and `/connect` kind
  (`EUROUTER_API_KEY`, base `https://api.eurouter.ai/api/v1`). Its
  `/api/v1/models` is OpenRouter's shape plus `supported_api_endpoints` and a
  currency per price. It lists 159 models: speech, embeddings, rerank and
  chat. The reader keeps the 98 chat models that call tools. 55 of the 159
  are priced in euros. A price is kept in its own currency (`Model.Currency`,
  blank for dollars, `pkg/price`) and shown with its sign, so
  `mistral-small-3.2-24b` reads `€0.09 / €0.28` next to OpenRouter's
  `$0.07 / $0.20`. `memdoor savings` compares dollar prices only. EUrouter is
  last in the vendor order: `EUROUTER_API_KEY` + `MEMDOOR_MODEL` alone run a
  gateway where EUrouter is the only host contacted. Its key (`eur_…`) has
  no shape `pkg/modelkey` checks, so it is found under its name. The request
  sends the bare id; the answer names the EU host that served it
  (`melious/`, `tensorix/`, `aki/`, `berget/`). Every request, the model
  list too, names the app the way EUrouter's rankings read it
  (`HTTP-Referer` + `X-EUrouter-Title`) and carries the `MEMDOOR_ATTRIBUTION`
  fields, connected or as the company's key (the chat request:
  `TestATurnOnEURouterCarriesTheAttribution`). EUrouter cannot be asked
  which app a call was counted under: its `/api/v1/generation?id=` found
  none of ours.

  The fast-model task (an off-by-one in `sum.go`, a failing test, "fix the
  bug (not the test)") on two of its models, pinned in the window:

  | Provider · model | Steps | Time | Result |
  |---|---|---|---|
  | EUrouter · `deepseek-v4.1-flash-1017` ($) | Bash ×2, Read ×2, Edit, Bash | 11 s | ok, +1 −1 |
  | EUrouter · `mistral-small-3.2-24b` (€) | notes, locate, Read, Edit, Bash | 11 s | ok, +1 −1 (indented with spaces) |

- **LLM Gateway** (2026-11-09, #53): a built-in provider and `/connect` kind
  (`LLMGATEWAY_API_KEY`, base `https://api.llmgateway.io/v1`). One key reaches
  many vendors over chat/completions. Its `/v1/models` is OpenRouter's shape
  plus `architecture.output_modalities`, `max_output` and `deactivated_at`.
  It lists 291 models: chat, image, video, speech, embeddings, rerank, and
  the `auto` and `custom` routes with no window. The reader keeps the 177
  that answer in text only, call tools, state a window and are not switched
  off. Prices are dollars per token, some in exponent form (`0.2e-6`). The
  key has a shape (`llmgtwy_…`), so `memdoor connect` finds it under any
  name. LLM Gateway is last in the vendor order: `LLMGATEWAY_API_KEY` +
  `MEMDOOR_MODEL` alone run a gateway where it is the only host contacted.
  Every request carries `X-Source: memdoor.ai`, the header its apps page
  reads (`TestATurnOnLLMGatewayCarriesXSource`). The request sends the bare
  id; the answer names the host that served it (`greenference/`,
  `fireworks/`).

  A new account answers 402 (`billing_error`) until it has credit. The probe
  and the turn said to add credit at openrouter.ai: every chat/completions
  refusal was worded as OpenRouter's. A 402 or a refused key now names the
  host that refused it (`providerRefusal`, `noCreditAnswer`); OpenRouter's
  wording is unchanged.

  The fast-model task, pinned in the window:

  | Provider · model | Steps | Time | Result |
  |---|---|---|---|
  | LLM Gateway · `glm-5.3-flash` | Edit, Bash ×3 (`go` was not on the test gateway's PATH) | 11 s | ok, +1 −1 |
  | LLM Gateway · `deepseek-v4.1-flash` | Read ×2, Edit, Bash ×2 | 8 s | ok, +1 −1 |

- **Vercel AI Gateway** (2026-11-09, #56): a built-in provider and
  `/connect` kind (`vercel`, base `https://ai-gateway.vercel.sh/v1`). The key
  is `AI_GATEWAY_API_KEY`, Vercel's name, or `VERCEL_API_KEY` when it holds
  a `vck_…` key (that name also holds Vercel deploy tokens, which run no
  model and stay in the agent's commands). `VERCEL_BASE_URL` replaces the
  base. Vercel's key name starts like the
  company gateway's variables (`AI_GATEWAY_BASE_URL`, `*_AI_GATEWAY_TOKEN`)
  and neither reads the other's. With `AI_GATEWAY_BASE_URL` set the company
  gateway answers, on its token only. Without a token the reason names
  Vercel's key as the wrong one
  (`TestTheCompanyGatewayAndVercelReadNeitherOthersVariables`). Vercel is
  last in the vendor order. The key (`vck_…`) has a shape `pkg/modelkey`
  checks.

  `/v1/models` answers without a key and lists 498 models: language, image,
  video, embedding, speech, transcription, rerank, realtime. The reader keeps
  the language models with `tools` in `supported_parameters` and drops one
  past its `deprecated_at` (302 on 2026-11-09). Prices are dollars per token.
  `reasoning_options` of type `effort` marks Effort. The list being public,
  the connect probe is the chat call: a wrong key is a 401. A key on an
  account with no card is a 403 `customer_verification_required`. The probe
  connects it and says the account needs a card, as for an OpenRouter
  account with no credit. The app headers are `pkg/attribution`'s pair (`HTTP-Referer` +
  `X-Title`).

  A free-tier account serves some models and refuses others with a 403
  ("Free tier users do not have access to this model",
  `anthropic/claude-haiku-5.5`). Vercel's ids are OpenRouter's
  (`deepseek/deepseek-v4-flash`). The flat `/model-search <text>` keeps one
  row per id, the first in answer order, so a model both list shows as
  OpenRouter's. `/model-search vercel` lists Vercel's 302, and
  `/model vercel:<id>` pins one.

  The fast-model task (an off-by-one in `sum.go`, a failing test, "fix the
  bug in sum.go (not the test)"), pinned in the window. Every request went to
  `ai-gateway.vercel.sh` (`memdoor meter`):

  | Provider · model | Steps | Time | Result |
  |---|---|---|---|
  | Vercel · `deepseek/deepseek-v4-flash` | glob, notes, Read ×2, Edit, Bash | 23 s | ok, +1 −1 |
  | Vercel · `openai/gpt-oss-20b` | Bash ×3, Read, Edit, notes | 21 s | ok, +5 −1 (a comment added) |

  `model check` on four: `deepseek/deepseek-v4-flash`,
  `alibaba/qwen3-coder-next` and `openai/gpt-oss-20b` passed every check;
  `anthropic/claude-haiku-5.5` was refused on the free tier.


- **Attribution as a field** (`gateway/providers/attribution.go`): user, team,
  project, session as headers on every vendor and gateway request —
  `MEMDOOR_ATTRIBUTION="user=…;team=…"` from the company's preset,
  `MEMDOOR_ATTRIBUTION_HEADERS` for the gateway's names (default
  `X-Memdoor-*`), the project's directory name and the session id from the
  turn; a field with no value sends nothing. `TestAttributionHeaders`.
- **A public Providers page** (`/docs/providers`, the maintainer: "document it in
  docs public for all providers"): each of the eight and the company
  gateway — key name, base, what its API states, the fast model that did
  the coding turn, the quirks (max_completion_tokens, thought_signature,
  Baseten's case, Groq's rate limits) — plus attribution, the
  `provider:model` form and where a window comes from.

- **Attribution tested against a company gateway** (the maintainer: "test
  attribution with the company gateway"): no such gateway is reachable from
  here, so a stand-in that speaks its API (`POST /ai/v1/responses`, `GET
  /ai/v1/models`, Bearer PAT) recorded every header while a scratch Memdoor
  gateway ran on it with the preset's variables (`AI_GATEWAY_BASE_URL`,
  `CORP_AI_GATEWAY_TOKEN`, `MEMDOOR_MODEL=corp-claude`,
  `MEMDOOR_ATTRIBUTION="user=greg@corp.example;team=data-eng"`,
  `MEMDOOR_ATTRIBUTION_HEADERS="user=X-Email;team=X-Team"`). The coder's turn
  arrived as `model corp-claude · X-Email: greg@corp.example · X-Team:
  data-eng · X-Memdoor-Project: memdoor-coder · X-Memdoor-Session:
  workspace:corp:channel:… · Authorization: Bearer pat-…`, the model
  list the same without a session; the reply came back through
  (`attributed ok`); the brain line read *your company's gateway key
  serves this gateway; nothing else is contacted* and every other provider
  showed as not connected. Two finds fixed: a slug declared by
  `MEMDOOR_MODEL` kept the 128k default although the gateway's list said
  200k (a declared model now takes the list's figures), and the credential's
  source named `AI_GATEWAY_TOKEN` when it came from `CORP_AI_GATEWAY_TOKEN`.

- **`/model-search` names the provider on every row** (the maintainer, 2026-10-03:
  "/model-search should work for deepseek"): `deepseek · deepseek-flash`,
  `baseten · deepseek-ai/DeepSeek-V4.1-Flash`, `openrouter ·
  deepseek/deepseek-v4.1-flash` side by side in the shell and the picker;
  Enter on a row of another provider pins it as `provider:id` (hosts are
  OpenRouter's notion, so only its rows open the hosts picker). A vendor
  whose API states no price (Anthropic, DeepSeek's own ids) shows its list
  price from the reference when the reference names the id; DeepSeek's
  `deepseek-flash` alias is not in it, so its rows stay $0.00 rather than
  guessed.

- **Onboarding, 2026-10-03** (the maintainer: "prefill default KEY NAME or value if
  you find it … unless it's not obvious like for company gateway", "once
  it is connected store it so we don't need to reconnect at every start",
  "when not connected suggest /connect"): the token step opens with the
  kind's variable name prefilled — `> ANTHROPIC_API_KEY`, *is set on the
  gateway — enter keeps it* — shown as a name (no secret on screen), empty
  for a company gateway whose variable is its own; a built-in connected
  through `/connect` is kept in `providers.json` under its id and fills
  the built-in at the next start (the environment wins when it has one —
  `TestAConnectedBuiltInSurvivesARestart`); a model whose provider is not
  connected points at it — *That looks like an Anthropic model, and
  Anthropic is not connected — /connect anthropic adds it (the key's name
  is prefilled)*, `groq:…` → `/connect groq`, otherwise *Not connected
  yet: … — /connect <kind>* (`connectHint`).

- **A bare `/model-search` is a picker** (the maintainer, 2026-10-03, after
  `/connect deepseek` left `/model-search` answering with stale help:
  "the user doesn't know the provider name; it should be without
  inputting text"): the providers with their counts and which is
  answering, cursor on the first connected one; enter opens that
  provider's whole list (`/api/models?q=<provider id>`), enter again pins
  `provider:id`, esc goes back a level; a provider not connected says
  `/connect <id>`. `/model` lists the other connected providers and what is
  not connected. `TestProviderPickerThenAProvidersModels`.

- **A vendor key without `MEMDOOR_MODEL`** (2026-11-09, #206). `MEMDOOR_MODEL`
  named the model and, with a vendor key, switched the gateway to that
  vendor alone. A vendor key with no `MEMDOOR_MODEL` logged a warning and
  the gateway fell back to OpenRouter. Now the gateway picks the model:
  the cheapest, by the vendor's list price, with a passing
  `memdoor model check` report (`gateway/vendor_default_model.go`). With
  no passing report it checks the cheapest models it has no report for,
  at most three, and keeps the reports. The next start reads them and
  spends nothing. The vendor key then wins over an OpenRouter key in the
  same environment. A model with no price is not ranked, so a vendor whose
  list prices none needs `MEMDOOR_MODEL`. If no checked model passes, the
  gateway falls back as before. `MEMDOOR_MODEL` still names the model when
  set. The company gateway still requires it, and Anthropic keeps its
  default. `/model` in the window shows the model with
  `← picked: the cheapest that passed model check`.

  Live, a gateway with only `GROQ_API_KEY` and an OpenRouter key, an empty
  `HOME`: Groq's two cheapest, `meta-llama/llama-prompt-guard-2-22m` and
  `-86m`, refused tool calls (HTTP 400, under 200 ms each);
  `openai/gpt-oss-20b` passed in 1.6 s and serves the gateway.

## How a provider knows it is Memdoor (2026-10-24)

Nothing phones home. Some providers read an app header and some do not.
Researched provider by provider, web only. What the binary sends is
`pkg/attribution` (one definition, the User-Agent from
`gateway/providers/app_headers.go`).

| Provider | Reads an app header? | Where it shows |
|---|---|---|
| OpenRouter | `HTTP-Referer` + `X-Title` | openrouter.ai/apps, public, daily chart |
| Vercel AI Gateway | `http-referer` + `x-title` | vercel.com/ai-gateway/leaderboards/apps (public, CC-BY data) |
| LLM Gateway | `X-Source` (falls back to `HTTP-Referer`, then the User-Agent of known agents) | llmgateway.io/apps and /rankings |
| EUrouter | `HTTP-Referer` + `X-EUrouter-Title` | eurouter.ai rankings, 7/30/90-day |
| Requesty | `HTTP-Referer` + `X-Title` | The customer's own analytics only |
| Google Gemini | `x-goog-api-client: memdoor-oai/<version>`, required of every partner integration | Google's side only (segmentation, outreach on error patterns) |
| OpenAI API | None (`OpenAI-Organization` is the key holder) | Nothing |
| Sign in with ChatGPT | `agent_name_hint=Memdoor` at registration (we send it, `pkg/chatgpt`) | Nothing public |
| Anthropic | None documented; first-party-only headers were used by third parties to ride plans and cut off | Console by key/workspace only |
| DeepSeek | None | Nothing |
| Baseten | None (`x-session-affinity` is cache pinning) | Nothing |
| Groq | None | Nothing |
| xAI | None | Nothing |
| Z.ai (GLM Coding Plan) | None | Nothing |
| Moonshot (Kimi for Coding) | None; tampering with the User-Agent is a violation (ours is real) | Nothing |
| Mistral, Together, Fireworks, Cerebras | None | Nothing |
| Hugging Face Inference Providers | None | Nothing |

What changed in the binary: the pair goes to every host that reads it, Google
gets its partner header, and a vendor key (Anthropic, OpenAI, xAI, Baseten,
Groq, DeepSeek) still carries only the User-Agent and the company's own
`MEMDOOR_VENDOR_HEADERS`. No new host is contacted (`gateway/egress_test.go`).

Z.ai's and Moonshot's coding plans are limited to the tools they list. A
person on one of those plans cannot use it in Memdoor until Memdoor is on
that list.

## Sign in with ChatGPT (2026-10-10)

A ChatGPT Plus or Pro plan is a provider: `memdoor connect chatgpt` (or
`/connect chatgpt` in the window) opens OpenAI's Sign in with ChatGPT in the
browser, and the plan's allowance answers Memdoor's turns through the
Responses API, no API key. OpenAI opened this to open-source, locally run apps
at DevDay (2026-09-29); OpenCode, Pi, Kilo and Amp ship it. The pieces:

- `pkg/chatgpt`: the OAuth flow (PKCE, a loopback listener on 127.0.0.1,
  dynamic client registration on the first sign-in, the issued client id and
  a per-install `ext_agent_host_id` kept), the ID token verified against
  OpenAI's JWKS, the access token (an hour) renewed with the refresh token (30
  days, rotated on use). The credential is encrypted in
  `~/.memdoor/chatgpt-login.json` (0600); `memdoor connect --remove chatgpt`
  signs out.
- `gateway/providers/chatgpt.go`: the built-in provider `chatgpt`, connected
  when a sign-in is on disk; its model list from `GET /v1/models` (the entries
  marked for listing); the Responses client in plan mode: `store: false`,
  `stream: true`, none of the fields the preview refuses (temperature,
  max_output_tokens, …); refusals in the person's terms (the weekly allowance
  used up, a disconnected sign-in).
- `gateway/providers_chatgpt.go`: the browser step runs on the gateway, like
  the MCP sign-in (`/api/providers/chatgpt`: login, login/wait, login/paste,
  login/cancel, logout); on success the plan is probed the way `/connect`
  probes a key.
- **The plan answers by default** (2026-10-31, the maintainer: "if you login to chatgpt
  you should get chatgpt model by default in the list"): a signed-in plan is
  the engine after a company's vendor key and before an OpenRouter key, on
  the first model the plan lists (`ChatGPTPlanEngine`,
  `gateway/providers/chatgpt_engine.go`); the sign-in and the sign-out choose
  again (`selectModelEngine`). A kept pin on the key for an id the plan lists
  (`openai:<id>`) moves to `chatgpt:<id>` at sign-in (`pinsToPlan`), and the
  ✓ says the default and each move (`ui.PlanNotes`); an open conversation
  keeps its own route. `/model` offers the providers in `AnswerOrder` (the
  answering one, then the plan before the key), so its one row per id is the
  plan's. Live before: signed in, turns still went to OpenRouter, and
  picking `gpt-6-astra` pinned the OpenAI key and billed the API. Tests:
  `TestASignedInPlanIsAnEngineOnItsFirstModel`,
  `TestNoPlanEngineWithoutAListedModel`, `TestAnswerOrderPutsThePlanBeforeTheKey`,
  `TestASignedInPlanAnswersBeforeTheOpenRouterKey`,
  `TestASignedOutPlanLeavesNoEngine`, `TestAVendorKeyAnswersBeforeThePlan`,
  `TestPinsOnTheKeyMoveToThePlan`, `TestTheModelListKeepsTheAnsweringProvidersRow`,
  `TestAChatGPTSignInSaysThePlanAnswersAndThePinMoved`.
- **From any door** (2026-10-31): the link sends the browser back to the
  gateway's loopback (`http://127.0.0.1:<port>/callback`). With the gateway on
  this machine that is the gateway itself. With the gateway elsewhere (the ssh
  door, a box) the CLI catches that return on the same port here and passes
  the whole address on like a pasted one (`oidc.StartRelay`,
  `cmd/cli/cmd/signin.go relayBack`); the gateway's own state check decides,
  and a refusal (`error=` in the address) ends the sign-in in OpenAI's words.
  Where no browser can come back to this machine (an ssh session into it), or
  the port is taken here, the shell asks `Address:` for the page the browser
  landed on (it does not load; that is expected). The window takes a pasted
  address at any time. No device code: OpenAI makes a person turn it on in
  ChatGPT's security settings first.

The decision model is off on a plan alone (no OpenRouter or TypeSafe key), as
on any vendor key. A Claude subscription has no equivalent: Anthropic's terms
forbid it outside Claude Code, and the company acted on it against OpenCode in
March 2026.

## A local runtime, no key (2026-10-25)

The maintainer: "the next move can be local llm that you ssh from a different machine",
"it should be a different provider", "janis should work like a llama.cpp or
ollama, independent from memdoor". The `local` kind in `/connect` and
`memdoor connect local` (`cmd/tui/ui/connect_flow.go` `ConnectKinds`,
`Keyless`): an OpenAI-compatible server on this machine or the box — Ollama,
llama.cpp, janis, LM Studio, vLLM — connected with no token. The request
carries `keyless`; the gateway skips the key rule, keeps `"keyless": true`
in `providers.json` with an empty key, and `Provider.Connected()` counts it
as connected. No `Authorization` header is sent when the key is empty
(`catalogue.go bearer`, `oai_native.go`, `responses.go`); the key source
reads `no key: a local runtime`. The base is prefilled with Ollama's
`http://localhost:11434/v1` and edited to the runtime's port. janis gets its
own OpenAI-compatible server in its own repo (`janis serve`), so a box runs
it the way it runs llama.cpp; Memdoor knows no runtime by name. Tests:
`TestConnectALocalRuntimeNeedsNoKey` (gateway),
`TestConnectALocalRuntimeAsksNoKey` (window).

**A keyless runtime may read in silence (2026-10-28).** It sends nothing
while it reads the prompt, and a CPU reads slowly: a GitHub runner's 4 vCPUs
read a 9B's review prompt at 11.8 tokens a second, eight minutes before the
first token, and the cloud's 60 s stall limit dropped it twice. Its client
(`RemoteEngine.Keyless`, `oaiClient.keyless`) waits `keylessStallAfter`, 10
minutes, and an hour in all; a step's `thinking off` reaches it as
`reasoning_effort: "none"`, Ollama's think=false (no other effort is sent: a
model with no thinking mode refuses one). Test:
`TestAKeylessRuntimeMayReadInSilence`.

**A local decision model** (the maintainer: "there should be a janis jev model"): janis
serve also answers `POST /v1/systemone`, the System One wire Memdoor's
decision client speaks (`pkg/decision/systemone`), from the engine's
`Decide` (one prefill per question, the probability read off the option
letters). `connect local` on a keyless runtime also tries that endpoint at
the runtime's origin and keeps it as `<id>-decisions` (API `decisions`,
keyless); `providers.DecisionProvider()` returns it, and `systemOneFromEnv`
accepts an empty key on localhost, so the decision model runs on the
machine with no key. The decision service is rebuilt on the spot
(`gateway/decisions.go`: `wireDecisions` keeps `s.rewireDecisions`, the
connect handler calls it after a decisions provider is kept or removed), so
it judges from the next turn, no gateway start. `--remove local` drops
both. A runtime without the endpoint stays chat-only.

**Minimum friction (the maintainer, 2026-10-25: "minimum friction for install and
quick getting started")**: a blank base on `connect local` finds the runtime
on the gateway's machine (`findLocalRuntime`, `localRuntimeBases`: janis
8090, Ollama 11434, llama.cpp 8080, LM Studio 1234, the first whose model
list answers), so the whole local setup is `janis pull && janis serve` then
`memdoor connect local`. Nothing listening is said in words with what to
start.

## A key already on the machine (2026-10-31)

The maintainer: "they dont know the key", "when they connect we should scan keys … easy
frictionless". The one outside install before the launch (2026-10-20) ran the
binary, upgraded, and sent no token. A key is now found by its VALUE, never
its variable's name (`pkg/modelkey`): `sk-or-v1-` OpenRouter, any `sk-ant-`
kind Anthropic (`sk-ant-api03-`, `sk-ant-usr-`, …), `sk-proj-`/`sk-svcacct-`
OpenAI, `AIza` or the newer `AQ.` Gemini, `gsk_` Groq, `xai-` xAI, a bare
`sk-` DeepSeek or OpenAI (the probe decides, the variable's NAME breaking the
tie: `DEEPSEEK_API_KEY` is DeepSeek's). A key with no shape of its own
(Baseten's, TypeSafe's) is found under its provider's own name
(`providerNames`), when the value looks like a key. A Claude subscription's
`sk-ant-oat` token and an Anthropic admin key (`notKeys`) run no model and
are not offered. It looks in this shell's environment, then the project's
`.env`, `.env.local` and `.envrc`; nothing else is read and nothing leaves but
the connect probe to the key's own provider.

- **First run** (`memdoor`, `memdoor run`, at a terminal, the gateway with no
  model; the maintainer: "less friction", "better than every other"), with as few
  questions as can be (`offerFoundKey`, `cmd/cli/cmd/connect_found.go`):
  a key in this shell is connected without a question, the first that works,
  and the line says which and how to forget it ("No model connected yet:
  trying the OpenRouter key in $MY_OR_KEY … ✓ … memdoor connect --remove
  openrouter forgets it"); a key in the project's files is asked about once
  (it may be the project's production key, another's bill); other tools'
  saved keys after a yes, the first that works connected; with none, a
  sign-in is offered (below).
- **`memdoor connect` with no kind** (`connectMenu`): the found keys that
  would change something are numbered first, then the sign-ins under "Sign
  in, no key:" (never buried; live 2026-10-31 Sign in with OpenRouter was
  6th under five keys already in use), then the kinds that take a key;
  Enter takes the first found key, else Sign in with OpenRouter. The keys
  the gateway already reads from its environment under the same variable
  (`splitInUse`, `/api/providers` key sources; OpenRouter's two names are
  one) are named on a line of their own, not numbered, on a gateway on this
  machine. Numbers are right-aligned; a name too long for its column puts the
  hint on the next line. Tests: `TestTheNewerKeyShapes`,
  `TestTheNameBreaksATieAndFindsAShapelessKey`,
  `TestKeysTheGatewayAlreadyUsesAreSetAside`,
  `TestTheConnectListPutsTheSignInsBeforeTheKeyedKinds`.
- **Name, not copy**: a shell variable is sent by its NAME, so the gateway
  reads it and a rotated key keeps working; when the gateway cannot read it
  (`providers.KeyNameNotSet`: started from another shell, a service, a box)
  the value follows and is kept in `providers.json` (0600) like a pasted key.
  A `.env` key goes by value.
- **The agent never inherits one**: `shell.Env` drops a variable by its value
  too, so a key under a name of the person's own is not in a command's
  environment.
- **The name we show is `OPENROUTER_API_KEY`**, OpenRouter's own;
  `OPEN_ROUTER_API_KEY` is still read and wins when both are set
  (`providers.ByokKeyVar`), and `memdoor providers` names the one in use.
  `GOOGLE_API_KEY` is not read as Gemini's (Maps and other Google APIs use it
  too): the scan finds its `AIza` value and the probe decides.

Tests: `TestProvidersOf`, `TestFindByValueNotName` (pkg/modelkey),
`TestConnectFoundSendsTheNameThenTheValue`,
`TestConnectFoundFileKeyAndTwoProviders`, `TestCommandHidesModelKeys`.
- **Other tools' saved keys, after a yes** (`modelkey.ToolStores`,
  `FindInTools`): when the shell and the project hold none, `memdoor` and
  `memdoor connect` ask "Codex, Claude Code, … saved keys on this machine:
  look in them? [Y/n]", naming only the tools whose file exists: Codex
  (`~/.codex/auth.json`), OpenCode, Claude Code (`~/.claude.json`, an API key
  only), Hermes (`~/.hermes/.env`), Goose (`~/.config/goose/secrets.yaml`),
  Crush, Gemini CLI (`~/.gemini/.env`), Continue, OpenClaw, llm, aider
  (`~/.aider.conf.yml`, `~/.aider/oauth-keys.env`). Read by value; an OAuth
  token is no key; a file past 16 MB, a directory or an unreadable file
  offers nothing. Hermes imports four tools' setups; this reads eleven. The
  window's `/connect` lists the shell's and the project's keys first
  (`ConnectOps.Found`), never another tool's file without the question.

### A kept OpenRouter key is the OpenRouter path (2026-10-31)

`providers.ByokKey()` is the environment's key, else the one `memdoor
connect` kept (`keptKey`: a sign-in, a key found on the machine, a pasted
one). Until this, a kept key got none of the OpenRouter path (the ladder, the
privacy policy, the decision model, web search) and answered on whatever
model OpenRouter listed first; live, the kept key's turn now runs on the
ladder's first rung. Connecting or removing `openrouter` chooses the engine
again and rewires the decision model, no gateway start; the key removed, its
engine is cleared. The registry's built-in entry still reads the environment
alone (`byokEnvKey`): a kept key reaches it through the file's own entry,
whose base and models the merge keeps.

### A free account works on its first turn (2026-10-31)

A new OpenRouter account has no credit and the ladder is paid: aider asks
OpenRouter whether the key is on the free tier, and so does the engine choice
(`providers.OpenRouterFreeTier`, `GET /api/v1/key`, `is_free_tier`, 4 s). A
free account's every agent and rung run on one `:free` model with tools,
ranked by its paid twin's list price, then context, then id
(`FreeTierModel`; live: `thinkingmachines/inkling:free` of 13, not
`inkling-small:free`, which a context-then-id order picked). The connect and
sign-in lines say so ("Your OpenRouter account has no credit yet: every turn
runs on …, free (its hosts may train on what they are sent)"), and the
decision model stays off until there is credit. Anything short of a clear
answer (offline, a refused key, no field, no free model with tools, an
unreadable catalogue) keeps the paid ladder: a private turn that asks for
credit, never a model that may train on the code by default. A probe's 402
("no credit") is a good key on an empty account and connects
(`hasNoCredit`).

### Sign in with OpenRouter (2026-10-31)

For the person with no key at all: OpenRouter's OAuth PKCE
(openrouter.ai/docs/use-cases/oauth-pkce), `pkg/openrouter`. It runs in the
CLI, where the person's terminal and browser are, and hands the key to the
gateway like a pasted one (`/api/providers/connect`), so it works the same
with a gateway on this machine, over the ssh door or on a box: no port to
forward, no device code (Codex and Hermes need one or the other). The page
`openrouter.ai/auth` gets `callback_url=http://localhost:<port>/callback`
(listening on 127.0.0.1 and ::1), an S256 challenge, a state and
`key_label=Memdoor`. Where no browser can reach this machine (an ssh session,
a Linux with no display: `noBrowserHere`) it is headless: no callback, the
page shows a code, `Code:` reads it (an empty line asks again, a refused one
says OpenRouter's words). One POST to `/api/v1/auth/keys` returns a key of
the person's own; a gateway that does not take it at once is asked three
times before the person is told the key is on their account.

- **Where**: the first run with no key that works offers "1. Sign in with
  OpenRouter (free) 2. Sign in with ChatGPT" (both with any gateway: ChatGPT's
  browser return is relayed, above); `memdoor connect openrouter-login`;
  `memdoor connect openrouter` with no key asks "1. sign in 2. paste a key";
  the window's `/connect` lists it first. The "no model yet" line names it.
- **Shared code**: the gateway's sign-in registry and steps
  (`gateway/provider_signin.go`) are ChatGPT's; the client's start, wait,
  paste and cancel (`cmd/cli/cmd/signin.go`) drive both; the PKCE helpers are
  `oidc.PKCEChallenge` and `oidc.RandomString` for ChatGPT, OpenRouter and
  MCP alike (the maintainer: "common code is better").

Tests: `TestSignInThroughTheCallback`, `TestSignInByPaste`,
`TestHeadlessSignIn`, `TestSignInRefused` (a fake OpenRouter),
`TestSignInSteps`, `TestSignInRefusedCancelledUnknown`,
`TestByokKeyEnvironmentThenKept`, `TestByokKeyNegativePaths`,
`TestOpenRouterFreeTier`, `TestOpenRouterFreeTierNegativePaths`,
`TestFreeTierModel`, `TestOnFreeTier`, `TestAKeptKeyChoosesTheOpenRouterEngine`,
`TestAFreeAccountRunsOnAFreeModel`, `TestTheFreeModelNeedsAClearAnswer`,
`TestTheEngineFollowsTheKeptKey`, `TestConnectAKeyWithNoCreditYet`,
`TestConnectListsFoundKeysFirst`, `TestSignInScreenNamesItsProvider`,
`TestRedirectOfTakesOnlyALoopback`, `TestRelayPassesTheReturnOn`,
`TestRelayPortTakenAndStop`, `TestARelayedRefusalEndsTheSignIn`,
`TestChatGPTSignInWithAGatewayElsewhereCatchesTheReturnHere`,
`TestChatGPTSignInRelayNegativePaths`,
`TestChatGPTSignInWithNoBrowserHereTakesThePastedAddress`,
`TestFindInTools`, `TestToolStoresNegativePaths`; the key, the free-tier
answer, the engine's choice and the screen's title each mutation-checked.
Live (isolated gateway, empty environment): the real key under `MY_OR_KEY`
connected with no question and the turn ran on `z-ai/glm-5.3-flash`;
`--remove` cleared the engine; a headless sign-in took an empty line, then
OpenRouter refused a bogus code ("HTTP 400: Invalid code"); a refused Codex
key fell through to the sign-in offer; the window's sign-in started and Esc
cancelled it. Not yet: one complete sign-in with a real new account.

## Next

- Attribution as a field (user, team from the sign-in identity) rather
  than only static headers; `!command` credentials cached per process,
  cleared on a 401 (omp's rule).
- Streaming on the `responses` and `anthropic` wires.
