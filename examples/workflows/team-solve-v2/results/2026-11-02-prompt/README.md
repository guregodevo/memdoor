# team-solve-v2 runs, 2026-11-02: the prompt defines the task

Three runs of the built-in on a scratch gateway with a fresh HOME (the seed
alone provided the workflow), every step on z-ai/glm-5.3-flash through
`--model`, started at the same time. Each folder holds the run's own files
(`run/`: the prompt, the filled `task.md`, `plan.md`, the `notes.md` ledger,
`changes.diff`, the check logs, `review.md`, `verdict.md`), the progress
lines, the files the task produced and `tree.diff`, what the run changed in
the project.

| run | the task | steps | wall time | check | verdict | calls | cost |
|---|---|---|---|---|---|---|---|
| refactor-llamafit | "Rename ReadGGUFMetadata to ParseGGUFMetadata everywhere in this repository … go build ./... and go test ./... must still pass", in a clone of [llamafit](https://github.com/guregodevo/llamafit) | 10 of 10 | 15 min 17 s | `go test ./...` passes | review PASS, judge PASS | 96 | $0.035 |
| document-from-prompt | "Write answer.md: a 120-word note for a new teammate on how to name a git branch …", in an empty repository | 10 of 10 | 15 min 14 s | judge (nothing to run) | judge PASS, 120 words | 80 | $0.022 |
| sample-empty-folder | no prompt: the sample problem copied into an empty folder | 10 of 10 | 16 min 08 s | 5 samples of 5 pass | review PASS | 65 | $0.026 |

The refactor changed four files (`gguf.go`, `gguf_test.go`, `server.go`,
`cmd/llamafit/main.go`): the definition, both callers, the three test names
and the comments; no `ReadGGUFMetadata` is left. `plan` filled the template
on its own: the deliverable "changes to this tree", the check
`go test ./...` run once on the untouched tree first, the scope four files.

Not a benchmark: one task each, one pass. The three ran at once on one
gateway, so the wall times include waiting for each other.
