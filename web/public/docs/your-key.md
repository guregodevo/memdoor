# Your key and the models

Memdoor runs on your key at the provider you choose. One environment
variable is the whole setup:

```bash
export OPENROUTER_API_KEY=sk-or-...    # or ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, DEEPSEEK_API_KEY, …
```

or `memdoor connect`, which probes a key and keeps it. Put it in your shell
profile or in a `.envrc` if you use direnv. Any `memdoor` command starts the
gateway. `memdoor providers` shows each provider connected or not and the one
answering. From then on turns go from your machine straight to that
provider at its list price. Nothing of your code passes through memdoor.ai.
No seat to buy, no account to create.


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

Prices change daily and differ from host to host, so none is quoted here.
`/model` shows today's per host and `memdoor model search` prints the
catalogue's. Most turns finish on rung 1.

## Choosing a model yourself

```bash
memdoor model                        # each agent's ladder, first rung first
memdoor model search glm             # the catalogue: tool-capable models, newest first, real prices
memdoor model providers z-ai/glm-5.3 # who serves it: precision, context, uptime, price
```

In the terminal, without leaving the conversation:

- `/model`: which model is answering and the ladder it sits on
- `/model 3`: pin rung 3 (`rung 3/3 · pinned by you`). The pin is kept in
  `~/.memdoor/model.json` and every new conversation starts on it
- `/model z-ai/glm-5.3 price`: pin any catalogue model, cheapest host first.
  `throughput`, `latency` and `default` are the other orderings
- `/model z-ai/glm-5.3 order baidu,morph`: your host order
- `/model auto`: hand the choice back to the ladder and drop the kept pin
- `/model-search sonnet`: find a model to pin, with its price, from the chat

Anything tool-capable that a connected provider lists can be pinned. A pin by
id goes to the provider that lists it. Models that cannot call a tool cannot
run an agent turn, so the listing leaves them out.

## What travels, and what does not

Every request carries `data_collection: deny` and `require_parameters`. A
host that might train on your code, or that silently drops the tool schema,
does not serve you. A `:free` model is the exception, one you pin or the one
an OpenRouter account with no credit answers on. Free endpoints exist only on
hosts that train, so the request says `allow` and Memdoor says so at the pin
or when the key is connected. Use one to try Memdoor and keep private code off
it. Hosts known to train on paid inputs are excluded from the ladders.

Your files, sessions, memory and the output of every command the agent runs
stay on your machine. What leaves is the prompt and the excerpts a model needs
to answer. With the decision model on, fewer excerpts leave.

## What it saved you

```
memdoor savings          # this month, from your own turns
```

Judged reads, the tool schemas a narrowed toolbox did not carry, and the turns
ended for want of progress. They are priced at the cheapest model you ran, so
the figure is a floor. The ledger behind it is `~/.memdoor/savings.jsonl`,
append-only. `/usage` shows the same summary inside the terminal.

## The decision model

The judged reads, the per-turn toolbox and the stop come from a decision
model (Jev, by TypeSafe). It is free and runs on your key. An OpenRouter
key turns it on, and so does a decision key (`memdoor connect typesafe`, or
`MEMDOOR_SYSTEMONE_API_KEY`) next to any chat provider. With neither, nothing is
judged and every tool returns its unjudged output.

## Next

- **[Getting started](/docs/getting-started)** — install to first turn.
- **[The TUI](/docs/tui)** — the screens and the keys.
- **[How it works](/docs/how-it-works)** — where each part sits.
