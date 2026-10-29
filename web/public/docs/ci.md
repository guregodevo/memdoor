# In CI

A review on every pull request, a failing suite fixed, release notes on a
tag: one step in a GitHub job, nothing added to your repository. The model
runs on your key, or with no key at all on a model inside the runner. The job
passes or fails on a check, never because the model said so.

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

The action is [on the GitHub Marketplace](https://github.com/marketplace/actions/memdoor).
It installs Memdoor on the runner (one binary), runs the workflow and ends the
job with the run's outcome. The review lands in the job summary.

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

**With a key** the review is an agent: it reads the diff, opens the files
around the change, runs the code to check each finding before it writes it
down, and answers on whatever model you pick (`model:`). `anthropic-api-key`,
`openai-api-key` and `gemini-api-key` work in place of OpenRouter; so does a
key already in the job's environment.

**With no key** the action downloads [janis](https://github.com/guregodevo/janis),
one file, runs the model with it on the runner's CPU, caches the model, and
points Memdoor at it. Nothing installed, no secret, no API account, and
nothing leaves the runner. Linux runners. A free runner has 4 CPUs and 16 GB,
so a 9B model is about the largest that fits, and the review is one model
call: no agent, nothing run to prove a finding. A free first pass, not a
reviewer.

## What each one catches

The same three pull requests on [llamafit](https://github.com/guregodevo/llamafit),
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

The local model catches bugs and can fail a correct pull request: read its P0
before you believe it. Since v1.714.0 each of its P0s gets a second look on
the same model, one question: is it a problem of the code after the change, or
of the code before it? One answered "before" is demoted to P2. On the correct
change that turned one of two runs green; the other run's "P0" was praise
("which is correct … no bug here"), which the question does not catch
([#12](https://github.com/guregodevo/llamafit/pull/12),
[#13](https://github.com/guregodevo/llamafit/pull/13),
[#14](https://github.com/guregodevo/llamafit/pull/14)). `Qwen3.5-9B-4bit` is the quiet one: it passed the
correct change and missed the subtle bug. The agent on a key opens the files
around the change and runs the code to prove what it finds, and was right all
three times. Use local where a key cannot go (a public repository you would
rather not put a secret in, a team with no API account yet) and a key where
the review has to be right.

On a runner's CPU janis decodes the 8B at 5.6 tokens a second where Ollama
does 4.45, and reads a prompt at the same speed. The model is cached per
branch, and GitHub shares only a cache saved on the default branch: run the
workflow once on a push to `main` and every pull request starts from it.

## The built-in workflows

Eight, none of them a file in your repository. Each runs on your key (an
agent that reads the code around the change and runs it) or on a model
inside the runner (one model call with everything it needs in the prompt).

| `workflow:` | Run it on | What it does | The job fails when |
|---|---|---|---|
| `pr-review` | `pull_request` | Reviews the change against its base branch and writes **P0** / P1 / P2 findings. | a P0: something that must not merge |
| `security-review` | `pull_request` | The same, for security only: secrets, injection, path traversal, missing authorization, unsafe deserialization, SSRF, weak crypto, an exploitable CI workflow. | an exploitable P0 |
| `write-tests` | `pull_request` | Writes tests for the functions the change touches and leaves them in the working tree. Never edits an existing test. | an existing test already fails with the change, or a new test shows the change is wrong |
| `pr-description` | `pull_request` | Writes the pull request's description: title, What, Why, How to test, Risk. | — |
| `explain-failure` | `workflow_run` on failure | Reads the failed run's logs and the commit's diff: the Cause (step, file:line), Why, the Fix. | — |
| `issue-triage` | `issues` opened | Labels the repository has, the kind, a summary, duplicates among open issues, what the reporter should add. Applies nothing. | — |
| `fix-tests` | `push`, or by hand | Finds the test command, fixes the code until it passes, leaves the fix in the working tree. | the tests still fail |
| `release-notes` | a tag push | The commits since the previous tag as Added / Changed / Fixed. | — |

Measured on [llamafit](https://github.com/guregodevo/llamafit) fixtures, each
both ways: with a key, `write-tests` on a change with a subtle bug failed the
run with a test showing the bug, and `explain-failure` named the failing test,
the changed line and the exact bytes missing. On Qwen3-8B, `explain-failure`,
`pr-description`, `security-review`, `issue-triage` and `release-notes` wrote
their reports in under a minute each; `fix-tests` fixed an off-by-one with a
one-line edit. `write-tests` on the 8B kept no test in five tries: its tests
compile but do not match how the code behaves, and a test that does not pass is
removed, never kept. Writing tests that pass needs the agent, on a key.

Explain a failed CI run (`gh` reads the logs on the job's own token):

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

`workflow: <name>` runs `.memdoor/workflows/<name>/` from your repository,
the same files `memdoor workflow run <name>` runs on your machine: build one
by asking the coder ([Workflows](/docs/workflows)), try it locally, commit
it, and CI runs it unchanged. Each step is done when its target exists, so
the job's outcome is that check.

A workflow with an approval gate stops the job there with exit 3, since
nobody in CI can approve; `stop-at-gate: false` waits instead, for an
approval from another door, bounded by `timeout`.

| Exit | Meaning |
|---|---|
| 0 | done |
| 2 | a step failed (for `pr-review`, a P0) |
| 1 | stopped (timeout) |
| 3 | stopped at an approval gate |

The step's outputs are `run-id`, `exit-code` and `results` (the files the
run produced, one per line), for the steps after it.

## Your key in CI

The agent runs your repository's code on the runner, where your key is.
Trigger on `pull_request`, **never** `pull_request_target`: GitHub gives a
fork's pull request no secrets, so a stranger's code never runs beside your
key, while `pull_request_target` runs their code with your secrets. Memdoor
also keeps model keys out of the environment of every command the agent or a
step runs, so a test that prints its environment cannot carry one into the
model's context or the log. That is a second line, not the boundary.

Every input, and macOS runners, are in the
[action's README](https://github.com/guregodevo/memdoor-action#inputs).

## Without the action

The action wraps an install and one command, with the key in the
environment. Any CI that can run a shell runs them:

```bash
curl -fsSL "https://memdoor.ai/install.sh?ci=1" | MEMDOOR_CI=1 bash
export PATH="$HOME/.local/bin:$PATH"
memdoor workflow run pr-review --wait --stop-at-gate    # OPEN_ROUTER_API_KEY in the env
```

To watch or steer a CI run from your phone, see
[Headless, then attached](/docs/headless#in-ci).
