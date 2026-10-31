# team-solve

A multi-agent harness for programming problems, written as a
[Memdoor](../../../README.md) workflow. Five roles (planner, two ideators,
solver, manager) and a repair worker each start from a fresh context and talk
only through files on disk. A shell script that runs the samples decides
success, not a model. It is the method of
[GVS5H](https://github.com/slee-persis/GVS5H) (Lee, 2026: five Qwen3.8-27B
models match Fable 5 on LiveCodeBench Hard).

Rebuilt with the authors' exact prompts on Qwen3.8-27B, their loop scored a
mean of **9.8 of 10** over five passes on the first ten LiveCodeBench Hard
problems, against 9.6 for their own harness and 8.0 for Fable 5 alone
([below](#five-passes-on-the-authors-ten-2026-10-31), every run published).

The coder wrote these files itself from a description of the method
(2026-10-25). Nobody hand-wrote them.

## Five passes on the authors' ten (2026-10-31)

GVS5H's own harness, rebuilt as a Memdoor workflow on the same model:
Qwen3.8-27B, a 128K reply cap, the authors' prompts and loop. The first ten
problems of their LiveCodeBench Hard list, five passes, every program graded
on every public and hidden test with their corrected grader.

| pass | Memdoor workflow | authors' 27B harness | Fable 5 alone |
|---|---|---|---|
| 1 | **10** / 10 | 9 | 7 |
| 2 | **10** / 10 | 10 | 8 |
| 3 | **10** / 10 | 10 | 10 |
| 4 | **10** / 10 | 9 | 8 |
| 5 | **9** / 10 | 10 | 7 |
| mean | **9.8** | 9.6 | 8.0 |

The authors' columns are their repository's regraded result files
(`q38_multiagent_pN`, `fable5_single_pN`), the numbers in their paper.
[`bench/results/parity-5pass/`](bench/results/parity-5pass/) holds all 50
runs: every request and reply, the program, the grader's full output, each
call's cost and the host that served it, and every attempt that did not count.
49 of 50 solved; the 50 runs cost $35.79 (15.3M output tokens), $38.77 with
the 16 attempts that were cut by a clock, stopped for a fix or paused.

<sub>Note: ten problems, five passes each, not the authors' 100. The contests
date from 29 March to 6 April 2025, and the model may have seen them in
training: contamination cannot be ruled out. Fable 5's column is a single call per
problem from the authors' files; it was not run in this harness. The loop
sees only the public samples; the score is the hidden tests. A run took a
median of 40 minutes of model time (the longest 4 hours) and $0.39. All on
OpenRouter, not a local machine; Modal does not publish its precision.
Local paths in the run scripts and in four tracebacks a manager was shown
read `<scratch>` and `$HOME`; nothing else in the runs is edited.</sub>

**Same as the authors'.** Every message, byte for byte:
[`lib/test_parity.py`](bench/results/parity-5pass/workflow/lib/test_parity.py)
runs their `multiagent_solve` and our workflow against one scripted model and
compares each request (four cases, all identical; a one-character change makes
them differ). The temperatures (plan 0.3, ideation 0.4, manager and worker
0.2), top_p 0.95 and top_k 20 (what their vLLM applied from the model's own
config), the 128K cap, up to 10 rounds, a cut-off worker summarized, and a
reply with no text answered by its reasoning.

**Different.** The host. Theirs was their own vLLM in FP8. Ours is
OpenRouter, asked in the order Modal, CoreWeave, Parasail, Wafer until Modal
slowed to 14 tokens/s, then the FP8 hosts only (Wafer, Parasail, CoreWeave):
203 calls were served by Modal, 27 by CoreWeave and 115 by Wafer, every one
of the FP8 order's calls by Wafer. Three runs were moved mid-run, their
finished steps kept (`log/host-switch.txt`).

**What it took.** The problem text was the mismatch: our first version wrote
its own task file around each problem, and the 27B planned worse from it than
from the authors' exact LiveCodeBench text (arc196_c's plan: 0 of 2 against 2
of 2). Four Memdoor changes let a workflow ask what their harness asks: a task
answers with the model's reasoning when it wrote no text, `top_p`/`top_k` on a
model line, `mark_cutoff` for a reply cut at the cap, and a call that runs on
its own four-hour clock.

**Where it lost.** Twice, the same two gaps in the authors' loop:

- Pass 5, abc399_e (32 of 44 hidden): round 1's program was right; the
  manager invented a counterexample with a wrong expected answer, the team
  rewrote it, and the new program passed the samples, which do not cover the
  case.
- Pass 4, arc196_c and arc196_d: a program passing every hidden test was
  overwritten by a later round because the manager did not say done; both
  got a correct program back before the loop's end (arc196_d at round 8,
  arc196_c through the final call), at $3.44 and $4.24, the two most
  expensive runs.

Both are fixable without changing the method (check a manager's counterexample
against a brute force on small inputs; keep the last program that passed the
samples). Neither is in these numbers.

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
