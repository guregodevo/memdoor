# Models

You do not have to pick a model to start. Each agent has a ladder with the
cheapest rung first. The footer names the model that answered and why. When
you want to choose, everything below is one command.

## Your company's key

At a company that approves vendors (Claude Code on an Anthropic key, Codex
on an OpenAI one, Gemini) Memdoor runs on the key the company gave you.
Nothing else leaves your machine: no OpenRouter, no memdoor.ai, no
catalogue, no web search. The data path is the one your security team
reviewed for those tools.

```
export ANTHROPIC_API_KEY=…            # or OPENAI_API_KEY / GEMINI_API_KEY
export MEMDOOR_MODEL=claude-sonnet-5  # OpenAI and Gemini need the model named
cd your-project && memdoor
```

A company with an **AI gateway** of its own, with provider integrations set
up in its UI and a provider slug per model, needs two variables and the
slug:

```
export AI_GATEWAY_BASE_URL=https://ai-gateway.example.com/ai
export AI_GATEWAY_TOKEN=…             # or any *_AI_GATEWAY_TOKEN your preset sets
export MEMDOOR_MODEL=my-provider-slug
```

Memdoor calls `POST <base>/v1/responses` with `{"model": "<slug>", "input":
…}` as the gateway's own curl example does, and nothing else.
`AI_GATEWAY_API=chat` if yours speaks chat/completions instead. A proxy in
front of a vendor is `ANTHROPIC_BASE_URL`, `OPENAI_BASE_URL` or
`GEMINI_BASE_URL`. Cost-attribution tags are
`MEMDOOR_VENDOR_HEADERS="X-Team: data; X-Email: you@corp.example"`, sent
on every request. The gateway's first log line says which key serves it
and `/model` shows that one model. Workflows run locally with no memdoor.ai
call. The decision model stays off unless the gateway holds a decision key. The full table of the hosts the binary can name, and when, is
[docs/SECURITY.md](/docs/security), the page to hand your reviewer.

## Connect a provider

Every provider, its key, what its API states and its fast model:
[Providers](/docs/providers).

Models come from providers, each with a list: OpenRouter (the
catalogue with real prices), the company AI gateway, Anthropic, OpenAI,
Gemini, Groq, xAI Grok, DeepSeek, Baseten and any OpenAI-compatible endpoint
you add. `memdoor providers` shows them connected or not, with where each
credential came from and ● on the one answering. `/model search` and
`/model <id>` search and pin across every connected provider.

```
memdoor connect                 # pick the kind, base URL prefilled, token unseen
memdoor connect anthropic       # straight to one kind
memdoor connect custom --probe --base https://host/v1 --key MY_TOKEN_VAR
```

Before anything is kept the gateway reads the provider's model list and
makes one small call. A wrong URL or token fails there, in words. The
token can be the value, the NAME of an environment variable that holds it,
or a `!command` that prints it (Keychain, 1Password, Vault). The file keeps
what you typed and never the resolved secret. `--remove <id>` forgets one.
When two providers list the same id, name the provider:
`/model groq:qwen/qwen3.8-27b`.

## A local LLM with janis

A model on your machine, with no key and no provider:
[janis](https://github.com/guregodevo/janis) is Memdoor's runtime. One
Go server runs the open models on a Mac's GPU through MLX and on an NVIDIA
box through CUDA. It serves both the chat and the
[decision model](/docs/no-babysitting#who-decides-the-decision-model) to
Memdoor. Ollama, llama.cpp and LM Studio work the same way for the chat
half.

```
git clone https://github.com/guregodevo/janis && cd janis
make build && go build -o janis ./cmd/janis   # on Linux with a GPU: ./scripts/fetch-ggml.sh --cuda first, then -tags ggml
./janis pull                                  # the model that fits this machine; or: pull Qwen3-Coder-30B-A3B-Instruct-4bit
./janis serve                                 # http://127.0.0.1:8090/v1 — chat completions, tools, decisions
memdoor connect local                         # finds it; chat and decisions, no token
```

In the window `/model search` lists what janis serves at no price.
`/model <id>` pins one and the footer names it on every answer. janis loads
a model at the first request that names it and swaps when another is asked
for. Pin the one you mean.

On a box with a GPU, the same thing through the ssh door, with your laptop
and phone as windows onto it:

```
memdoor --gateway ssh://you@box connect local   # installs Memdoor on the box, finds janis there
cd your-project && memdoor --gateway ssh://you@box
```

Measured on a Google L4 at $0.71 an hour and nothing per token: Qwen3-8B
about 38 tokens a second and Qwen3-Coder-30B-A3B (128 experts, 8 active)
11. The numbers and what limits them are in janis's README. On janis the
decision model runs there too. On the other runtimes decisions stay off.

## The catalogue, at real prices

Your OpenRouter key sees the catalogue, so the listing carries the
list price you pay per million tokens:

```
memdoor model                        # each agent's ladder, first rung first
memdoor model search glm             # tool-capable models matching "glm", newest first
memdoor model providers z-ai/glm-5.3 # its hosts: precision, context, uptime, price
```

A model that cannot call a tool cannot run an agent turn, so the listing leaves
those out.

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

Pinning changes nothing else: same session, same history, same memory, same
tools. The footer shows `pinned by you`. The pin is kept in
`~/.memdoor/model.json` on the gateway. Every new conversation starts on it.
A conversation that pins its own keeps that.

## What each turn cost

```
memdoor meter          # every turn: model served, tokens, duration
```

Appended to `~/.memdoor/meter.jsonl` and never rewritten. Check it against
your provider's invoice.

## Next

- **[Your key and the models](/docs/your-key)** — the ladder and the policy behind it.
- **[The TUI](/docs/tui)** — the screens.
