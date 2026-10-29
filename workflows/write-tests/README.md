# write-tests

Writes tests for the code a change touches, and leaves them in the working tree with a report. It adds tests; it never changes the code under test or a line of an existing test.

`detect` finds the test command (`MEMDOOR_TEST_COMMAND`, else `go test ./...`, `cargo test`, `npm test`, pytest or `make test`, as fix-tests does) and `diff` takes the change (a pull request against its base, else the branch against the default branch, or the last commit). `baseline` runs the tests before any is written: when an existing test already fails with the change, that is the finding — the run fails and says which test, and writes nothing, since tests written around a break would only lock it in. Otherwise `write` adds tests for each function the change touches (its normal case, an edge case, the case the change is about), done when `check.sh` passes: at least one test file is new or added to, no existing test line is changed or removed, and the test command passes. `check` writes `.memdoor/runs/write-tests-<partition>.md`: the tests, the diff of the test files, the command's last lines. Nothing is committed.

Run it with `memdoor workflow run write-tests --wait`, or in GitHub Actions with `uses: guregodevo/memdoor-action@v1` and `workflow: write-tests`, after the steps that install the project's dependencies.

On a slow model (`MEMDOOR_LEAN=1`, which the Action sets with `local-model:`), one model call writes one new test file from the diff and the changed files' contents; `land` writes it (never over an existing file) and `check` keeps it only when the check passes, else removes it and says so.
