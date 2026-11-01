# How It Works

A small server on your machine, the gateway, that the terminal talks to.
It runs the agent, its tools, memory and sessions. Model calls go to your
provider on your key. Everything else stays local.

## The architecture

```
  the window · memdoor run · the phone (/remote) · the editor (memdoor acp)
  CI · cron · --gateway ssh://you@box · --gateway <box> --dir
          │
          ▼
  gateway on your machine (or the box) ──► tools · sessions · git
          │
          ├─► decision model (Jev)  — what to read, when to stop
          │
          └─► your provider key ──► the model that writes code

  memdoor.ai relay ── only for a gateway kept to localhost (MEMDOOR_HOST=localhost)
```

Decision model key: a decision key (`memdoor connect typesafe`), else the
OpenRouter key you code with. On a vendor key, a company's AI gateway or a
ChatGPT plan alone: off until a decision key is set. No code passes through
memdoor.ai.

## What happens in a turn

1. **Toolbox.** The decision model is asked which kind of work the request
   is; only that kind's tools go in the request. The coder's full palette is
   about 5,200 tokens of schema per call.
2. **Judged reads.** Search, file reads and log reads pass through the
   decision model, which scores each hit, section or line against the task
   and returns the ones that count, with a probability each. A file small
   enough that judging would keep nearly all of it comes back whole.
3. **Edit and run.** Patches apply with exact context lines. Build and tests
   run. Real output is read back.
4. **Stop.** After each step the decision model judges progress. No progress:
   the turn wraps up with what it has and says so. No call cap.
5. **Meter.** Model served, tokens in and out, duration, appended to
   `~/.memdoor/meter.jsonl`, never rewritten.

## The models

Each agent has a ladder, cheapest rung first. The footer names the model and
the rung. `/model` pins any rung or any tool-capable model from the catalogue
and orders its hosts by price, throughput or latency. Every request carries
`data_collection: deny` and `require_parameters`: no host that trains on your
code or drops the tool schema. Exception: a `:free` model, pinned or the one
an OpenRouter account with no credit answers on. Its hosts train and the
request says `allow`. Memdoor says so at the pin or when the key is
connected. See **[Your key and the models](/docs/your-key)**.

## Sessions

Persist across restarts. Compact when they grow (`/compact`). A derailed
turn is undone with `memdoor sessions rewind`, an operator command left out
of `--help`.

## What it is not

- **Not a token reseller.** You hold the provider account and pay list
  price. The subscription is never for inference on your key. Workflows
  are free.
- **Not a hosted agent.** Engine, database and files are on your machine.
- **Not an opaque router.** Model, rung and reason are in the footer. Any
  catalogue model can be pinned by hand.
