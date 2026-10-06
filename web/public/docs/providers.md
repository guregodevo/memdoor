# Providers

Models come from providers, each with its own list, read from the
provider's own API — windows, output caps, prices and modalities as the
vendor states them, never typed into Memdoor. `/model-search` alone is a
picker of your providers: enter opens one's models, enter again pins one
for the conversation, no name to know. `/model-search <text>` searches
them all; `/model` shows what is connected and what is not; the footer
names the provider that answers.

```
memdoor providers                 # each provider, connected or not, ● the one answering
memdoor connect <kind>            # pick, paste the key unseen, probed, kept
memdoor providers --models groq   # one provider's list: window · max out · $ per million
memdoor providers --audit         # where every window came from, and which are guesses
```

A key goes in the environment under the name below, or through
`/connect` — which opens the token step with that name prefilled (Enter
keeps it when the gateway already has it; the value is never shown) and
keeps what it connects in `~/.memdoor/providers.json`, so a provider is
there at every start. A key can be the value, the NAME of a variable, or
`!command`; `*_BASE_URL` points a provider at a proxy or a company gateway
in front of it. Pin a model whose provider is not connected and the window
says which one to `/connect`. Every provider below has done
the same coding turn on its fast model — a planted bug, a failing test,
"fix the code, not the test" — in 8 to 45 seconds.

## OpenRouter — the catalogue

- **Key**: `OPEN_ROUTER_API_KEY` · base `https://openrouter.ai/api/v1`
- **Its API states**: window, the top host's completion cap, prices,
  supported parameters — for every vendor's models, which is why it is
  also the catalogue the other providers fall back to.
- **Fast model**: `z-ai/glm-5.3-flash` (the ladder's first rung).
- One key for 380 tool-capable models; the ladder, `/model` with real
  prices and host ordering live here.

## Anthropic

- **Key**: `ANTHROPIC_API_KEY` · base `https://api.anthropic.com`
- **Its API states**: `max_input_tokens`, `max_tokens`, capabilities —
  image input, thinking, effort levels — per model.
- **Fast model**: `claude-haiku-4-5-20251001` (200k / 64k); Sonnet 5,
  Opus 4.6 and Fable 5.1 are 1M / 128k.
- A Claude subscription is not an API key; the key comes from
  console.anthropic.com. Streams.

## OpenAI

- **Key**: `OPENAI_API_KEY` · base `https://api.openai.com/v1`
- **Its API states**: nothing beyond the id — the one vendor without
  model metadata. Windows come from the same model in OpenRouter's
  catalogue (`openai/gpt-5-codex`, 400k), then the public reference;
  audio, image, realtime and embedding ids stay marked `~`.
- **Fast model**: `gpt-5-mini` (400k / 128k).
- Newer models take `max_completion_tokens`; Memdoor sends it on this host
  and switches to it on any host that asks.

## Google Gemini

- **Key**: `GEMINI_API_KEY` · base `https://generativelanguage.googleapis.com/v1beta/openai`
- **Its API states**: `inputTokenLimit`, `outputTokenLimit`, thinking —
  read from the native `/v1beta/models`, not the OpenAI-shaped list, so
  the figures are Google's own; embedding and image models are left out.
- **Fast model**: `gemini-3-flash-preview` (1M). The 2.5 line is closed to
  new keys; Gemini 3 needs billing enabled on the key.
- Gemini 3's thinking models return a `thought_signature` with each tool
  call; Memdoor carries it back on the follow-up, as they require.

## Groq

- **Key**: `GROQ_API_KEY` · base `https://api.groq.com/openai/v1`
- **Its API states**: `context_window`, `max_completion_tokens`, prices,
  modalities.
- **Fast model**: `openai/gpt-oss-20b` — 8 seconds on the coding turn,
  the fastest of the eight; `qwen/qwen3.8-27b` for a stronger one.
- The free tier rate-limits quickly; Memdoor waits what `Retry-After`
  says (up to a minute) before asking again.

## xAI Grok

- **Key**: `XAI_API_KEY` or `GROK_API_KEY` · base `https://api.x.ai/v1`
- **Its API states**: `context_length` and prices on `/v1/models`,
  modalities and aliases on `/v1/language-models`.
- **Fast model**: `grok-4.20-0309-non-reasoning` (1M); `grok-4.3` for
  reasoning.

## DeepSeek

- **Key**: `DEEPSEEK_API_KEY` · base `https://api.deepseek.com/v1`
- **Its API states**: `context_window`, `max_output_tokens`, effort,
  modalities.
- **Fast model**: `deepseek-flash` (1M / 393k); `deepseek-v4-pro` for the
  larger one.

## Baseten

- **Key**: `BASETEN_API_KEY` · base `https://inference.baseten.co/v1`
- **Its API states**: `context_length`, `max_completion_tokens`, prices,
  modalities, supported features (only tool-capable models are listed).
- **Fast model**: `zai-org/GLM-5.3-Flash` ($0.15 / $0.50 per million);
  also DeepSeek V4, Kimi K3, Nemotron. Ids carry case — type them as
  listed.

## Your company's AI gateway

- **Variables**: `AI_GATEWAY_BASE_URL` + `AI_GATEWAY_TOKEN` (or any
  `*_AI_GATEWAY_TOKEN`) + `MEMDOOR_MODEL` = the provider slug the gateway
  serves; `AI_GATEWAY_API=chat` if it speaks chat/completions rather than
  the Responses API.
- **What it states**: whatever its `/models` lists (window and cap are
  read when present); a slug it does not list is declared by
  `MEMDOOR_MODEL` and `MEMDOOR_MODEL_CONTEXT`.
- With it set, nothing but the gateway is contacted — no OpenRouter, no
  memdoor.ai, no catalogue, no public reference (the last copy on disk
  answers). [Where your data goes](/docs/security).

## Any OpenAI-compatible endpoint

`memdoor connect custom` with a base URL and its key: a vLLM, an internal
proxy, a hosted model. Window and cap are read when the endpoint lists
them (`context_length`, `max_model_len`), else the reference, else a
marked guess you can declare.

## Attribution — whose request this is

A company gateway tags requests by person and team for cost attribution.
Memdoor sends four fields as headers on every vendor and gateway request,
under the names your gateway expects:

```
export MEMDOOR_ATTRIBUTION="user=you@corp.example;team=data-eng"
export MEMDOOR_ATTRIBUTION_HEADERS="user=X-Email;team=X-Team"   # default X-Memdoor-User, X-Memdoor-Team …
```

`project` (the project's directory name) and `session` (the
conversation's id) are filled from the turn; a field with no value sends
no header. `MEMDOOR_VENDOR_HEADERS="Name: value; …"` adds anything static
beside them.

## When two providers list the same id

A bare id goes to the active provider; name the provider to be exact:
`/model groq:qwen/qwen3.8-27b`, `/model baseten:zai-org/GLM-5.3-Flash`.
The footer then reads `groq · qwen/qwen3.8-27b · pinned by you`.

## Where a window comes from

In order: the provider's own API; the same model in OpenRouter's
catalogue; the public reference catalogue (models.dev, read once a day,
kept on disk, never contacted on a company gateway); the provider's
default — shown as `~` after the figure, warned once in the log, and
declarable in `providers.json`. `memdoor providers --audit` prints the
count: 609 models across the eight, 94% from a stated figure.
