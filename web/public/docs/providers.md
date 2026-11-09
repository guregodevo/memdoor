# Providers

A provider's model list is read from its API: windows, output caps, prices
and modalities as the vendor states them, never typed into Memdoor.
`/model-search`: your providers; enter opens one's models, enter again pins
one for the conversation. `/model-search <text>`: all of them. `/model`:
connected or not. The footer names the provider that answers.

```
memdoor providers                 # each provider, connected or not, ● the one answering
memdoor connect                   # a list: keys found on this machine first (Enter connects the first),
                                  # then "Sign in, no key:" (OpenRouter, ChatGPT), then the keyed kinds
memdoor connect <kind>            # pick, paste the key unseen, probed, kept
memdoor connect openrouter-login  # no key: Sign in with OpenRouter in the browser (free account)
memdoor connect chatgpt           # no key: Sign in with ChatGPT, your Plus/Pro plan answers
memdoor providers --models groq   # one provider's list: window · max out · $ per million
memdoor providers --audit         # where every window came from, and which are guesses
```

Key: the environment variable named below, or `/connect`, which opens the
token step with that name prefilled (Enter keeps a key the gateway already
has; the value is never shown). Connected providers are kept in
`~/.memdoor/providers.json`. A key value: the key, the NAME of a variable, or
`!command`. `*_BASE_URL`: a proxy or a company gateway in front of the
provider. A pin on a provider that is not connected: the window names the one
to `/connect`. Each provider below did the same coding turn on its fast model
(a planted bug, a failing test, "fix the code, not the test") in 8 to 45
seconds.

`memdoor connect` with no kind scans the shell environment and the project's
`.env`, `.env.local` and `.envrc` for keys by shape (`sk-or-…`, `sk-ant-…`,
`AIza…`/`AQ.…`, `gsk_…`, `xai-…`, `llmgtwy_…`, a bare `sk-…`, or the provider's variable
name for a key with no shape, like `BASETEN_API_KEY`). Order: keys that would
change something, numbered, Enter connects the first; keys the gateway already
reads, named and not offered; the sign-ins under "Sign in, no key:"
(OpenRouter, ChatGPT); the kinds that take a key.

## OpenRouter — the catalogue

- **Key**: `OPENROUTER_API_KEY` · base `https://openrouter.ai/api/v1`
- **Its API states**: window, the top host's completion cap, prices,
  supported parameters, for every vendor's models, so it is the
  catalogue the other providers fall back to.
- **Fast model**: `z-ai/glm-5.3-flash` (the ladder's first rung).
- One key for 380 tool-capable models; the ladder, `/model` with real
  prices and host ordering live here.

## ChatGPT Plus or Pro — no key

- **Sign in**: `memdoor connect chatgpt`, or `/connect chatgpt` in the
  window. The browser opens OpenAI's Sign in with ChatGPT. You approve. The plan
  is probed like a key and kept encrypted in `~/.memdoor/chatgpt-login.json`.
  `memdoor connect --remove chatgpt` signs out.
- **Any door**: with the gateway on a box or over ssh, sign in from your
laptop. The browser's return is passed on to the gateway. In an ssh
session with no browser, open the link anywhere and approve. Then paste the
address the browser lands on (the page does not load).
- **By default**: once signed in, the plan answers your turns on the first
model it lists, before an OpenRouter key (a company's key still comes first).
A model you had pinned on your OpenAI key that the plan also lists moves to
the plan at sign-in, which says so. `memdoor connect --remove
chatgpt` hands the turns back to your key.
- **What answers**: the models your plan lists, over the Responses API, on
  the plan's own allowance. OpenAI opened this to open-source, locally run
  apps in September 2026. A plan is a provider like any other. `/model` shows
  its models. A bare pin that both the plan and an OpenAI key list goes to the
  plan. `openai:<id>` names the key.
- **Limits**: a weekly allowance per app. When it is used up the window says
so and the turn ends there. The decision model needs an OpenRouter
  or decision key, so on a plan alone it is off.
- A Claude subscription has no equivalent. Anthropic's terms forbid it outside
Claude Code, so Claude needs an API key (below).

## OpenAI

- **Key**: `OPENAI_API_KEY` · base `https://api.openai.com/v1`
- **Its API states**: nothing beyond the id, the one vendor without
  model metadata. Windows come from the same model in OpenRouter's
  catalogue (`openai/gpt-5-codex`, 400k), then the public reference. Audio,
  image, realtime and embedding ids stay marked `~`.
- **Fast model**: `gpt-5-mini` (400k / 128k).
- Newer models take `max_completion_tokens`. Memdoor sends it on this host and
switches to it on any host that asks.

## Google Gemini

- **Key**: `GEMINI_API_KEY` · base `https://generativelanguage.googleapis.com/v1beta/openai`
- **Its API states**: `inputTokenLimit`, `outputTokenLimit`, thinking. They
are read from the native `/v1beta/models` rather than the OpenAI-shaped list,
so the figures are Google's own. Embedding and image models are left out.
- **Fast model**: `gemini-3-flash-preview` (1M). The 2.5 line is closed to new
keys. Gemini 3 needs billing enabled on the key.
- Gemini 3's thinking models return a `thought_signature` with each tool call.
Memdoor carries it back on the follow-up as they require.

## Groq

- **Key**: `GROQ_API_KEY` · base `https://api.groq.com/openai/v1`
- **Its API states**: `context_window`, `max_completion_tokens`, prices,
  modalities.
- **Fast model**: `openai/gpt-oss-20b`: 8 seconds on the coding turn, the
fastest here. `qwen/qwen3.8-27b` for a stronger one.
- The free tier rate-limits quickly. Memdoor waits what `Retry-After` says (up
to a minute) before asking again.

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
- **Fast model**: `deepseek-flash` (1M / 393k). `deepseek-v4-pro` for the
larger one.

## Baseten

- **Key**: `BASETEN_API_KEY` · base `https://inference.baseten.co/v1`
- **Its API states**: `context_length`, `max_completion_tokens`, prices,
  modalities, supported features (only tool-capable models are listed).
- **Fast model**: `zai-org/GLM-5.3-Flash`. Also DeepSeek V4, Kimi K3 and
Nemotron. Ids carry case. Type them as listed.

## EUrouter

- **Key**: `EUROUTER_API_KEY` · base `https://api.eurouter.ai/api/v1`.
  `memdoor connect eurouter` takes the key. EUrouter routes each request to a
  host in the EU and the answer names the host (`aki/…`, `berget/…`).
- **Its API states**: `context_length`, `top_provider.max_completion_tokens`,
  `supported_parameters`, the endpoints each model serves, and a price in
  dollars or euros. Only chat models that call tools are listed (98 of 159 on
  2026-11-08). A price in euros is shown in euros (`€0.09 / €0.28`).
- **Fast model**: `deepseek-v4.1-flash-1017` (1M / 393k, $0.20 / $0.60).
  `mistral-small-3.2-24b` and `gpt-oss-120b` are priced in euros.
- With `EUROUTER_API_KEY` and `MEMDOOR_MODEL` set and no other vendor key,
  EUrouter is the only host contacted.

## LLM Gateway

- **Key**: `LLMGATEWAY_API_KEY` · base `https://api.llmgateway.io/v1`.
  `memdoor connect llmgateway` takes the key. One key reaches many vendors;
  the answer names the host that served it (`fireworks/…`).
- **Its API states**: `context_length`, `max_output`, prices, input and
  output modalities, `supported_parameters`, the date a model is switched
  off. Only text models that call tools are listed (177 of 291 on
  2026-11-09).
- **Fast model**: `glm-5.3-flash` (1M / 128k, $0.09 / $0.25).
- A new account answers 402 until it has credit: add some at llmgateway.io.
- With `LLMGATEWAY_API_KEY` and `MEMDOOR_MODEL` set and no other vendor key,
  LLM Gateway is the only host contacted.

## Vercel AI Gateway

- **Key**: `AI_GATEWAY_API_KEY` (Vercel's name), or `VERCEL_API_KEY` holding a
  `vck_…` key · base
  `https://ai-gateway.vercel.sh/v1`; `VERCEL_BASE_URL` replaces it.
  `memdoor connect vercel` takes the key.
- **Its API states**: `type`, `context_window`, `max_tokens`, prices,
  `supported_parameters`, reasoning options and `deprecated_at`. Only
  language models that call tools and are not past their deprecation are
  listed (302 of 498 on 2026-11-09). Ids are `vendor/model`
  (`alibaba/qwen-3-14b`).
- **Fast model**: `deepseek/deepseek-v4-flash` (1M / 384k, $0.13 / $0.26).
  `openai/gpt-oss-20b` for a cheaper one.
- The account needs a card on file before it serves a request. Until then
  `memdoor connect vercel` connects the key and says the account needs a card.
  The free tier refuses some models (`anthropic/claude-haiku-5.5`).
- Vercel's ids are OpenRouter's. `/model-search vercel` lists Vercel's
  models; `/model vercel:<id>` pins one.
- `AI_GATEWAY_API_KEY` is not your company gateway's token. With
  `AI_GATEWAY_BASE_URL` set, the company gateway answers.

## Anthropic

- **Key**: `ANTHROPIC_API_KEY` · base `https://api.anthropic.com`
- **Its API states**: `max_input_tokens`, `max_tokens`, capabilities
  (image input, thinking, effort levels) per model.
- **Fast model**: `claude-haiku-4-5-20251001` (200k / 64k). Sonnet 5, Opus 4.6
and Fable 5.1 are 1M / 128k.
- A Claude subscription is not an API key. The key comes from
console.anthropic.com. Responses stream.

## Your company's AI gateway

- **Variables**: `AI_GATEWAY_BASE_URL` + `AI_GATEWAY_TOKEN` (or any
  `*_AI_GATEWAY_TOKEN`) + `MEMDOOR_MODEL` = the provider slug the gateway
  serves. `AI_GATEWAY_API=chat` if it speaks chat/completions rather than the
  Responses API.
- **What it states**: whatever its `/models` lists (window and cap are read
  when present). A slug it does not list is declared by `MEMDOOR_MODEL` and
  `MEMDOOR_MODEL_CONTEXT`.
- With it set, nothing but the gateway is contacted: no OpenRouter, no
  memdoor.ai, no catalogue, no public reference (the last copy on disk
  answers). [Where your data goes](/docs/security).

## A local runtime — no key

`memdoor connect local` (or `/connect local`): an OpenAI-compatible server
on this machine or on the gateway's box: janis, Ollama, llama.cpp, LM Studio,
vLLM. No token asked or sent. Blank base: the runtime on the gateway's machine
(janis 8090, Ollama 11434, llama.cpp 8080, LM Studio 1234); or an address.
Probe: the runtime's model list and one call. `/model search` lists what it
serves at no price; `/model <id>` pins one. Over `--gateway ssh://you@box`
the runtime runs on the box and the window on the laptop. Nothing leaves the
machine the runtime runs on.

Ollama's default window is 4,096 tokens, whatever the model allows. A coder
prompt is longer: the model answers a task it never saw in a few lines and
calls no tool (the meter shows exactly 4096 input tokens on every call).
Start Ollama with `OLLAMA_CONTEXT_LENGTH=32768`, or make a tag with the window
set and pin it:

```
printf 'FROM qwen3:8b\nPARAMETER num_ctx 32768\n' > Modelfile
ollama create qwen3:8b-32k -f Modelfile
```

A runtime that also judges is connected as both.
[janis](https://github.com/guregodevo/janis) answers the decision model's
questions on the same loaded model; `connect local` keeps that as
`local-decisions`, and the [decision
model](/docs/no-babysitting#who-decides-the-decision-model) runs on this
machine with no key. Without that endpoint (Ollama, llama.cpp) decisions stay
off, as on a vendor key.

### The box's model answers the laptop

A gateway on a GPU box is a peer of the laptop's. `memdoor peers add box
http://box:18789 --dir ~/app` names it once signed in (the ssh door signs in
on the way). `@box:coder …` in any conversation runs the turn on the box's
model and returns the answer as the reply. The laptop needs no GPU and no key
for that model. The box's tokens are on its ledger (`memdoor meter` there).

![A laptop with no GPU asks @box:coder; the box runs Qwen3.8-27B through janis on its L4, patches main.go, runs it, and the answer lands in the laptop's window](/local-model.cast)

Recorded 2026-10-26: g2-standard-16, one L4, Google Cloud; janis serving
Qwen3.8-27B 4-bit (the Qwen3.5 hybrid on CUDA); Memdoor installed on the box
through the ssh door. About 5 tokens a second out, a coder prompt read at 250.
A turn that edits and runs a file: a few minutes, all on the box.

## Any OpenAI-compatible endpoint

`memdoor connect custom` with a base URL and key: a vLLM, an internal proxy,
a hosted model. Window and cap from the endpoint when listed
(`context_length`, `max_model_len`), else the reference, else a marked guess
you can declare.

## Attribution — whose request this is

Four fields as headers on every vendor and gateway request, under the names
your gateway expects, for cost attribution by person and team:

```
export MEMDOOR_ATTRIBUTION="user=you@corp.example;team=data-eng"
export MEMDOOR_ATTRIBUTION_HEADERS="user=X-Email;team=X-Team"   # default X-Memdoor-User, X-Memdoor-Team …
```

`project` (the project's directory name) and `session` (the conversation's id)
are filled from the turn. A field with no value sends no header.
`MEMDOOR_VENDOR_HEADERS="Name: value; …"` adds anything static beside them.

## When two providers list the same id

A bare id goes to the active provider. `/model groq:qwen/qwen3.8-27b`,
`/model baseten:zai-org/GLM-5.3-Flash` name the provider. The footer reads
`groq · qwen/qwen3.8-27b · pinned by you`.

## Where a window comes from

In order: the provider's API; the same model in OpenRouter's catalogue; the
public reference (models.dev, read once a day, on disk, never contacted on a
company gateway); the provider's default, shown as `~`, warned once in the
log, declarable in `providers.json`. `memdoor providers --audit`: the count
per provider and how many figures are stated rather than guessed.