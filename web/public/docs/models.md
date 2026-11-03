# Models

No model to pick at the start. Each agent has a ladder, cheapest rung first.
The footer names the model that answered and why.

## Your company's key

A company that approves vendors (Claude Code on an Anthropic key, Codex on
an OpenAI one, Gemini): Memdoor runs on that key. Nothing else leaves the
machine: no OpenRouter, no memdoor.ai, no catalogue, no web search. The
data path is the one your security team reviewed for those tools.

```
export ANTHROPIC_API_KEY=…            # or OPENAI_API_KEY / GEMINI_API_KEY
export MEMDOOR_MODEL=claude-sonnet-5  # OpenAI and Gemini need the model named
cd your-project && memdoor
```

A company AI gateway, with provider integrations in its UI and a provider
slug per model: two variables and the slug.

```
export AI_GATEWAY_BASE_URL=https://ai-gateway.example.com/ai
export AI_GATEWAY_TOKEN=…             # or any *_AI_GATEWAY_TOKEN your preset sets
export MEMDOOR_MODEL=my-provider-slug
```

Request: `POST <base>/v1/responses` with `{"model": "<slug>", "input": …}`,
as the gateway's own curl example, nothing else. `AI_GATEWAY_API=chat` for
chat/completions. A proxy in front of a vendor: `ANTHROPIC_BASE_URL`,
`OPENAI_BASE_URL`, `GEMINI_BASE_URL`. Cost-attribution tags:
`MEMDOOR_VENDOR_HEADERS="X-Team: data; X-Email: you@corp.example"`, on every
request. The gateway's first log line names the key serving it; `/model`
shows that one model. Workflows run locally with no memdoor.ai call. The
decision model stays off without a decision key. Every host the binary can
name, and when: [docs/SECURITY.md](/docs/security).

## Connect a provider

Every provider, its key, what its API states and its fast model:
[Providers](/docs/providers).

Providers, each with a list: OpenRouter (the catalogue with real prices),
the company AI gateway, Anthropic, OpenAI, Gemini, Groq, xAI Grok, DeepSeek,
Baseten, any OpenAI-compatible endpoint you add. `memdoor providers`: each
one connected or not, where its credential came from, ● on the one
answering. `/model search` and `/model <id>` work across every connected
provider.

```
memdoor connect                 # pick the kind, base URL prefilled, token unseen
memdoor connect anthropic       # straight to one kind
memdoor connect custom --probe --base https://host/v1 --key MY_TOKEN_VAR
```

Before keeping anything the gateway reads the provider's model list and
makes one small call; a wrong URL or token fails there, in words. Token
forms: the value, the NAME of an environment variable, or a `!command` that
prints it (Keychain, 1Password, Vault). The file keeps what you typed, never
the resolved secret. `--remove <id>` forgets one. Two providers with the
same id: `/model groq:qwen/qwen3.8-27b`.

## A local LLM with janis

A model on your machine, no key, no provider:
[janis](https://github.com/guregodevo/janis) is Memdoor's runtime, the one
we recommend. One Go server runs open models on a Mac's GPU through MLX and
on an NVIDIA box through CUDA. It serves the chat model and the
[decision model](/docs/no-babysitting#who-decides-the-decision-model).
Ollama, llama.cpp and LM Studio serve the chat half.

```
memdoor connect local                         # nothing running: installs janis and the model that fits, serves it, connects
```

By hand (a CUDA build: janis's README):

```
curl -fsSL https://github.com/guregodevo/janis/releases/latest/download/install.sh | sh   # Linux, macOS (Apple silicon)
janis pull                                    # the model that fits this machine; or: pull Qwen3-Coder-30B-A3B-Instruct-4bit
janis serve                                   # http://127.0.0.1:8090/v1 — chat completions, tools, decisions
memdoor connect local                         # finds it; chat and decisions, no token
```

`/model search` lists what janis serves, at no price. `/model <id>` pins
one; the footer names it. janis loads a model at the first request naming
it and swaps when another is asked for, so pin the one you mean.

On a GPU box, through the ssh door, laptop and phone as windows onto it:

```
memdoor --gateway ssh://you@box connect local   # installs Memdoor on the box, finds janis there
cd your-project && memdoor --gateway ssh://you@box
```

Measured on a Google L4 at $0.71 an hour, nothing per token: Qwen3-8B about
38 tokens a second, Qwen3-Coder-30B-A3B (128 experts, 8 active) 11. Details
in janis's README. On janis the decision model runs there too; on the other
runtimes decisions stay off.

## The catalogue, at real prices

An OpenRouter key sees the catalogue, with the list price per million
tokens:

```
memdoor model                        # each agent's ladder, first rung first
memdoor model search glm             # tool-capable models matching "glm", newest first
memdoor model providers z-ai/glm-5.3 # its hosts: precision, context, uptime, price
```

Models that cannot call a tool are left out: they cannot run an agent turn.

![/model: each agent's ladder at real prices; pin a rung or any model](/demo-model.cast)

## Pinning one

In the TUI:

| Type this | What happens |
|---|---|
| `/model` | The ladder, the current rung, and the reason |
| `/model 3` | Pin rung 3; the pin stays for every new conversation until `/model auto` |
| `/model z-ai/glm-5.3` | Pin that model, cheapest qualifying host first |
| `/model z-ai/glm-5.3 throughput` | Pin it, fastest host first (`latency`, `price`, `default`) |
| `/model z-ai/glm-5.3 order baidu,morph` | Your own host order |
| `/model auto` | Hand the choice back to the ladder, and drop the kept pin |
| `/model-search sonnet` | Search the catalogue from the chat, then pin from the result |

A pin changes nothing else: same session, history, memory, tools. Footer:
`pinned by you`. Kept in `~/.memdoor/model.json` on the gateway; every new
conversation starts on it. A conversation with its own pin keeps that.

## What each turn cost

```
memdoor meter          # every turn: model served, tokens, duration
```

Appended to `~/.memdoor/meter.jsonl`, never rewritten. Check it against
the provider's invoice.

## Next

- **[Your key and the models](/docs/your-key)** — the ladder and the policy behind it.
- **[The TUI](/docs/tui)** — the screens.
