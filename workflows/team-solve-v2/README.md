# team-solve-v2

A team of fresh contexts does the task a prompt defines, in the project it runs in, and checks the work before the run passes:

```
memdoor workflow run team-solve-v2 "Rename ReadGGUFMetadata to ParseGGUFMetadata everywhere; go test ./... must pass"
memdoor workflow run team-solve-v2 - < task.md           # a long prompt from a file
memdoor workflow run team-solve-v2 --model qwen/qwen3.8-27b "…"
```

The output is whatever the task asks for: changes to the tree (a refactor, a fix, a feature across many files), a new program, a document. The run's own files never mix with the project's: they are in `.memdoor/runs/team-solve-v2-<run>/`, out of git.

**The task.** The prompt goes into the workflow's task template (`task.template.md`) as `task.md` in the run's folder. `plan` reads the project and fills in the rest of it: the Deliverable, the Check (one command that exits 0 when the work is right, such as `go test ./...`; or the samples; or the judge), the Scope (the files the work may touch) and Done when. Every later role works from that file.

**The team.** `problem` writes the task and records the tree's state. `plan` fills the task and writes `plan.md` and `notes.md`, the ledger. `ideate-a` and `ideate-b`, in parallel, each write a proposal to its own file, `proposal-a.md` and `proposal-b.md`, and change no project file. `solve` does the work within the Scope. `test` writes `changes.diff` (everything the run changed) and runs the Check. `review` reads the diff and the check's output and writes `review.md`, `verdict: PASS` or `verdict: FAIL` first, with repair instructions. `repair` is one fresh worker on them. `judge`, unless the samples are the check, is a fresh context that reads only the task and the diff and writes `verdict.md`. `verify` runs the Check on the final tree and fails the run when it fails or the verdict is FAIL. Each role starts from nothing and reads only the files on disk.

**A step is done when its file is.** Each agent step's target is `done.sh <step>`, copied into the run's folder, so the step's own turn cannot edit it:

| step | done when |
|---|---|
| `plan` | `task.md` has the Deliverable, a Check (a command in backticks, samples or judge), the Scope and Done when filled in, the Prompt unchanged; `plan.md` and `notes.md` are written |
| `ideate-a`, `ideate-b` | `proposal-a.md`, `proposal-b.md` have five lines or more |
| `solve` | `changes.diff` is not empty |
| `review`, `judge` | `review.md`, `verdict.md` start with `verdict: PASS` or `verdict: FAIL` |
| `repair` | `repair.md` is written; skipped when the review passed |

A turn that ends with its target failing is told what is missing and goes on; when it never passes, the step fails and the run stops there instead of handing an empty file to the next role.

**Without a prompt** a `task.md` in the folder is the prompt, and an empty folder gets the sample problem: `task.md` and five `tests/N.in` / `tests/N.out`, checked by `check-samples.sh`.

**The model.** `--model <id>` puts every step on one model, an id `memdoor model search` shows; without it, the `/model` pin, else the agent's ladder. With no key, `memdoor connect local` connects Ollama, llama.cpp, vLLM or LM Studio on this machine ([how](https://memdoor.ai/docs/providers#a-local-runtime-no-key)); Ollama needs a 32K window (`OLLAMA_CONTEXT_LENGTH=32768`).

**The time.** On a hosted model a run is minutes. On a model on this machine every step is slower: the agent steps allow 40 minutes, solve and repair 90, and a run takes hours, as a background job (`--timeout` raises the run's clock, 2 hours by default).

**The score.** The method is [GVS5H](https://github.com/slee-persis/GVS5H)'s. Its published benchmark, 9.8 of 10 on the first ten LiveCodeBench Hard problems against Fable 5's 8.0, is the `team-solve` workflow's, run with the authors' exact prompts: [examples/workflows/team-solve](../../examples/workflows/team-solve/). v2's prompts differ, so v2 has no benchmark score of its own yet; its runs are under [examples/workflows/team-solve-v2/results](../../examples/workflows/team-solve-v2/results/).

Needs bash, git for a change to a repository, and Python 3 for the samples.
