# explain-failure

Explains why a failed run failed, and what to change — it never changes code.

`context` gathers the failure: in GitHub Actions (started on `workflow_run` for a failed run) it reads the failed run's failed jobs' logs with `gh run view <id> --log-failed` (the last few hundred lines) and the commit's diff against its parent; elsewhere it detects the project's test command (`MEMDOOR_TEST_COMMAND`, `go test`, `cargo test`, `npm test`, `pytest`, `make test`) and runs it — when the tests pass it writes the report itself saying there is nothing to explain, and the run ends there. `explain` reads the logs and the diff, opens the files around the failure and verifies what it can, and writes `.memdoor/runs/explain-failure-<partition>.md`: **Cause** (the failing step and the file:line it points at), **Why** (two or three sentences), **Fix** (the concrete change to make). The run passes once the report is written. Nothing in the project is modified.

Run it with `memdoor workflow run explain-failure --wait`, or in GitHub Actions with `uses: guregodevo/memdoor-action@v1` and `workflow: explain-failure` (`gh` and `GH_TOKEN` come with the runner; the workflow_run payload supplies the failed run's id). Partition it by the commit: `partition: ${{ github.event.workflow_run.head_sha }}` on the Action, or `git rev-parse --short HEAD` on a laptop.

On a slow model, set `MEMDOOR_LEAN=1` (the Action sets it with `local-model:`, a small model on the runner's CPU): `explain-lean` explains instead, in one model call with the logs and the diff inlined in the prompt, thinking off, no tools and no loop. Keep the log tail short — a CPU reads a few thousand tokens a minute.
