# pr-create

Opens a pull request from the current branch. It checks the branch has commits of its own and merges into the base branch, runs the repository's checks, takes the title and text from `pr-description`, writes the body from a template, and opens the pull request. It never merges. The default `open: draft` pushes nothing and writes the body for you to read:

```
memdoor workflow run pr-create --wait     # .memdoor/runs/pr-create-<run>/body.md
```

```
ahead ─ merges ─ checks ───┬─ body ─ open ─ ci
(pr-description) describe ─┘
```

Every step is YAML. There is no script in this directory.

- `ahead`: a command. The branch has at least one commit beyond `origin/<base>`. On the base branch, or with nothing committed, it fails and the run stops.
- `merges`: a command, `git merge-tree`. It merges the base into the branch in memory. A conflict fails the step and its output names the files: rebase first. Being behind is not a failure. It needs git 2.38 or later.
- `checks`: a command. It runs the `checks` line and prints the last 40 lines of output, then `exit N`. Empty `checks`: the step is skipped.
- `pr-description` is required, not run by a script: its `diff`, `commits` and `describe` steps come into this run and write the title and the What, Why, How to test and Risk.
- `body`: a `template` step. The description, `Fixes #<issue>` when `issue` is set, `notes`, then the checks: a box ticked when their output ends `exit 0`, and the output under it. No check run reads `None run.` The first line of the body is the title, as `pr-description` wrote it.
- `open`: a command, only with `open: now`. `git push -u origin HEAD`, then `gh pr create` against `base`, titled with the description's first line. Its output is the pull request's URL.
- `ci`: a command, only with `open: now` and `ci: yes`. It waits 45 seconds for GitHub to register the checks, then `gh pr checks <url> --watch`. A failed check fails the step. On a repository with no CI on pull requests set `ci: no`: `gh` reports "no checks" as an error.

Args belong to the step that reads them, edited in `~/.memdoor/workflows/pr-create/steps/`:

| arg | default | step | |
|---|---|---|---|
| `base` | `main` | `ahead`, `merges`, `open` | the branch the pull request goes to; change all three |
| `checks` | empty | `checks` | one shell line, e.g. `make build && make test` |
| `issue` | empty | `body` | the issue the pull request fixes |
| `notes` | empty | `body` | text added after the description |
| `open` | `draft` | `open`, `ci` | `now` pushes and opens the pull request; change both |
| `ci` | `yes` | `ci` | `no` on a repository with no CI on pull requests |

A workflow that requires `pr-create`'s `ci` binds these with `with:`. The body's shape is this workflow's template, not the repository's `.github/PULL_REQUEST_TEMPLATE.md`.

`gh` must be signed in (or `GH_TOKEN` set) for `open: now`. To open it as another account, set `GH_TOKEN` for the run.
