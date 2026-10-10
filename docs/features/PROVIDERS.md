# Providers — each with its list of models

Greg, 2026-10-02: "we had an OpenRouter integration … we can continue to
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
  (`providers/factory.go: GetClientFor`), the active provider winning a tie.
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

## Receipts (Greg's gateway, 2026-10-02)

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
  next; a refusal with its advice; Esc cancels at any step. Greg: "omp
  does it with a slash command right?" — omp's `/login`, with the probe
  omp lacks. Receipt in a tmux window on Greg's gateway: `/connect` → 5 →
  `orx` → OpenRouter's base → the key as `OPEN_ROUTER_API_KEY` (shown as
  19 dots) → `z-ai/glm-5.3-flash` → `✓ orx connected · responses API ·
  465 models … kept`. `TestConnectFlowPicksTypesProbesAndSays`.

- **The first real Anthropic turn** (Greg's key, 2026-10-02): `memdoor
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
  (`providers/sse.go`, `consumeAnthropicStream`, `consumeResponsesStream`):
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

- **Context windows, dynamically** (Greg: "make sure you have the correct
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

- **Each provider's own metadata API, behind one interface** (Greg:
  "check if each provider has an API that provides model metadata … add
  this in our provider interface, getModel"). `ModelReader`
  (`providers/catalogue.go`): `ListModels` and `GetModel`, one reader per
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
  key is `GROK_API_KEY` in Greg's .envrc, accepted as the alias of
  `XAI_API_KEY`). Grok and Groq are different companies; both are in.

## The fast-model round (Greg, 2026-10-02: "test each provider with haiku equivalent model")

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
steps in before its quota ran out. Greg enabled billing on the key and Gemini 3 Flash then did the fix in
16 s: all six providers have the same receipt.

- **DeepSeek and Baseten** (Greg: "DEEPSEEK and BASETEN providers as
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

- **Attribution as a field** (`providers/attribution.go`): user, team,
  project, session as headers on every vendor and gateway request —
  `MEMDOOR_ATTRIBUTION="user=…;team=…"` from the company's preset,
  `MEMDOOR_ATTRIBUTION_HEADERS` for the gateway's names (default
  `X-Memdoor-*`), the project's directory name and the session id from the
  turn; a field with no value sends nothing. `TestAttributionHeaders`.
- **A public Providers page** (`/docs/providers`, Greg: "document it in
  docs public for all providers"): each of the eight and the company
  gateway — key name, base, what its API states, the fast model that did
  the coding turn, the quirks (max_completion_tokens, thought_signature,
  Baseten's case, Groq's rate limits) — plus attribution, the
  `provider:model` form and where a window comes from.

- **Attribution tested against a company gateway** (Greg: "test
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

- **`/model-search` names the provider on every row** (Greg, 2026-10-03:
  "/model-search should work for deepseek"): `deepseek · deepseek-flash`,
  `baseten · deepseek-ai/DeepSeek-V4.1-Flash`, `openrouter ·
  deepseek/deepseek-v4.1-flash` side by side in the shell and the picker;
  Enter on a row of another provider pins it as `provider:id` (hosts are
  OpenRouter's notion, so only its rows open the hosts picker). A vendor
  whose API states no price (Anthropic, DeepSeek's own ids) shows its list
  price from the reference when the reference names the id; DeepSeek's
  `deepseek-flash` alias is not in it, so its rows stay $0.00 rather than
  guessed.

- **Onboarding, 2026-10-03** (Greg: "prefill default KEY NAME or value if
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

- **A bare `/model-search` is a picker** (Greg, 2026-10-03, after
  `/connect deepseek` left `/model-search` answering with stale help:
  "the user doesn't know the provider name; it should be without
  inputting text"): the providers with their counts and which is
  answering, cursor on the first connected one; enter opens that
  provider's whole list (`/api/models?q=<provider id>`), enter again pins
  `provider:id`, esc goes back a level; a provider not connected says
  `/connect <id>`. `/model` lists the other connected providers and what is
  not connected. `TestProviderPickerThenAProvidersModels`.

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

The decision model is off on a plan alone (no OpenRouter or TypeSafe key), as
on any vendor key. A Claude subscription has no equivalent: Anthropic's terms
forbid it outside Claude Code, and the company acted on it against OpenCode in
March 2026.

## Next

- Attribution as a field (user, team from the sign-in identity) rather
  than only static headers; `!command` credentials cached per process,
  cleared on a 401 (omp's rule).
- Streaming on the `responses` and `anthropic` wires.
