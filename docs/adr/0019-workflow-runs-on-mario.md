# ADR 0019 — Workflow runs: mario is the DAG engine, a target is what "done" means

Status: ACCEPTED (2026-10-02, proven live, deployed as `v1.360.0-wiki` —
`../features/WORKFLOWS.md`) ·
Affects `pkg/workflow`, `gateway/workflow_runs.go`, the TUI (`/workflow`),
`memdoor workflow` · Depends on
[`github.com/guregodevo/mario`](https://github.com/guregodevo/mario) (public,
Go 1.23), imported as a library and changed upstream — never copied

## Context

Greg, 2026-10-01: "in my repos in gh i built a workflow framework that works
locally. We need to revisit it for workflow runs… you will take care of a dag
run, a dag of tasks that is orchestrated… the workflow can outcompete any
coding agents." 2026-10-02: "i think we can use it for workflow use case for
the TUI."

Read in full (39 files, 4,038 lines; `go build` and `go test ./...` clean —
no-cycle, no-deadlock and random-DAG tests on both repositories):

- A task is a `Workflow` (name, version, max retries, `IsExternal`) with
  per-partition instances; a `WorkflowExecution` records id, partition,
  retries, start/end, status, error and parameters.
- A `TaskFactory` supplies, per task name, the `RunFunc` and a `DataEndpoint`;
  an `Executor` runs `RunFunc`; a `WorkflowRepository` (static in memory, or
  SQLite) holds lineage — `Upstreams`/`Downstreams`, deep both ways — and
  execution history; a `LocalScheduler` with two queues (tasks, retries)
  runs the graph concurrently with `backfill` and `loop` modes.
- **Readiness is Luigi's rule**: a task may start when every upstream is
  `Done` **or its target `Exists()`**. A dependency marked `external: true`
  is waited for but never run — somebody else completes it.
- The YAML loader (`templates.Walk` → `factory.BuildDAG`) is one file per
  task at `<root>/<project>/<dataset>/<table>.yaml`, shaped for BigQuery —
  but it already carries `prompt`, `model`, `tools`, `requires`, `args`.
  No test runs that path end to end.

What Memdoor has that mario does not: a bounded agent turn (the cron tool's
discipline — a workdir, an agent, a count, a visible answer), a queue that
serialises per session, and a window to watch it in. What mario has that
Memdoor does not, and that the hooks research (`../internal/HOOKS_AND_CRON_2026_10.md`
§3) says every scheduled agent lacks: **a deterministic notion of done**.
"A green status means the session started and exited without an
infrastructure error. It does not mean the task succeeded." A target that
exists means it did.

## Decision

1. **mario is the engine, imported as a library, and mario changes to fit.**
   Greg, 2026-10-02: "we can change mario and import mario lib", "let's not
   copy it". Memdoor depends on `github.com/guregodevo/mario` at a pinned
   version and never copies it; what Memdoor's use needs lands **in mario**,
   as general engine features: a context on a run (cancellation, a per-node
   timeout), events a caller can watch (a node started, finished, failed,
   retried), instances passed by pointer (an instance carries a mutex, and a
   copied mutex is a different lock — `go vet` refuses it), a logger
   interface instead of a global coloured one, a factory lookup that answers
   "not found" instead of exiting the process, and a single-file loader with
   a task factory per `type`. Memdoor's `pkg/workflow` is then thin: the
   agent task type, the file and command targets, and the callbacks the TUI
   watches, declared as interfaces in the consumer (duck typing, the rule
   for every callback out of `pkg/`). Memdoor's SQLite driver is
   `modernc.org/sqlite`; mario's `sqlite` package (mattn) is not imported,
   so runs persist in Memdoor's own store.

2. **A node is one bounded agent turn, and its target is a check.** Memdoor's
   `TaskFactory` builds, per task: a `RunFunc` that runs **one gateway turn**
   — an agent, a prompt, the run's workdir, a timeout and a retry count — and
   a `DataEndpoint` whose `Exists()` is the task's **verifiable target**: a
   file that must exist, a command that must exit 0 (`go test ./...`, a
   `curl` against a health endpoint), or a run record in Memdoor's store. The
   model's prose never marks a node done; the target does. That is the whole
   claim behind "outcompete": every step is proven before the next starts,
   and a re-run skips what already exists.

3. **mario's YAML, in mario's layout, loaded by mario.** Amended the same
   day: the first draft had a single `workflows/<name>.yaml` of Memdoor's
   own and a loader for it. Greg: "why you create mario tasks in golang? You
   should do it in yaml files … everything is already done in yaml … no need
   to reinvent the wheel … remove your boilerplate … if you miss anything
   change it in mario or mario-llm." So a workflow is a **directory** of
   task files, one per task, at `<workflow>/<group>/<task>.yaml` — the
   project / dataset / table tree `templates.Walk` reads and
   `factory.BuildDAG` wires — and Memdoor registers one task **type**,
   `agent`, with its own JSON schema, exactly as `mario-llm` registers
   `llm_agent`:

   ```yaml
   # workflows/release/steps/release.yaml
   type: agent
   prompt: Write RELEASE.md with "{{.version}} released" and the date {{.partition}}, then commit.
   args: {version: v0.1.0}
   requires:
     - table_pattern: changelog
     - table_pattern: approve
       external: true
   target: {file: RELEASE.md}
   ```

   What the agent type needed that mario's definition lacked went into
   mario: `agent`, `target {file|command}`, requirement defaults. The
   directory can live in a repo or be written for one run by a person or an
   agent: "the yaml can be created temporarily, or you can ask an agent to
   build the yaml files; it can be in a repo also and from a repo you run
   the workflow."

   **External is mario's meaning, exactly.** Greg: "external tasks are not
   triggerable … unlike standard tasks that are triggered by workflow." An
   external task is a leaf made outside the workflow — the workflow never
   runs it, only checks its target, and it requires nothing. A run that
   reaches a missing external target builds everything else and ends
   **waiting**; approval writes the target and triggers the same run again,
   and what exists is skipped. (The first live run, with the gate in the
   middle of the chain, did nothing: mario skips an external and never
   builds what it requires. A poll-the-target workaround was written and
   removed the same hour — it was not mario's semantics.)

4. **The run is visible, in the TUI.** `/workflow run release` starts it from
   the project the TUI was launched in; every node's turn answers into that
   window the way a cron run does (a `▶ release.tests —` note on start, its
   answer on finish, `✓`/`✗` from the target check); `/workflow` lists runs
   with each node's status; `/workflow approve <task>` completes an external
   node; `/workflow stop <run>` cancels. A run nobody can see is a run that
   did not happen.

5. **Bounded, like the cron tool.** Per node: a timeout and a retry count
   (mario's `MaxRetries` through its retry queue); per run: a node cap and an
   expiry; no two runs of one workflow at once in one project. The mechanism
   that makes a loop safe is a counter, not a better prompt.

6. **Task types are mario's; Memdoor adds one.** Greg, 2026-10-02: "mario
   is an orchestrator with different task types, with clean interface …
   the yaml defines the type and behind the factory builds it … minimum
   changes in memdoor, max in mario … never use concrete type". mario's
   `tasks` package: `Base` (everything the engine needs from a definition;
   a type is one `Run` function), `Outputs` (every task's output at
   `<dir>/<task>/<partition>`, the proof when no target is named, where an
   external's maker writes, and what `{{ output "name" }}` reads),
   `command` (a shell line), `llm` (one model call through a `Completer`
   the host provides), `Mixed` (routes a DAG by each file's type), and
   `factory.NewValidator` (the registered types validate their own files).
   Memdoor registers `agent` (a turn through its queue) beside them and
   hands in its model client as the `Completer`. Events are a channel the
   engine owns and closes, read in order by `pkg/workflow` and handed to
   the host's callbacks — mario knows no host.

## Not deciding yet

- Partitions by branch or PR: today the partition is the person's day (the
  same day resumes, a new day starts over); anything finer is a later
  question.
- `llm_agent` from mario-llm as a second type in the same DAG, run on
  Memdoor's models through the gateway: next, `../roadmap/MUST.md` item 0.
- Cross-workflow dependencies (mario's targets across DAGs): not until a
  second workflow exists.

## Consequences

- One engine for scheduled work: the cron tool's jobs stay as they are;
  a workflow is a DAG of the same bounded turns, gated by targets.
- Proven 2026-10-02 on a scratch project: tests (skipped, its command
  already passed) → changelog (ran) → waiting at approve → `a` → release
  (ran, RELEASE.md + a commit), 4/4 done, watched in the TUI; then a task
  that cannot meet its target (two turns, failed, downstream waiting), the
  run clock, an ad-hoc DAG by path, a same-day resume (all skipped, no
  turn), stop while waiting and while running, a retry that succeeds, a
  cross-workflow requirement through the sibling's target, the coder's
  `workflow` tool (list/run/status/stop/approve — "the coder agent can stop
  a workflow … gracefully") and Esc as the window's stop. Receipts in
  `../features/WORKFLOWS.md`. Greg, the same day: "let's not change too much
  mario design or add extra logic. A dag is a dag and the orchestration is
  handled by mario" — the tool and Esc call the gateway's own run/stop, and
  nothing orchestrates outside mario. Then: "you are rewriting mario in
  memdoor, this is wrong … use callback and duck typing … workflow in
  memdoor just uses mario's factory interface and passes interface
  callbacks … make sure the memdoor logger is passed in" — the gateway's
  per-task state was deleted; every state is read from mario's repository
  (`Execution.Status()`), the host's logger is handed in with the run's
  options, and what mario got wrong (a scheduled execution stamped with a
  start date) was fixed in mario (`76e4e83`).
- mario gained an end-to-end user it never had, and with it: a context on a
  run, events, a logger interface, errors instead of exits, `agent`/`target`
  on a task, requirement defaults, walks without templates, and Luigi's
  rule that an existing target is complete (branch `workflow-engine`).
