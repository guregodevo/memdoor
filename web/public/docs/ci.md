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
          local-model: qwen3.5:9b
```

**With a key** the review is an agent: it reads the diff, opens the files
around the change, runs the code to check each finding before it writes it
down, and answers on whatever model you pick (`model:`). `anthropic-api-key`,
`openai-api-key` and `gemini-api-key` work in place of OpenRouter; so does a
key already in the job's environment.

**With no key** the action starts [Ollama](https://ollama.com) on the runner,
downloads and caches the model, and points Memdoor at it. No secret, no API
account, and nothing leaves the runner. A free runner has 4 CPUs and 16 GB,
so a 9B model is about the largest that fits, and the review is one model
call: no agent, nothing run to prove a finding. A free first pass, not a
reviewer.

## What each one catches

The same three pull requests on [llamafit](https://github.com/guregodevo/llamafit),
a 3,000-line Go project, each reviewed both ways on `ubuntu-latest`
([#6](https://github.com/guregodevo/llamafit/pull/6),
[#7](https://github.com/guregodevo/llamafit/pull/7),
[#8](https://github.com/guregodevo/llamafit/pull/8)):

| Pull request | No key: `qwen3.5:9b` in the runner | Your key: OpenRouter, Memdoor's default model |
|---|---|---|
| An obvious bug: a loop that now skips the last layer | P0 on the right line, but explained by a panic that cannot happen | P0: the last layer is left out of every estimate |
| A subtle bug: the returned path built from the lowercased file name | missed, the job passed | P0: `MODEL.GGUF` comes back as a path that does not exist |
| The same change done correctly | passed, no false P0 | passed, no false P0 |
| Review | 66–78 s | 48–60 s |
| Job | 3½–5 min, most of it the model's first download | about 1 min |

The local model points at an obvious mistake, not always for the right
reason, and stays quiet on correct code. A bug you have to follow through the
code to see needs the agent, which means a key.
Use local where a key cannot go (a public repository you would rather not
put a secret in, a team with no API account yet) and a key where the review
has to be right.

The model is cached per branch, and GitHub shares only a cache saved on the
default branch: run the workflow once on a push to `main` and every pull
request starts from it instead of downloading 6.6 GB. A smaller model is not
faster: `qwen3:4b` wrote ten minutes of review where the 9B wrote one.

## The built-in workflows

| `workflow:` | What it does | The job fails when |
|---|---|---|
| `pr-review` | Reviews the pull request against its base branch and writes **P0** / P1 / P2 findings. | there is a P0: something that must not merge |
| `fix-tests` | Finds the test command (`go test`, `cargo test`, `npm test`, pytest, `make test`, or `MEMDOOR_TEST_COMMAND`), fixes the code until it passes, and leaves the fix in the working tree with a report. Skipped, no model called, when the tests already pass. | the tests still fail |
| `release-notes` | Turns the commits since the previous tag into Added / Changed / Fixed. | — |

`fix-tests` and `release-notes` need a key. Notes on every tag:

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
