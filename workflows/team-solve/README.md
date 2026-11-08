# team-solve

A team of fresh contexts does the task a prompt defines, in the project it runs in, and checks the work before the run passes:

```
memdoor workflow run team-solve "Rename ReadGGUFMetadata to ParseGGUFMetadata everywhere; go test ./... must pass"
memdoor workflow run team-solve - < task.md           # a long prompt from a file
memdoor workflow run team-solve --model qwen/qwen3.8-27b "…"
```

In the window: `/w team-solve <the task>`, or Shift+Tab to team mode and type the task.

The output is whatever the task asks for: changes to the tree (a refactor, a fix, a feature across many files), a new program, a document. The run's own files never mix with the project's: they are in `.memdoor/runs/team-solve-<run>/`, out of git.

**The task.** The prompt goes into the workflow's task template (`task.template.md`) as `task.md` in the run's folder. `plan` settles the rest of it: the Deliverable, the Check (one command that exits 0 when the work is right, such as `go test ./...`; or the samples; or the judge; and the word `reproduce` when the prompt describes behaviour that is wrong), the Scope (the files and directories the work may touch, in backticks) and Done when. Every later role works from that file.

**First the single call, then the team if it is needed.** The run starts the way one agent turn would, and spends the team only when that turn's attempt does not pass:

```
problem ─ survey ─┬─ paths ─ direct (3 to 5 attempts at once, each in its own worktree) ─ pick ─┬─ test-0 ─┬─ (passed) ──────────────────────────────┬─ test ─ review ─ review-rule ─ review-score ─ repair ─ recheck ─ judge ─ judge-rule ─ judge-score ─ keep ─ verify
                  └─ outline ─ plan ─ settle ─ baseline ─ reproduce ───────────────────────────────────┘          └─ (failed) context ─ ideate-a ‖ ideate-b ─ solve ┘
```

- `survey`, one read-only agent turn before anything else, lists every code path the task's behaviour goes through, the siblings included (a second function that formats the same output, an older and a newer interface), in `survey.md`. The attempts read it, and `outline` carries it to the planner, whose Scope then covers those files. On SWE-bench Verified the reference fix needed a file outside the planner's Scope in 32 of 100 unresolved instances, and in 23 of them no step ever touched it.
- `paths`, one agent turn after the survey and before any attempt, writes one check per code path the survey lists where the task's expected behaviour applies, in `paths/` in the run's folder: each fails until that path behaves as the task asks, and `done.sh paths` sets aside one that passes on the tree as found. They are written by a turn that does not fix, from the survey, so they are no attempt's own: `cross.sh` scores every attempt on every one, and `check.sh` runs them beside the Check, so a change that fixed the example and left a sibling fails test-0 and the team takes over. The attempts' own checks are written from the issue's one example, and a wrong attempt passed its own and its siblings' (24 of 28 on SWE-bench Verified).
- `direct` is a fan-out (mario's `fanout:`): single-call attempts on the prompt as written, each one agent turn in a git worktree of its own made from the tree as the run found it, so they run at once and none sees another's change. Three start together; `cross.sh`, the fan-out's `between`, then runs every attempt's check on every other attempt's change in a scratch worktree, and while the top of that matrix is a tie on checks that discriminate, one more attempt runs, five at most (while it says tie, a decision model reads the matrix against `until` and may stop sooner; it never runs more than the matrix would). Each attempt's patch is `attempts/<n>.patch` in the run's folder. Each attempt also writes a check in the run's folder, one executable that fails on the tree as found and passes with its change. `pick` runs every attempt's check on every other attempt's change and puts the change with the most passes in the tree; a tie goes to the smaller change; an attempt never scores on its own check; with no check, the smallest change stands. Offline on SWE-bench Verified, one of three attempts was right on 85% of the instances against 78% for one, and where the checks told the attempts apart the pick was right 11 times of 12 (#159). At the same time `outline` gathers the project's shape and `plan`, one model call, settles the Check and the Scope; `settle` writes them into `task.md` (the Prompt untouched) and fills a missing one safely: the Check the prompt names, else the judge; the Scope from the paths the prompt names. `baseline` runs the Check on a clean copy of the tree.
- `reproduce`, when the Check names it: one fresh turn writes `repro` in the run's folder, one executable that exits non-zero while the behaviour the prompt describes is present and 0 once it is as the prompt expects, in whatever the project runs. Its target proves it on the clean copy: a reproduction that passes there reproduces nothing. `check.sh` runs it beside the Check's command from then on, and the work is right only when both pass: the project's tests were written before the bug and pass around it (87 of 107 wrong attempts on SWE-bench Verified had passed their Check, #150). The judge is the guard against a reproduction that misreads the prompt: `reproduction: wrong` in its verdict discards it, and the Check is the command alone. Nothing names a language or a test framework: the harness reads one exit code.
- `test-0` runs the Check and the Scope (`scope.sh`: every changed file and the lines it added and removed, a file outside the Scope fails) on the pick. When it passes, nothing more is built: `review` checks it.
- When it does not, the team takes over from it: `context` gathers `task.md`, `plan.md` and the Scope's files; `ideate-a` and `ideate-b` (one file, `ideate.yaml`, `each: [a, b]`), one model call each with no tools, propose two approaches side by side; `solve` corrects the attempt from them.
- `test` runs the Check and the Scope on the work; `review`, one model call on the task, the diff, the Check's output and the Scope report, writes `verdict: PASS` or `verdict: FAIL` with repair instructions; `review-score`, a decision task (#160), scores the findings and the Check's output: how likely the change does what the Prompt asks; `repair`, one fresh agent turn, runs only on a FAIL (or no verdict with a failing Check); `recheck` takes the diff again; `judge`, one model call that has seen none of the team's reasoning, decides what a command cannot (no Check, a repaired change, a review that did not pass); `judge-score` scores its reasons with the final Check the same way; `verify` fails the run on a failing Check, a file outside the Scope, or a FAIL from the judge.

The planner, the ideators, the reviewer and the judge are model calls with no tools (`type: llm`): they read what the commands gathered and answer. Only the three attempts, `solve` and `repair` are agent turns that change the tree. Every role starts from nothing and reads only the files on disk; no role keeps a rule with the notes tool (it writes the project's AGENTS.md).

**Measured** (2026-11-05, #93; deepseek/deepseek-v4.1-flash; five real tasks, each in a clean worktree, each judged by an independent check written before the runs): the team that ran every role as an agent took 8 to 10 minutes a task; this one took 31 s to 6 minutes and passed all five, the single call's time when its attempt passes, with the Check, the Scope and the review on top. On the open-ended task (review a commit) the single call passed 2 runs of 4: twice it read for 11 and 15 minutes and wrote nothing, once it checked out the parent commit and left the tree there; this team passed 2 of 2.

**A step is done when its file is.** An llm step's target is the file its answer goes to; an agent step's target is `done.sh <step>`, copied into the run's folder, so the step's own turn cannot edit it:

| step | done when |
|---|---|
| `solve` | `changes.diff` is not empty |
| `repair` | `repair.md` is written |

`verdict.sh` reads a review's or a judge's verdict (PASS, FAIL, or none when the call narrated instead of answering).

`review-score` and `judge-score` are decision tasks (#160): each writes `{"p": …, "threshold": …, "pass": …, "by": …}`, the decision model's score of the verdict's evidence (`"by": "decision"`). `review-rule` and `judge-rule` are the branch without a model, command tasks of the DAG that read the verdict the deterministic way (PASS, or no verdict with a passing Check); a score task names its rule as `else:` and passes its answer through when no decision model answers (`"by": "review-rule"`). `repair`, `keep` and `verify` read `pass` and nothing else.

A turn that ends with its target failing is told what is missing and goes on. A step stopped at its clock with its target there is done: an attempt's target is a changed tree, so one cut at 30 minutes is still saved and judged. A step that never meets its target fails, and the run goes on without it: the model calls (`plan`, `ideate`, `review`, `judge`) and the agent turns (the three attempts, `solve`, `repair`) are `optional`, so `settle` fills a missing plan safely, a missing verdict is none, and `keep` compares the tree to the direct attempt and puts it back when nothing proved better. The commands that are the harness (`test-0`, `test`, `recheck`, `keep`, `verify`) are not optional. On SWE-bench Verified, 23 runs of 500 had ended on a step's clock, 13 of them on a tree nothing had checked (#151).

**Without a prompt** a `task.md` in the folder is the prompt, and an empty folder gets the sample problem: `task.md` and five `tests/N.in` / `tests/N.out`, checked by `check-samples.sh`.

**The model.** `--model <id>` puts every step on one model, an id `memdoor model search` shows; without it, the `/model` pin, else the agent's ladder. With no key, `memdoor connect local` connects Ollama, llama.cpp, vLLM or LM Studio on this machine ([how](https://memdoor.ai/docs/providers#a-local-runtime-no-key)); Ollama needs a 32K window (`OLLAMA_CONTEXT_LENGTH=32768`).

**The time.** On a hosted model a run is minutes. On a model on this machine every step is slower: the agent steps allow 40 minutes, solve and repair 90, and a run takes hours, as a background job (`--timeout` raises the run's clock, 2 hours by default).

**The score.** The method is [GVS5H](https://github.com/slee-persis/GVS5H)'s. The published benchmark, 9.8 of 10 on the first ten LiveCodeBench Hard problems against Fable 5's 8.0, was run with the authors' exact prompts, in the steps kept at [examples/workflows/team-solve](../../examples/workflows/team-solve/). This built-in's prompts differ (a task from a prompt, a target on every step), so it has no benchmark score of its own yet; its runs are under [examples/workflows/team-solve/built-in](../../examples/workflows/team-solve/built-in/).

Needs bash and awk, git for a change to a repository, and Python 3 only for the samples.
