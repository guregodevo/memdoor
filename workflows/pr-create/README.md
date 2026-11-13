# pr-create

Opens a pull request from the current branch. It runs the repository's checks, has `pr-description` write the title and text, fills the repository's PR template from them and the recorded check output, and opens the pull request. It never merges and never touches the default branch. The default `open: draft` pushes nothing and writes the title and body for you to read:

```
memdoor workflow run pr-create --wait     # .memdoor/runs/pr-create-<run>/title.txt and body.md
```

```
sync ─┬─ checks ─┬─ body ─ open ─ ci
      └─ describe ┘
```

- `sync`: `sync.sh` fetches the default branch and asks git to merge it with the branch in memory. A conflict lists the files and stops the run before any check runs: rebase first. Being behind is reported, not a failure.
- `checks`: `checks.sh` runs each command of the `checks` arg (comma-separated, e.g. `make build,make test,make check,make scan`) and records its exit status and last 15 lines. A command that fails is recorded as it is, and does not stop the next. Empty: none run, and the body says so.
- `describe`: `pr-description` on this branch, nested on this gateway: a title, then What, Why, How to test and Risk.
- `body`: `body.sh` writes `title.txt` and `body.md`. With `.github/PULL_REQUEST_TEMPLATE.md` it replaces the Description section, gives `Fixes #` the `issue` arg, ticks a checkbox whose backticked command passed, leaves a failed one unticked with its exit status, and puts the recorded output where the template has `<test output>`. Without a template the body is the description and the test results. Nothing is added that a run did not record: no attribution line, no claim of a check that did not run.
- `open`: `open.sh` with `open: now` pushes the branch to `origin` and runs `gh pr create` against the default branch. It refuses on the default branch and when `sync` found a conflict.
- `ci`: with `open: now`, `ci.sh` polls `gh pr checks` every 20 seconds for up to `ci_wait` minutes, then writes `ci.txt`: each check's result and the pull request's mergeable state. It exits 1 when a check failed or the pull request conflicts. Checks still pending at the end are reported, not failed. A repository with no CI reports `no checks`.

Args belong to the step that reads them, edited in `~/.memdoor/workflows/pr-create/steps/`:

| arg | default | step | |
|---|---|---|---|
| `checks` | empty | `checks` | commands to run, comma-separated |
| `model` | empty | `describe` | the model `pr-description` runs on |
| `issue` | empty | `body` | the issue number for `Fixes #` |
| `open` | `draft` | `open`, `ci` | `now` pushes and opens the pull request, then reads its CI; change both |
| `ci_wait` | `10` | `ci` | minutes to wait for pending checks |

`gh` must be signed in (or `GH_TOKEN` set) for `open: now`. To open it as another account, set `GH_TOKEN` for the run.
