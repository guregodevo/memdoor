# careful-change

One coding task in checked steps: assumptions first, your approval, a failing check, the smallest change that passes it, a review that every changed line traces to the task.

```
memdoor workflow run careful-change "Reject an empty name in ParseSlug; go test ./pkg/... must pass"
```

```
assumptions ─ (approve) ─ check ─ lock ─ change ─ diff ─ review ─ trim ─ verify ─ report
```

The four rules of [andrej-karpathy-skills](https://github.com/multica-ai/andrej-karpathy-skills), each held by a step of the DAG instead of a line in a prompt:

| Rule | Held by |
|---|---|
| Think before coding | `assumptions`, then the `approve` gate |
| Simplicity first | `change`, then `review` and `trim` |
| Surgical changes | `review` (every changed line traces to the task), `trim` |
| Goal-driven execution | `check` (fails first), `lock`, `verify` |

| Step | Kind | Does | Done when |
|---|---|---|---|
| `assumptions` | agent | Reads the code. Writes `assumptions.md`: readings, assumptions, a simpler way, questions, the check. Changes no file. | `assumptions.md` exists |
| `approve` | gate | Waits. Correct `assumptions.md` in place, then open it: `a` in `/workflow` or `memdoor workflow approve <run> approve`. Later steps take the file over the prompt. | opened |
| `check` | agent | Adds a test to the project's test files and writes `check`, an executable that runs it. | `check` exists and fails |
| `lock` | command | Copies `check` to `check.lock`. Every later step runs the copy. | — |
| `change` | agent | The smallest change that passes `check.lock`: nothing the task did not ask for, no edit around it, no change to the check. | `check.lock` passes |
| `diff` | command | `git status` and `git diff`, new files included, `.memdoor` left out. | — |
| `review` | llm | One call that did not write the change. Lists each hunk not traced to the task or speculative, then `verdict: PASS` or `verdict: FAIL`. | — |
| `trim` | agent | On a FAIL, puts back the lines the review listed. | skipped on a PASS |
| `verify` | command | Runs `check.lock` on the tree the run leaves. | fails the run when it fails |
| `report` | template | `report.md`: the assumptions, the review, the diff stat. | `report.md` exists |

| Leaves | Where |
|---|---|
| The change and its test | the working tree, uncommitted |
| `assumptions.md`, `check`, `check.lock`, `report.md` | `.memdoor/runs/careful-change-<run>/`, out of git |

Needs a model on your key.

| Run, 2026-11-21 | Model | Result | Calls | $ |
|---|---|---|---|---|
| Go: `Parse` rejects an empty name | DeepSeek V4.1 Flash | 3-line guard, review PASS | 37 | 0.008 |
| LiveCodeBench arc192_b | DeepSeek V4.1 Flash 1017 | 42 of 43 tests and 400 of 400 random inputs against an exact search; the failure is a wrong stored test. The coder had read the hidden tests. | 101 | 0.051 |
