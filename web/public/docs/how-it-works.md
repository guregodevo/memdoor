# How It Works

Memdoor is a small server on your machine that the terminal talks to. The gateway
runs the agent, its tools, memory and sessions; it sends the *thinking* to a model
on your own account at the provider you chose, and everything else stays local.

## The architecture

```
  memdoor tui / CLI ──► gateway on your machine ──► tools · sessions · git
                              │
                              ├─► decision model (Jev)  — what to read, when to stop
                              │
                              └─► your provider key ──► the model that writes code
```

The decision model and the chat model sit behind the same key. Nothing
downstream can tell which model answered, and nothing of your code passes
through memdoor.ai.

## What happens in a turn

1. **The request is read once.** Before the first call, the decision model is
   asked which kind of work this is, and only the tools that kind needs are put
   in the request. The coder's full palette is about 5,200 tokens of schema, and
   it would otherwise be resent on every call of the turn.
2. **The agent searches and reads.** Search, file reads and log reads pass
   through the decision model: it scores every hit, section or line
   against the task and returns the ones that count, with a probability beside
   each. A file small enough that judging would keep nearly all of it comes back
   whole instead.
3. **It edits and runs.** Patches are applied with their exact context lines, the
   build and the tests are run, and the real output is read back.
4. **It stops when it is done, or when it is not getting anywhere.** After each
   step the decision model judges whether the turn is still making progress. When
   it is not, the turn is wrapped up with what it has and says so. There is no
   call cap.
5. **The turn is metered.** Model served, tokens in and out, duration, appended
   to `~/.memdoor/meter.jsonl` and never rewritten.

## The models

Each agent has a ladder, cheapest rung first. The footer names the model and the rung, and `/model` pins
any rung or any tool-capable model from the catalogue, ordering its hosts by
price, throughput or latency. Every request carries `data_collection: deny` and
`require_parameters` (a pinned `:free` model is the one exception: its hosts
train, the request says `allow`, the pin warns), so no host that might train on your code, or that would
silently drop the tool schema, serves you. See **[Your key and the
models](/docs/your-key)**.

## Sessions

Sessions persist across restarts, compact when they grow (`/compact`), and
rewind when a turn derails (`memdoor sessions rewind`).

## What it is not

- **Not a token reseller.** You hold the account at your provider and pay
  their list price. The subscription is never for inference on your key; workflows are free.
- **Not a hosted agent.** The engine, database and files are on your
  machine.
- **Not an opaque router.** The model, the rung and the reason are in the footer,
  and any model in the catalogue can be pinned by hand.
