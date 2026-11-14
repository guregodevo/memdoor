# team-backlog

Works a repository's open issues unattended: the next issue nobody is on, team-solve on it in a branch of its own, a pull request when the Check passed, a comment on the issue when it did not. One issue a run; the schedule is the budget:

```
memdoor workflow run team-backlog --wait             # one issue, now
memdoor cron add --workflow team-backlog "0 2 * * *" # one issue a night
```

Needs `gh` signed in with push rights to the repository (or `GH_TOKEN`). Every step is YAML: one command or one template. There is no script in this directory.

```
next ─┬─ task ───┬─ solve ─ report ─┬─ commit ─ pr-body ─ pr ──────┬─ done
      └─ branch ─┘                  └─ comment-body ─ comment ─────┘
```

- `next`: one `gh issue list` search: open, unassigned, no pull request linked, not commented on by this account, oldest first, with `label` when set. Its output is the issue number. At `max_open_prs` pull requests of this account open, or with no issue left, it is empty and every step after it is skipped.
- `task`: the issue's title, body and number, as the task team-solve is given.
- `branch`: a git worktree under `.memdoor/backlog/<n>` on `memdoor/issue-<n>`, from `origin/<base>`. Its output is the worktree.
- `solve`: `memdoor workflow run team-solve --wait` in the worktree, the task on its standard input, up to three hours. team-solve is run as it is. The step's output is `exit 0` when the Check passed.
- `report`: `memdoor workflow report` on that run: the attempts, the Check, the review, the judge.
- `commit`, `pr-body`, `pr`: when `solve` printed `exit 0`. The change is committed and pushed, the body is a template (`Closes #<n>` and the report), and `gh pr create` opens the pull request.
- `comment-body`, `comment`: when it did not. The report goes on the issue as a comment.
- `done`: the worktree is removed, the branch stays.

One pull request at a time by default (`max_open_prs: 1`): the next issue is not taken until the last pull request is merged or closed, so each fix starts from the one before. With more, two issues about the same lines are worked from the same base and their pull requests conflict (guregodevo/llamafit#33 and #34, both one line of `PickDraftModel`).

What keeps an issue from being tried twice is on GitHub, not in a file: a pull request linked to it, or this account's comment on it. An issue a person assigned is never taken.

Args, in the step that reads them, edited in `~/.memdoor/workflows/team-backlog/steps/`:

| arg | default | step | |
|---|---|---|---|
| `label` | empty | `next` | only issues with this label |
| `max_open_prs` | `1` | `next` | this account's open pull requests at once |
| `base` | `main` | `branch` | the branch the work starts from |
| `model` | empty | `solve` | the model team-solve runs on |

What it costs is one team-solve run an issue: about $0.02 on DeepSeek V4.1 Flash for a SWE-bench Verified instance, up to a few cents on a long one.
