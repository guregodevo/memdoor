# What is Memdoor

**A lightweight coding and automation agent in a single CLI binary. Run it locally or connect over SSH to another machine, on a gateway you own, using your own provider account. No Docker or Python required.**

Lightweight means simple to deploy, not limited to small tasks. Memdoor is
headless first: start a run from a shell script, CI or a schedule, or work
interactively from a terminal, your IDE or phone, and finish it from any of
those. It combines four things:

- **Token optimization:** filter reads, select tools and stop stalled turns
  before they consume more model calls. Savings depend on the task.
- **A single binary:** install on macOS, Linux or Windows and connect to
  another machine with `memdoor --gateway ssh://you@box`.
- **A headless-first CLI:** prompts in, answers on stdout and exit codes
  that reflect checks, with the same conversation available to resume.
- **Durable workflows:** task files in your repo, output checks, approval
  gates and resume after failure, run by hand or on a schedule.

See [Why Memdoor](/docs/why) for the reasons and measurements.

`cd` into a project, run `memdoor`, and type what you want done. It reads
your files, writes patches, runs the build, reads the errors and fixes what it
broke. That part is table stakes.

The window is one door. Every run is a conversation on a gateway on your
machine: `memdoor run "<prompt>"` is a turn with no window, its exit code the
receipt of its checks, so a script or a CI job can start one; `memdoor cron add`
puts one on a schedule; the editor and the phone open the same gateway; and
`memdoor resume <id>` opens any of those runs in a window, live, your next line
its next turn. See [Headless, then attached](/docs/headless) and [The gateway](/docs/gateway).

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

44 runs, 22 pairs: the same tasks on the same model, decisions off and on,
alternating, on the cheapest rung of the ladder (10 October 2026). Three of
the four tasks are below; the fourth, a second code question (3 pairs, +10%),
is in the full table in
[DECIDE.md](https://github.com/guregodevo/memdoor/blob/main/docs/features/DECIDE.md).

| The work | Without | With | Change |
|---|---|---|---|
| A question about this codebase (3 pairs) | 54,661 input tokens | 14,367 | **−74%** |
| An edit with a test, run green (3 pairs) | 64,274 input tokens | 69,746 | +9% |
| Ten one-file fixes, test green (10 pairs) | 37,374 input tokens | 41,621 | +11% |

22 of 22 passed under each condition: every edit and fix under `go test` on its
own copy, every question naming the right file. The saving is on reads with
something to judge; where the task reads the same files either way, nothing
changes. Reading less
never cost a correct answer — that is the bar, and the reason each mechanism
errs on the side of sending too much rather than too little.

Per-run variance on edits is wider than the effect, so each row is several
pairs, never a single run.

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

**Every door onto your own gateway is free:** the window on a laptop, ssh,
your phone on the gateway's own address, and runs with the laptop closed
on [a box that stays on](/docs/gateway). **Pro is $10 a month** for what
memdoor.ai hosts for you: the relay for a gateway kept to localhost, and
every workflow run's state kept on memdoor.ai, one table per workspace, so
a workflow can wait on what another produced, from any machine; next, a box
that memdoor.ai runs for you. The models always run
on your own key — a seat never supplies one. Subscribe at
[memdoor.ai/pricing](https://memdoor.ai/pricing), or run `memdoor account
subscribe`; cancel any month.

## Today

Command line only. One binary: no Docker, no Python, no app; it runs a small
gateway in the background on your machine.

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
export OPEN_ROUTER_API_KEY=sk-or-...   # or any provider's key, or: memdoor connect
cd your-project && memdoor
```

## Next

- **[Getting started](/docs/getting-started)** — install to first turn.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[Why Memdoor](/docs/why)** — the argument in full.
