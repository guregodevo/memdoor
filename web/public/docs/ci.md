# In CI

A review on every pull request, tests written for it, a failed run explained,
a new issue triaged, release notes on a tag. Each is one step in a GitHub job.
Nothing is added to your repository. The model runs on your key or inside the
runner with no key. The job passes or fails on a check, never on what the
model said.

```yaml
on: pull_request
permissions:
  contents: read
jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: guregodevo/memdoor-action@v1
        with:
          workflow: pr-review
          openrouter-api-key: ${{ secrets.OPEN_ROUTER_API_KEY }}
```

The action is [on the GitHub
Marketplace](https://github.com/marketplace/actions/memdoor). It installs
Memdoor on the runner as one binary, runs the workflow and ends the job with
the run's outcome. The review lands in the job summary.

## Local or a key

The model behind the step is one input.

```yaml
      # a key: any model on OpenRouter, or a vendor's own key
      - uses: guregodevo/memdoor-action@v1
        with:
          workflow: pr-review
          openrouter-api-key: ${{ secrets.OPEN_ROUTER_API_KEY }}

      # no key: a model inside the runner
      - uses: guregodevo/memdoor-action@v1
        with:
          workflow: pr-review
          local-model: Qwen3-8B-4bit
```

**With a key** the review is an agent. It reads the diff and opens the files
around the change. It runs the code to check each finding before writing it
down. It answers on the model you pick (`model:`). `anthropic-api-key`,
`openai-api-key` and `gemini-api-key` work in place of OpenRouter. So does a
key already in the job's environment.

**With no key** the action downloads
[janis](https://github.com/guregodevo/janis), one file. It runs the model on
the runner's CPU, caches it and points Memdoor at it. No secret, no API
account, nothing leaves the runner. Linux runners only. A free runner has 4
CPUs and 16 GB, so a 9B model is about the largest that fits. Each workflow is
one model call with everything it needs in the prompt. No agent runs the code
to prove a finding. Treat it as a first pass.

## What each one catches

Three pull requests on [llamafit](https://github.com/guregodevo/llamafit),
a 3,000-line Go project, each reviewed both ways on `ubuntu-latest`, twice
([#9](https://github.com/guregodevo/llamafit/pull/9),
[#10](https://github.com/guregodevo/llamafit/pull/10),
[#11](https://github.com/guregodevo/llamafit/pull/11)):

| Pull request | No key: `Qwen3-8B-4bit` on janis in the runner | Your key: OpenRouter, Memdoor's default model |
|---|---|---|
| An obvious bug: a loop that now skips the last layer | P0, the right reason, 2 of 2 | P0: the last layer is left out of every estimate |
| A subtle bug: the returned path built from the lowercased file name | P0, the right reason ("uses `name` instead of `e.Name()`"), 2 of 2 | P0, proved by a test it wrote: `MODEL.GGUF` comes back as a path that does not exist |
| The same change done correctly | **a false P0**, 2 of 2: it filed the change's own fix as the fault; with the re-check (below), 1 of 2 | passed |
| Review | 45–90 s | 35–60 s |
| Job | 2–3 min, most of it the model's first download | about 1 min |

The local model catches bugs. It files opinions as P0s. Since v1.731.0
its P0s are advisory: they appear in the review and the job does not fail on
them. With a key a P0 still fails the check. Read a local P0 before you
believe it. Since v1.714.0 each local P0 gets a second look on the same model
with one question: is it a problem of the code after the change or of the code
before it? An answer of "before" demotes it to P2. On the correct change that
turned one of two runs green. The other run's "P0" was praise ("which is
correct … no bug here"), which the question does not catch
([#12](https://github.com/guregodevo/llamafit/pull/12),
[#13](https://github.com/guregodevo/llamafit/pull/13),
[#14](https://github.com/guregodevo/llamafit/pull/14)). `Qwen3.5-9B-4bit` is
quieter. It passed the correct change and missed the subtle bug. The agent on
a key opens the files around the change and runs the code to prove what it
finds. It was right all three times. Use local where a key cannot go: a public
repository or a team with no API account yet. Use a key where the review has
to be right.

On a runner's CPU janis decodes the 8B at 5.6 tokens a second and reads a
prompt at the same speed; Ollama decodes at 4.45. The model is cached per
branch. GitHub shares only a cache saved on the default branch, so run the
workflow once on a push to `main` and pull requests start from it. A
`workflow_run` or `issues` job (`explain-failure`, `issue-triage`) reads that
cache but GitHub refused both a write. They download the model when `main` has
none saved; a `pull_request` job saved it.

## The built-in workflows

Nine, none of them a file in your repository. Each runs on your key as an
agent that reads the code around the change and runs it, or on a model inside
the runner as one model call with everything it needs in the prompt.

| `workflow:` | Run it on, with | What it does | The job fails when |
|---|---|---|---|
| `pr-triage` | `pull_request`; `pull-requests: read`, `issues: read` | Sorts the pull request for a maintainer: **ready for review**, **needs work** or **likely slop**, with the reasons; the verdict comes from checks run in the job (the tests, tests claimed and none changed, functions nothing calls, and whether the diff does what the title says), plus labels, a summary, what is missing and duplicates. Advisory. | never: the verdict is for a person |
| `pr-review` | `pull_request` | Reviews the change against its base branch and writes **P0** / P1 / P2 findings. | a P0: something that must not merge (with a key; advisory on a local model) |
| `security-review` | `pull_request` | The same, for security only: secrets, injection, path traversal, missing authorization, unsafe deserialization, SSRF, weak crypto, an exploitable CI workflow. | an exploitable P0 |
| `write-tests` | `pull_request` | Writes tests for the functions the change touches and leaves them in the working tree. Never edits an existing test. | an existing test already fails with the change, or a new test shows the change is wrong |
| `pr-description` | `pull_request` | Writes the pull request's description: title, What, Why, How to test, Risk. | — |
| `explain-failure` | `workflow_run` on failure; `actions: read` | Reads the failed run's logs and the commit's diff: the Cause (step, file:line), Why, the Fix. | — |
| `issue-triage` | `issues` opened; `issues: read` | Labels the repository has, the kind, a summary, duplicates among open issues, what the reporter should add. Applies nothing. | — |
| `fix-tests` | `push`, or by hand | Finds the test command, fixes the code until it passes, leaves the fix in the working tree. | the tests still fail |
| `release-notes` | a tag push | The commits since the previous tag as Added / Changed / Fixed. | — |

Every one needs `contents: read`. The three that read GitHub (`gh`) use the
job's token, passed as `github-token` (the default), with the permission
above. `pr-triage` reads the labels and the open pull requests and issues for
duplicates and leaves out what it may not read. Tests, fixes and notes stay in
the runner's working tree and appear in the job summary. Commit them in a
later step if you want them kept.

`pr-triage` runs the project's tests on the pull request's code: `go test
./...`, `cargo test`, `npm test`, `pytest` or `make test`, found from the
repository's files, or `MEMDOOR_TEST_COMMAND` in the job's environment. Set up
what they need (`actions/setup-go`, a build step) before the action. On six
pull requests on llamafit, three honest and three slop
([#19](https://github.com/guregodevo/llamafit/pull/19) to
[#24](https://github.com/guregodevo/llamafit/pull/24)), the 8B asked for a
verdict called all six slop. With the verdict made from those checks the three
honest ones were ready for review and the three slop ones flagged: one likely
slop (tests claimed, none changed) and two needs work (functions nothing
calls, the tests fail). The same in three runs each on `ubuntu-latest`, 3 to
4½ minutes a job. On a key the agent checks each claim of the description
against the code. It called all three slop ones likely slop. The write-up:
[Qwen3-8B couldn't judge AI slop PRs, so I let the tests
decide](https://guregodevo.github.io/2026/10/30/triage-ai-slop-prs/).

Measured on GitHub's runners
([llamafit](https://github.com/guregodevo/llamafit)
[#15](https://github.com/guregodevo/llamafit/pull/15),
[#16](https://github.com/guregodevo/llamafit/issues/16) and
[#17](https://github.com/guregodevo/llamafit/pull/17)), each both ways. A pull
request with a path traversal: `security-review` red on a key and on the 8B.
The same kind of request done correctly: green both ways. On the 8B that takes
a re-check. Each P0 is asked once more whether the code after the change lets
someone read, write or run something they could not before, and a no turns it
into a P2. `write-tests` on a key left four passing tests on a correct change.
On a change that broke an existing test it went red at once and named that
test. `explain-failure` named the failing test and the changed loop both ways
(the 8B put the line three off). `issue-triage` and `pr-description` wrote
their reports both ways. On the 8B the description has one line per changed
file, so a one-line change is never left out. A job on the 8B takes two to
five minutes with the model cached, about one on a key.

`write-tests` on the 8B kept no test in ten tries. Its tests compile but do
not match how the code behaves, and a failing test is removed. The job stays
green and says so after about a quarter of an hour on a runner. Writing tests
that pass needs the agent on a key. `fix-tests` and `release-notes` were
measured on fixtures on a Mac, not yet on a runner. On the 8B `fix-tests`
fixed an off-by-one with a one-line edit and `release-notes` sorted the
commits since the last tag.

Explain a failed CI run (`gh` reads the logs on the job's token):

```yaml
on:
  workflow_run:
    workflows: [CI]
    types: [completed]
permissions:
  contents: read
  actions: read
jobs:
  explain:
    if: github.event.workflow_run.conclusion == 'failure'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: guregodevo/memdoor-action@v1
        with:
          workflow: explain-failure
          openrouter-api-key: ${{ secrets.OPEN_ROUTER_API_KEY }}
```

Triage each new issue (the labels, the kind, likely duplicates in the job
summary; nothing is applied):

```yaml
on:
  issues:
    types: [opened]
permissions:
  contents: read
  issues: read
jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: guregodevo/memdoor-action@v1
        with:
          workflow: issue-triage
          openrouter-api-key: ${{ secrets.OPEN_ROUTER_API_KEY }}
```

`workflow_run` and `issues` jobs run from the workflow file on the default
branch: commit them to `main` before they fire.

Notes on every tag:

```yaml
on:
  push:
    tags: ['v*']
jobs:
  notes:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: guregodevo/memdoor-action@v1
        with:
          workflow: release-notes
          partition: ${{ github.ref_name }}
          openrouter-api-key: ${{ secrets.OPEN_ROUTER_API_KEY }}
```

## Your own workflow

`workflow: <name>` runs `.memdoor/workflows/<name>/` from your repository.
They are the same files `memdoor workflow run <name>` runs on your machine.
Build one by asking the coder ([Workflows](/docs/workflows)), try it locally
and commit it. CI runs it unchanged. Each step is done when its target exists,
so the job's outcome is that check.

A workflow with an approval gate stops the job there with exit 3, since nobody
in CI can approve. `stop-at-gate: false` waits instead for an approval from
another door, bounded by `timeout`.

| Exit | Meaning |
|---|---|
| 0 | done |
| 2 | a step failed: a P0 in `pr-review` or `security-review`, a test showing the change is wrong in `write-tests`, tests still failing after `fix-tests` |
| 1 | stopped (timeout) |
| 3 | stopped at an approval gate |

The step's outputs are `run-id`, `exit-code` and `results` (the files the run
produced, one per line) for the steps after it.

## Your key in CI

The agent runs your repository's code on the runner, where your key is.
Trigger on `pull_request`, **never** `pull_request_target`. GitHub gives a
fork's pull request no secrets, so a stranger's code never runs beside your
key. `pull_request_target` runs their code with your secrets. Memdoor
keeps model keys out of the environment of every command the agent or a step
runs, so a test that prints its environment cannot carry one into the model's
context or the log. That is a second line of defence. The trigger is the
boundary.

The inputs (`model`, `partition`, `timeout`, `github-token`…) and macOS
runners are in the [action's
README](https://github.com/guregodevo/memdoor-action#inputs).

## Without the action

The action wraps an install and one command with the key in the environment.
Any CI that can run a shell runs them:

```bash
curl -fsSL "https://memdoor.ai/install.sh?ci=1" | MEMDOOR_CI=1 bash
export PATH="$HOME/.local/bin:$PATH"
memdoor workflow run pr-review --wait --stop-at-gate    # OPENROUTER_API_KEY in the env
```

To watch or steer a CI run from your phone see [Headless, then
attached](/docs/headless#in-ci).