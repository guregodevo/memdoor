# team-backlog

Works a repository's open issues unattended: the next issue nobody is on and nobody tried, team-solve on it in a branch of its own, a pull request when the Check passed, a comment on the issue when it did not. One issue a run; the schedule is the budget:

```
memdoor workflow run team-backlog --wait             # one issue, now
memdoor cron add --workflow team-backlog "0 2 * * *" # one issue a night
```

Needs `gh` signed in with push rights to the repository. On a repository whose issues include notes that are not tasks (a roadmap item, a question), give the ones to work a label and set `label` to it; without one, the oldest open issue is the next, whatever it says. The caps are the workflow's, in `steps/next.yaml`: `max_open_prs` (its own pull requests open at once, by the branch prefix `memdoor/issue-`), `label` (only issues with it; empty: any). The model every step of team-solve runs on is `model` in `steps/solve.yaml`; edit the library copy in `~/.memdoor/workflows/team-backlog/` to change them.

```
next ─ branch ─ solve ─┬─ pr ──────┬─ done
                        └─ comment ─┘
```

- `next`: the open issues from `gh`, minus the assigned, the tried (`.memdoor/backlog/tried.txt`, or a branch `memdoor/issue-<n>`) and, with a label, the unlabelled; the lowest number is the issue. At the cap, or with none left, the run ends here and says so.
- `branch`: a git worktree under `.memdoor/backlog/<n>` on `memdoor/issue-<n>`, from the default branch.
- `solve`: `memdoor workflow run team-solve --wait` in the worktree, the issue's title and body as the prompt, up to three hours. team-solve settles the Check and the Scope from the project, as for any task; its net holds: nothing the Check did not pass is kept.
- `pr`: when the run passed, the change committed and pushed, a pull request that closes the issue and carries keep's line, the Check's status, the review and the judge, and the Check's last lines.
- `comment`: when it did not, the same findings as a comment on the issue.
- `done`: the issue goes into `tried.txt`; the worktree is removed, the branch stays.

Never twice on one issue, never on an assigned one, never more than `max_open_prs` of its own open. What it costs is one team-solve run an issue: about $0.02 on DeepSeek V4.1 Flash for a SWE-bench Verified instance, up to a few cents on a long one.
