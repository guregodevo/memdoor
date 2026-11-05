# team-solve

<a href="https://news.ycombinator.com/item?id=50320995"><img alt="Hacker News" src="https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fhacker-news.firebaseio.com%2Fv0%2Fitem%2F50320995.json&query=%24.score&label=Hacker%20News&suffix=%20points&logo=ycombinator&logoColor=white&color=ff6600"></a>

Five small models on one programming problem: a planner, two ideators, a
solver, a manager and a repair worker, each from a fresh context, talking
only through files on disk; a script that runs the samples decides success.
The method of [GVS5H](https://github.com/slee-persis/GVS5H) (Lee, 2026) as a
[Memdoor](../../../README.md) workflow: eight YAML files, no code, no model
named.

Try it on your own problem, free models included: `curl -fsSL https://memdoor.ai/install.sh | bash`,
then [Run it](#run-it). Memdoor is a coding agent that runs workflows like this one
headless or in a terminal UI: [memdoor.ai](https://memdoor.ai).

## The result

The authors' harness on the same model, Qwen3.8-27B, with their prompts,
loop and 128K reply cap. The first ten problems of their LiveCodeBench Hard
list, five passes each, graded on every hidden test with their corrected
grader:

| pass | this workflow | authors' harness | Fable 5, one call |
|---|---|---|---|
| 1 | **10** / 10 | 9 | 7 |
| 2 | **10** / 10 | 10 | 8 |
| 3 | **10** / 10 | 10 | 10 |
| 4 | **10** / 10 | 9 | 8 |
| 5 | **9** / 10 | 10 | 7 |
| mean | **9.8** | 9.6 | 8.0 |

The authors' columns: the regraded result files in their repository, the
numbers in their paper. Ours:
[`bench/results/parity-5pass/`](bench/results/parity-5pass/), all 50 runs,
each with every request and reply, the program, the grader's output and the
cost. A median of 40 minutes and $0.39 a run; $35.79 for the 50.

What this does and does not show:

- Sample: ten problems, five passes, against the authors' 100. 9.8 and 9.6
  are within noise. The claim is parity with their harness.
- Contamination: the contests date from spring 2025; the model may have seen
  them. Not ruled out for any column.
- Fable 5: one call per problem, from the authors' files, not run in this
  harness.
- On these ten, the authors' own five-pass data: Fable 5 at 80, their harness
  at 96. The workflow's 98 reproduces that gap. Across their full 100: 92.4
  and 90.4 (their `patched_verdicts.json`).
- The loop sees the public samples; the score is the hidden tests.
- Host: OpenRouter, not a local machine.
- The lost problem: the manager invented a wrong counterexample and the team
  rewrote a correct program. Twice elsewhere a correct program was overwritten
  and then recovered. Both are gaps in the authors' loop, kept as is.

Parity check (every request byte for byte against their code), hosts, the
changes it took, the first run's findings: [NOTES.md](NOTES.md). The result
on one page: [memdoor.ai/docs/team-solve](https://memdoor.ai/docs/team-solve).

## Run it

    curl -fsSL https://memdoor.ai/install.sh | bash
    memdoor connect openrouter        # or anthropic, openai, gemini, chatgpt
    # or no key: memdoor connect local   (Ollama, llama.cpp, vLLM, LM Studio; Ollama needs a 32K window, not its 4K default)
    git clone --depth 1 https://github.com/guregodevo/memdoor

    mkdir -p my-problem/.memdoor/workflows/team-solve && cd my-problem
    cp -r ../memdoor/examples/workflows/team-solve/{steps,check-samples.sh} .memdoor/workflows/team-solve/   # these steps, not the built-in's
    cp -r ../memdoor/examples/workflows/team-solve/sample/* .    # task.md + tests/1.in … 5.out
    memdoor workflow run team-solve --wait   # exit 0 when every sample passes

Your problem: `task.md` and paired `tests/N.in` / `tests/N.out`. Program:
`solution.py`, stdin in, compared to the samples with `diff`. Needs Python 3
and bash. `/workflow` in `memdoor tui` draws the run live.

`team-solve` is also built in, with its own prompts (a task from a prompt,
any project): [workflows/team-solve](../../../workflows/team-solve/). The
copy in `.memdoor/workflows` runs these steps instead, the ones measured above.

The steps name no model; the `/model` pin answers, a local one included
([how to connect one](https://memdoor.ai/docs/providers#a-local-runtime-no-key)).
A role is its prompt in `steps/`.

## How it works

```
plan ──┬── ideate-a ──┬── solve ── test ── review ── repair ── verify
       └── ideate-b ──┘
```

| step | kind | reads | writes |
|---|---|---|---|
| `plan` | agent | `task.md` | `plan.md`, `notes.md` |
| `ideate-a`, `ideate-b` | agent, in parallel | the plan | a proposal each: the most direct provable algorithm, and one from another family |
| `solve` | agent | both proposals | `solution.py`, the decision in `notes.md` |
| `test` | command | `tests/` | `test-output.log` (never fails the run) |
| `review` | agent | the test output | repair instructions in `notes.md` |
| `repair` | agent | the instructions | a fixed `solution.py`, or nothing |
| `verify` | command | `tests/` | the verdict: a nonzero exit fails the run |

Every agent prompt: "fresh context, read the files, do not rely on chat"; the
paper credits that ledger discipline for the gain. One YAML file per step in
`steps/`, in the format of [mario](https://github.com/guregodevo/mario), the
engine underneath. `requires:` is the DAG.
