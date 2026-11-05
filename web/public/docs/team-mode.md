# Team mode

Shift+Tab flips the window between `auto` and `team`. In team, a turn is the
[team-solve](/docs/team-solve) workflow on this project and the model the
footer names, with your prompt as its task. The run reports back into the
conversation when it ends.

```
  openrouter · deepseek/deepseek-v4.1-flash  ·  pinned by you  ·  team (shift+tab)  ·  effort auto
```

```
> scripts/scan.sh must pass outside a git checkout too …

  ▶ Team mode: team-solve is on it as team-solve-20261105-191225, 18 steps, each
  role a fresh context. It reports here when it ends, with the verdict.
```

Shift+Tab again is `auto`: one turn, as before. The thinking effort is a
separate setting. From the shell, the same run:
`memdoor workflow run team-solve --model <id> "<task>"`.

![Team mode: Shift+Tab, a task, the team's steps, the verdict](/team-mode.cast)

## What runs

First the single call, then the team if it is needed.

```
problem ─┬─ direct ─────────────────────────────┬─ test-0 ─┬─ passed ─────────────────────────────────────────┬─ test ─ review ─ repair ─ recheck ─ judge ─ keep ─ verify
         └─ outline ─ plan ─ settle ─ baseline ─┘          └─ failed ─ context ─ ideate-a ‖ ideate-b ─ solve ─┘
```

- `direct` is one agent turn on your prompt, in the tree. Beside it, `plan`
  (one model call) settles the Check and the Scope, and `baseline` runs the
  Check on the tree as it was.
- `test-0` runs the Check on the direct attempt. When it passes, the rest
  only verifies it: a `review`, and a `judge` when a command cannot decide.
- When it fails, two ideators propose side by side (one model call each) and
  `solve` corrects the attempt from their proposals.
- `keep` decides what stays (below); `verify` fails the run on a failing
  Check or a judge's FAIL.

The planner, the ideators, the reviewer and the judge are model calls with no
tools. Only `direct`, `solve` and `repair` change the tree. Every role starts
from nothing and reads only the files on disk, in
`.memdoor/runs/team-solve-<run>/`: `task.md`, `plan.md`, `proposal-a.md`,
`proposal-b.md`, `review.md`, `verdict.md`, `keep.txt`.

## Never worse than its own single call

The direct attempt is saved before the team touches it. When it passed
`test-0`, it stays: a repair after the review is saved as `repair.patch`
and not applied, since a model's verdict is no proof. When it failed, the
team's work replaces it only when it passes a check the attempt did not: the
Check passes, or, with no Check to run, the judge says PASS. Otherwise the
attempt is put back, new and deleted files included, and `keep.txt` says why.

The Check decides, not the planner's Scope: a change outside the Scope goes
to the review, which fails it only when the task does not need it (an
instruction file, an unrelated module). A Check that already fails on the
tree as it was does not count; the judge decides then. HEAD stays where the
run found it.

## Measured

SWE-bench Verified, the nine Django instances labelled "1-4 hours" from
versions 3.2 and 4.1; deepseek/deepseek-v4.1-flash; the official harness;
one run of each.

| | single call | team mode |
|---|---|---|
| resolved | 8 of 9 | 9 of 9 |

On django-13212 the single call fixed `validators.py` and missed
`forms/fields.py`; the team's direct attempt fixed both, its review failed
it, the repair and the judge did not do better, and `keep` put the attempt
back: resolved. One run per instance is a sample, not a rate.

On five tasks of this repository and of mario, each checked by a test
written before the runs, the single call and the team passed all five; the
team took 31 s to 6 minutes a task where it had taken 8 to 10 before the
single call came first.

## Cost

About two to three times the single call: the planner, the reviewer and the
judge add a few model calls to every run, and the team path adds two
proposals and a solver when the first attempt fails. On deepseek-v4.1-flash
that was one to five cents a task.
