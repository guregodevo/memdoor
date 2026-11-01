# Five small models against a frontier model: the team-solve result

Five Qwen3.8-27B agents run as a Memdoor workflow scored a mean of **9.8 of
10** on the first ten problems of GVS5H's LiveCodeBench Hard list over five
passes. The authors' harness scored 9.6 on the same problems. Fable 5 in a
single call scored 8.0. All 50 runs are published with every request and
reply.
The thread: [Hacker News, 2026-10-31](https://news.ycombinator.com/item?id=50320995).

| pass | Memdoor workflow | authors' harness | Fable 5, one call |
|---|---|---|---|
| 1 | **10** / 10 | 9 | 7 |
| 2 | **10** / 10 | 10 | 8 |
| 3 | **10** / 10 | 10 | 10 |
| 4 | **10** / 10 | 9 | 8 |
| 5 | **9** / 10 | 10 | 7 |
| mean | **9.8** | 9.6 | 8.0 |

## What was measured

[GVS5H](https://github.com/slee-persis/GVS5H) (Lee, 2026) is a multi-agent
harness: a planner, two ideators, a solver and a manager, plus a repair
worker. Each starts from a fresh context and talks only through files on disk.
Success is decided by a script that runs the samples. The paper's claim is
that five copies of an open 27B model organised this way match Fable 5 on hard
competitive-programming problems.

Memdoor rebuilt that harness as a workflow: eight YAML files, no code, and no
model named. The prompts, temperatures, the 128K reply cap, the ten-round loop
and the cut-off handling are the authors'. They were checked byte for byte
against their code with a scripted model. The model was the same Qwen3.8-27B
through OpenRouter. The first ten problems of their list were run five times each and
graded on every hidden test with their corrected grader. The authors' two
columns are the regraded result files in their repository.

Each run took a median of 40 minutes and $0.39. The 50 runs cost $35.79.

## What it does and does not show

- Ten problems and five passes, against the authors' 100. 9.8 and 9.6 are
  within noise on that sample. The result is parity with their harness.
- The contests date from spring 2025 and the model may have seen them in
training. Contamination cannot be ruled out for any column.
- Fable 5's column is one call per problem, from the authors' files. It was
  not given the harness.
- On these ten the authors' own five-pass data put Fable 5 at 80 and their
harness at 96. The workflow's 98 reproduces that gap. Across their full 100,
  harness and Fable are 92.4 and 90.4 (their `patched_verdicts.json`).
- The loop sees only the public samples; the score is the hidden tests.
- The model ran on OpenRouter hosts rather than a local machine.
- The one lost problem: the manager invented a counterexample with a wrong
  answer and the team rewrote a correct program. Twice elsewhere a correct
  program was overwritten by a later round and recovered before the end. Both
  are gaps in the authors' loop and were kept as is.

The runs, with each call's tokens, cost and the host that served it:
[bench/results/parity-5pass](https://github.com/guregodevo/memdoor/tree/main/examples/workflows/team-solve/bench/results/parity-5pass).
The parity check, the hosts, what the rebuild took and where it lost, in
detail: [NOTES.md](https://github.com/guregodevo/memdoor/blob/main/examples/workflows/team-solve/NOTES.md).

## Run it on your own problem

```
curl -fsSL https://memdoor.ai/install.sh | bash
memdoor connect openrouter          # or anthropic, openai, gemini, chatgpt
memdoor connect local               # or no key: a model on this machine (Ollama, llama.cpp, vLLM, LM Studio)
git clone --depth 1 https://github.com/guregodevo/memdoor

mkdir -p ~/.memdoor/workflows
cp -r memdoor/examples/workflows/team-solve ~/.memdoor/workflows/
mkdir my-problem && cd my-problem
cp -r ../memdoor/examples/workflows/team-solve/sample/* .     # task.md + tests/1.in … 5.out
memdoor workflow run team-solve --wait                        # exit 0 when every sample passes
```

Replace `task.md` with your problem and put paired `tests/N.in` /
`tests/N.out` files beside it. The program is `solution.py`. It reads stdin
and is compared to the samples with `diff`. The steps name no model. Whatever
`/model` is pinned to answers, a local one included.

With no key, `memdoor connect local` reaches Ollama, llama.cpp, vLLM or LM
Studio on this machine ([how](/docs/providers#a-local-runtime-no-key); [the
whole path](/docs/open-weight)). Ollama's default window is 4,096 tokens, too
short for the coder's prompt. Give the model a 32K window first, as that page
shows. A hard problem takes about 300K output tokens a run. A 27B at Q5 or
above on a 24 GB card is the floor.

## How it works

```
plan ──┬── ideate-a ──┬── solve ── test ── review ── repair ── verify
       └── ideate-b ──┘
```

Every agent prompt says "fresh context, read the files, do not rely on chat".
The paper credits that ledger discipline for the gain. `test` and `verify` are
command steps that run the samples. `verify` alone decides the run. Each step
is one YAML file. `requires:` is the DAG. To change a role, edit its prompt.
The files:
[examples/workflows/team-solve](https://github.com/guregodevo/memdoor/tree/main/examples/workflows/team-solve).

![The team on a ledger: a planner, two ideators in parallel, a solver, the samples, a manager, a repair round, the final check](/team-solve.cast)
