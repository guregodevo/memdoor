# karpathy

One coding task in checked steps: assumptions first, your approval, a failing check, the smallest change that passes it, a review that every changed line traces to the task.

```
memdoor workflow run karpathy "Reject an empty name in ParseSlug; go test ./pkg/... must pass"
```

```
folder ─ think-before-coding ─ (approve) ─ goal ─ lock ─ simplicity-first ─ diff ─ surgical-review ─ surgical-trim ─ verify ─ report
```

The four rules of [andrej-karpathy-skills](https://github.com/multica-ai/andrej-karpathy-skills), each held by a step of the DAG instead of a line in a prompt:

| Rule | Held by |
|---|---|
| Think before coding | `think-before-coding`, then the `approve` gate |
| Simplicity first | `simplicity-first`, then `surgical-review` and `surgical-trim` |
| Surgical changes | `surgical-review` (every changed line traces to the task), `surgical-trim` |
| Goal-driven execution | `goal` (fails first), `lock`, `verify` |

| Step | Kind | Does | Done when |
|---|---|---|---|
| `folder` | command | Creates `.memdoor/runs/karpathy-<run>/lab/`, the only folder the later steps write in. | — |
| `think-before-coding` | agent | Reads the code. Writes `assumptions.md`: readings, assumptions, a simpler way, questions, the check. Its experiments go in `lab/`. Changes no file of the project. | `assumptions.md` exists |
| `approve` | gate | Waits. Correct `assumptions.md` in place, then open it: `a` in `/workflow` or `memdoor workflow approve <run> approve`. Later steps take the file over the prompt. | opened |
| `goal` | agent | Reuses what `lab/` holds, adds a test to the project's test files and writes `check`, an executable that runs it. | `check` exists and fails |
| `lock` | command | Copies `check` to `check.lock`. Every later step runs the copy. | — |
| `simplicity-first` | agent | The smallest change that passes `check.lock`: nothing the task did not ask for, no edit around it, no change to the check. | `check.lock` passes |
| `diff` | command | `git status` and `git diff`, new files included, `.memdoor` left out. | — |
| `surgical-review` | llm | One call that did not write the change. Lists each hunk not traced to the task or speculative, then `verdict: PASS` or `verdict: FAIL`. | — |
| `surgical-trim` | agent | On a FAIL, puts back the lines the review listed. | skipped on a PASS |
| `verify` | command | Runs `check.lock` on the tree the run leaves. | fails the run when it fails |
| `report` | template | `report.md`: the assumptions, the review, the diff stat. | `report.md` exists |

| Leaves | Where |
|---|---|
| The change and its test | the working tree, uncommitted |
| `assumptions.md`, `lab/`, `check`, `check.lock`, `report.md` | `.memdoor/runs/karpathy-<run>/`, out of git |

`think-before-coding`, `goal` and `simplicity-first` run at `thinking medium`: at `high` they hit the 32K-token cap on single replies on hard problems and ended without their file. The line has no model id, so `--model` still picks the model.

Needs a model on your key.

| Run, 2026-11-21, as careful-change | Model | Result | Calls | $ |
|---|---|---|---|---|
| Go: `Parse` rejects an empty name | DeepSeek V4.1 Flash | 3-line guard, review PASS | 37 | 0.008 |
| LiveCodeBench arc192_b | DeepSeek V4.1 Flash 1017 | 42 of 43 tests and 400 of 400 random inputs against an exact search; the failure is a wrong stored test. The coder had read the hidden tests. | 101 | 0.051 |
| LiveCodeBench abc399_e, gate opened unedited | DeepSeek V4.1 Flash 1017 | 44 of 44 (`fair.py`). team-solve-fanout: 43 of 44 on both of its runs. | 221 | 0.183 |
| LiveCodeBench 3562, gate opened unedited | DeepSeek V4.1 Flash 1017 | 42 of 42 (`fair.py`) | 116 | 0.070 |

| Run, 2026-11-21, as karpathy, `goal` reusing `lab/` | Model | Result | Calls | $ |
|---|---|---|---|---|
| LiveCodeBench abc399_e, gate opened unedited | DeepSeek V4.1 Flash 1017 | 44 of 44 (`fair.py`), 939 s | 105 | 0.075 |
