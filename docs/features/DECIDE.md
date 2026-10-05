# Decisions: Jev through OpenRouter, the OpenClaw way

A decision is a typed question about a piece of state — pick one of these
labels, yes or no, where on this rubric — answered with a probability for
every option and no text to parse. Memdoor gets them from **Jev**, TypeSafe's
decision model, through **OpenRouter**, and uses them in three places: tools
that return only what matters (`jgrep`, `jread`, `jlogs`), per-turn tool routing, the turn-end verdict, subagent result acceptance, and the message gate. Why a vendor call
and where the line is: [ADR-0015](../adr/0015-decisions-through-jev-on-openrouter.md).

The shapes follow OpenClaw's decision-model API (`decisionModel`,
`api.runtime.decisions.evaluate`): one setting picks the model, providers sit
behind it, every consumer handles `unavailable` itself, and **nothing ever
falls back to a chat completion**.

## Setup

**THE DECISION MODEL IS NOT PRO** (Greg, 2026-10-03: "Jev at user key by
default", "jev decision model is not pro"). It reverses 2026-09-27, when the
seat was the decision model: Pi 1.0 hands any user Jev on their own key, so
selling access to it sells nothing. Pro is remote control and the hosted
scheduler. The decision model is
chosen in this order (`autoDecisionModel`, gateway/decision_model.go):

1. **A decision key of the gateway's own**: `MEMDOOR_SYSTEMONE_API_KEY` (or
   `TYPESAFE_API_KEY`). An `sk-or-…` value defaults to OpenRouter's decisions
   endpoint and `~typesafe/jev-latest`.
2. **The person's own OpenRouter key**, the one they code with
   (`OPEN_ROUTER_API_KEY`). On by default: nothing
   to configure. The log says `System One decision provider registered
   endpoint=… own_openrouter_key=true`.
No seat supplies a decision key (Greg, 2026-10-04: "Pro seat never offer
key. It's always with user key"): with neither of the above, decisions are
off. The broker no longer serves decisions at all.

**On a company's vendor key** (providers/vendor.go) nothing but the vendor may
be contacted, so neither the broker nor OpenRouter on the coding key judges;
only an explicit decision key does. With none of the above, decisions answer
`unavailable: not-configured` and the agent works unjudged.

There is no switch (Greg, 2026-09-29: "remove the toggle on and off"): the
`decision_model` setting and `memdoor config decision-model` are gone, and a
row left in a workspace is ignored. A seat changes nothing: decisions run on
the person's own key or none.

The decisions are the harness's, never asked by hand (`memdoor decide` was
removed on 2026-10-03). What they did this month is counted:

```bash
memdoor savings     # judged reads: bytes read by the judge, bytes passed on, calls
```

The typed-question API stays for programs: `POST /api/decisions/evaluate`
with a state and `choice` / `yesno` / `score` questions.

**Without OpenRouter** (2026-10-03, Greg: "how pi do without OpenRouter?"):
Jev is TypeSafe's model and TypeSafe serves it directly. `memdoor connect
typesafe` (or the window's `/connect`) takes a TypeSafe key, probes it with
one real yes/no, and keeps it; decisions then run on it whatever provider
answers the chat (DeepSeek, Anthropic, Baseten …). It sits in `memdoor
providers` as a decisions provider, never in `/model`. When no decision
model is available the window's empty input says so: "Decisions are off ·
memdoor connect typesafe turns them on with any provider", and `/usage`
names the same command.

## Settings

| setting (`PUT /api/workspace/settings`) | values | effect |
|---|---|---|
| `decision_tool_routing` | `on` / `off` | narrow the tools each turn submits (below) |
| `tool_families` | JSON | override or add routing families per agent |
| `decision_turn_verdict` | `on` (default) / `off` | ask whether a turn-ending reply only announces work, and retry it once |
| `decision_turn_stop` | `on` (default) / `off` | when the shadow score says the last steps made no progress, ask whether the run is stuck and end it with a wrap-up |
| `decision_turn_done` | `on` (default) / `off` | at the end of a codebase turn, judge the receipts (files changed, checks run and what they returned) against the request; an unfinished turn gets one more round with the receipts' reason at the top of its prompt |
| `decision_turn_done_threshold` | `0.7` | P(unfinished) that sends a turn back |
| `decision_ask_gate` | `on` (default) / `off` | before `ask_user_question` reaches the person, ask whether the agent could answer it itself by reading, querying, running or checking; a yes sends the question back to the agent with "find out yourself". The same question asked again gets through |
| `decision_ask_gate_threshold` | `0.8` | P(answerable alone) that holds a question back |
| `turn_token_budget` | tokens, e.g. `300000` (default `0`) | run unattended: after the first continuation the turn keeps going, its own "next" as the prompt, while judged unfinished and the budget is not spent; the system prompt tells the model to ask its questions first. `0` is one continuation |
| `decision_result_acceptance` | `on` (default) / `off` | judge a spawned agent's result before it is announced; hand the first unfinished one back |
| `decision_result_acceptance_threshold` | `0.7` | P(unfinished) that hands a result back |
| `decision_message_gate` | `on` / `shadow` / `off` | offer a message nobody @mentioned to the channel's agents; `shadow` asks and logs `would_reply` without waking anyone |
| `decision_gate_threshold` | `0.8` | P(reply) an agent needs to take a gated turn |

All are read per call or per turn. Environment: `OPEN_ROUTER_API_KEY` (or
`OPENROUTER_API_KEY`); `MEMDOOR_SYSTEMONE_URL`, `_API_KEY`, `_MODEL` override
it — `TYPESAFE_API_KEY` for TypeSafe directly, a `http://127.0.0.1:…` URL and
no key for a local Kev server. An origin gets `/v1/systemone` appended; a URL
with a path is used as the endpoint.

## Providers (`pkg/decision`)

- **`systemone`** (`pkg/decision/systemone`) — the System One wire format:
  hosted Jev on OpenRouter (default) or TypeSafe, or a local Kev. Choice ≤ 255
  options, score ≤ 10 levels. 413/422 → `unsupported-input`, 429/503/529 →
  `overloaded`, timeout → `deadline`.

## Consumers

**Reasoning effort** (`gateway/turn_effort.go`) — once per turn, when the
person has not chosen one with Shift+Tab: how open-ended is the request (not
how much work), low / medium / high, ties to the lower, 4 s. No answer, or no
decision model: high. Sent as OpenRouter's `reasoning.effort` to a model that
lists the parameter; GLM 5.3 Flash left without it thinks at its maximum —
on 4 rewrite tasks high cut its output 64% (13,651 → 4,879 tokens), 4/4 passing
either way (2026-09-30). The footer shows the effort in use and where it came
from.

**Jev tools** (`tools/jev_core.go`, `tools/jev_tools.go`, `tools/jgrep.go`) —
standard tools, in the palettes of the agents that get them; each runs its
usual source, asks one yes/no per item against the agent's task in parallel
batches, and returns only the kept items with probabilities. With no model
they return the unjudged output and say `UNJUDGED`. An overloaded answer
(429/503/529) is retried in place before that: at most 3 asks in the decision
service (400 ms, then 800 ms apart, within the caller's timeout), one more
spaced try per batch in the judge — a lasting overload still answers in
seconds (2026-09-28: a
single 503 upstream reset turned a whole turn's tool output UNJUDGED).

| tool | source it reuses | returns | palettes |
|---|---|---|---|
| `notes` read | the folder's notes, split at `## ` | the sections that help with the turn's brief + the latest 3 (`all: true` for the raw tail; unjudged under 12 KB) | coder |
| `jgrep` | grep's walk (`grepWalk`) | matching hunks that matter for the task, ranked | coder |
| `jread` | the file, split into ≤40-line sections | the sections that matter, in file order | coder |
| `jlogs` | `agent_log`'s query | the log events that matter, in time order | none (2026-09-29: it reads Memdoor's own gateway log, not the project's; jgrep and jread read any log file) |
| `decision_evaluate` | — | raw typed answers for any state | none yet |

`jgrep` and `jread` are inside the coder's workdir fence
(`coderPathEscape`, `confineToCoderWorkdir`) like `grep` and `read_file`.

**Tool routing** (`gateway/turn_tools.go`, `gateway/tool_routing.go`) —
OpenClaw's `before_prompt_build` → `toolsAllow`: once per turn, one choice
question over the agent's tool families; the chosen family plus an
always-allowed core becomes the tools *submitted* for the turn. It narrows
what the model sees, never what it may call (enforcement keeps the full
palette). Applied at confidence ≥ 0.75; fails open on timeout (2.5 s), error,
low confidence, unknown family or a prompt over 12 000 characters. Built-in
families for the coder (change, answer, investigate, research);
`tool_families` adds agents. The coder's families were added 2026-09-27 for the cost of the
schemas themselves: its 17 tools serialise to 20,916 bytes (~5,200 tokens)
submitted on EVERY call of a turn — 37% of a 12-call edit turn's input.
Narrowing sends 12 tools and 25,043 bytes instead of 17 and 32,854 (live,
family "change", confidence 1.00, 524 ms). Families err wide because a tool
the turn needs but cannot see gets called with guessed arguments: an investigate turn keeps `apply_patch` because an
investigation that finds the bug fixes it, and the always-allowed core keeps
the file, shell and notes tools. The cut is smaller for it (change −20%,
investigate −16%, answer −37%) — Greg, 2026-09-27: "no risk no quality
degradation". Since 2026-09-29 the coder carries no `jlogs` and no `cron`
(and no "schedule" family): ~760 tokens off every call, routed or not.
Since 2026-09-30 it carries `web_search` and `web_fetch`
([WEB_SEARCH.md](WEB_SEARCH.md)): the `research` family is theirs, and
`investigate` has them too, for an error message or a library the machine
has no answer for.

**Turn-end verdict** (`gateway/turn_verdict.go`) — a reply
with no tool call ends the turn. When it ends on an announcement ("Now
running the tests, then fixing what fails…") the turn is retried once. A phrase list
catches the known wordings for free; when it finds none, one yes/no asks the
decision model whether the reply announces work not yet done, retried at
P ≥ 0.7, 2.5 s timeout. Unavailable leaves the phrase list's answer. Built
after a turn ended on such a sentence with the work not done (2026-09-26);
on six labeled replies Jev scored the two announcements 0.82–0.83 and the
four finished answers 0.02–0.03. The same call asks a second question —
does the reply address the request at all? — retried at P ≥ 0.8 (three
off-request replies 0.89–0.98; answers, a clarifying question and an honest
"it does not exist" 0.06–0.29). Costs one decision per turn end.

**When verifiable, don't ask** (`gateway/ask_gate.go`, 2026-10-03, Greg:
"anything verifiable could save question babysitting", "when verifiable",
"dont ask") — the one interruption a turn has is a question to the person,
and models reach for it to settle what the repository or a command would
settle. Before `ask_user_question` opens the picker, one yes/no over the
task, the question and its options: could the agent answer this alone? At
P ≥ 0.8 the call is refused with the instruction to read, query, run or
check and act on what it finds; the same question asked again reaches the
person, so an insisting model is not walled off. Not yet measured live.

**Done means the receipts show it** (`gateway/turn_done.go`, 2026-10-03,
Greg: "let's use Jev for the harness", "let's start with no babysitting") —
the verdict above reads the model's last sentence; this reads what the turn
did. From the tool records: the files a mutating tool changed, every `verify`
result and every build `apply_patch` folded into its own result, and whether
the last change was checked at all. Every codebase turn that changed or
checked something ends on one receipt line (`✓ checked: 1 file changed ·
\`go test ./...\` PASS`, `⚠ unverified: 1 file changed · no build, test or
run after it`, `⚠ failing after the change: …`). A passing check after the last change is proof and ends the turn
with no question asked (live 2026-10-03: the judge scored a correct,
vet-and-race-tested rate limiter 0.84 and 0.85 unfinished, twice, and a
16k-token round was wasted; proof beats judgment). Otherwise one yes/no,
once per turn: do the receipts contradict the final message — a change
nothing checked, a check that failed, a claim with no change? At P ≥ 0.7 (`decision_turn_done_threshold`) the turn gets one
more round with the receipts' own reason at the top of its prompt ("Not done
yet: nothing was built, tested or run after your last change"). Unavailable
or `decision_turn_done=off`: the receipt line stays, the turn ends as before.
A bash run that exits 0 is a passing check only if its output agrees: the
decision model reads the output of the runs the receipt shows (the last
few, the deciding one among them) and one that shows a failure counts as
failing — live 2026-10-03, `… 2>&1 | tail -20` printed FAIL and returned
tail's 0. The question names no language, tool or test runner, and a run
that only lists or reads is not a failure; each output is judged once.
Two refinements measured on Jev (2026-10-05): a **read-back** — the file
just written, then read (`ls`, `cat`, `head` …) and found — is that file's
check, the proof a file with no build or test can have (a child that wrote
`hello.txt` and `cat`ed it scored 0.90 unfinished without it; with it the
turn is proof and no question is asked); and the evidence carries each
check's **first line of output** (`Check: test -f READY … → PASS · printed:
NO`), because a command that exits 0 either way read as "it exists" against
an honest "not yet" (0.67 → 0.06). `gateway/turn_done_live_test.go` measures
these on the decision model (`MEMDOOR_INTEGRATION_PAID=1`).

A turn with a **declared done** — a workflow task's target, `done_when` on
its session — is not judged: the gateway checks the file or runs the
command, and the receipt line opens with `✓ target X exists` or `⚠ target X
missing` (WORKFLOWS.md). With `turn_token_budget` set, the turn runs on: the continuation's prompt
is the model's own last words ("Continue. You ended on: 'Next I will run go
test'…"), round after round while unfinished, until the budget is spent;
then the window says so and waits. The budgeted turn's system prompt says it
runs unattended, must find out or verify what it can itself (read, query,
run, validate) rather than ask, and must ask with `ask_user_question` before
changing anything only for what cannot be verified without the person. Not yet measured live.

**Turn stop** (`gateway/turn_verdict.go` `stuck`, `gateway/route_shadow.go`)
— the per-turn call cap is gone (2026-09-26: a healthy app-building turn
used all 50 and was cut off). What ends a run going nowhere is a decision:
the first time the shadow score fires (identical calls, error runs, a
test-fail streak that does not shrink — window of the last 8 calls), one
yes/no over those steps in words asks whether the run is stuck; at P ≥ 0.8
the turn ends with a wrap-up round (no tools: what is done and verified,
what failed and why, what to try next) and a notice. Unavailable or `off`
means the run goes on. Measured 2026-09-26 on Jev: the real window of a
healthy app-building turn (two edits, a build, a curl, a new test file, a
compile error fixed, a wrong expectation) scored 0.27; the same failing
test run five times with two edits between, 0.96.

**Subagent result acceptance** (`gateway/result_acceptance.go`) — when a
spawned run ends, before its result is announced to the requester, one
yes/no over the task and the result: is it unfinished (a plan, a partial, a
question back, an error, a bare claim)? The first time, the child runs
again in its own session with its task and a note, and its next result is
announced instead. That one is announced whatever the verdict, with the
verdict in the announcement, so the requester never reports it as done
unknowingly. Failed and timed-out runs and task-flow steps are not judged.
On eight labeled results Jev scored the six unfinished kinds 0.93–0.99 and
the two finished ones 0.08 and 0.18. The session, run registry and queue
are reached through small interfaces naming only the methods used.

**Message gate** (`gateway/decisions.go`, `pkg/message/decision_gate.go`) —
a human message in a channel that @mentions no agent and is not a thread is
offered to each agent member through one yes/no over the last 8 messages;
agents at or above the threshold take a turn. Agent-authored messages are
never gated; unavailable means nobody answers, as before the gate.

**HTTP** — `POST /api/decisions/evaluate` in OpenClaw's shape: `state`,
`questions` keyed by your ids (`type` choice | score | boolean,
`instructions`, `criteria`), `options` (`agentId`, `purpose`, `timeoutMs` ≤ 30 s).
Answers `{"status":"ok","result":{"answers":…,"provider","model"}}` or
`{"status":"unavailable","reason",…}` — HTTP 200 either way. Scores are
zero-based rubric positions, probability-weighted.

## Cost, speed, privacy

- **Cost**: $0.042 per million input tokens on OpenRouter, output free — a
  3-question call on a ticket ≈ $0.0000175; a `jgrep` judging 192 hunks ≈
  $0.002. Billed to the OpenRouter account whose key the gateway holds.
- **Speed**: 0.3–0.5 s for a decision call; `jgrep`/`jread` 0.4–1.1 s on this
  repo, the batches running in parallel.
- **Privacy**: the state — ticket text, code hunks, log lines, the agent's
  task — is sent to OpenRouter and TypeSafe, on the same OpenRouter account
  the chat already uses. Without a key nothing is judged, and on a
  company's vendor key nothing leaves for it unless a decision key is set.

## Measured (2026-09-26, coder on DeepSeek V4.1 Flash)

Coder, 4 tasks (3 code questions on a source snapshot in its workdir, 1 log
investigation), 3 rounds each, Jev on vs off. "Off" still offers the jev
tools; they return their input unjudged.

| run | condition | correct | seconds | LLM calls | tool calls |
|---|---|---|---|---|---|
| after "jgrep first" in the prompt | off | 12/12 | 327 | 83 | 44 |
| | Jev | 12/12 | 291 | 53 | 26 |
| before it (jgrep in 4 of 9 code runs) | off | 11/12 | 796 | 78 | 54 |
| | Jev | 12/12 | 737 | 57 | 41 |

- With Jev the coder needs about a third fewer model calls and 40% fewer
  tool calls for the same answers, and **63% fewer input tokens** (987 k vs
  2.69 M; uncached 182 k vs 383 k; output 15.5 k vs 18.4 k): the judged tool
  output is smaller, so every later call resends less. Wall time differs
  less and varies run to run.
- Long coder task on DeepSeek (a Go module with TTL store, tests, CLI and
  README), twice: both pass `go vet` and `go test -race`, 240 and 321 s.
- An earlier single-run comparison (4/4 vs 4/4, 142 vs 122 s) was flattered:
  jgrep then bypassed the coder's workdir fence, which has been closed.

### Paired runs, 2026-09-27 (coder on its ladder, Jev on vs off)

Greg: "Let's take a task and compare results on and off … prove it's
worth." Three tasks, each run with `decision_model=off` and `systemone`
in alternating order (that setting is since removed: to reproduce, run without
and with a decision key), a fresh channel per run, on rung 1 (GLM 5.3 Flash,
$0.04/M in) and again pinned to rung 3 (GLM 5.3, $0.38/M in). Chat cost is
the broker's metered `cost_usd`; Jev's own calls ran on the gateway's key
and are not metered here (ADR-0015: a judged grep ≈ $0.002, a verdict ≈
$0.00002). All 14 runs were correct: the questions by regex on the reply,
the edit by `go test` and `go vet` on the copy plus the env var present in
code and test. Harness: [`scripts/ab_task.py`](../../scripts/ab_task.py) — it has
since lost its edit task and its decision-model axis (2026-10-04), and now
extracts HEAD itself and asks the two questions below.

- **fail-limit**, **subagent-rerun**: the two code questions from the
  2026-09-26 A/B, on the source snapshot ("find where …, give file:line").
- **throttle edit**: make the broker throttle's 30 s wait ceiling
  configurable by env, add a test, run it — the task names the file
  (`throttle.go`, 160 lines).

| rung | tasks | condition | runs | input tokens | model calls | seconds | chat cost |
|---|---|---|---|---|---|---|---|
| 1 (GLM 5.3 Flash) | questions | off | 2 | 157,470 | 10 | 75 | $0.0095 |
| 1 (GLM 5.3 Flash) | questions | Jev | 2 | 55,026 | 6 | 60 | $0.0035 |
| 1 (GLM 5.3 Flash) | throttle edit | off | 2 | 308,753 | 22 | 315 | $0.0133 |
| 1 (GLM 5.3 Flash) | throttle edit | Jev | 2 | 360,855 | 26 | 390 | $0.0178 |
| 3 (GLM 5.3) | questions | off | 2 | 194,541 | 9 | 60 | $0.0522 |
| 3 (GLM 5.3) | questions | Jev | 2 | 68,825 | 5 | 30 | $0.0156 |
| 3 (GLM 5.3) | throttle edit | off | 1 | 121,793 | 9 | 105 | $0.0248 |
| 3 (GLM 5.3) | throttle edit | Jev | 1 | 134,896 | 11 | 135 | $0.0276 |

What it says:

- **Read-heavy work: −65% input tokens on both rungs** (157 k → 55 k and
  195 k → 69 k), 40–45% fewer model calls, and on GLM 5.3 half the wall
  time. Same answers. This is the 2026-09-26 result (−63%) again, on a
  different model and a month later.
- **A named-file edit: no saving, +11–17% tokens — found and fixed the same
  day** (Greg: "Fix edit tokens with jev. it should not be higher"). See
  *The edit regression* below: after the fix the same task runs at −48%.
- **The dollar saving scales with the price of the model being saved.**
  On rung 1 the chat model's input ($0.04/M) costs what Jev's does
  ($0.042/M): the questions saved $0.006 of chat and spent about that on
  Jev — break-even in money, a win in calls and time. On rung 3 the same
  −65% saved $0.037 against about $0.004 of Jev, nine to one. At the
  prices most people pay a coding agent today ($2–3/M for Claude or GPT)
  the same ratio is sixty to one. That was the arithmetic behind "Pro:
  the decision model for $10/month" (2026-09-27, superseded 2026-10-03: it
  runs free on the person's key now); it pays back on the bill the person
  already has, not on the cheapest rung.
- **Where the next tokens are:** the `jread`-first prompt line (SHOULD.md)
  — every whole-file `read_file` in these runs was a judged read not
  taken — and the escalation that is still in shadow (0 of 22 turns would
  have fired; no false positives, no evidence yet either).

### The edit regression, and the fix (2026-09-27)

Why the edit task cost more with decisions on, from the runs' own logs:

- **Not the judged reads.** `throttle.go` is 4 KB, well under the 32 KB
  `readFileJudgeMin`, so no read was judged either way.
- **Not the per-call prompt.** At equal message counts the Jev runs were
  *smaller* (13 messages: 57,264 vs 61,824 bytes; 23: 62,726 vs 66,516).
- **It was the step count.** 17 assistant turns against 12, and four of the
  extra steps were `todo_write` bookkeeping on a one-file change. Input
  tokens grow with the square of the steps, because every step resends the
  transcript.

Two changes, both measured:

1. **The coder's tool families** (`gateway/tool_routing.go`): the decision
   model now narrows the coder's palette per turn, so the schema block —
   the single largest fixed cost of a turn — shrinks on every call. It
   chose "change" for both edit turns and "answer" for the question turn,
   at confidence 1.00.
2. **One prompt line** (`gateway/agent_seeding.go`): no todo list for work
   that is one or two files. The todo list stays for work that spans
   several files or outlives the turn. This one helps both conditions.

Re-run on the fixed build, same tasks, alternating order, 8 runs, all
correct (`go test` and `go vet` green on the copy, the env var present in
code and test; the questions by regex on the reply):

| tasks | condition | runs | input tokens | model calls | seconds | chat cost |
|---|---|---|---|---|---|---|
| throttle edit | off | 2 | 233,836 | 17 | 330 | $0.0120 |
| throttle edit | Jev | 2 | 121,015 (−48%) | 13 | 240 | $0.0082 (−32%) |
| questions | off | 2 | 114,372 | 11 | 90 | $0.0066 |
| questions | Jev | 2 | 66,341 (−42%) | 6 | 60 | $0.0054 (−18%) |

That −48% is one round of two runs and did **not** reproduce; the honest
figure is the 20-run aggregate at the end of this section.

Then a second round, on the same tasks, testing two more read levers: the
keep cap scaled to the file (a fixed cap of 12 sections kept everything on a
mid-size file, which is why `read_file` only judged past 32 KB), the whole
file returned when judging would keep nearly all of it, and the judged-read
threshold dropped from 32 KB to 10 KB. Eight runs, all correct:

| tasks | condition | runs | input tokens | model calls | seconds | chat cost |
|---|---|---|---|---|---|---|
| throttle edit | off | 2 | 358,286 | 24 | 525 | $0.0149 |
| throttle edit | Jev | 2 | 275,285 (−23%) | 23 | 645 | $0.0130 |
| questions | off | 2 | 75,979 | 6 | 75 | $0.0045 |
| questions | Jev | 2 | 57,385 (−24%) | 7 | 90 | $0.0024 (−47%) |

Jev wins both task types again, but this round is noisier in *both*
conditions (the off runs went from 233,836 to 358,286 on the same task and
build-independent code path), so the threshold's own effect is not separable
at two runs each — and the single worst turn of the day (495 s, 15 calls,
15 k output, a skill loaded mid-turn) was a Jev run on that build. So
**the 10 KB threshold was reverted**. The tighter keep cap went with it:
it would have dropped sections the judge had already scored above the bar,
which is a quality risk taken for a saving this round could not demonstrate.
What stayed is the one piece that can only add fidelity: **when judging would
keep nearly all of a text, the whole text is returned instead**. That also
removes a cost nobody had noticed — a judged render adds a path header, line
numbers and a probability per section, so on a small file the judged copy was
*bigger* than the file it replaced.

#### What it settles: 20 runs, 10 pairs, nothing wrong in any of them

Every run of the three rounds above that ran on a build with the coder's
families, off against Jev, same tasks, alternating order, rung 1:

| tasks | condition | runs | correct | input tokens | per run | model calls | chat cost |
|---|---|---|---|---|---|---|---|
| edit | off | 5 | 5/5 | 704,313 | 140,862 | 50 | $0.0333 |
| edit | Jev | 5 | 5/5 | 518,876 | **103,775 (−26%)** | 45 | $0.0267 (−20%) |
| code question | off | 5 | 5/5 | 287,117 | 57,423 | 24 | $0.0164 |
| code question | Jev | 5 | 5/5 | 147,294 | **29,458 (−49%)** | 16 | $0.0087 (−47%) |

- **The edit task is no longer more expensive with decisions on**, which is
  what it was measured to be this morning (+11–17%): about a quarter fewer
  input tokens now, a fifth less money.
- **Read-heavy work is about half**, which matches the 2026-09-26 A/B
  (−63%) and the earlier rounds here.
- **Per-run variance is larger than the effect on the edit task** (Jev runs
  of 52 k to 203 k input on the same task), so a single pair proves nothing
  either way: one pair on the final build came out 9% *above* its off
  sibling. Five pairs is the smallest claim worth making, and the variance
  is the model's own wandering (a skill loaded mid-turn, an extra
  verification round), not the decisions.
- **All 20 answers were right**: every edit passed `go test` and `go vet`
  on its own copy with the env var present in code and test, every question
  named the right file. "No risk no quality degradation" (Greg,
  2026-09-27) is why the families carry more than they strictly need and why
  the tighter keep cap was dropped.
- **The dollar saving is smaller than the token saving** because most of the
  tokens removed were cache hits, and a family change between turns of one
  conversation re-reads the prefix once — worth watching in a long session.

## Any model can run the coding agent (32 probed, 2026-09-27)

`memdoor model check [vendor/name …]` asks a model to do what a coder turn
needs, on the person's own key: emit a native tool call, honour the schema of
the tool it named, carry on after a tool result, and produce an `apply_patch`
hunk this harness accepts. The catalogue only says a model *advertises* tools
(`gateway/model_check.go`; reports in `~/.memdoor/model-checks.json`,
`--saved` prints them without spending anything).

Greg, 2026-09-27: "we need our coding agent to work with any model … try to
identify common pattern of failure so we can generalize the adapter and not have
too many coding paths".

**Result: 29 of 32 run a turn and write patches. The three that do not never
answered at all** — `deepseek/deepseek-v4.1` ("is not a valid model ID"),
`meta-llama/llama-4-scout` and `nvidia/nemotron-3.5-lightning:free` ("no
endpoints"). Every model that answered, ran. The OpenRouter leaderboard's whole
top ten passes: DeepSeek V4.1 Flash, GLM 5.3 Flash, Hy4 preview, Space Bunny
Alpha, GPT-5.6 Luna, DeepSeek V4 Flash 0731, Nemotron 3.5 Lightning,
MiMo-V2.6-Flash, MiMo-V2.5 — with Claude (Sonnet 5, Opus 5.5), GPT-6 Luna Pro,
o4-mini-high, Gemini 3.8 Flash, Grok 4.6 and 4.7, Kimi K3, Qwen 3.8 (max-prime,
omni-flash), Mistral Medium 3.5, Command R and R+, GLM 5.3 (prime, flashx) and
Llama 4 Maverick alongside.

### One failure pattern, four rules

It did not start that way. Three models failed, and all three failed the *same
way*: **the right payload in the wrong envelope.** So there is one generic
adapter rather than one per vendor (`gateway/providers/model_adapter.go`,
`ModelAdapter` with a registry, so a genuinely vendor-specific quirk can still
be added as its own implementation).

| Measured | What the model sent | Rule |
|---|---|---|
| google/gemini-3.8-flash | `grep{"query":"cache.go"}` when the schema said `read_file{path}` | **rename** the alias to the declared field, then **reroute** when the value names a file and a file tool was offered |
| xiaomi/mimo-v2.6-flash | `input` carrying the whole argument object again, JSON-encoded inside itself | **unwrap** a field whose value is the same object doubled |
| meta-llama/llama-4-maverick | a correct Codex patch written as prose, no tool call | **lift** the patch into the call it meant — only when our own parser accepts the text |

Every rule fires on unambiguous evidence and is a no-op otherwise, which the
tests assert from both directions: the three measured calls are repaired, and
correct calls, prose about a file, a unified diff, half a patch, a tool nobody
offered and a string that merely starts with a brace are all left exactly as
they came.

### What the probe learned about itself

- **A model that calls `grep` before `read_file` is not broken.** The first
  version marked GLM 5.3 Flash — our own first rung — as sending invalid
  arguments because it grepped. Choosing a different tool is a strategy;
  ignoring the schema is an incompatibility. `Runs()` requires a call, arguments
  that match *the tool it named*, and the turn continuing.
- **Answering with another tool call is continuing.** Gemini called a second
  tool after the result and the probe called that silence.
- **One sample is not a verdict.** MiMo failed the patch once and passed twice
  after; the patch check now takes a second attempt before it reports a miss.

Cost: three or four calls per model, about a thousand input tokens each.

### The pin says what the probe saw

A report is only worth keeping if it reaches the person at the moment it matters,
which is the pin, not a table in a doc. `/model vendor/name` and the picker's
pin both read the stored report and add one line when it saw trouble
(`cmd/cli/cmd/model_pin_warning.go`):

```
→ deepseek/deepseek-v4.1   ← this conversation · pinned by you (off the ladder)
⚠ the probe never got an answer out of this model: deepseek/deepseek-v4.1 is not
  a valid model ID (probed 2h ago). `memdoor model check deepseek/deepseek-v4.1`
  probes it again.
```

Four rules keep it honest:

- **A warning, never a refusal.** The pin goes through. A vendor ships a fix the
  next day, the adapter repairs some of these on the fly, and the text-tag
  fallback covers a patch a model cannot emit as a call.
- **Silence unless there is evidence.** No report, a clean report, or a gateway
  that will not answer all say nothing. Warning on a failed read would put a
  scare next to every pin on a machine whose gateway is down.
- **One fact, worst first.** A model that never called a tool failed everything
  after it too; listing five failures would bury the one that matters.
- **The host's own words when the probe never landed.** "Did not call a tool"
  would be a lie about a dead model id, a model with no endpoints, or a data
  policy that excludes every host — the three failures out of 32 above, each
  fixed by something different. A message the stored note cut off keeps its
  ellipsis so it reads as cut off.

Every warning dates itself, because a model is a moving target and a person
deciding whether to trust a two-hour-old probe needs to know it is two hours old.

### Re-probing is the person's cron line, not a timer of ours

A probe is three or four real calls on somebody's own key. A background job of
ours keeping our table warm would be spending their money on our behalf, so
nothing re-probes on its own. What exists instead is the one command a schedule
can call:

```
memdoor model check --stale     # only the reports older than a week
0 9 * * 1 memdoor model check --stale    # a Monday line, if you want one
```

`--stale` asks the gateway for the reports that have aged past a week, oldest
first so an interrupted run refreshed the least trustworthy ones, and prints
`Every report is fresh — nothing probed, nothing spent.` when none have. A week
is one number with no setting behind it: a knob there would be a knob about
money.

Proven on a gateway with two stored reports, one dated a month back and one
current (2026-09-27): the first run re-probed only the month-old model and left
the current one alone, the second run probed nothing. The model it re-probed was
`deepseek/deepseek-v4.1`, a dead id, so even the proof cost zero tokens — the
host refuses the id before any model runs.

### Battle tested in a real window, not only in the probe

Passing a probe is not the same as doing the work, so the two repaired models
and the stealth entry at rank four were each pinned in a live TUI session on a
small Go package (2026-09-27):

- **google/gemini-3.8-flash** — asked for a test covering below, inside and
  above a range: wrote a table-driven `TestClamp` with the three cases, and
  `go test ./...` passed.
- **meta-llama/llama-4-maverick** — asked to add a `ClampFloat` beside `Clamp`:
  landed the function and `go build` passed. No adapter rule fired that turn, so
  the lift is a safety net rather than a crutch.
- **stealth/space-bunny-alpha** (rank 4, an unnamed vendor) — asked to grep the
  package and document every undocumented exported function, then vet: it
  documented `ClampFloat`, argued in one line that the remaining symbol did not
  need a comment, and `go vet` came back clean.

The jump bar showed up on its own during the third run ("↓ 13 new messages — End
jumps to latest"), which is the scroll promise behaving in a live window.

### What a judged search keeps (2026-09-27)

The site says a judged search returns about a twelfth of what grep would.
The source is the savings ledger on the development machine
(`~/.memdoor/savings.jsonl`, every `jgrep` entry with raw bytes): 59 searches
on 2026-09-27, median kept 7.7% of the bytes plain grep returned, 11.9%
over all of them (240 KB raw). The window's demo ("judged 41 hits, kept 3",
7.3%) is an illustration at that median, not a recorded run.

### Battle test in the TUI, and the project's instructions judged (2026-09-28)

`scripts/bench_tui.py` drives the real `memdoor tui` in tmux on a fresh
checkout of this repository (its 20 KB CLAUDE.md included, the prebuilt
tokenizer copied in so `go test` runs), three tasks, decision model off and
on, three runs each, GLM 5.3 Flash on rung 1. Checked: the answer names the
right file:line; the edit passes `go test` and reads the variable. Cost is what
OpenRouter billed.

The first finding was the system prompt: 29,136 characters on every call,
20,008 of them the repository's CLAUDE.md, whole, whatever the task — about
80% of a code question's input. Round B judges the instruction file against
the conversation's first task (`gateway/project_instructions.go`): the
sections it needs stay, the other titles are listed for `jread`, the choice is
kept for the session so the cache prefix holds. It also drops `jev_filter` and
`jev_rank` from the coder, which no session had ever called (both tools were
deleted on 2026-10-03). Round C trimmed
the coder's own rules from 8,506 to 4,449 characters; it was reverted.

Decision model on, averages of three runs:

| task | round | correct | system prompt | input tokens | cost |
|---|---|---|---|---|---|
| edit with a test | baseline | 3/3 | 29,136 | 176,339 | $0.00456 |
| edit with a test | B | 3/3 | 11,450 | **78,966 (−55%)** | **$0.00221 (−52%)** |
| edit with a test | C | 3/3, one run wandered past 15 min | 7,345 | 105,874 | $0.01002 |
| code question (fail limit) | baseline | 3/3 | 29,136 | 24,977 | $0.00373 |
| code question (fail limit) | B | 3/3 | 9,504 | **12,945 (−48%)** | $0.00397 |
| code question (rerun) | baseline | 3/3 | 29,136 | 48,669 | $0.00405 |
| code question (rerun) | B | 3/3 | 9,504 | 56,508 | **$0.00332 (−18%)** |

- For questions the judge kept 52 of 20,008 bytes (the title); for the edit,
  870 to 4,328 (the testing section).
- Decision model off, the same questions cost $0.006–$0.010 and got one of
  three wrong in two rounds out of three; on is cheaper and was always right.
- The dollar saving on questions is smaller than the token saving: with less
  prompt, less of each call is a cache hit, and uncached tokens bill in full.
- Noise is large at three runs — the off condition, which no round changed,
  moved up to ±40% — so only the edit's halving is beyond doubt.
- Round C shows the coder's rules still carry weight on a frontier model:
  without them it explored (it read the benchmark script and git log) and one
  edit ran out of time. Fewer instructions is not the same as a smarter model.

## Not wired yet

The queue lives in the roadmap: [`../roadmap/SHOULD.md`](../roadmap/SHOULD.md),
"Decisions (Jev) — where next".
