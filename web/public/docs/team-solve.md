# Five small models against a frontier model: the team-solve result

Five Qwen3.8-27B agents as a Memdoor workflow: a mean of **9.8 of 10** on
the first ten problems of GVS5H's LiveCodeBench Hard list, five passes. The
authors' harness: 9.6. Fable 5, one call: 8.0. All 50 runs published with
every request and reply.
Thread: [Hacker News, 2026-10-31](https://news.ycombinator.com/item?id=50320995).

| pass | Memdoor workflow | authors' harness | Fable 5, one call |
|---|---|---|---|
| 1 | **10** / 10 | 9 | 7 |
| 2 | **10** / 10 | 10 | 8 |
| 3 | **10** / 10 | 10 | 10 |
| 4 | **10** / 10 | 9 | 8 |
| 5 | **9** / 10 | 10 | 7 |
| mean | **9.8** | 9.6 | 8.0 |

## What was measured

[GVS5H](https://github.com/slee-persis/GVS5H) (Lee, 2026): a planner, two
ideators, a solver, a manager and a repair worker, each from a fresh context,
talking only through files on disk; a script that runs the samples decides
success. The paper's claim: five copies of an open 27B organised this way
match Fable 5 on hard competitive-programming problems.

The rebuild: eight YAML files, no code, no model named. Prompts,
temperatures, the 128K reply cap, the ten-round loop and the cut-off handling
are the authors', checked byte for byte against their code with a scripted
model. Model: the same Qwen3.8-27B through OpenRouter. Problems: the first
ten of their list, five passes each, graded on every hidden test with their
corrected grader. The authors' columns: the regraded result files in their
repository.

Cost: a median of 40 minutes and $0.39 a run; $35.79 for the 50.

## What it does and does not show

- Sample: ten problems, five passes, against the authors' 100. 9.8 and 9.6
  are within noise. The result is parity with their harness.
- Contamination: the contests date from spring 2025; the model may have seen
  them. Not ruled out for any column.
- Fable 5: one call per problem, from the authors' files, no harness.
- On these ten, the authors' own five-pass data: Fable 5 at 80, their harness
  at 96. The workflow's 98 reproduces that gap. Across their full 100: 92.4
  and 90.4 (their `patched_verdicts.json`).
- The loop sees the public samples; the score is the hidden tests.
- Host: OpenRouter, not a local machine.
- The lost problem: the manager invented a counterexample with a wrong answer
  and the team rewrote a correct program. Twice elsewhere a correct program
  was overwritten by a later round and recovered before the end. Both are gaps
  in the authors' loop, kept as is.

Runs, with each call's tokens, cost and host:
[bench/results/parity-5pass](https://github.com/guregodevo/memdoor/tree/main/examples/workflows/team-solve/bench/results/parity-5pass).
Parity check, hosts, what the rebuild took, where it lost:
[NOTES.md](https://github.com/guregodevo/memdoor/blob/main/examples/workflows/team-solve/NOTES.md).

## Run it on your own problem

```
curl -fsSL https://memdoor.ai/install.sh | bash
memdoor connect openrouter          # or anthropic, openai, gemini, chatgpt
memdoor connect local               # or no key: a model on this machine (Ollama, llama.cpp, vLLM, LM Studio)
mkdir my-problem && cd my-problem
memdoor workflow run team-solve-v2 --wait   # built in; an empty folder gets the sample problem; exit 0 when every sample passes
```

`team-solve-v2`, the built-in version of this workflow, ships in the
binary, seeded into `~/.memdoor/workflows` at the gateway's first boot. The
published runs used the steps in `examples/workflows/team-solve`, kept as they
were. Your problem: `task.md` and paired `tests/N.in` /
`tests/N.out` in the folder. Program:
`solution.py`, stdin in, compared to the samples with `diff`. The steps name
no model; the `/model` pin answers, a local one included.

No key: `memdoor connect local` reaches Ollama, llama.cpp, vLLM or LM Studio
on this machine ([how](/docs/providers#a-local-runtime-no-key); [the whole
path](/docs/open-weight)). Ollama's default window is 4,096 tokens, too short
for the coder's prompt: set 32K first, as that page shows. A hard problem:
about 300K output tokens a run. Floor: a 27B at Q5 or above on a 24 GB card.

## How it works

```
plan ──┬── ideate-a ──┬── solve ── test ── review ── repair ── verify
       └── ideate-b ──┘
```

Every agent prompt: "fresh context, read the files, do not rely on chat"; the
paper credits that ledger discipline for the gain. `test` and `verify` are
command steps that run the samples; `verify` alone decides the run. One YAML
file per step; `requires:` is the DAG; a role is its prompt. The files:
[examples/workflows/team-solve](https://github.com/guregodevo/memdoor/tree/main/examples/workflows/team-solve).

![The team on a ledger: a planner, two ideators in parallel, a solver, the samples, a manager, a repair round, the final check](/team-solve.cast)
