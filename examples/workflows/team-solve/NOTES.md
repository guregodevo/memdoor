# team-solve: working notes

The record behind the [README](README.md) and
[memdoor.ai/docs/team-solve](https://memdoor.ai/docs/team-solve): the parity
check, what differed, what it took, where it lost, what the first run found.
Every run: [`bench/results/parity-5pass/`](bench/results/parity-5pass/).

## Same as the authors'

Every message, byte for byte.
[`lib/test_parity.py`](bench/results/parity-5pass/workflow/lib/test_parity.py)
runs their `multiagent_solve` and our workflow against one scripted model and
compares each request. Four cases, all identical; a one-character change makes
them differ. The same: the temperatures (plan 0.3, ideation 0.4, manager and worker
0.2), top_p 0.95 and top_k 20 (what their vLLM applied from the model's own
config), the 128K cap, up to 10 rounds, a cut-off worker summarized, and a
reply with no text answered by its reasoning.

## Different: the host

Theirs: their own vLLM in FP8. Ours: OpenRouter, host order Modal, CoreWeave,
Parasail, Wafer until Modal slowed to 14 tokens/s, then the FP8 hosts only
(Wafer, Parasail, CoreWeave). Calls served: Modal 203, CoreWeave 27, Wafer
115; every call of the FP8 order went to Wafer. Three runs moved mid-run with
their finished steps kept (`log/host-switch.txt`). Modal does not publish its
precision.

Local paths in the run scripts and in four tracebacks a manager was shown read
`<scratch>` and `$HOME`. Nothing else in the runs is edited.

## What it took

The mismatch was the problem text: the first version wrote its own task file
around each problem, and the 27B planned worse from it than from the authors'
exact LiveCodeBench text (arc196_c's plan: 0 of 2 against 2 of 2). Four
Memdoor changes so a workflow can ask what their harness asks: a task answers
with the model's reasoning when it wrote no text; `top_p`/`top_k` on a model
line; `mark_cutoff` for a reply cut at the cap; a call on its own four-hour
clock.

## Where it lost

Twice, the same two gaps in the authors' loop:

- Pass 5, abc399_e (32 of 44 hidden). Round 1's program was right. The manager
invented a counterexample with a wrong expected answer and the team rewrote
it. The new program passed the samples, which do not cover the case.
- Pass 4, arc196_c and arc196_d. A program passing every hidden test was
overwritten by a later round because the manager did not say done. Both got a
correct program back before the loop's end (arc196_d at round 8, arc196_c
through the final call). At $3.44 and $4.24 they are the two most expensive
runs.

Both are fixable without changing the method (check a manager's counterexample
against a brute force on small inputs; keep the last program that passed the
samples). Neither fix is in the published numbers.

## Cost

49 of 50 solved. The 50 runs cost $35.79 (15.3M output tokens), or $38.77 with
the 16 attempts that were cut by a clock, stopped for a fix or paused. A run
took a median of 40 minutes of model time (the longest 4 hours) and $0.39.

## What the first run found (2026-10-25)

The coder wrote the workflow's files from a description of the method.
Sample: count the substrings of a string over {a, b, c} with equal counts of
each letter, up to 200,000 characters. Run on the default ladder through
OpenRouter:

- 8 of 8 steps, 5 of 5 samples. The solver took ideator A's prefix-state
  method, O(N) with a dictionary of (a−b, a−c) pairs. A brute force on 300
  random strings found no mismatch afterwards. It runs in 0.04 s at N =
  200,000. The manager found nothing to repair.
- One ideator's provider dropped the stream mid-answer. The coder had written
  `max_retries: 0`, so the run died there. The agent steps now carry
  `max_retries: 1`. The run was resumed with `--partition`, which kept the
  planner's and the other ideator's work.
- The ideator prompts the coder wrote named the sample's methods (A: a
  streaming prefix-state count, B: sorting prefix states), which would steer
  any other problem the wrong way. They now ask A for the most direct provable
  algorithm and B for one from a different family of technique (2026-10-28).
- The first attempts failed every step in 0 s with "no agent named coder". A
  run looked its agent up only in its own workspace. That was fixed in the
  gateway the same day (v1.692.0).