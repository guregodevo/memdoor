# Team mode

Shift+Tab flips the window between `auto` and `team`. In team, a turn is the
[team-solve](/docs/team-solve) workflow on this project and the model the
footer names, with your prompt as its task. The run reports back into the
conversation when it ends.

```
  ⎇ main  ·  openrouter · deepseek/deepseek-v4.1-flash  ·  pinned by you  ·  team (shift+tab)  ·  effort auto
```

```
> scripts/scan.sh must pass outside a git checkout too …

  ▶ Team mode: team-solve is on it as team-solve-20261106-113402, 18 steps, each
  role a fresh context. It reports here when it ends, with the verdict.
```

While it runs, the window draws the graph: the run's total, each step's time,
tokens and cost, and under a running step one line per call of its turn
(ctrl+o shows every line). The graph is taller than the window; PgUp reads
the steps above, End jumps back to the latest.

```
  team-solve · running · 6/18 done · 54.1k tokens · $0.001
  ✓ problem   done 0s
  │
  ▶ direct    running 51s · 26.5k tokens · $0.001  ← problem
      … 1 more (ctrl+o)
      $ bash scripts/scan.sh; echo "exit=$?" → exit 0
      $ tmp=$(mktemp -d); rsync -a --exclude .git ./ "$tmp/repo/"; bash "$tmp/repo/…
      ✎ scripts/scan.sh
  ✓ outline   done 0s  ← problem
      wrote outline.md
  │
  ✓ plan      done 13s · 27.6k tokens · $0.001  ← outline
      wrote plan.md
  │
  ✓ settle    done 0s  ← plan
  │
  ✓ baseline  done 5s  ← settle
```

At the end, the verdict, what was kept, the Scope and the diff:

```
  ■ team-solve done — results: .memdoor/runs/team-solve-2026-11-06T113402/context.md ·
  .memdoor/runs/team-solve-2026-11-06T113402/outline.md · … /plan.md · … /review.md

  verdict: review PASS
  kept: the direct attempt, untouched

  Scope:
      scope: scripts/scan.sh
      changed: scripts/scan.sh +3 -1

  The diff:
      diff --git a/scripts/scan.sh b/scripts/scan.sh
```

Shift+Tab again is `auto`: one turn, as before. The thinking effort is a
separate setting. Without leaving auto: `/w team-solve <task>`. From the
shell, the same run: `memdoor workflow run team-solve --model <id> "<task>"`.

![Team mode: Shift+Tab, a task, the team's steps, the verdict](/team-mode-2026-11-06.cast)

## What runs

First the single call, then the team if it is needed.

```
problem ─┬─ direct-a ─ save-a ─ direct-b ─ save-b ─ direct-c ─ save-c ─ pick ─┬─ test-0 ─┬─ passed ─────────────────────────────────────────┬─ test ─ review ─ repair ─ recheck ─ judge ─ keep ─ verify
         └─ outline ─ plan ─ settle ─ baseline ─ reproduce ───────────────────┘          └─ failed ─ context ─ ideate-a ‖ ideate-b ─ solve ─┘
```

- `direct-a`, `direct-b` and `direct-c` are three agent turns on your prompt,
  one after another on the tree as it was, each saved as a patch. Each also
  writes a check that fails on the tree as it was and passes with its change.
  `pick` runs every attempt's check on every other attempt's change and keeps
  the change the most checks pass; a tie goes to the smaller change. Beside them, `plan`
  (one model call) settles the Check and the Scope, and `baseline` runs the
  Check on the tree as it was.
- `reproduce`, when the prompt describes behaviour that is wrong: one turn
  writes a reproduction from the prompt's words, one executable in whatever
  the project runs, proven on the clean copy of the tree, where it must
  fail. From then on the Check is the project's command and the
  reproduction, and the work is right only when both pass. A Check that
  already passed before any change proves no fix.
- `test-0` runs the Check on the pick. When it passes, the rest
  only verifies it: a `review`, and a `judge` when a command cannot decide.
- When it fails, two ideators propose side by side (one model call each) and
  `solve` corrects the attempt from their proposals.
- `keep` decides what stays (below); `verify` fails the run on a failing
  Check or a judge's FAIL.

The planner, the ideators, the reviewer and the judge are model calls with no
tools. Only the three attempts, `solve` and `repair` change the tree. Every role starts
from nothing and reads only the files on disk, in
`.memdoor/runs/team-solve-<run>/`: `task.md`, `plan.md`, `proposal-a.md`,
`proposal-b.md`, `review.md`, `verdict.md`, `keep.txt`.

## Never worse than its own single call

The direct attempt is saved before the team touches it. What replaces it,
the team's work after a failed `test-0` or the repair after the review's
FAIL, has to prove itself the same way: the Check passes and the judge does
not fail it, or, with no Check to run, the judge says PASS. Otherwise the
attempt is put back, new and deleted files included, a repair is saved as
`repair.patch`, and `keep.txt` says why.

The Check decides, not the planner's Scope: a change outside the Scope goes
to the review, which fails it only when the task does not need it (an
instruction file, an unrelated module). When the Check already fails on the
tree as it was, only the tests the change makes fail count, as pytest,
unittest, Django's runner or go test list them; when its output lists none,
the judge decides. HEAD stays where the run found it.

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
