# Workflows

A workflow is the deterministic harness: a graph of tasks, each proven done
by something you can check. Say the steps; the coder builds one and runs
it. `/w <name> <the task>` runs one yourself (`/w team-solve <the task>`).
Independent tasks run in parallel. The graph is drawn in the window.
Free on your machine, schedules included: `memdoor cron add --id nightly
--schedule "0 7 * * *" --workflow <name> --partition today` runs it in this
project every morning. Hosted gateway: run state on memdoor.ai, one table
per workspace, so a workflow there can wait on a task another produced,
from any machine. Runs with the laptop closed: `memdoor gateway install`
on [a box that stays on](/docs/gateway).

```
I want a digest workflow: list the last 10 commits, then summarize them in
three bullets, then have an agent write them into DIGEST.md. Run it.
```

The coder writes three task files, calls its `workflow` tool, and the
window draws the graph:

```
  digest · running · 2/3 done
  ✓ log      done 0s
  │
  ✓ summary  done 12s  ← log
  │
  ▶ write    running 16s  ← summary
      ⏺ apply_patch
```

A running task shows elapsed time and the tool it is calling. The run ends
with `■ digest done`. You write no file.

![A chain: command → llm → agent, from one sentence](/workflow-digest.cast)

## What a task is

One YAML file per task under `.memdoor/workflows/<name>/`. The
`.memdoor/.gitignore` Memdoor writes ignores `runs/`, `runs.db`,
`worktrees/` and this machine's `workspace` marker, never `workflows/`. A
file declares `type` and `requires`; the engine orders the graph from the
requirements.

Three kinds, mixed freely:

| type | does | its proof |
|---|---|---|
| `agent` | a turn of a Memdoor agent — reads, edits, runs, judges | its target, or its answer |
| `command` | a shell line in your project (`git log`, a build, a fetch) | its target, or its stdout |
| `llm` | one model call, no tools — summarize, classify, rewrite | its target, or the answer |

Done: the target exists, a file or a command that exits 0. The model's
claim does not count. No target: the answer is the proof, and the next
task reads it as `{{ output "log" }}` in its prompt.

A task cannot change its own check. Programs a target command runs from
the project (`./check.sh`, `sh scripts/verify.sh`) are snapshotted at task
start; a write to one is refused with the reason; one changed by other
means is restored after the tool call and before the check. Origin: a run
whose check could never pass, where on the third try the agent rewrote the
script to exit 0. What the check depends on (the tests `go test` runs, a
Makefile recipe) remains the task's work.

## Keeping one

Commit `.memdoor/workflows/<name>/`: the team runs it.
`~/.memdoor/workflows/`: the library, any project, in the dropdown, run by
`/w <name>`; a project's workflow of the same name wins. `args`: one
workflow, many asks (`{{.version}}`). Partition: what a run is for
(`{{.partition}}`). `README.md` beside the steps: what it does, needs, how
to run; its first paragraph is what `/workflow` shows beside the name. To
share: the directory, by `cp -R`, a gist, or a repo cloned into
`~/.memdoor/workflows/<name>`.

## Gates, failures, re-runs

- **Approval is a task nobody runs.** A step can require `approve` marked
  `external`. The run builds everything else and waits. `a` on it in
  `/workflow`, or `memdoor workflow approve <run> approve`; the same run
  continues.

![A gate: two tasks at once, a merge, the run waiting for a person, going on](/workflow-release.cast)
- **Send it back.** At a gate the run shows its commits and files. `d`:
  every changed line (`memdoor workflow diff <run>` from the shell).
  `/workflow changes <step> <what to change>`: rerun that step with the
  comment and the steps after it, back to the same gate. CLI: `memdoor
  workflow changes <run> <step> "<comment>"`.

![The review loop: a GitHub issue reproduced, fixed, sent back with one comment, approved, and the PR opened](/review-loop.cast)
- **Failure.** A task that cannot meet its target after its retries reads
  `✗` with the reason; what depends on it waits. Fix the cause, resume:
  finished tasks are skipped.
- **Every run is fresh**, with its own partition; every task runs. Resume
  continues one run. The partition is the thing the run is about (a
  commit, a day, a release): the same commit resumes, the next reruns.

## In the TUI

`/w`: the project's workflows, then its runs. Enter on a workflow
runs it; on a run, opens its graph. `/w <name>` runs one, and the dropdown
completes the name; the words after it are the task, for a workflow that
reads one (`/w team-solve <the task>`), and `--partition <p>` names
the partition. In the graph: `a` approves, `d` the full diff, `c`
continues a failed or stopped run, `s` twice stops one, `Esc` stops this
window's run.

![The panel: runs, and a run's graph with every task's time](/demo-panel.cast)

## From the shell

```
memdoor workflow                       the project's workflows and runs
memdoor workflow run <name>            a fresh run
memdoor workflow run <name> --model <id>   every agent and llm step on that model
memdoor workflow run <name> "<prompt>"     the task, for a workflow that takes one (team)
memdoor workflow status <run-id>
memdoor workflow resume <run-id>       continue a failed or stopped run
memdoor workflow approve <run-id> <task>
memdoor workflow diff <run-id>         every line the run changed
memdoor workflow stop <run-id>
```

The model: `--model` names it for the run, an id `memdoor model search`
shows, `provider:id` when two providers list it. Without it, the person's
`/model` pin, else each agent's ladder. A step with a `model:` of its own
keeps it.

## Bigger ones

Fan-out is requirements: five tasks that each require `fetch` run at once;
one that requires all five waits. Research on the top Hacker News stories:
a fetch step, one read-and-summarize step per story in parallel, a merge.
Each summary quotes the page it read; the proof is the file.

![A fan-out: one fetch, five readers at once, one merge](/workflow-hn.cast)

A team is requirements too. [GVS5H](https://github.com/slee-persis/GVS5H):
five fresh contexts of one open model matching a frontier model on hard
coding problems, communicating only through files (plan, notes, current
solution). As tasks: a planner, two ideators in parallel, a solver, the
samples run by a `command` step, a manager reading the captured result,
one repair round, a final check that alone decides. In the repo:
[`examples/workflows/team-solve`](https://github.com/guregodevo/memdoor/tree/main/examples/workflows/team-solve),
with the problem it ran on and what the run found. First ten problems of
the paper's list, five passes: 9.8 of 10 against the authors' 9.6 and Fable
5's 8.0 in one call: [the result](/docs/team-solve). The paper's 100
hardest LiveCodeBench problems, Qwen3.8-27B with thinking off: the team 47,
one call 35. Harness, per-problem tables, caveats: the example's `bench/`
folder.

![A team on a ledger: a planner, two ideators in parallel, a solver, the samples, a manager, a repair round, the final check](/team-solve.cast)


Memdoor releases itself with a workflow, reviews its commits with one,
tests its terminal with one. All three are in its repo:

![The real terminal, tested: a tmux pane runs the TUI, types a prompt, and the assertion is on the file the turn made](/workflow-tui-check.cast)

Engine: [mario](https://github.com/guregodevo/mario), an open DAG runner
in Go. Memdoor adds the task types, the proofs and the live graph.
