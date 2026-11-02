# Task

## Prompt

Rename ReadGGUFMetadata to ParseGGUFMetadata everywhere in this repository: the function, every caller, the tests and their names, and the comments that name it. go build ./... and go test ./... must still pass.

## Deliverable

changes to this tree: the identifier `ReadGGUFMetadata` renamed to `ParseGGUFMetadata` in every Go file (definition in gguf.go, callers in cmd/llamafit/main.go and server.go) and in gguf_test.go (function names, test names, strings, and comments that name it). No new files.

## Check

`go test ./...` (plus `go build ./...`; both must pass). Baseline on the untouched tree: PASS as of 2026-11-02 (module github.com/guregodevo/llamafit, package ok 0.334s).

## Scope

gguf.go, gguf_test.go, cmd/llamafit/main.go, server.go, and README.md if and only if it names the function. Everything else (Makefile, go.mod, go.sum, hf.go, hf_test.go, picker.go, picker_test.go, runner.go, runner_test.go, calibration.go, LICENSE) stays as it is. No new files, no dependency changes.

## Done when

Reviewer: `grep -rn ReadGGUFMetadata .` over Go sources returns nothing, every replacement is `ParseGGUFMetadata`, comments naming the old name are updated too, and no unrelated line changed. Judge: `go build ./...` and `go test ./...` both exit 0 on the modified tree.
