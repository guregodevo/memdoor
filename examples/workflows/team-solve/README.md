# team-solve

<a href="https://news.ycombinator.com/item?id=50320995"><img alt="Hacker News" src="https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fhacker-news.firebaseio.com%2Fv0%2Fitem%2F50320995.json&query=%24.score&label=Hacker%20News&suffix=%20points&logo=ycombinator&logoColor=white&color=ff6600"></a>

Five small models solve a programming problem together: a planner, two
ideators, a solver and a manager, plus a repair worker. Each starts from a
fresh context and talks only through files on disk. A script that runs the
samples decides success, not a model. It is the method of
[GVS5H](https://github.com/slee-persis/GVS5H) (Lee, 2026), written as a
[Memdoor](../../../README.md) workflow: eight YAML files, no code, no model
named.

## The result

The authors' harness rebuilt on the same model, Qwen3.8-27B, with their
prompts, loop and 128K reply cap. The first ten problems of their
LiveCodeBench Hard list, five passes each, graded on every hidden test with
their corrected grader:

| pass | this workflow | authors' harness | Fable 5, one call |
|---|---|---|---|
| 1 | **10** / 10 | 9 | 7 |
| 2 | **10** / 10 | 10 | 8 |
| 3 | **10** / 10 | 10 | 10 |
| 4 | **10** / 10 | 9 | 8 |
| 5 | **9** / 10 | 10 | 7 |
| mean | **9.8** | 9.6 | 8.0 |

The authors' two columns are the regraded result files in their repository,
the numbers in their paper. Our column is
[`bench/results/parity-5pass/`](bench/results/parity-5pass/): all 50 runs,
each with every request and reply, the program, the grader's output and the
cost. A run took a median of 40 minutes and $0.39; the 50 cost $35.79.

What this does and does not show:

- Ten problems, five passes, not the authors' 100. On a sample this small,
  9.8 and 9.6 are the same number. The claim is parity with their harness, not
  a gain on it.
- The contests date from spring 2025 and the model may have seen them in
  training. Contamination cannot be ruled out, for any column.
- Fable 5's column is one call per problem, from the authors' files. It was
  not run in this harness.
- On these ten, the authors' own five-pass data put Fable 5 at 80 and their
  harness at 96; the workflow's 98 reproduces that gap. Across their full 100,
  harness and Fable are 92.4 and 90.4 (their `patched_verdicts.json`).
- The loop sees only the public samples; the score is the hidden tests.
- The model ran on OpenRouter, not a local machine.
- The one lost problem: the manager invented a wrong counterexample and the
  team rewrote a correct program. Twice elsewhere a correct program was
  overwritten and then recovered. Both gaps are the authors' loop, kept as is.

The parity check (every request byte for byte against their code), the hosts,
the changes it took and the first run's findings are in [NOTES.md](NOTES.md).
The same result, on one page: [memdoor.ai/docs/team-solve](https://memdoor.ai/docs/team-solve).

## Run it

    curl -fsSL https://memdoor.ai/install.sh | bash
    memdoor connect openrouter        # or anthropic, openai, gemini, chatgpt
    # or no key: memdoor connect local   (Ollama, llama.cpp, vLLM, LM Studio; Ollama needs a 32K window, not its 4K default)
    git clone --depth 1 https://github.com/guregodevo/memdoor

    mkdir -p ~/.memdoor/workflows
    cp -r memdoor/examples/workflows/team-solve ~/.memdoor/workflows/
    mkdir my-problem && cd my-problem
    cp -r ../memdoor/examples/workflows/team-solve/sample/* .    # task.md + tests/1.in … 5.out
    memdoor workflow run team-solve --wait   # exit 0 when every sample passes

For your own problem, replace `task.md` and put paired `tests/N.in` /
`tests/N.out` beside it. The program is `solution.py`, read from stdin,
compared to the samples with `diff`; it needs Python 3 and bash. `/workflow`
in `memdoor tui` draws the run live.

The steps name no model: whatever `/model` is pinned to answers, a local one
included ([how to connect one](https://memdoor.ai/docs/providers#a-local-runtime-no-key)).
To change a role, change its prompt in `steps/`.

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

Every agent prompt says "fresh context, read the files, do not rely on chat".
The paper credits that ledger discipline for the gain. Each step is one YAML
file in `steps/`, in the format of [mario](https://github.com/guregodevo/mario),
the engine underneath; `requires:` is the DAG.
