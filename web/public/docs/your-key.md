# Your key and the models

Memdoor runs on **your** account at the provider you choose. One environment
variable is the whole setup:

```bash
export OPEN_ROUTER_API_KEY=sk-or-...   # or ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, DEEPSEEK_API_KEY, …
```

or `memdoor connect`, which probes a key and keeps it. Put it in your shell
profile, or in a `.envrc` if you use direnv, and start the gateway (any
`memdoor` command does). `memdoor providers` shows each provider, connected or
not, and the one answering. From then on every turn goes from your machine
straight to that provider, billed to you at their list price. Nothing of
your code passes through memdoor.ai; there is no seat to buy and no account to
create to write code.


## The ladder

Each agent has a ladder of models, cheapest rung first. A conversation starts on
rung 1 and the footer says so:

```
  ⎇ main  ·  z-ai/glm-5.3-flash  ·  rung 1/3 · first rung · held for this session
```

The coder's ladder today:

| Rung | Model |
|---|---|
| 1 | GLM 5.3 Flash |
| 2 | DeepSeek V4.1 Flash |
| 3 | GLM 5.3 |

Prices change daily and differ from host to host, so none is quoted here:
`/model` shows today's, per host, and `memdoor model search` prints the
catalogue's. Most turns finish on rung 1 — that is the point of it.

## Choosing a model yourself

```bash
memdoor model                        # each agent's ladder, first rung first
memdoor model search glm             # the catalogue: tool-capable models, newest first, real prices
memdoor model providers z-ai/glm-5.3 # who serves it: precision, context, uptime, price
```

In the terminal, without leaving the conversation:

- `/model` — which model is answering, and the ladder it sits on
- `/model 3` — pin rung 3 for this conversation (`rung 3/3 · pinned by you`)
- `/model z-ai/glm-5.3 price` — pin any catalogue model, cheapest host first;
  `throughput`, `latency` and `default` are the other orderings
- `/model z-ai/glm-5.3 order baidu,morph` — your own host order
- `/model auto` — hand the choice back to the ladder
- `/model-search sonnet` — find a model to pin, with its price, from the chat

Anything tool-capable in OpenRouter's catalogue can be pinned. Models that
cannot call a tool cannot run an agent turn, so they are filtered out of the
listing.

## What travels, and what does not

Every request carries `data_collection: deny` and `require_parameters`, so a
host that might train on your code, or that silently drops the tool schema, does
not serve you. Hosts known to train on paid inputs are excluded from the ladders
outright.

Your files, sessions, memory and the output of every command the agent runs
stay on your machine. What leaves is the prompt and the excerpts a model needs to
answer — and with the decision model on, that is a fraction of what it would
otherwise be.

## What it saved you

```
memdoor savings          # this month, from your own turns
```

Judged reads, the tool schemas a narrowed toolbox did not carry, and the turns
ended for want of progress — priced at the cheapest model you actually ran, so
the figure is a floor. The ledger behind it is `~/.memdoor/savings.jsonl`,
append-only, and `/usage` shows the same summary inside the terminal.

## The decision model

The judged reads, the per-turn toolbox and the stop all come from a decision
model (Jev, by TypeSafe). It is free and runs on your own key: an OpenRouter
key turns it on, and so does a decision key (`memdoor connect typesafe`, or
`MEMDOOR_SYSTEMONE_API_KEY`) next to any chat provider. With neither, nothing is
judged and every tool returns its unjudged output.

## Next

- **[Getting started](/docs/getting-started)** — install to first turn.
- **[The TUI](/docs/tui)** — the screens and the keys.
- **[How it works](/docs/how-it-works)** — where each part sits.
