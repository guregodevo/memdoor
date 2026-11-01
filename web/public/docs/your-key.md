# Your key and the models

One environment variable is the setup:

```bash
export OPENROUTER_API_KEY=sk-or-...    # or ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, DEEPSEEK_API_KEY, …
```

Or `memdoor connect`, which probes a key and keeps it. Put the key in the
shell profile or a `.envrc`. Any `memdoor` command starts the gateway.
`memdoor providers` lists providers, connected or not, and the one
answering. Turns go from your machine to the provider at list price. No
code passes through memdoor.ai. No seat, no account.

## The ladder

Each agent has a ladder of models, cheapest rung first. A conversation
starts on rung 1; the footer says so:

```
  ⎇ main  ·  z-ai/glm-5.3-flash  ·  rung 1/3 · first rung · held for this session
```

The coder's ladder today:

| Rung | Model |
|---|---|
| 1 | GLM 5.3 Flash |
| 2 | DeepSeek V4.1 Flash |
| 3 | GLM 5.3 |

Prices change daily and differ by host, so none is quoted here. `/model`
shows today's per host; `memdoor model search` prints the catalogue's. Most
turns finish on rung 1.

## Choosing a model yourself

```bash
memdoor model                        # each agent's ladder, first rung first
memdoor model search glm             # the catalogue: tool-capable models, newest first, real prices
memdoor model providers z-ai/glm-5.3 # who serves it: precision, context, uptime, price
```

In the terminal:

- `/model`: the model answering and its ladder
- `/model 3`: pin rung 3 (`rung 3/3 · pinned by you`). Kept in
  `~/.memdoor/model.json`; every new conversation starts on it
- `/model z-ai/glm-5.3 price`: pin a catalogue model, cheapest host first.
  Other orderings: `throughput`, `latency`, `default`
- `/model z-ai/glm-5.3 order baidu,morph`: your host order
- `/model auto`: back to the ladder; drops the kept pin
- `/model-search sonnet`: find a model to pin, with its price

Any tool-capable model a connected provider lists can be pinned. A pin by
id goes to the provider that lists it. Models that cannot call a tool are
left out: they cannot run an agent turn.

## What travels, and what does not

Every request: `data_collection: deny` and `require_parameters`. A host that
might train on your code or drops the tool schema does not serve you.
Exception: a `:free` model, pinned or the one an OpenRouter account with no
credit answers on. Free endpoints exist only on hosts that train, so the
request says `allow`; Memdoor says so at the pin or when the key is
connected. Keep private code off it. Hosts known to train on paid inputs
are excluded from the ladders.

Stays on your machine: files, sessions, memory, every command's output.
Leaves: the prompt and the excerpts a model needs. With the decision model
on, fewer excerpts.

## What it saved you

```
memdoor savings          # this month, from your own turns
```

Counts judged reads, tool schemas a narrowed toolbox did not carry, and
turns ended for want of progress, priced at the cheapest model you ran: a
floor. Ledger: `~/.memdoor/savings.jsonl`, append-only. `/usage` shows the
same in the terminal.

## The decision model

Judged reads, the per-turn toolbox and the stop come from a decision model
(Jev, by TypeSafe). Free, on your key. On with an OpenRouter key, or with a
decision key (`memdoor connect typesafe`, or `MEMDOOR_SYSTEMONE_API_KEY`)
next to any chat provider. With neither: nothing judged, every tool returns
its full output.

## Next

- **[Getting started](/docs/getting-started)** — install to first turn.
- **[The TUI](/docs/tui)** — the screens and the keys.
- **[How it works](/docs/how-it-works)** — where each part sits.
