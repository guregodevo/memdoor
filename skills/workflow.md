# Workflow (build a DAG of agent tasks from what the person wants, then run it)

Use this skill when the person asks for steps that depend on each other —
"run the tests, then write the notes, then wait for my approval, then
announce" — or for several things done in parallel, or for a gate a person
opens. You turn the sentence into task files, run them with the `workflow`
tool, and the DAG draws itself in the conversation as it runs. The person
never writes a file and never needs to know the format: that is your job.

## What a workflow is

A directory of task files, one file per task:

    .memdoor/workflows/<name>/steps/<task>.yaml

The directory name is the workflow's name; `steps` is a group (any word);
the file name is the task's name. Every file:

```yaml
type: agent
prompt: Run `go test ./...` and answer the ok/FAIL line.   # the WORK, as a message to an agent
agent: coder            # optional: who runs it (coder, verifier, runner …) — default coder
model: groq:qwen/qwen3.8-27b   # optional: which model answers, as /model names them (also on llm tasks)
requires:               # optional: the tasks this one waits for, by name
  - table_pattern: vet
  - table_pattern: approve
    external: true      # made OUTSIDE the workflow (a person) — see gates
target:                 # optional: what PROVES it done
  file: RELEASE_NOTES.md            # a file that must exist under the project, OR
  command: go test ./pkg/workflow/  # a command that must exit 0 there
timeout: 10m            # optional, per task
max_retries: 1          # optional: tries again on failure, this many times
args:                   # optional: values the prompt can use as {{.version}}
  version: v1.2.0
```

Only these keys. Anything else is refused with the file and the key named.

## Three kinds of task

`type:` says which; the factory behind it builds the task. Mix them freely
in one workflow.

- **`agent`** — a turn of a Memdoor agent: `prompt` (+ `agent`). For work
  that needs reading, editing, running, judgement.
- **`command`** — a shell line, `command: git log --oneline -8`, run in the
  project; its stdout is kept as the output. For anything deterministic: a
  build, a fetch, a script, a page through `memdoor chrome run`. Cheaper and
  surer than asking an agent to run it.
- **`llm`** — one model call, no tools, no loop: `prompt` (+ `model`); the
  answer is kept. For summarizing, classifying, rewriting what an upstream
  task produced. **A prompt reads an upstream output with
  `{{ output "log" }}`** — the kept output of the required task `log`,
  inlined. Without it the model sees only your sentence.

Prefer `command` when a shell line does it, `llm` when one answer does it,
`agent` only when the step needs hands.

```yaml
# .memdoor/workflows/digest/steps/log.yaml
type: command
command: git log --oneline -10

# .memdoor/workflows/digest/steps/summary.yaml
type: llm
prompt: |
  Summarize these commits in three bullets for a changelog:
  {{ output "log" }}
requires:
  - table_pattern: log
```

## The rules that matter

1. **Prompt = the work, target = the proof.** A target is a CHECK of
   something the task PRODUCES, never the task itself: a target that holds
   before the run means "already done" and the task is skipped without
   running. So a check (`go vet`, `go test`, `gofmt -l`) is a `command`
   task with NO target — its exit code is the proof, and it runs on every
   run (live 2026-10-02: `vet` with `target: {command: go vet ./...}` was
   skipped on a fresh run because the tree already vetted). A task with no
   `target` keeps its answer as its proof (the tasks after it can read it).
   **A file target is one file, done once it exists on ANY run** — a fresh
   run finds last run's `DIGEST.md` and skips the task. When each run must
   make its own, put the partition in the name: `target: {file:
   "DIGEST-{{.partition}}.md"}` (and write that name in the prompt), or
   leave the target off and let the answer be the proof.
2. **Dependencies are `requires`, by task name.** Tasks nothing depends on
   each other run **in parallel**; a task runs once everything it requires
   is done. Order comes from the graph, never from a sleep.
3. **A gate is an external requirement — NOT a task file.** When the person
   must approve, do not write `approve.yaml`. On the task that must wait,
   add `- table_pattern: approve` with `external: true`. The run builds
   everything else, stops *waiting* at the gate, and the person opens it
   with `a` in `/workflow` or `memdoor workflow approve <run> approve`.
   A file for an external task is refused ("has a definition … delete it").
4. **A run is fresh**: every `run` runs every task. To continue an
   earlier run instead — one that failed and you fixed the cause, one that
   was stopped — `resume` it by its run id: what it finished is skipped,
   what it did not runs. A run waiting at a gate is approved, not resumed.
   **The partition is the identity of what the DAG is for**, and the kind
   of workflow says which — pass it as `partition` on `run`:
   - CI on a commit → the commit (`git rev-parse --short HEAD`): the same
     commit resumes, a new commit runs everything;
   - a nightly or daily digest → `today`: a later trigger finishes the day;
   - a release → the version (`v1.2.0`): approval resumes that release;
   - a PR review → the PR number;
   - "run it" with none of these → nothing: the run's own stamp.
   Prompts can read it as `{{.partition}}`.
5. **A workflow can fail**; that is it working. A task that cannot meet its
   target after its retries reads `✗` with the reason and what depends on
   it stays `waiting`. Report it; do not paper over it with a looser target.

## A README beside the steps

Every workflow gets `README.md` next to its `steps/` — one paragraph a
stranger can read: what it does, what it needs (`args`, the partition, an
approval), what it leaves behind, and how to run it (`/workflow:<name>`).
Its first paragraph is what `/workflow` and `memdoor workflow` show beside
the name. A workflow is shared by copying its directory — the README is
how the next person knows what they got.

## How you do it

1. Name the workflow from the ask (`release-check`, `nightly`, `fanout`),
   and write its README.md first (above).
2. One file per step the person named. Independent steps get no
   `requires`; dependent ones name what they need; the person's approval
   is an external requirement on the step after it (rule 3).
3. A step that produces a file gets a `target` naming it; a check is a
   `command` with no target; the rest answer.
4. `workflow(action: "run", name: "<name>")` — the TOOL, never `memdoor
   workflow run` in bash: a run started from the shell has no window and
   draws nothing. If the tool refuses the run, say so in its words and
   stop; never try the shell instead. Then stop. The run goes on
   in the background and draws itself in the conversation; the person
   watches it. If they asked you to follow it, `workflow(action:
   "status", run_id: …)` or a `cron` every 30 s, never a sleep.
5. If the run is refused, the error names the file and the key: fix that
   file and run again.
6. Never stop a run the person did not ask you to stop. A run that is
   still going is not a problem; a run that failed reports why.
7. **"Nightly", "every morning", "each Monday"** — Memdoor schedules it, in
   bash from the project directory:

       memdoor cron add --id <name> --schedule "0 6 * * *" --workflow <name> --partition today

   Never launchd, crontab or a wrapper script: the gateway runs the
   workflow on that schedule in this project, and the run shows in
   `/workflow` like any other. Run it once with the tool first, so the
   person sees it work before it runs unattended. (The `cron` tool is for
   re-checking something a few times, not for a standing schedule.)

The directory can be committed (a repo's `.memdoor/workflows/`), written
for one run anywhere and passed to `run` as a path, or kept in the person's
**library, `~/.memdoor/workflows/<name>/`**, where it runs in any project
(the project's own of the same name wins). When the person says "keep this
one" or "for every project", write it there.

## Example: "vet and test in parallel, then release notes, then my approval, then announce"

```
.memdoor/workflows/release-check/steps/vet.yaml
  type: command
  command: go vet ./...

.memdoor/workflows/release-check/steps/tests.yaml
  type: command
  command: go test ./...

.memdoor/workflows/release-check/steps/notes.yaml
  type: agent
  prompt: From `git log --oneline -8` write RELEASE_NOTES.md, one bullet per commit.
  requires: [{table_pattern: vet}, {table_pattern: tests}]
  target: {file: RELEASE_NOTES.md}

.memdoor/workflows/release-check/steps/announce.yaml
  type: agent
  prompt: Answer the first bullet of RELEASE_NOTES.md.
  requires: [{table_pattern: notes}, {table_pattern: approve, external: true}]
```

Four files, no `approve.yaml`. `vet` and `tests` run together; `notes`
after both; the run waits at `approve`; `announce` runs once the person
approves.
