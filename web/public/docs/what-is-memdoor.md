# What is Memdoor

The CLI coding agent with Team Mode: cheap models, frontier results, on your machine.
Five Qwen3.8-27B agents beat Fable 5 on LiveCodeBench Hard, every run published. Ollama, llama.cpp, or any key. One binary, no account.

Headless first: a run starts from a shell script, CI, a schedule, a
terminal, an IDE or a phone, and finishes from any of them. Four parts:

- **Token optimization:** judged reads, a per-turn toolbox, a stop for
  stalled turns. Savings depend on the task.
- **A single binary:** macOS, Linux, Windows. `memdoor --gateway
  ssh://you@box` reaches another machine.
- **A headless-first CLI:** prompt in, answer on stdout, exit code from the
  checks, the conversation resumable.
- **Durable workflows, the deterministic harness:** steps with output
  checks, approval gates, resume after failure, by hand or on a schedule.

Reasons and measurements: [Why Memdoor](/docs/why).

`cd` into a project, run `memdoor`, type the task. It reads files, writes
patches, runs the build, fixes the errors.

Every run is a conversation on the gateway on your machine. Doors: the
window; `memdoor run "<prompt>"`, no window, exit code from the checks;
`memdoor cron add`, a schedule; the editor; the phone; `memdoor resume
<id>`, any run back in a window. See [Headless, then attached](/docs/headless)
and [The gateway](/docs/gateway).

Decision model, in front of the chat model: typed questions (is this hit
relevant, is the turn progressing, does this claim hold) answered with a
calibrated probability in under half a second. The harness acts on the
answers; the chat model reads a fraction of the text.

## Team Mode

Shift+Tab flips the window from `auto` to `team`. In team, a turn is the
[team-solve](/docs/team-solve) workflow on this project and the model the
footer names: a single call first, then a team of fresh contexts when the
check fails, and the single call is kept when the team does no better.
[Team mode](/docs/team-mode) says how it works; [the result](/docs/team-solve)
has the runs.

## The three places it saves you money

- **Judged reads.** Search, file and log reads pass through the decision
  model. A judged search returns about a twelfth of grep's output (median
  over 59 searches), each hit with its probability.
- **A toolbox sized to the turn.** Tool schemas are resent on every call
  (the coder's: 37% of an edit turn's input). Only the tools for the kind
  of work are sent; an unadvertised tool still runs if called.
- **A stop instead of a cap.** No cap on tool calls. After each step the
  decision model judges progress; none, and the turn wraps up with what it
  has.

## What that measured

44 runs, 22 pairs: same tasks, same model, decisions off and on,
alternating, on the cheapest rung (10 October 2026). Three of four tasks
below. The fourth, a second code question (3 pairs, +10%), is in
[DECIDE.md](https://github.com/guregodevo/memdoor/blob/main/docs/features/DECIDE.md).

| The work | Without | With | Change |
|---|---|---|---|
| A question about this codebase (3 pairs) | 54,661 input tokens | 14,367 | **−74%** |
| An edit with a test, run green (3 pairs) | 64,274 input tokens | 69,746 | +9% |
| Ten one-file fixes, test green (10 pairs) | 37,374 input tokens | 41,621 | +11% |

22 of 22 passed under each condition: edits and fixes under `go test` on
their own copy, questions naming the right file. The saving is on reads
with something to judge; each mechanism errs toward sending too much.
Per-run variance on edits exceeds the effect, hence several pairs per row.

## Your key, your bill

Export a provider key (`OPENROUTER_API_KEY`, `ANTHROPIC_API_KEY`,
`OPENAI_API_KEY`, `GEMINI_API_KEY`, `DEEPSEEK_API_KEY`, …) or run `memdoor
connect`. Turns go from your machine to the provider at list price. No code
passes through memdoor.ai. [Providers](/docs/providers) lists them.

Ladder: per agent, cheapest rung first (the coder starts on GLM 5.3
Flash). The footer names the model that answered and its rung. `/model`
searches tool-capable models with prices, pins one, orders its hosts by
price, throughput or latency.

Training: every request says `data_collection: deny`; hosts that train on
paid inputs are off the ladders. Exception: a `:free` model, pinned or the
one a creditless OpenRouter account answers on. Its hosts train on what
they are sent; the request says `allow`; Memdoor says so at the pin or at
connect.

## Price

Free: the terminal, the ladder, the model picker, the decision model,
workflows with local schedules.

`memdoor savings`: what the decision model kept out of the bill this
month: bytes the judge read that the chat model did not, tool schemas not
sent, turns ended for want of progress. On by default with an OpenRouter
key or a decision key.

Workflows, the deterministic harness: free. Graphs of agent tasks, each
step proven, gated where you put a gate, by hand or on local cron
(`memdoor cron add --workflow <name>`).

Doors onto your gateway: free. The window, ssh, the phone on the gateway's
address, runs with the laptop closed on [a box that stays on](/docs/gateway).
Open source, Apache-2.0; a company self-hosts the same way. Enterprise:
support, by invoice.

Install:

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
export OPENROUTER_API_KEY=sk-or-...    # or any provider's key, or: memdoor connect
cd your-project && memdoor
```

## Next

- **[Getting started](/docs/getting-started)** — install to first turn.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[Why Memdoor](/docs/why)** — the argument in full.
