# Workflows — a DAG of agent tasks, done when the target exists

Status: **shipped — main, deployed 2026-10-02; task types, fresh runs,
resume and partitions as release `v1.363.0`** — proven live in the TUI
(receipts at the end). Decision record: `../adr/0019-workflow-runs-on-mario.md`.

A workflow is a directory of task files. Each task is one turn of an agent
with the tasks it requires; it is done when its **target** exists — a file,
a command that exits 0, or the agent's answer — never because the model said
so. The engine is [`guregodevo/mario`](https://github.com/guregodevo/mario)
(Luigi-style DAG runner, imported as a library); the YAML is mario's own, and
Memdoor adds one task type, `agent`, the way
`mario-llm` (a sibling, private factory) adds `llm_agent`.

```
/workflow run release        in the TUI: starts it, draws the graph as it runs
memdoor workflow run release from the shell, same gateway
```

## The files

One YAML per task, at `<workflow>/<group>/<task>.yaml` — mario's layout
(project / dataset / table). In a project, `.memdoor/workflows/<name>/…`; anywhere
else, a path. The directory can be committed in the repo, or written for one
run — by you, or by an agent you ask to build it — and thrown away.

```
.memdoor/workflows/release/steps/tests.yaml
.memdoor/workflows/release/steps/changelog.yaml
.memdoor/workflows/release/steps/release.yaml
```

```yaml
# .memdoor/workflows/release/steps/tests.yaml
type: agent
agent: coder                      # who runs it (default coder)
prompt: Run `go test ./...`. If a test fails, fix the code, not the test.
target:
  command: go test ./...          # the PROOF: a green tree. Already green → skipped
timeout: 20m
max_retries: 1
```

```yaml
# .memdoor/workflows/release/steps/changelog.yaml
type: agent
prompt: Write CHANGELOG.md for {{.version}} from `git log`. One short section.
args:
  version: v0.1.0                 # the prompt is a Go template over args + partition
requires:
  - table_pattern: tests          # a task of this workflow, by name
target:
  file: CHANGELOG.md              # done when this file exists
```

```yaml
# .memdoor/workflows/release/steps/release.yaml
type: agent
prompt: Write RELEASE.md with "{{.version}} released" and the date {{.partition}}, then commit.
args:
  version: v0.1.0
requires:
  - table_pattern: changelog
  - table_pattern: approve
    external: true                # made OUTSIDE the workflow — never run, only checked
target:
  file: RELEASE.md
```

A file target is one file, done once it exists on any run (Greg's fresh
`digest` run: `log` and `summary` ran again, `digest` was skipped —
`DIGEST.md` was still there). When each run must make its own, the target
carries the partition: `target: {file: "DIGEST-{{.partition}}.md"}` — a
target is a template over args and partition, like a prompt.

A `target` is a **check**, not the work: a command that already exits 0
means the task is already done, and mario skips it without a turn (Greg's
first run, 2026-10-02: `target: {command: sleep 10 && echo A}` skipped all
three tasks; dropping the targets made each answer the proof and the turns
ran). Put the work in the prompt; put what proves it in the target.

Every file is validated against the `agent` schema (`pkg/workflow/schema/`)
before anything runs; an unknown key, a missing prompt or a requirement with
no definition is refused with the file and the key named. A task with no
`target` keeps the agent's **answer** as its proof, at
`.memdoor/runs/<workflow>/<group>/<task>/<partition>` in the project — the
tasks after it can read it.

## Three kinds of task

`type:` says which; the factory registered for it builds the task — mario's
`tasks.Base` does everything the engine needs from a definition and a type
is one function. This gateway offers three, mixed freely in a DAG:

| type | does | the proof |
|---|---|---|
| `agent` | a turn of a Memdoor agent (`prompt`, `agent`) — Memdoor's own type | its target, or its answer |
| `command` | a shell line in the project (`command: git log --oneline -8`) — mario's | its target, or its stdout |
| `llm` | one model call, no tools (`prompt`, `model`) on this gateway's client — mario's; a `file` target is where the answer is written | its target, or the answer |

Every output lives at `.memdoor/runs/<workflow>/<group>/<task>/<partition>`
(mario's `Outputs`), and a prompt reads one with `{{ output "log" }}` — the
kept output of the required task `log`, inlined (found live: an `llm`
summary of a `command`'s git log answered "the commits weren't included in
your message"). A prompt reads a file of the project with `{{ file "path" }}`, as it is when the task runs (pr-review's one-call review reads the diff its `diff` command wrote, 2026-10-28). A person's approval is written there too. A browser step is a `command` whose line is
`memdoor chrome run`.


A task's `model:` is read the way /model reads its line (2026-10-25): the id, then
an optional host preference (`price`, `throughput`, `latency`, `default`) and an
optional `order host1,host2`, and an optional `thinking off|low|medium|high`, `max_tokens N` (the reply's ceiling for the task, under the provider's) (the
step's reasoning effort; `off` sends OpenRouter's `reasoning.enabled=false`), for agent
and llm tasks alike. The id may be left out: `model: thinking off` is the model that answers here, with thinking off (on a runtime connected with no key, Ollama's `reasoning_effort: none`). The LCB-100 bench needed all of it: Qwen3.8-27B answered at 885
tokens a second on OpenRouter's throughput host (Cerebras) and at 44 on the cheapest
(Darkbloom); with thinking on it ran past that host's 32K output cap on a planning
prompt and wrote nothing, with thinking off it planned in 3 s; and an llm task has no
window to type /model or Shift+Tab in.
## The target is the turn's definition of done

An agent task's `target` is handed to its turn as the declared done
(`done_when` on the task's session, `gateway/turn_done.go`): at the end of
the turn the gateway checks the file or runs the command itself. Present,
the turn ends; missing, the turn continues with "the target X does not
exist — produce it", under the turn's budget (`turn_token_budget`). The
receipts judge (DECIDE.md) is not asked when a target is declared: the
artifact is there or it is not. Found live 2026-10-03: without this, a
trading workflow's two tasks made their files and were judged unfinished
twice each.

**The task cannot edit its own check (2026-10-24, `gateway/target_guard.go`).**
Measuring what a retry loop costs, `target: {command: ./never.sh}` with a
script that always exits 1 was marked done on its third attempt: the agent
rewrote `never.sh` to exit 0 and the engine ran it. Now, when a task starts
(every attempt; a resume after the person's own edit takes the new file),
`targetPrograms` names the project files in command position of the target
command, or given to an interpreter (`sh x.sh`, `python x.py`), and
`snapshotTargetPrograms` keeps their bytes, mode and hash on the session
(`done_when_programs`). `executeTool` refuses `write_file`, `edit_file`,
`apply_patch` and `search_replace` calls that name one
(`targetGuardRefuses`, on the file the call names, `namedFiles`: its path
field or a patch's file lines, never the content being written), after
every tool call `restoreTargetPrograms` puts back any that changed (a shell
redirect, a patch) and returns the tool an error saying so, and
`restoreBeforeCheck` puts them back once more right before `doneWhen` is
evaluated, so a change that lands after the last tool call (a background
process the turn left running) never reaches the check (found by review,
2026-10-24). A run spawned with `sessions_spawn` inherits the kept programs
(`tools.KeptProgramsKey`), so a task cannot change its check through a
child. The check mario and `doneWhen` run is then the declared one. Data
the check reads (`test -f grades/x.json`) is not a program and stays the
task's to produce; what a program depends on (tests, a Makefile) is not
covered, nor is a program the kept one calls (`./run.sh` calling
`./never.sh`: run.sh is kept). Receipts: the never fixture, three attempts,
✗, the script unchanged in git, one refusal logged on the first attempt;
the background fixture (`nohup sh -c 'sleep 2; … > never.sh' &`, then no
tool call), ✗, "Kept programs put back before the check" logged, the
script unchanged in git.

`budget: 300000` on an agent task is the tokens its turn may spend running
on alone while its target is missing (mario's YAML, beside `model:`);
without it the workspace's `turn_token_budget` applies. `memdoor workflow
status` and `/workflow` show each finished agent task's receipt line under
it: what proved the step, not only that it is done.

## External tasks

`external: true` on a requirement is mario's rule: the task is produced by
someone else — a person's approval, a file another system drops — so the
workflow **never triggers it**, it only checks whether its target is there.
A run that reaches a missing external target builds everything it can and
ends **waiting**:

```
⏸ release waiting for approve — made outside this workflow. a in /workflow
  approves it, or memdoor workflow approve 20261001-232930 approve.
```

Approving writes the target (`.memdoor/runs/release/steps/approve/<day>`)
and triggers the **same run** again. Tasks whose targets exist are skipped,
so only what is downstream of the gate runs.

Before answering, the person reads what the run changed: the panel lists
its commits and a diff stat, `d` puts the full diff (committed or not, since
the run's first commit) in the conversation, and `memdoor workflow diff
<run-id>` prints it. `/workflow changes <step> <comment>` sends it back
instead: that step reruns with the comment, then every step after it, and
the run returns to the same gate.

## A library of workflows

A workflow is reusable because it is files (Greg: "workflow can be
reusable"): commit `.memdoor/workflows/<name>/` in a repo, or keep it in
**`~/.memdoor/workflows/<name>/`**, the person's shared library, and it runs
in any project — `workflow.Names(workdir)` lists the project's own then the
library's it does not shadow, `workflow.Find(workdir, name)` resolves in
that order and then as a path. The dropdown, the panel, the CLI and the
tool all read the same two places. Receipt: a unit test over both.
Worked examples, each built by the coder from a description and run before
it was added, live in `examples/workflows/` (nightly-rule, 2026-10-07;
team-solve, the GVS5H ledger method, 2026-10-25, with the sample problem and
the run's receipt in its README) and are copied the same way.

### Built in

Three workflows ship in the binary (`workflows/`, embedded) and are seeded
into the library at the gateway's boot (`SeedEmbeddedWorkflows`, the skill
seed's rules): so they run in a repository with no `.memdoor` at all, a
GitHub job's first included. A project's own workflow of the same name wins;
an edited library copy is never overwritten; a library directory of that
name the seed did not write is left alone.

| Workflow | What it does | Fails when |
|---|---|---|
| `pr-review` | The change (a PR against its base, else the branch or the last commit) reviewed by the coder with the `review` skill, verified by running; `.memdoor/runs/pr-review-<partition>.md` with **P0**/**P1**/**P2** findings | there is a P0: what must not merge |
| `fix-tests` | The project's test command (`$MEMDOOR_TEST_COMMAND`, else found from go.mod, Cargo.toml, package.json, pyproject.toml, a Makefile); when it fails, the coder fixes the code and reruns until it passes; the fix stays in the working tree, the report in `.memdoor/runs/fix-tests-<partition>.md` | the tests still fail |
| `release-notes` | Every commit since the tag before the release (the partition, else HEAD) into `.memdoor/runs/release-notes-<partition>.md`: Added / Changed / Fixed, what a person notices | — |

Receipt, 2026-10-27, each through the action's own step on a fresh HOME:
pr-review on a shallow PR merge checkout that narrowed a loop bound found
it as a P0 verified by running (`Sum([]int{1,2,3})` returns 3) and failed
the job, exit 2, the review in the summary, while a correct change (a `Max`
with its test) passed with "No findings" after `go vet` and `go test`, exit 0;
the change is taken right from a PR's shallow or full checkout, a shallow
push, a local branch and a first commit; fix-tests on a Go package whose
`Reverse` dropped the first rune fixed `i > 0` to `i >= 0` in 18 s, tests
untouched; release-notes for v0.2.0 kept the two visible commits and left
the refactor out; fix-tests on a passing suite skipped the agent, no key.

## The partition

mario runs a DAG per *partition*, and **the partition is the identity of
what the DAG is for** (Greg: "there are different partition schemes — a
CI/CD run, what is the partition?"): CI on a commit → the commit, so the
same commit resumes and a new one reruns everything; a nightly → `today`;
a release → its version, so approval resumes that release; a PR review →
the PR number; "run it" from the TUI → the run's own stamp, **fresh by
default** ("fresh is the default"). The caller passes it (`--partition`,
`partition:` on the tool, `/workflow run <name> <partition>`); prompts read
it as `{{.partition}}`.

The person names **runs**, not partitions: to continue an earlier run — one
that failed and was fixed, one that was stopped — **resume** it by its run
id (`c` on it in `/workflow`, `/workflow resume <run-id>`, `memdoor
workflow resume <run-id>`, `action: "resume"` on the tool). What it
finished is skipped, what it did not runs; a run waiting at a gate is
approved, not resumed. A resumed DAG still resolves root to sink — the
upstream settles before the downstream, never the reverse (mario PR #1).

## In the TUI

A run started from a window — by you or by the coder's tool — draws its DAG
**in the conversation** as one block, redrawn in place on every event mario
sends, so you watch it run without opening anything:

```
  abc · running · 1/3 done
  ✓ B  done 16s
  │
  ▶ A  running 4s  ← B
  ▶ C  running 4s  ← B
```

The block is live: a running task's time ticks every second between
events, and the tools its turn calls appear under it (`⏺ bash`, `⏺
apply_patch`) — the runtime's own event stream, relayed for the run's
sessions. One line per transition in the conversation (`▶ digest · 3 tasks`,
`▸ summary — …`, `■ digest done`); the block carries the rest.

`/workflow` opens the panel: the project's workflows, then its runs. Enter on
a workflow starts it; Enter on a run opens its graph, drawn in layers by
dependency (`← requires`) and redrawn on every event the gateway sends:

```
release · run 20261001-232930 · waiting · 2/4 done
╭───────────────────────────────────────────────────────╮
│   ○ approve    waiting · external — a approves        │
│   ✓ tests      skipped                                │
│   ✓ changelog  done 14s  ← tests                      │
│   ○ release    waiting  ← changelog, approve          │
╰───────────────────────────────────────────────────────╯
  a approve · s stop (twice) · esc back · r refresh
```

`▶` running · `✓` done or skipped (the target was already there) · `✗`
failed · `↻` retrying · `○` waiting. Each task's turn answers into the
conversation (`▸ release.changelog — …`), the way a cron run does, and the
run's end is one line (`■ release done`). `s` twice stops a run: nothing more
starts, the running turn is told.

`/workflow:<name>` runs one straight from the slash dropdown, which
completes it with the project's workflows (Greg: "autocomplete with existing
workflows"). `/workflow run <name> [partition]` · `/workflow resume
<run-id>` · `/workflow approve <task>` (the open run) ·
`/workflow stop <run-id>`. A run is named `<workflow>-<yyyymmdd-hhmmss>`.
**Esc** in the conversation stops this window's run, the way it interrupts
a turn (Greg: "the user can escape using its esc").

## From the shell

```
memdoor workflow                        the project's workflows and runs
memdoor workflow run <name|path> [--timeout 90m]
memdoor workflow status <run-id>
memdoor workflow approve <run-id> <task>
memdoor workflow diff <run-id>          every line the run changed, before you approve
memdoor workflow changes <run-id> <task> <comment>
memdoor workflow stop <run-id>
memdoor workflow resume <run-id>
memdoor workflow history <name> [--limit 20]
```

`--dir` names the project (default: here). The run happens on the gateway;
the shell and the TUI read the same state.

`--wait` follows the run to its end: progress on stderr, the files it
produced on stdout, exit 0 done, 2 failed, 1 stopped. A gate is waited on;
`--stop-at-gate` (implies `--wait`) ends there with exit 3 and the gate's
review on stderr, for a job nobody can approve from.

## In GitHub Actions

The action, [guregodevo/memdoor-action](https://github.com/guregodevo/memdoor-action)
(its own repository, so its README is the Marketplace page), makes a workflow one step of a GitHub job:
the runner installs memdoor, the first command starts a gateway on
localhost and sets the runner up, and the job ends with the run's exit code.
The key is yours, from the repo's secrets.

```yaml
# .github/workflows/review.yml
on: pull_request
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: guregodevo/memdoor-action@v1
        with:
          workflow: pr-review                  # built in; or your own .memdoor/workflows/<name>/
          openrouter-api-key: ${{ secrets.OPEN_ROUTER_API_KEY }}
          timeout: 45m
```

**Your key.** The agent runs the repository's code on the runner, where your
key is. Trigger on `pull_request`, never `pull_request_target`: GitHub gives a
fork's pull request no secrets, so a stranger's code never runs beside your
key, while `pull_request_target` runs their code with your secrets. Memdoor
also keeps model keys out of the environment of every command the agent or a
workflow runs (`shell.Env`: the bash tool, the turn's checks, and a step's
`command:`, `when:` and command target, through mario's `Base.Env`), so an
`env` or a test that dumps its environment cannot carry one into the model's
context or the job summary. That is a second line, not the boundary: a
process of the same user can read a parent's starting environment
(`/proc/<pid>/environ`). A project's tests that need a model key get it under
another name.

Inputs: `workflow`, `working-directory`, `timeout`, `partition`,
`stop-at-gate` (default `true`: a gate fails the job with exit 3; `false`
waits for an approval from another door, bounded by `timeout`), `install`
(`false` uses the memdoor on PATH), and a key: `openrouter-api-key`,
`anthropic-api-key`, `openai-api-key`, `gemini-api-key` (a key already in
the job's env works too), `model`. Outputs: `run-id`, `exit-code`,
`results` (the files the run produced, one per line, a failed run's
included). The job summary is `memdoor workflow status` of the run, then
every Markdown file it produced (the review, the fix's report, the notes).
Linux and macOS runners.

`.github/workflows/action-selftest.yml` here runs the published action
(`install: false`) on memdoor built from the commit, and the action repository's
own self-test installs memdoor from memdoor.ai on Ubuntu and macOS: a run that passes (0), one that fails (2), one that stops
at its gate (3), keyless command steps, and the built-in fix-tests on a
suite that passes (0, nothing changed, no key).

## Runs you can see later

Every run is kept in the project's **run table**, `.memdoor/runs.db` — mario's
sqlite repository, one per project, beside the run outputs. `memdoor workflow
status` reads the run in hand; `memdoor workflow history <name>` reads the
table, newest first, with each run's state and task count. The difference
matters after a restart: a gateway that never ran anything still answers for
every run the project has done.

Two properties the table has to have, both tested:

- **It survives a restart.** mario's repository used to `DROP TABLE` on open
  and delete its file on close — a persistence layer that kept nothing. It
  now creates the tables only if they are absent and leaves what it holds
  alone.
- **A run table that cannot be opened is not worth a dead gateway.** It is
  opened through `OpenWorkflowRepositoryAt`, which returns the error; the
  gateway logs it, runs on mario's in-memory repository, and only the
  surviving part is lost.

## From a sentence

Nobody has to know the format (Greg: "the prompter should not know about
mario … it says what it wants and the agent builds it"). The coder has the
`workflow` **skill** (`skills/workflow.md`, seeded to `~/.memdoor/skills`
like every other, so a user's edit wins): what a workflow is, the five
rules (a target is a check; dependencies by name, parallel otherwise; a
gate is an external requirement and never a file; re-runs skip what
exists; a DAG can fail and that is it working), the procedure, and a worked
example. The person says "vet and test in parallel, then release notes,
then my approval, then announce"; the coder loads the skill, writes the
files, runs the tool, and stops.

mario refuses the one mistake a writer makes most — a file for the gate
beside `external: true` on the task after it — when the files are read,
naming both ("has a definition in this DAG: an external task is made
outside it — delete the definition, or drop external"), instead of running
the gate as a task.

## From an agent

The coder has a `workflow` tool with the same five actions — `list`, `run`,
`status`, `stop`, `approve` — on the gateway's own run functions (Greg:
"the coder agent can stop a workflow … gracefully"). A run it starts goes on
in the background and reports into the window the turn came from; `stop`
is graceful: nothing more starts, the running task is told and ends on its
own, and the run reads `stopped` with every task's state kept. The tool is
in the coder's palette and the `investigate` routing family beside `cron`.

## Bounds

Per task: `timeout` and `max_retries` (mario's retry queue, 2 s apart). Per
run: a clock (`--timeout`, default 2 h, at most 24 h) — past it nothing more
starts and the running turn is told — and one run of a workflow per project
at a time. A workflow left to itself is a loop; the bound is a counter and a
clock, not a better prompt.

## Where the state lives

Nowhere in Memdoor. Every task's state the TUI, the CLI and the tool show is
read from **mario's repository** — its record of each execution (status,
start, end, retries, error) — through `pkg/workflow`'s `Execution.Status()`.
mario calls back into a duck-typed `Events` listener (started, done,
skipped, failed) and the gateway's implementation only logs the line and
broadcasts the fresh state; the gateway keeps one thing of its own, the
run's state as a whole (running, waiting, done, failed, stopped), which
mario has no word for. mario's own engine lines go through Memdoor's logger
(handed in with the run's options), so `memdoor logs query --regex "Started
task"` shows the scheduler beside the turns it drove.

## Free locally, Pro hosted

Greg, 2026-10-04: "everyone can run schedule with local cron". Running a
workflow on your own machine needs no seat: the 2026-10-02 check in
`startWorkflow` ("workflow is for Pro only") is gone
(`TestLocalWorkflowsNeedNoSeat`). Schedule one with the local cron:

```
memdoor cron add --id nightly --schedule "0 7 * * *" --workflow peer-comps --partition today
```

stores a job whose message is `/workflow run peer-comps today` with this
project's directory (`--dir` names another); when it fires, the gateway
starts the workflow there instead of sending an agent turn
(`scheduledWorkflow` in `gateway/server_jobs.go`). `--partition today` runs
it at most once a day; without it each firing is a fresh run. It fires
while this gateway is up.

**Pro keeps every run's state on memdoor.ai** (2026-10-05, `gateway/hosted_state.go`,
the private `mario-state` service: mario's own sqlite repository behind
HTTP, one table per workspace, the account token on every request). The
workspace is the unit of sharing: every project and every person on it
write one table, so an `external: true` task of one workflow is a task
another workflow of the workspace produced, by name and partition, as in
mario — from whichever machine ran it. Where the state lives is decided when
the run starts and cannot change after, so a Pro run whose store does not
answer is refused in words rather than quietly kept locally; when billing
does not answer, the plan it last named for that account
(`~/.memdoor/plan-<hash of the account token>`) stands — Pro stays strict,
free stays local — and an account it never named cannot start a run until it
does once. The tasks
still run on your gateway, on your key; only the state travels. Free keeps
the project's `.memdoor/runs.db`, complete for anything self-contained.
Runs with the laptop closed are a gateway on a box that stays on (`memdoor
gateway install`, 2026-10-23); still to come, a box memdoor.ai runs for you.
`MEMDOOR_STATE_URL` points a gateway at another store, loopback or memdoor.ai only.

## Review of the run table (2026-10-03)

Reviewed the two commits the way the coder reviewed mine: diff, build,
tests three times, then a real run in a fresh project. Two findings, both
fixed:

1. **`memdoor workflow history <name>` listed nothing** — for a run that
   had just completed, and after a restart. The table had the run (the
   durability work was right); the read was wrong: mario's `Runs(name)`
   queried `WHERE Name = <workflow>`, but every row a run writes has the
   TASK's full name (`ping.steps.say`); the workflow is only in the
   `Version` (`<workflow>@<partition>`). Both the mario test and the
   gateway test had inserted rows by hand with the workflow's name as
   `DName` — a shape no real run produces — and passed. mario `1a5fb24`:
   the listing keys on the version prefix; its fixture writes the task's
   name. Memdoor: `TestWorkflowHistoryListsARealRun` runs the release
   workflow for real and reads it back through a new Server.
2. **The run table was not gitignored** — `.memdoor/.gitignore` said
   `runs/`, and `runs.db`, `-wal`, `-shm` sat untracked in every project
   that ran a workflow. `EnsureIgnored` now lists them and adds the lines
   to an older file (one that ignores everything is left alone).

Receipt: the run recorded by the previous binary lists after the restart,
a second run stacks above it, `git status` is clean.

## What Memdoor changed in mario

Everything the run needed went **upstream** (branch `workflow-engine`),
never copied: a `context` on a run, `Events` a host watches (started, done,
skipped, failed), a slog-shaped `Logger` interface, errors instead of
`os.Exit`, pointer instances, `agent` and `target` on a task definition,
requirement defaults (`project_id`/`dataset_id` fall back to the task's),
walks without a template directory, and Luigi's rule in the scheduler: a
task whose target already exists is complete, announced as skipped, and its
downstreams go on. `pkg/workflow` is ~500 lines: the schema, the `agent`
factory, `Load` and `Run`.

## Receipt — 2026-10-02, a test gateway on :18799, the TUI from a scratch project

`.memdoor/workflows/release/steps/{tests,changelog,release}.yaml` as above, on a
project with one Go test.

1. `/workflow run release` — `tests` **skipped** (its command already exits
   0), `changelog` ran 14 s and wrote CHANGELOG.md, the run ended
   **waiting** at `approve` with the ⏸ line above. No turn was spent on the
   gate.
2. `a` in the graph — the approval file written, the same run triggered
   again: `tests`, `changelog` skipped, `release` ran 7 s: RELEASE.md with
   `v0.1.0 released 2026-10-02`, commit `84fe059 v0.1.0` in the project.
   `■ release done · 4/4`.
3. Negative paths: a run of a workflow that does not exist is refused by
   name; `status ghost` → `no run ghost`; approving a non-external task, a
   task that does not exist, or a run that does not exist each refused in
   words; stopping a stopped run says it is already stopped.

Second batch, same day ("Test more"), each on a real run:

4. **A task that cannot meet its target** (`target: {command: test -f
   /nonexistent/never}`, `max_retries: 1`): two turns (`▸` twice), then
   `✗ doomed failed — task doomed finished but its target is not there
   (command: …)`, `after` left waiting, `■ flaky failed`. The model's
   "done" never counted.
5. **The run's clock**: `memdoor workflow run slow --timeout 25s` on a turn
   told to sleep → `✗ nap failed 25s · context deadline exceeded`, run
   failed. The turn itself keeps its ctx and ends on its own.
6. **An ad-hoc DAG by path**, outside the project: `memdoor workflow run
   <scratch>/adhoc` — the prompt rendered from `args` and the partition
   (`hello from an ad-hoc DAG on 2026-10-02`), kept at
   `.memdoor/runs/adhoc/steps/hello/2026-10-02` in the project. The
   day is the local one.
7. **Resuming a partition** (then the default; now `--partition`): a second `run release` on the same partition ended waiting at the
   gate at once, `tests`/`changelog`/`release` all **skipped** (3/4, not a
   turn spent); `memdoor workflow approve <run> approve` from the shell
   finished it 4/4 without a turn.
8. **Stop**: `s s` on a run waiting at its gate → `■ release stopped while
   waiting`; `memdoor workflow stop` one second into a running turn →
   `stopping` → `stopped`, the task `failed 1s · context canceled`; a
   second `run slow` while one runs → `slow is already running here as
   <run> — stop it or wait for it`.
9. Found and fixed on the way: two runs started in the same second shared
   an id and one overwrote the other in the list. A run is now
   `<workflow>-<yyyymmdd-hhmmss>`, with `-2`, `-3` … when a second one
   starts in the same second.

Third batch ("yes try it"):

10. **A retry that succeeds** (`max_retries: 2`, a target command that passes
    only on its fourth check): attempt 1 failed at the target, attempt 2 →
    `✓ second done 23s`. The limit is the task's YAML, nothing else.
11. **A cross-workflow requirement** (`project_id: release, table_pattern:
    changelog, external: true` from `ship`): with `CHANGELOG.md` there,
    `changelog skipped (external)` and `announce` ran (answered the file's
    first heading); with it removed, the run **waited** without a turn. The
    sibling's own target is what is checked — `pkg/workflow` reads
    `.memdoor/workflows/<project_id>/<group>/<task>.yaml` for it.
12. **The coder's `workflow` tool**: asked to list, run `slow`, stop it and
    read its status, it called `workflow(action: "list")`, `workflow(name:
    "slow")` (`▶ slow started as slow-… It goes on in the background and
    reports here`), `workflow(action: "stop")` (`■ … stopping — nothing more
    starts; a running task is told and ends on its own`; the window then
    showed `✗ slow.nap failed — context canceled` and `■ slow stopped`),
    `workflow(action: "status")` → `stopped · 0/1`.
13. **Esc**: `/workflow run slow`, Esc five seconds in → `■ slow stopped`,
    the run listed as `stopped 0/1`.
14. **Two complex DAGs from a sentence** (Greg's gateway, 2026-10-02,
    "try to build more workflow … like complex ones"): `workflow-review` —
    three parallel agent reviews of `pkg/workflow` (43 s, 1m12s, 1m36s) →
    an `llm` merge → the gate → `REVIEW.md` (6/6 done; the merged review
    found `run.go:252` discarding `WaitForCompletion`'s error); and
    `release-train` — vet/tests/gofmt → notes → gate → tag → final-check,
    which **failed honestly** (the scratch repo has one commit, the notes
    three lines, the check wants five). Two harness fixes came out of it:
    the `llm` task's call took `GetClient()` (the Hermes envelope for a
    brokered seat — no reasoning set-up, no LLM log line) and GLM 5.3
    Flash's reasoning-only reply read as "the model wrote nothing" after
    51 s; it now takes `GetClientFor` like `/handoff`, logs as
    `agent=workflow`, caps the reply at the model's limit and names a
    reasoning-only cut-off (the merge then took 3m56s, 12.8k output
    tokens). And `vet` with `target: {command: go vet ./...}` was skipped
    on a fresh run — the tree already vetted, so the target held before
    the task ran; the skill now says a check is a `command` task with no
    target (its exit code is the proof) and a target names what a task
    PRODUCES.

Fourth ("Test"): **an agent authors the DAG, and tasks run in parallel.**
Asked in one sentence for a workflow `fanout` — `count` (the .go files),
`lines` (wc on add.go), `report` requiring both with `target: {file:
REPORT.md}` — the coder read two existing task files for the shape, wrote
the three YAML files in mario's layout, ran it with its tool and polled
`status` until `done · 3/3`. The scheduler's own log shows `count` and
`lines` **started the same second** and `report` started once both were
complete; the kept answers are `2` and `3`; REPORT.md reads `2 .go files` /
`3 lines in add.go`. Nothing in Memdoor decided the order — mario did.

Fifth (Greg's own run, then "I expected a tool to run workflow that we can
see in real time the dag running, instead of bash"): in Greg's window the
coder had **no `workflow` tool** — the turn was routed as a change and the
tool sat only in the `investigate` family — so it ran `./memdoor workflow`
through bash, and a shell-started run has no window to draw in. Fixed: the
tool rides on change turns, and the DAG is drawn in the conversation.
Re-run on a test gateway: the coder wrote `.memdoor/workflows/abc/steps/{A,B,C}.yaml`,
the schema refused its first try with the key named (`requires.0:
Additional property letter_b is not allowed`), it fixed the file, called
`workflow(name: "abc")`, polled `status`, then scheduled a `cron` to
re-check — and the block in the conversation went from `running · 0/3` to
`✓ B done 16s` / `✓ A done 15s ← B` / `✓ C done 15s ← B`, `done · 3/3`,
redrawn in place. Also found on Greg's run: `target: {command: sleep 10 &&
echo A}` is a check, not the work — all three skipped without a turn; the
tool now says so.

Sixth, from a sentence only ("the prompter should not know about mario"):
*"I want a release-check workflow: run go vet and the tests in parallel,
then write RELEASE_NOTES.md from the last 8 commits, then wait for my
approval, then announce the first bullet of the notes. Run it."* The coder
called `skill(workflow)` first, wrote **four** files (`vet`, `tests`,
`notes`, `announce` — no file for the gate; `external: true` on
`announce`'s requirement), `workflow(name: "release-check")`, polled
`status`, scheduled a `cron` every 30 s and stopped it when the run
settled. The block: `vet`/`tests` skipped (their commands already pass),
`notes done 11s ← vet, tests`, `⏸ waiting for approve`, 3/5. Nothing about
the format was in the ask.

Seventh — task types, from a sentence ("I want a digest workflow: list the
last 10 commits, then summarize them in three bullets, then have an agent
write them into DIGEST.md"): the coder chose `command` for `git log`, `llm`
for the summary with `{{ output "log" }}` in its prompt, `agent` for the
file. `log done 0s` → `summary done 20s` (three real bullets) → `write done
27s`, DIGEST.md on disk. A chain — the three types in one DAG, each built by
the factory of its type.

Two things to know: a run started from the shell has no window, so its
`■`/`⏸` lines go to the log, not a conversation (the TUI's own runs get
both); and an answer kept as a proof is the agent's whole reply, including
the honesty guard's trailing *(note: this turn used no tools …)* when it
fires — a task that reads an upstream answer should expect prose, not data.

The first design had the gate in the middle of the chain and the loader of
its own; the first live run did nothing (mario skips an external and never
builds what it requires). Greg: "external tasks are not triggerable …
unlike standard tasks that are triggered by workflow" — the gate is a leaf,
the run waits, approval triggers again. "Everything is already done in yaml
… no need to reinvent the wheel … remove your boilerplate" — the loader
went, mario's YAML stayed, and what was missing was added to mario.

## Next

- `type: llm_agent` from mario-llm, adapted to run on Memdoor's models
  through the gateway (BYOK): a single model call with tools, templated from
  `args`, in the same DAG — `../roadmap/MUST.md` item 0.
- A skill that writes a workflow directory from a sentence, so "release
  this" becomes a DAG the person can read before it runs.
