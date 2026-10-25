# team-solve

A programming problem solved by a team of fresh contexts of one model that
talk only through files on disk, the method of
[GVS5H](https://github.com/slee-persis/GVS5H) (Lee, 2026: five Qwen3.8-27B
models match Fable 5 on LiveCodeBench Hard). The coder built this workflow
from a description of that method on 2026-10-25; the task files are its.

Eight steps. A planner reads `task.md` and writes `plan.md` and a `notes.md`
ledger with one anchor per worker. Two ideators run in parallel and each
inserts a proposal at its own anchor. A solver reads the ledger and writes
`solution.py`. A `command` step runs every `tests/N.in` against `tests/N.out`
and records the status. A manager reads the captured result, not a model's
claim, and writes repair instructions if a sample failed. A repair worker
applies them. A final `command` step reruns the samples and is the only thing
that decides success:

    type: command
    requires:
      - table_pattern: repair
    command: |
      bash .memdoor/workflows/team-solve/check-samples.sh > final-test-output.log 2>&1
      result=$?
      cat final-test-output.log
      exit "$result"

Every agent step says "fresh context, read the files, do not rely on chat":
that is the ledger discipline the paper attributes the gains to.

## Use it

Copy the folder into a project, or into the library every project sees, then
put a `task.md` and paired `tests/N.in` / `tests/N.out` files at the project
root. `sample/` holds the problem it was built on:

    cp -r examples/workflows/team-solve ~/.memdoor/workflows/
    cp -r examples/workflows/team-solve/sample/* <your-project>/
    cd <your-project> && memdoor workflow run team-solve

It leaves `plan.md`, `notes.md`, `solution.py`, `review.md` and the test logs
in the project. To run it on a local model, pin one (`/model`, or
`memdoor connect local` for janis, Ollama or llama-server) before the run; the
steps name no model.

## What the first run found

The sample problem: count the substrings of a string over {a, b, c} with
equal counts of each letter, up to 200,000 characters. Run on the default
ladder through OpenRouter:

- 8 of 8 steps, 5 of 5 samples. The solver chose the prefix-state method
  ideator A proposed, O(N) with a dictionary of (a−b, a−c) pairs. Checked
  afterwards against a brute force on 300 random strings: no mismatch;
  0.04 s at N = 200,000. The manager found nothing to repair.
- One ideator's provider dropped the stream mid-answer. The coder had written
  `max_retries: 0`; the run died there. The agent steps now carry
  `max_retries: 1`, the one line the run proved wrong, and the run was
  resumed with `--partition`, which kept the planner's and the other
  ideator's work.
- The first attempts failed every step in 0 s with "no agent named coder":
  a run looked its agent up only in its own workspace. Fixed in the gateway
  the same day (v1.692.0).
