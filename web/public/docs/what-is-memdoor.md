# What is Memdoor

**A lightweight coding and automation agent in a single CLI binary. Run it locally or connect over SSH to another machine, on your gateway, with your provider account. No Docker or Python required.**

Lightweight means simple to deploy. Memdoor is
headless first. Start a run from a shell script, CI or a schedule, or work
interactively from a terminal, your IDE or phone. Finish it from any of
those. It combines four things:

- **Token optimization:** filter reads, select tools and stop stalled turns
  before they consume more model calls. Savings depend on the task.
- **A single binary:** install on macOS, Linux or Windows and connect to
  another machine with `memdoor --gateway ssh://you@box`.
- **A headless-first CLI:** prompts in, answers on stdout and exit codes
  that reflect checks, with the same conversation available to resume.
- **Durable workflows, the deterministic harness:** steps with output
  checks, approval gates and resume after failure, run by hand or on a schedule.

See [Why Memdoor](/docs/why) for the reasons and measurements.

`cd` into a project, run `memdoor`, and type what you want done. It reads
your files, writes patches, runs the build, reads the errors and fixes what it
broke.

The window is one door. Every run is a conversation on a gateway on your
machine. `memdoor run "<prompt>"` is a turn with no window and its exit code is the
receipt of its checks, so a script or a CI job can start one. `memdoor cron add`
puts one on a schedule. The editor and the phone open the same gateway.
`memdoor resume <id>` opens any of those runs in a window, live. Your next line
is its next turn. See [Headless, then attached](/docs/headless) and [The gateway](/docs/gateway).

A **decision model** sits in front of the model that writes your
code. It answers typed questions (is this search hit relevant to the task, is
this turn still making progress, does this claim hold up) with a calibrated
probability in under half a second. The harness acts on those answers. The
expensive model reads a fraction of the text and gives the same answer.

## The three places it saves you money

- **Judged reads.** Search, file reads and log reads pass through
  the decision model first. A judged search returns about a twelfth of what
  grep would (the median over 59 searches). Each hit carries its probability.
- **A toolbox sized to the turn.** Tool schemas are resent on every call. The
  coder's came to 37% of an edit turn's input when we measured.
  The decision model picks the kind of work the request needs and only those
  tools are sent. A tool it did not advertise still runs
  if the agent calls it.
- **It stops instead of looping.** Tool calls have no cap. A cap
  cuts good turns short and cannot rescue bad ones. After each step the decision
  model judges whether the turn is still making progress. When it is not, the turn
  wraps up with what it has.

## What that measured

44 runs, 22 pairs: the same tasks on the same model with decisions off and on,
alternating, on the cheapest rung of the ladder (10 October 2026). Three of
the four tasks are below. The fourth, a second code question (3 pairs, +10%),
is in the full table in
[DECIDE.md](https://github.com/guregodevo/memdoor/blob/main/docs/features/DECIDE.md).

| The work | Without | With | Change |
|---|---|---|---|
| A question about this codebase (3 pairs) | 54,661 input tokens | 14,367 | **−74%** |
| An edit with a test, run green (3 pairs) | 64,274 input tokens | 69,746 | +9% |
| Ten one-file fixes, test green (10 pairs) | 37,374 input tokens | 41,621 | +11% |

22 of 22 passed under each condition: every edit and fix under `go test` on its
own copy, every question naming the right file. The saving is on reads with
something to judge. Reading less never cost a correct answer. That is why each mechanism
errs on the side of sending too much rather than too little.

Per-run variance on edits is wider than the effect, so each row is several
pairs.

## Your key, your bill

You hold the account. Export a provider key (`OPENROUTER_API_KEY`,
`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `DEEPSEEK_API_KEY`, …) or
run `memdoor connect`. Every turn goes straight from your machine to that
provider at their list price. Nothing of your code passes through memdoor.ai. [Providers](/docs/providers) lists them.

Each agent has a ladder of models, cheapest rung first (the coder starts on GLM
5.3 Flash). The footer names the model that
answered and why it was on that rung. `/model` searches every tool-capable model
in the catalogue with its real price, pins one for the conversation, and orders
its hosts by price, throughput or latency.

Every request says `data_collection: deny`. Hosts that train on paid inputs
are excluded from the ladders. A `:free` model is the exception: one you pin, or the
one an OpenRouter account with no credit answers on. Its hosts train on what
they are sent and the request says `allow`.
Memdoor says so at the pin or when the key is connected.

## Price

**Free** is the whole agent: the terminal, the ladder, the
model picker, the decision model and workflows with local schedules.

`memdoor savings` prints what it kept out of your bill this month, from your
turns: bytes the judge read that the chat model did not, tool schemas not sent,
turns ended for want of progress. Your provider's invoice says what you paid.
That says what you did not.

**The decision model is free** and on by default, on an OpenRouter key or a
decision key:
the judged reads, the toolbox sizing, the stop. It reads a fraction of what the
chat model would.

Workflows, the deterministic harness, are free too: graphs of agent tasks,
each step proven, gated on your approval where you say so. Run them by hand or on a schedule
with your local cron (`memdoor cron add --workflow <name>`).

**Every door onto your gateway is free:** the window on a laptop, ssh,
your phone on the gateway's address, and runs with the laptop closed
on [a box that stays on](/docs/gateway). All of it is open source
(Apache-2.0). A company self-hosts it the same way, for free.
Enterprise adds full support, by invoice. **The
hosted gateway** is the one paid thing. memdoor.ai relays your phone to a
gateway kept to localhost and keeps every workflow run's state, one table
per workspace, so a workflow can wait on what another produced from any
machine. Next comes a box that memdoor.ai runs for you. It is $10 a month,
cancel any month. memdoor.ai never supplies a model key. See [memdoor.ai/enterprise](https://memdoor.ai/enterprise).

## Today

Command line only. One binary: no Docker, no Python, no app. It runs a small
gateway in the background on your machine.

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
export OPENROUTER_API_KEY=sk-or-...    # or any provider's key, or: memdoor connect
cd your-project && memdoor
```

## Next

- **[Getting started](/docs/getting-started)** — install to first turn.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[Why Memdoor](/docs/why)** — the argument in full.
