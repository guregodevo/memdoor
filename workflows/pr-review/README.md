# pr-review

Reviews the change in front of it — a pull request, a branch or the last commit — and fails only on what must not merge.

`diff` takes the change: in a GitHub pull request, the PR against its base branch; elsewhere, the branch against the default branch, or the last commit, with uncommitted edits included. `review` reads it with the `review` skill, opens the files around it, verifies what it can by running it, and writes `.memdoor/runs/pr-review-<partition>.md`: a summary and one line per finding, **P0** (a bug, a security hole, data loss, a broken build or test — must not merge), **P1** (should fix), **P2** (a nit). `verdict` fails the run when there is a P0 and names each one, so a CI job goes red on a blocker and green otherwise. It changes no file in the project. Run it with `memdoor workflow run pr-review --wait`, or in GitHub Actions with `uses: guregodevo/memdoor-action@v1` and `workflow: pr-review` (check out with `fetch-depth: 0` or the default; the base branch is fetched). Needs a model on your key.

On a slow model, set `MEMDOOR_LEAN=1` (the GitHub Action sets it with `local-model:`, a model on the runner's CPU): `review-lean` reviews instead, in one model call with the diff in the prompt and thinking off, no tools and no loop, so nothing is run to verify it. A CPU reads a few thousand tokens a minute, so a big diff is slow.

Run it on `pull_request`, never `pull_request_target`: GitHub gives a fork's pull request no secrets, so a stranger's code never runs next to your key; `pull_request_target` runs their code with your secrets.
