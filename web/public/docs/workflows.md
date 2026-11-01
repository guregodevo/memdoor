# Workflows

A workflow is the deterministic harness: a graph of tasks, each proven done
by something you can check. Say what you want done in steps and the coder
builds one and runs it. Independent tasks run in parallel. The graph is
drawn in your window as it runs. Workflows are free on your machine,
including on a schedule with your local cron: `memdoor cron add --id nightly
--schedule "0 7 * * *" --workflow <name> --partition today` runs it in this
project every morning. The hosted gateway keeps run state on memdoor.ai, one
table per workspace shared by every project and person on it, so a workflow
there can wait on a task another workflow produced, from any machine. For
runs with your laptop closed: `memdoor gateway install` on [a box that stays
on](/docs/gateway).

```
I want a digest workflow: list the last 10 commits, then summarize them in
three bullets, then have an agent write them into DIGEST.md. Run it.
```

The coder writes three task files, calls its `workflow` tool, and your
window shows the graph filling in:

```
  digest · running · 2/3 done
  ✓ log      done 0s
  │
  ✓ summary  done 12s  ← log
  │
  ▶ write    running 16s  ← summary
      ⏺ apply_patch
```

A running task shows its elapsed time and the tool it is calling. The run
ends with one line, `■ digest done`. You write no file.

![A chain: command → llm → agent, from one sentence](/workflow-digest.cast)

## What a task is

One YAML file per task, in your project under `.memdoor/workflows/<name>/`. The `.memdoor/.gitignore` Memdoor writes ignores only the
run outputs `runs/`, the run table `runs.db`, `worktrees/` and this machine's
`workspace` marker, never `workflows/`. The file declares its `type` and
what it `requires`. The engine orders the graph from the requirements and
runs independent tasks in parallel.

Three kinds of task, mixed freely:

| type | does | its proof |
|---|---|---|
| `agent` | a turn of a Memdoor agent — reads, edits, runs, judges | its target, or its answer |
| `command` | a shell line in your project (`git log`, a build, a fetch) | its target, or its stdout |
| `llm` | one model call, no tools — summarize, classify, rewrite | its target, or the answer |

A task is done when its target exists: a file, or a command that exits 0.
The model's own claim does not count. A task with no target keeps its answer
as the proof and the next task reads it:
`{{ output "log" }}` in a prompt is the upstream task's answer, inlined.

A task cannot change its own check. The programs a target command runs from
the project (`./check.sh`, `sh scripts/verify.sh`) are snapshotted when the
task starts. A write to one of them is refused with the reason. One changed
by any other means is restored after the tool call and again before the
check runs. This rule came from a run whose check could never pass: on its
third try the agent rewrote the script to exit 0 and the run read done. What
the check depends on (the tests `go test` runs, a Makefile recipe) remains
the task's work.

## Keeping one

Commit `.memdoor/workflows/<name>/` and the whole team runs it. Put it in
`~/.memdoor/workflows/`, your library, and it runs in any project: the
dropdown offers it and `/workflow:<name>` runs it. A project's workflow of
the same name wins. `args` let one workflow serve many asks
(`{{.version}}`). The partition says what a run is for (`{{.partition}}`).
Each workflow carries a `README.md` beside its steps: what it does, what it
needs, how to run it. Its first paragraph is what `/workflow` shows beside
the name. To share one, share the directory: `cp -R`, a gist, or a repo
cloned into `~/.memdoor/workflows/<name>`.

## Gates, failures, re-runs

- **Your approval is a task nobody runs.** A step can require `approve`
  marked `external`. The run builds everything else and stops, waiting.
  Press `a` on it in `/workflow`, or run `memdoor workflow approve <run>
  approve`, and the same run continues.

![A gate: two tasks at once, a merge, the run waiting for a person, going on](/workflow-release.cast)
- **Or send it back.** At a gate the run shows what it changed: its
  commits and files. `d` shows every changed line (`memdoor workflow diff
  <run>` from the shell). `/workflow changes <step> <what to change>` reruns
  that step with your comment and the steps after it, back to the same
  gate. The CLI is `memdoor workflow changes <run> <step> "<comment>"`.

![The review loop: a GitHub issue reproduced, fixed, sent back with one comment, approved, and the PR opened](/review-loop.cast)
- **A workflow can fail.** A task that cannot meet its target after its
  retries reads `✗` with the reason, and what depends on it stays waiting.
  Fix the cause and resume the run: finished tasks are skipped.
- **Every run is fresh**, with its own partition, and every task runs.
  Resume continues one run. For a workflow about something (a commit, a
  day, a release) the partition is that thing: the same commit resumes, the
  next one reruns.

## In the TUI

`/workflow` opens the panel: your project's workflows, then its runs. Enter
on a workflow runs it. Enter on a run opens its graph. The dropdown
completes `/workflow:<name>` with what your project has. In the graph:
`a` approves, `d` shows the full diff, `c` continues a failed or stopped run, `s` twice stops one,
`Esc` stops the run of this window.

![The panel: runs, and a run's graph with every task's time](/demo-panel.cast)

## From the shell

```
memdoor workflow                       the project's workflows and runs
memdoor workflow run <name>            a fresh run
memdoor workflow status <run-id>
memdoor workflow resume <run-id>       continue a failed or stopped run
memdoor workflow approve <run-id> <task>
memdoor workflow diff <run-id>         every line the run changed
memdoor workflow stop <run-id>
```

## Bigger ones

Fan-out is expressed as requirements: five tasks that each require `fetch`
run at once, and one that requires all five waits for them. Asked for
research on the top Hacker News stories, the coder makes a fetch step, one
read-and-summarize step per story in parallel, and a merge. Each summary
quotes the page it read, because the proof is the file.

![A fan-out: one fetch, five readers at once, one merge](/workflow-hn.cast)

A team is requirements too. [GVS5H](https://github.com/slee-persis/GVS5H)
showed five fresh contexts of one open model matching a frontier model on
hard coding problems by communicating only through files on disk: a plan,
notes, the current solution. Described to the coder, it became eight tasks:
a planner, two ideators in parallel, a solver, the samples run by a
`command` step, a manager reading the captured result, one repair round, and
a final check that alone decides. It is in the repo as
[`examples/workflows/team-solve`](https://github.com/guregodevo/memdoor/tree/main/examples/workflows/team-solve),
with the problem it ran on and what the run found. On the first ten problems of
the paper's list, five passes each, it scored 9.8 of 10 against the authors' 9.6 and
Fable 5's 8.0 in one call: [the result](/docs/team-solve). On the paper's 100 hardest
LiveCodeBench problems against a single call of the same model, Qwen3.8-27B with
thinking off, the team solved 47 to the single call's 35. The harness, the per-problem
tables and the caveats are in the example's `bench/` folder:

![A team on a ledger: a planner, two ideators in parallel, a solver, the samples, a manager, a repair round, the final check](/team-solve.cast)


Memdoor releases itself with a workflow, reviews its commits with one,
and tests its terminal with one. All three live in its repo:

![The real terminal, tested: a tmux pane runs the TUI, types a prompt, and the assertion is on the file the turn made](/workflow-tui-check.cast)

The engine is [mario](https://github.com/guregodevo/mario), an open DAG
runner in Go. The task types, the proofs and the live graph are what
Memdoor adds.
