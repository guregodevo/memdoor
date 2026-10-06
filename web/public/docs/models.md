# Models

Memdoor never makes you pick a model to get started: each agent has a ladder,
cheapest rung first, and the footer names the model that answered and why. When
you do want to choose, everything below is one command.

## Your company's key

At a company that approves vendors — Claude Code on an Anthropic key, Codex
on an OpenAI one, Gemini — Memdoor runs on the key the company gave you,
and nothing else leaves your machine: no OpenRouter, no memdoor.ai, no
catalogue, no web search. The data path is the one your security team
already reviewed for those tools.

```
export ANTHROPIC_API_KEY=…            # or OPENAI_API_KEY / GEMINI_API_KEY
export MEMDOOR_MODEL=claude-sonnet-5  # OpenAI and Gemini need the model named
memdoor gateway
```

A company that runs its **own AI gateway** — provider integrations set up
in its UI, a provider slug per model, a personal or service-account token —
is two variables and the slug:

```
export AI_GATEWAY_BASE_URL=https://ai-gateway.example.com/ai
export AI_GATEWAY_TOKEN=…             # or any *_AI_GATEWAY_TOKEN your preset sets
export MEMDOOR_MODEL=my-provider-slug
```

Memdoor calls `POST <base>/v1/responses` with `{"model": "<slug>", "input":
…}` exactly as the gateway's own curl example does (`AI_GATEWAY_API=chat`
if yours speaks chat/completions instead), and nothing else. A proxy in
front of a vendor is `ANTHROPIC_BASE_URL` /
`OPENAI_BASE_URL` / `GEMINI_BASE_URL`, and cost-attribution tags are
`MEMDOOR_VENDOR_HEADERS="X-Team: data; X-Email: you@corp.example"`, sent
on every request. The gateway's first log line says which key serves it;
`/model` shows that one model. Workflows run locally with no memdoor.ai
call at all; the decision model stays off
unless the gateway holds a decision key of its own. The full table of every
host the binary can name, and when, is
[docs/SECURITY.md](/docs/security)
— the page to hand your reviewer.

## Connect a provider

Every provider, its key, what its API states and its fast model:
[Providers](/docs/providers).

Models come from providers, each with its own list: OpenRouter (the
catalogue with real prices), the company AI gateway, Anthropic, OpenAI,
Gemini, Groq, xAI Grok, DeepSeek, Baseten, and any OpenAI-compatible endpoint you add. `memdoor providers`
shows them — connected or not, where each credential came from, and ●
on the one answering. `/model search` and `/model <id>` are unchanged:
they search and pin across every connected provider.

```
memdoor connect                 # pick the kind, base URL prefilled, token unseen
memdoor connect anthropic       # straight to one kind
memdoor connect custom --probe --base https://host/v1 --key MY_TOKEN_VAR
```

Before anything is kept, the gateway reads the provider's model list and
makes one small call; a wrong URL or token fails there, in words. The
token can be the value, the NAME of an environment variable that holds it,
or `!command` that prints it (Keychain, 1Password, Vault) — the file keeps
what you typed, never the resolved secret. `--remove <id>` forgets one. When two providers list the same id, name
the provider: `/model groq:qwen/qwen3.8-27b`.

## The catalogue, at real prices

Your OpenRouter key sees the whole catalogue, so the listing carries the list
price you will actually pay per million tokens:

```
memdoor model                        # each agent's ladder, first rung first
memdoor model search glm             # tool-capable models matching "glm", newest first
memdoor model providers z-ai/glm-5.3 # its hosts: precision, context, uptime, price
```

A model that cannot call a tool cannot run an agent turn, so the listing leaves
those out.

![/model: each agent's ladder at real prices; pin a rung or any model](/demo-model.cast)

## Pinning one for a conversation

In the TUI:

| Type this | What happens |
|---|---|
| `/model` | The ladder, the current rung, and the reason |
| `/model 3` | Pin rung 3 for this conversation |
| `/model z-ai/glm-5.3` | Pin that model, cheapest qualifying host first |
| `/model z-ai/glm-5.3 throughput` | Pin it, fastest host first (`latency`, `price`, `default`) |
| `/model z-ai/glm-5.3 order baidu,morph` | Your own host order |
| `/model auto` | Hand the choice back to the ladder |
| `/model-search sonnet` | Search the catalogue from the chat, then pin from the result |

Pinning changes nothing else: same session, same history, same memory, same
tools. The footer shows `pinned by you` so you never wonder what answered.

## What each turn cost

```
memdoor meter          # every turn: model served, tokens, duration
```

Appended to `~/.memdoor/meter.jsonl` and never rewritten, so the ledger is
yours to check against your OpenRouter invoice.

## Next

- **[Your key and the models](/docs/your-key)** — the ladder and the policy behind it.
- **[The TUI](/docs/tui)** — the screens.
