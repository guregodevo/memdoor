# nightly-rule

A nightly three-step job for a trading desk, from a comment on the launch
thread: build a trading rule, grade it, file the result, where the filing
step must refuse if what it reads is older than step 1's start.

The date is the partition. Step 1 writes `rules/<date>.start` before anything
else. Steps 2 and 3 have a command target the engine runs itself:

    test "grades/<date>.json" -nt "rules/<date>.start"

A step is done only if that passes, so nothing older than step 1's start can
be filed, whatever a model says.

## What the replay found

Second run, same date, the start stamp touched: step 3 reran because its own
check failed, but trusted the old grade on its done record from the previous
run. Fixed in the engine the same day
([mario ef2919c](https://github.com/guregodevo/mario/commit/ef2919c)): a
step done in a past run is re-checked against its target before anything
downstream trusts it. Now the stale grade reruns first, then the filing.

One more thing the replay showed: the model on the grade step passed the
`-nt` check by touching the file instead of regrading. The gate is structural;
what an agent step does inside it is the model's. If you have a grader
script, make `grade` a `command` step that runs it, with the same target. A
script cannot touch its way past the check:

    type: command
    command: python grade.py {{.partition}}
    requires:
      - table_pattern: build
    target:
      command: test "grades/{{.partition}}.json" -nt "rules/{{.partition}}.start"

## Run

    memdoor workflow run nightly-rule --partition 2026-10-07
    memdoor cron add --id nightly-rule --schedule "0 1 * * *" --workflow nightly-rule --partition today
