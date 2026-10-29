# team-solve

A multi-agent harness for programming problems, written as a
[Memdoor](../../../README.md) workflow. Five roles (planner, two ideators,
solver, manager) and a repair worker each start from a fresh context and talk
only through files on disk. A shell script that runs the samples decides
success, not a model. It is the method of
[GVS5H](https://github.com/slee-persis/GVS5H) (Lee, 2026: five Qwen3.8-27B
models match Fable 5 on LiveCodeBench Hard).

On GVS5H's 100 hardest LiveCodeBench problems, with Qwen3.8-27B and thinking
off, the team solved **47** against **35** for a single call of the same
model, for $3.27 in all ([`bench/`](bench/) has the harness, every problem's
result and what the numbers do not show).

The coder wrote these files itself from a description of the method
(2026-10-25). Nobody hand-wrote them.

## Run it

    curl -fsSL https://memdoor.ai/install.sh | bash
    memdoor connect openrouter                 # or anthropic, openai, gemini: your own key
    git clone --depth 1 https://github.com/guregodevo/memdoor

    mkdir -p ~/.memdoor/workflows
    cp -r memdoor/examples/workflows/team-solve ~/.memdoor/workflows/   # the library: any project sees it
    mkdir my-problem && cd my-problem
    cp -r ../memdoor/examples/workflows/team-solve/sample/* .          # task.md + tests/1.in … 5.out
    memdoor workflow run team-solve --wait   # progress on stderr, exit 0 when every sample passes

`/workflow` in `memdoor tui` draws the same run live. For your own problem,
replace `task.md` and put paired `tests/N.in` / `tests/N.out` files beside it.
The program is `solution.py`, read from stdin, compared to the samples with
`diff`. It needs Python 3 and bash. To keep it with one repo instead, copy
the folder into that project's `.memdoor/workflows/`. The project's copy wins.

The steps name no model. Whatever `/model` is pinned to answers, a local one
included (`memdoor connect local` for janis, Ollama or llama-server).

## How it works

```
plan ──┬── ideate-a ──┬── solve ── test ── review ── repair ── verify
       └── ideate-b ──┘
```

| step | kind | reads | writes |
|---|---|---|---|
| `plan` | agent | `task.md` | `plan.md`, and a `notes.md` ledger with one anchor per ideator |
| `ideate-a`, `ideate-b` | agent, in parallel | `task.md`, `plan.md`, `notes.md` | a proposal at its own anchor: A the most direct provable algorithm, B one from a different family of technique |
| `solve` | agent | both proposals | the decision in `notes.md`, `solution.py` |
| `test` | command | `tests/` | `test-output.log`, `test-status.txt` (never fails the run) |
| `review` | agent | the captured test output, not a model's claim | `review.md`, repair instructions in `notes.md` |
| `repair` | agent | the manager's instructions | a fixed `solution.py`, or nothing if the samples passed |
| `verify` | command | `tests/` | `final-test-output.log`; nonzero exit fails the run |

Every agent prompt says "fresh context, read the files, do not rely on chat".
The paper credits that ledger discipline for the gain. There is one repair
round and no human gate. The last step is the whole verdict:

    type: command
    requires:
      - table_pattern: repair
    command: |
      check=.memdoor/workflows/team-solve/check-samples.sh
      [ -f "$check" ] || check="$HOME/.memdoor/workflows/team-solve/check-samples.sh"
      bash "$check" > final-test-output.log 2>&1
      result=$?
      cat final-test-output.log
      exit "$result"

Each step is one YAML file in `steps/`, the format of
[mario](https://github.com/guregodevo/mario), the engine underneath.
`requires:` is the DAG. Changing a role means changing its prompt.

## What the first run found

The sample: count the substrings of a string over {a, b, c} with equal
counts of each letter, up to 200,000 characters. Run on the default ladder
through OpenRouter:

- 8 of 8 steps, 5 of 5 samples. The solver took ideator A's prefix-state
  method, O(N) with a dictionary of (a−b, a−c) pairs. A brute force on 300
  random strings found no mismatch afterwards, and it runs in 0.04 s at
  N = 200,000. The manager found nothing to repair.
- One ideator's provider dropped the stream mid-answer. The coder had written
  `max_retries: 0`, so the run died there. The agent steps now carry
  `max_retries: 1`, and the run was resumed with `--partition`, which kept the
  planner's and the other ideator's work.
- The ideator prompts the coder wrote named the sample's methods (A: a
  streaming prefix-state count, B: sorting prefix states), which would steer
  any other problem the wrong way. They now ask A for the most direct
  provable algorithm and B for one from a different family of technique
  (2026-10-28).
- The first attempts failed every step in 0 s with "no agent named coder": a
  run looked its agent up only in its own workspace. That was fixed in the
  gateway the same day (v1.692.0).

## On the benchmark

[`bench/`](bench/) runs the same method with every role as one model call
(`team-llm-lcb`) against a single call (`single-lcb`). Both are Memdoor
workflows on the same model and host, graded on every public and hidden test
with LiveCodeBench's own checker:

| arm | solved | AtCoder (73) | LeetCode (27) |
|---|---|---|---|
| single call | 35 / 100 | 32.9% | 40.7% |
| team workflow | 47 / 100 | 42.5% | 59.3% |

These are thinking-off numbers, not the paper's setting (66.8% → 92.4% with
128K tokens of thinking). The finding is the gap between the two arms. The
bench README says why, and how to run it again for about four dollars.

## The paper's setting: thinking on, the harness rebuilt from one prompt (2026-10-28)

A second build reproduces GVS5H's own harness from a six-line prompt (five roles, the
manager and worker alternating up to ten rounds, thinking high, a 128K output cap) and
adds a brute-force stress test as the ground truth for "done". On the authors' first ten
problems, one pass:

| arm | solved of 10 |
|---|---|
| this harness, Qwen3.8-27B, one pass | 7, for $4.95 including every failed attempt |
| the authors' 27B harness, five passes | 9 to 10 by pass |
| the authors' 27B single call, five passes | 8 to 10 by pass |
| Claude Fable 5 alone, five passes | 7 to 10 by pass |

The workflow, the prompts, every run's ledger, the meter rows of every call, a chart and
the lessons live in [memdoor-demos](https://github.com/guregodevo/memdoor-demos); the
summary is on the [workflows page](https://memdoor.ai/docs/workflows#the-paper-s-own-harness-from-one-prompt).
The runner rows are in [`bench/results/gvs5h-stress.tsv`](bench/results/gvs5h-stress.tsv).
