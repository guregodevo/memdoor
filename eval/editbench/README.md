# edit-bench: the hashline measurement (docs/features/HASHLINE.md §7)

20 Go tasks, each a single mutation that breaks one test (flipped comparison,
wrong constant, missing nil check, off-by-one, wrong call, inverted condition),
stated as "make `go test ./...` pass".

Arms: `edit_format` workspace setting — `patch` (context hunks, today) or
`hashline` (reads numbered and tagged) — × GLM 5.3 Flash and DeepSeek V4.1
Flash. One run each, 80 runs.

The model gets the coder's own `read_file` and `apply_patch`: descriptions,
schemas, reads and edits all come from the `memdoor/tools` package, with
`tools.SetEditFormat` set to the arm. The loop around them is minimal (a
short system prompt, no decision model), so it measures the edit format,
not the whole coder.

Per run, recorded to `results/`: pass, output tokens, input tokens,
apply_patch calls and failures, turn time.

## Running

    eval/editbench/runall.sh                      # all 80 runs, resumable
    go run ./eval/editbench -model <openrouter-model> -format patch|hashline [-task N]

## Rule (§7)

hashline becomes a model's default when its pass rate is within 2 points of
the patch format's (or better) AND its output tokens per passed task are
lower. Otherwise that model keeps the patch format. Results and the decision
go in docs/features/HASHLINE.md.
