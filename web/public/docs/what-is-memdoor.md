# What is Memdoor

**Memdoor is a coding agent in your terminal that runs on your own key, any provider's, and cuts what it spends.**

`cd` into a project, run `memdoor tui`, and type what you want done. It reads
your files, writes patches, runs the build, reads the errors and fixes what it
broke. That part is table stakes.

The difference is a **decision model** in front of the model that writes your
code. It answers typed questions — is this search hit relevant to the task, is
this turn still making progress, does this claim hold up — with a calibrated
probability, in under half a second. The harness acts on those answers, so the
expensive model reads a fraction of the text and gives the same answer.

## The three places it saves you money

- **Judged reads.** Search, file reads and log reads all pass through
  the decision model first. A judged search returns about a twelfth of what
  grep would (the median over 59 searches), each hit with its probability
  beside it.
- **A toolbox sized to the turn.** Tool schemas are resent on *every* call — the
  coder's came to 37% of an edit turn's input when we measured.
  The decision model picks the kind of work the request needs and only those
  tools are sent. Nothing is taken away: a tool it did not advertise still runs
  if the agent calls it.
- **It stops instead of looping.** There is no cap on tool calls, because a cap
  cuts good turns short and cannot rescue bad ones. After each step the decision
  model judges whether the turn is still making progress, and wraps it up with
  what it has when it is not.

## What that measured

Twenty runs, ten pairs: the same tasks on the same model, decisions off and on,
alternating, on the cheapest rung of the ladder.

| The work | Without | With | Saved |
|---|---|---|---|
| A question about the codebase | 57,423 input tokens | 29,458 | **−49%** |
| An edit with a test, run green | 140,862 input tokens | 103,775 | **−26%** |

All twenty answers were right: every edit passed `go test` and `go vet` on its
own copy of the repository, every question named the correct file. Reading less
never cost a correct answer — that is the bar, and the reason each mechanism
errs on the side of sending too much rather than too little.

Per-run variance on edits is wider than the effect, so these come from five
pairs each, never a single run.

## Your key, your bill

You hold the account. Export a provider key (`OPEN_ROUTER_API_KEY`,
`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `DEEPSEEK_API_KEY`, …) or
run `memdoor connect`, and every turn goes straight from your machine to that
provider at their list price — nothing of your code passes through memdoor.ai,
and nothing is added to what you pay. [Providers](/docs/providers) lists them.

Each agent has a ladder of models, cheapest rung first (the coder starts on GLM
5.3 Flash). The footer names the model that
answered and why it was on that rung. `/model` searches every tool-capable model
in the catalogue with its real price, pins one for the conversation, and orders
its hosts by price, throughput or latency.

Every request says `data_collection: deny`, and hosts that train on paid inputs
are excluded from the ladders. A model you pin whose id ends in `:free` is the
exception: its hosts train on what they are sent, the request says `allow`, and
the pin warns.

## Price

**Free** is the whole agent on your own key: the terminal, the ladder, the
model picker, the decision model and workflows with local schedules.

`memdoor savings` prints what it kept out of your bill this month, from your own
turns: bytes the judge read that the chat model did not, tool schemas not sent,
turns ended for want of progress. Your provider's invoice says what you paid;
that says what you did not.

**The decision model is free** and on by default, on an OpenRouter key or a
decision key of your own:
the judged reads, the toolbox sizing, the stop. It reads a fraction of what the
chat model would, and it is your bill it returns.

Workflows are free too: graphs of agent tasks you keep as files, each step
proven, gated on your approval where you say so, run by hand or on a schedule
with your local cron (`memdoor cron add --workflow <name>`).

**Pro is $10 a month:** remote control from your phone through the
memdoor.ai relay, and every workflow run's state kept on memdoor.ai, one
table per workspace, so a workflow can wait on what another produced, from
any machine; coming, runs with your laptop closed. The models always run
on your own key — a seat never supplies one. Subscribe at
[memdoor.ai/pricing](https://memdoor.ai/pricing), or run `memdoor account
subscribe`; cancel any month.

## Today

Command line only. One binary: no Docker, no Python, no app; it runs a small
gateway in the background on your machine.

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
export OPEN_ROUTER_API_KEY=sk-or-...   # or any provider's key, or: memdoor connect
cd your-project && memdoor tui
```

## Next

- **[Getting started](/docs/getting-started)** — install to first turn.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[Why Memdoor](/docs/why)** — the argument in full.
