# qa

Uses the project the way a user does, finds bugs, proves each with a script, and writes them as issues. `file: draft` (the default) files nothing; `file: now` files them with `gh`:

```
memdoor workflow run qa --wait                          # one pass, issues written under .memdoor/runs/qa-<run>/bugs/
memdoor cron add --workflow qa "0 3 * * *"              # nightly
```

The explorer is the coder, read-only on the project. It reads the docs for what the project says it does, runs those commands with edge input, runs the tests, reads the last 20 commits and tries what they touched. It builds into a scratch directory and never stops or restarts a process. For each bug it writes a directory `bugs/<slug>/` holding `title.txt`, `expected.md`, `source.txt`, `repro.sh` and, when the cause is evident, `proposal.md`. `repro.sh` asserts the correct behaviour: it prints a `BUG: ` line and exits 1 while the bug is there, and exits 0 once it is fixed.

```
context ─ areas ─ pick ─ explore ─ confirm ─ dedupe ─ settle ─ file ─ done
```

- `context`: the open issues, `#number title` a line.
- `areas`: one model call, once: the project's areas, one a line, cheapest to test first and the ones that need a model, a GPU or a service last, from the README and the file list, kept in `.memdoor/qa/areas.txt` (delete it to ask again).
- `pick`: `pick-area.sh` takes the `focus` arg, else the next area `.memdoor/qa/tried.txt` does not hold, and starts over when all are tried. One area a run; the schedule is the budget.
- `explore`: the agent's pass over that area. At most `max_bugs` bugs, an empty `candidates.txt` when it found none.
- `confirm`: `confirm.sh` runs each `repro.sh` twice. A bug stays when both runs exit 1 with a `BUG: ` line, the project's files are unchanged (`git status`, outside `.memdoor`) and `source.txt` holds a path of the project and a line quoted from that file, character for character. Any other result drops it, with the reason in `confirm.log`: an expectation no file states is a guess. Only exit codes are compared: no language, no test framework.
- `dedupe`: one model call (`llm`, no tools). It names the confirmed bugs an open issue already describes, as `<slug> #<number>`.
- `settle`: `settle.sh` ignores a line whose slug is not confirmed or whose number is not an open issue, drops the duplicates, keeps the first `max_bugs` and writes each `issue.md` under four headings. **Context**: the commit, the OS and the date, read by the script, and `repro.sh` verbatim. **Expected results**: the explorer's sentence and the quote from the file. **Actual results**: the first run's output and the exit codes, verbatim. **Proposal**: the explorer's `proposal.md`, only when it wrote one. Only the title, the expected sentence and the proposal are the explorer's words. `summary.txt` lists them.
- `file`: runs only with `file: now`. `file.sh` creates one issue a bug with the labels in `labels` and `qa_label` that the repository has. With `max_open` issues carrying `qa_label` already open it files nothing.

Args belong to the step that reads them, edited in the library copy in `~/.memdoor/workflows/qa/steps/`:

| arg | default | step | |
|---|---|---|---|
| `focus` | empty | `pick` | where to look, in words; empty is the next untried area |
| `max_bugs` | `3` | `explore`, `settle` | bugs a run keeps; change both |
| `file` | `draft` | `file` | `now` files the issues |
| `labels` | `bug` | `file` | labels on a filed issue, comma-separated |
| `qa_label` | `qa` | `file` | the label that counts the workflow's own open issues |
| `max_open` | `5` | `file` | its own open issues at once |

`repro.sh` runs on your host with your environment, written by a model. Read the drafts before setting `file: now` on a repository you do not control. The workflow reads the issue list and creates issues with `gh`, so `gh` must be signed in with `issues: write`. To file as another account, set `GH_TOKEN` for the run. `qa` composes with `team-backlog`: set `label: bug` in `team-backlog`'s `steps/next.yaml` and the bugs `qa` filed become its next issues, each ending in a pull request a person merges.

A repro that needs a service, credentials or a display is dropped, because `confirm` cannot see the bug twice. What costs most is the `explore` step: its budget is 80,000 tokens, its soft deadline 8 minutes, its clock 12.
