# team-solve-fanout

team-solve with three additions. Opt in by name; `team-solve` is unchanged.

```
memdoor workflow run team-solve-fanout "Fix the -0.00 in the ledger report; python3 -m unittest must pass"
```

- `survey`: one read-only agent turn lists every code path the task goes through, siblings included, in `survey.md`. The attempts read it. `outline` passes it to the planner, whose Scope covers those files.
- `paths`: one agent turn writes a check per surveyed code path, in `paths/` of the run folder, each failing on the tree as found. None for a new program. `check.sh` runs them beside the Check.
- `direct`: a fan-out (mario `fanout:`). Three attempts at once, each in its own git worktree. Mario runs every check (path checks and the attempts' own) in every attempt's worktree. A check that passes on the tree as found is ignored; an attempt never scores on its own check. Settled when one attempt leads, no check discriminates, or every attempt passes every check; otherwise one more, five at most. The winner's change goes into the tree. The decision model may settle sooner, never run more.

The steps after `direct` are team-solve's. Run folder: `.memdoor/runs/team-solve-fanout-<partition>`. Scripts: team-solve's.

Measured on SWE-bench Verified (DeepSeek V4.1 Flash): on the 23 unresolved instances where no step touched the file the fix needed, the survey named every needed file in 17. With the survey, 3 of 9 of them resolved, against 1 of 9 without. The fan-out and the path checks have no SWE-bench measurement yet.
