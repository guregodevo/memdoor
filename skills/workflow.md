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
model: groq:qwen/qwen3.8-27b   # optional: which model answers, as /model names them (also on llm tasks); after it: price|throughput|latency, `order a,b`, `thinking off|low|medium|high`, `max_tokens N`;
                        # a host preference after it as /model takes one: `qwen/qwen3.8-27b throughput`, `… order cerebras,deepinfra`,
                        # and `thinking off|low|medium|high` for the step's reasoning (off = no thinking: answers within a host's output cap)
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
6. **The tasks ARE the orchestration.** A harness, a pipeline, a team of
   models: every role is a task (`llm` or `agent`) whose `model:` line routes
   the call through the gateway, and the ledger the roles share is files on
   disk, passed by `requires` and read by the next prompt (`{{ output "x" }}`,
   or the file by name). Never a Python or shell orchestrator, never a
   script that calls a model API itself; the only code is a `command` task
   that checks or lands something (runs the samples, builds, writes a reply
   to its file). A loop is unrolled: round 1, round 2, … to its bound, and
   a round whose predecessor already finished does nothing but pass the
   files on. (2026-10-26: asked to reproduce a published five-agent
   harness, the first two builds wrote a pipeline.py and asked how to call
   OpenRouter.)
7. **Decide, do not ask, when nobody is at the keyboard.** A build started
   by `memdoor run`, a cron, a peer or another workflow has no one to answer
   a picker; rule 6 answers the usual questions (through the gateway, files
   for the ledger, a command task for a check). Ask in a window only, and
   only when the answers would lead to materially different workflows.
8. **A condition is a check, never an instruction to the model.** "Do
   nothing after done", "skip when the tests passed", "keep the previous
   answer": a model told to do nothing still answers, and a write task that
   saves whatever came back saves that answer over the good one (2026-10-27:
   a harness solved its problem in round 1, the manager said done in round
   2, and rounds 3 to 10 overwrote the program twice because the pass-through
   lived in the prompt). Enforce it where the files are written: the
   `command` task that lands a reply reads the ledger first and declines
   (`if grep -qsx done status-*.txt; then exit 0; fi`), a target that already
   exists skips the task, a sticky state is copied forward by the command,
   not re-decided by the model. The model's task is to answer; the DAG's task
   is to decide whether the answer counts. **A task that must not even run
   carries `when:`**, a shell line checked just before it in the project
   directory; false and the task is skipped as done with an empty answer,
   nothing called (`when: '! grep -qsx done status-*.txt'` on every round
   after the first: a finished loop costs nothing more). A loop has a bound AND a stop
   rule, both mechanical: the bound is the number of rounds unrolled, the
   stop rule a check in the command (the manager named the same task twice:
   no progress, mark the loop stopped and let every later round decline,
   the way GVS5H's harness stops on a reissued task). Max retries on a task
   bound one failing call; they do not bound a loop that keeps "succeeding".
9. **A reply is landed by a parser that survives the last section.** Cut a
   section to the next header OR the end of the reply (`awk
   '/^### STATUS/{f=1;next} /^### /{f=0} f'`), never with a range that needs a
   header after it (`sed -n '/^### STATUS/,/^### /p' | sed '$d'` dropped the
   word "done" when STATUS was the final section, and the loop never knew it
   had finished). Before the run, feed the parser a reply that ends at each
   section and read back what it wrote.

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
