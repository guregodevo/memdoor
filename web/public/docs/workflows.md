# Workflows

Say what you want done in steps, and the coder builds a workflow and runs
it: a graph of tasks, each one proven done by something you can check, run
in parallel where nothing stands in the way, drawn in your window as it
goes. Workflows are free on your machine, and so is running one on a
schedule with your local cron: `memdoor cron add --id nightly --schedule
"0 7 * * *" --workflow <name> --partition today` runs it in this project
every morning, once a day. Pro keeps every run's state on memdoor.ai, one
table per workspace: every project and every person on the workspace share
it, so a workflow can wait on a task another workflow produced, from any
machine. Coming: runs with your laptop closed ([pricing](/pricing)).

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

A running task's clock ticks; the tool it is calling shows under it; the
end is one line, `■ digest done`. You never write a file.

![A chain: command → llm → agent, from one sentence](/workflow-digest.cast)

## What a task is

One YAML file per task, in your project under `.memdoor/workflows/<name>/`
(a folder the repo ignores; commit it if you want the workflow kept). The
file says its **type** and what the task **requires**; the engine runs the
graph in that order and in parallel where it can.

Three kinds of task, mixed freely:

| type | does | its proof |
|---|---|---|
| `agent` | a turn of a Memdoor agent — reads, edits, runs, judges | its target, or its answer |
| `command` | a shell line in your project (`git log`, a build, a fetch) | its target, or its stdout |
| `llm` | one model call, no tools — summarize, classify, rewrite | its target, or the answer |

A task is **done when its target exists** — a file that must be there, or
a command that must exit 0 — never because the model said so. A task with
no target keeps its answer as the proof, and the task after it reads it:
`{{ output "log" }}` in a prompt is the upstream task's answer, inlined.

## Keeping one

A workflow is files, so it is a thing you keep. Commit `.memdoor/workflows/
<name>/` and the whole team runs it. Put it in **`~/.memdoor/workflows/`**
— your own library — and it runs in **any** project: the dropdown offers it,
`/workflow:<name>` runs it, a project's own workflow of the same name wins.
`args` make one workflow serve many asks (`{{.version}}`), and the
partition says what a run is for (`{{.partition}}`). Each workflow carries
a `README.md` beside its steps — what it does, what it needs, how to run
it — and its first paragraph is what `/workflow` shows beside the name. To
share one, share the directory: `cp -R`, a gist, a repo cloned into
`~/.memdoor/workflows/<name>`.

## Gates, failures, re-runs

- **Your approval is a task nobody runs.** A step can require `approve`
  marked `external`: the run builds everything else and stops, *waiting*.
  Press `a` on it in `/workflow`, or `memdoor workflow approve <run> approve`,
  and the same run goes on.

![A gate: two tasks at once, a merge, the run waiting for a person, going on](/workflow-release.cast)
- **Or send it back.** At a gate the run shows what it changed: its
  commits and the files, and `d` puts every changed line in your window
  (`memdoor workflow diff <run>` from the shell). Not right yet? `/workflow changes <step> <what to
  change>` reruns that step with your comment, then every step after it,
  and the run comes back to the same gate for you to look again. The CLI is
  `memdoor workflow changes <run> <step> "<comment>"`.

![The review loop: a GitHub issue reproduced, fixed, sent back with one comment, approved, and the PR opened](/review-loop.cast)
- **A workflow can fail; that is it working.** A task that cannot meet its
  target after its retries reads `✗` with the reason, and what depends on
  it stays waiting. Fix the cause and **resume** the run: what it finished
  is skipped, what it did not runs.
- **Every run is fresh** — its own partition, every task runs. Resume is for
  continuing one. For a workflow that is *about* something — a commit, a
  day, a release — the partition is that thing: the same commit resumes,
  the next one reruns.

## In the TUI

`/workflow` opens the panel: your project's workflows, then its runs. Enter
on a workflow runs it; Enter on a run opens its graph. The dropdown
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

Fan-out is just requirements: five tasks that each require `fetch` run at
the same time; one that requires all five waits for them. Ask for deep
research on the top Hacker News stories and the coder makes a fetch step,
one reading-and-summarizing step per story in parallel, and a merge — and
every summary quotes the page it read, because the proof is the file, not
the sentence.

![A fan-out: one fetch, five readers at once, one merge](/workflow-hn.cast)

Memdoor releases itself with a workflow, reviews its own commits with one,
and tests its terminal with one — all three live in its repo:

![The real terminal, tested: a tmux pane runs the TUI, types a prompt, and the assertion is on the file the turn made](/workflow-tui-check.cast)

The engine is [mario](https://github.com/guregodevo/mario), an open DAG
runner in Go; the task types, the proofs and the live graph are what
Memdoor adds.
