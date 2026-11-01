<div align="center">
  <img src="web/public/icon-512.png" alt="" width="72" height="72">
  <h1>Memdoor</h1>
  <p><strong>Your daemon agents on every door.</strong><br>A coding and automation agent in one CLI binary. No Docker or Python. Runs on this machine or over SSH on another. Clients: terminal, IDE, phone, script, CI, schedule. On your gateway. On your key.</p>
  <p>A headless CLI with multi-agent harness</p>
  <p>
    <a href="https://news.ycombinator.com/item?id=50320995"><img alt="Hacker News" src="https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fhacker-news.firebaseio.com%2Fv0%2Fitem%2F50320995.json&query=%24.score&label=Hacker%20News&suffix=%20points&logo=ycombinator&logoColor=white&color=ff6600"></a>
    <a href="LICENSE"><img alt="Apache 2.0" src="https://img.shields.io/badge/license-Apache%202.0-blue"></a>
    <a href="https://github.com/guregodevo/memdoor/actions/workflows/lint.yml"><img alt="CI" src="https://github.com/guregodevo/memdoor/actions/workflows/lint.yml/badge.svg"></a>
    <a href="https://github.com/guregodevo/memdoor/actions/workflows/installers.yml"><img alt="Installers" src="https://github.com/guregodevo/memdoor/actions/workflows/installers.yml/badge.svg"></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-one%20static%20binary-00ADD8">
    <img alt="Platforms" src="https://img.shields.io/badge/macOS%20%C2%B7%20Linux%20%C2%B7%20Windows-lightgrey">
  </p>
</div>

> **Five Qwen3.8-27B agents beat Fable 5 on LiveCodeBench Hard, as a Memdoor workflow.**
> [GVS5H](https://github.com/slee-persis/GVS5H)'s harness rebuilt with the
> authors' exact prompts and loop. The first ten problems of their list, five
> passes each: a mean of **9.8 of 10**. The authors' own harness scored 9.6 and
> Fable 5 in a single call 8.0, the gap the authors measured on these ten
> (across their full 100: 92.4 and 90.4). All 50 runs are published: every
> request, reply, program and grader output. The workflow is YAML and names no
> model. Point it at yours, or at a local one with `memdoor connect local`
> (Ollama, llama.cpp, vLLM, LM Studio; [how to run it on an open-weight model,
> no key](https://memdoor.ai/docs/open-weight)).
> [examples/workflows/team-solve](examples/workflows/team-solve/) · [the result page](https://memdoor.ai/docs/team-solve) · [the HN thread](https://news.ycombinator.com/item?id=50320995)

On this machine:

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
export OPENROUTER_API_KEY=sk-or-...    # or ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, …
# or no key: memdoor connect local    # an open-weight model on this machine — https://memdoor.ai/docs/open-weight
cd your-project && memdoor             # the first run sets the machine up
```

On a box reachable by ssh, nothing installed there:

```bash
curl -fsSL https://memdoor.ai/install.sh | bash          # here, once
memdoor --gateway ssh://you@box                           # installs memdoor on the box, starts its gateway, opens the window on it
memdoor --gateway ssh://you@box connect openrouter        # your key, kept on the box; every turn there runs on it
memdoor --gateway ssh://you@box run "fix the failing test" # the same box from a script or CI
```

Runs live on the box and continue when the laptop disconnects. The phone or
the next terminal opens the same conversation there.

No account. Telemetry off by default. One static binary for macOS, Linux and
Windows. Apache 2.0.

### In GitHub Actions

One step. Your key as the secret. Nothing added to the repo
([Marketplace](https://github.com/marketplace/actions/memdoor)):

```yaml
on: pull_request
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

Built in:

- **`pr-triage`**: ready for review, needs work, or likely slop, from checks run in the job (the tests, claimed tests that are not there, functions nothing calls).
- **`pr-review`**: findings verified by running the code; the job fails only on a blocker.
- **`security-review`**: secrets, injection, path traversal, missing authorization.
- **`write-tests`**: tests for what the change touches; the job fails when one shows the change is wrong.
- **`pr-description`**: title, What, Why, How to test, Risk.
- **`explain-failure`**: a failed run's logs and the diff; the cause, file:line, the fix.
- **`issue-triage`**: labels, kind, summary, duplicates.
- **`fix-tests`**: finds the test command, fixes the code until the tests pass, leaves the fix in the working tree.
- **`release-notes`**: commits since the last tag as Added / Changed / Fixed.

`local-model: Qwen3-8B-4bit` in place of the key: no key, the model inside the
runner ([janis](https://github.com/guregodevo/janis), one file, cached), one
call. On a free runner it found an obvious and a subtle bug in about a minute
each. It can flag a correct change as a blocker. A first pass.

`workflow: <name>` runs yours from `.memdoor/workflows/<name>/`. Exit code:
0 done, 2 failed, 1 stopped, 3 stopped at an approval gate. The job summary
holds the review, the fix or the notes. Trigger on `pull_request`, never
`pull_request_target`: a fork's pull request gets no secrets, so a stranger's
code never runs beside your key.

Keys: a ChatGPT Plus or Pro plan signs in with `memdoor connect chatgpt` (or
`/connect chatgpt`) and its allowance answers the turns. A Claude subscription
cannot be used this way (Anthropic's terms); Claude needs an API key. With no
key, the first `memdoor` signs in with OpenRouter in the browser (or ChatGPT),
from this machine, over ssh or on a box. An account with no credit runs on a
free model: 50 requests a day, and the free hosts train on what they are sent.
For a trial, not private code.

`memdoor connect local` connects a model running on this machine
([janis](https://github.com/guregodevo/janis), Ollama, llama.cpp, LM Studio).
Nothing leaves it. janis: one Go server, open models on a Mac's GPU through
MLX or an NVIDIA box through CUDA, serves chat and the decision model, no key.
Runs Qwen3-Coder-30B-A3B on a $0.71-an-hour Google L4.
`memdoor --gateway ssh://you@box connect local` connects the box's runtime.
`memdoor peers add box http://box:18789 --dir ~/app` names the box;
`@box:coder …` in any conversation runs the turn there and returns the answer.
A recording on memdoor.ai: a laptop with no GPU, a turn on Qwen3.8-27B on an
L4 through janis, the answer in the laptop's window.

<p align="center"><img src="docs/img/memdoor-window.png" alt="The window: a task typed, the files read and patched, the tests run, and the receipt line: checked, 2 files changed, tests PASS" width="880"></p>

Every run is a conversation on a gateway on your machine, from any door: a
terminal window, `memdoor run` from a script or CI, a cron schedule, the
editor, the page on the phone. `memdoor resume <id>` opens the run a CI job
started. Doors and exit codes: [the walk through
them](https://memdoor.ai/docs/headless).

```bash
memdoor run "fix the failing test"        # one turn, no window; the exit code is the receipt of its checks
memdoor workflow run release --wait       # a workflow from a shell or CI, exit code the run's outcome
memdoor resume --last                     # the same run, in a window
```

---

`memdoor` reads files, writes patches, runs the build, fixes what it broke, in
the directory it was launched from. Model calls go from your machine to the
connected provider at list price.

**A decision model in front of the chat model.** Jev (TypeSafe) answers a
typed question such as *is this hit relevant to the task?* with a calibrated
probability in under half a second. Judged reads: search, file and log reads
return what counts. Per-turn toolbox: only the tools the turn needs are sent.
Stall detection: a turn that stops making progress ends instead of looping to
a cap. 44 paired runs, decisions off and on: 22 of 22 passed each way; a
codebase question 54,661 to 14,367 input tokens (−74%); an edit with a test
and ten one-file fixes read the same files either way (within ±11%)
([`docs/features/DECIDE.md`](docs/features/DECIDE.md)).

**Workflows, the deterministic harness.** Say the steps; the coder writes one
YAML per step. [`mario`](https://github.com/guregodevo/mario) runs them as a
graph. Independent steps run in parallel. A step is done when its check
passes, never on the model's word. A gate waits for your approval with the
diff. A failed run resumes at the failed step. By hand, from CI or on local
cron ([`docs/features/WORKFLOWS.md`](docs/features/WORKFLOWS.md)). Workflows
rather than skills: [the
write-up](https://guregodevo.github.io/2026/10/06/workflows-not-skills/).
Worked examples, each run before it was added:
[`examples/workflows/`](examples/workflows/). The step that grades a trading
rule nightly (`examples/workflows/nightly-rule/steps/grade.yaml`), no code:

```yaml
type: agent
prompt: |
  Today's partition is {{.partition}}.
  Read rules/{{.partition}}.md and grade today's trading rule: assess it
  briefly (clarity, risk profile, plausibility) and write a JSON object
  {"score": <0-100 integer>, "verdict": "<one sentence>"} into
  grades/{{.partition}}.json (create grades/ if needed).
requires:
  - table_pattern: build
target:
  # run by the engine, not by the agent: the grade must exist and be newer than step 1's start stamp
  command: test "grades/{{.partition}}.json" -nt "rules/{{.partition}}.start" && test -s "grades/{{.partition}}.json"
timeout: 10m
```

A recorded session of the review loop (findings verified by running, a gate,
the fixes applied): [memdoor.ai/workflows](https://memdoor.ai/workflows).

## Install

```bash
curl -fsSL https://memdoor.ai/install.sh | bash
export OPENROUTER_API_KEY=sk-or-...    # or ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, … or /connect inside
cd your-project && memdoor             # the first run sets the machine up
```

macOS (Apple Silicon and Intel), Linux (amd64, arm64; static), Windows
(`irm https://memdoor.ai/install.ps1 | iex`; commands run in Git Bash when
installed, otherwise PowerShell). One binary in `~/.local/bin`. No sudo, no
Docker, no Python. The installer ([`scripts/install.sh`](scripts/install.sh))
verifies the SHA-256 of the download.

From a clone with Go 1.25:

```bash
git clone https://github.com/guregodevo/memdoor && cd memdoor
make build                             # ./memdoor, stamped with the tag it was built from
cp memdoor ~/.local/bin/               # or anywhere on your PATH
```

## In the window

| Command | What it does |
|---|---|
| `/model` | Which model answers: the agent's ladder (cheapest rung first), or pin any model of your providers, with list prices |
| `/connect` | Add a provider: OpenRouter, Anthropic, OpenAI, Gemini, DeepSeek, Baseten, Groq, xAI, any OpenAI-compatible endpoint, your company's AI gateway; `/connect local` a runtime on this machine or the box (Ollama, llama.cpp, janis), no key; `/connect chatgpt` signs in with a ChatGPT Plus/Pro plan, no key |
| `/workflow` | The project's workflows and runs; enter opens a run's graph, `a` approves a gate, `/workflow:<name>` runs one |
| `/usage` | This month on your key, per model, and what the decision model kept out of the bill |
| `/remote` | This conversation on your phone: a link and a QR code, the terminal itself, end-to-end encrypted |
| `/remote view` | A watch-only link: whoever opens it sees the terminal live and cannot type, no account, up to 200 at once; `?live=1` is the terminal alone, for a stream or OBS |
| `/share` | A read-only link to the conversation, encrypted, secrets removed; `/unshare` deletes it |
| `/files` or `ctrl+f` | The project's files, the ones this conversation read or changed first, each with what happened to it; enter previews, `ctrl+d` diffs, `ctrl+e` opens in `$EDITOR`, tab puts `@path` in the prompt |
| `/graph` or `ctrl+g` | This turn as a graph: the reads, the edits that depended on them, the runs that depended on the edits, the failing check in red |
| `/mcp` | Connect MCP servers from a URL, a command or a `.mcp.json` snippet |
| `/help` | The rest |

`Esc` interrupts. `ctrl+f` files panel, `ctrl+g` the turn as a graph,
`ctrl+o` expands tool frames, `Shift+Tab` reasoning effort, `@path` mentions a
file. `MEMDOOR_APPROVE=changes` asks before every command and write.

## In your editor

`memdoor acp` serves the coder over the [Agent Client
Protocol](https://agentclientprotocol.com). Zed and JetBrains host it
natively. VS Code: an ACP client extension (for example "ACP Client") and one
setting:

```json
"acp.agents": { "Memdoor": { "command": "memdoor", "args": ["acp"] } }
```

An editor session is a conversation on the local gateway. A prompt is a turn
in the editor's workspace folder. Text, tool calls and approvals show as the
editor's own. Steps: [memdoor.ai/docs/editor](https://memdoor.ai/docs/editor).

## From a script

```bash
memdoor run "make the retry wait configurable and add a test"     # a headless turn, no window
git diff | memdoor run "review this diff; one line per finding"    # stdin is the input
memdoor run --json "fix the failing test" | jq 'select(.event=="done")'
memdoor workflow run release --wait                                 # progress on stderr, results on stdout
memdoor remote --last                                               # the link /remote gives, on stdout; the gateway holds the run for your phone
memdoor remote --view --last                                        # the watch-only link: an audience, or a pair of eyes
```

Exit code: 0 checked, 2 unverified or a failing check, 3 a question nobody
answered (`--yes` answers it), 1 an error, 130 interrupted. The gateway starts
if none is running. Every run is a conversation `memdoor resume` opens.
[Headless, then attached](https://memdoor.ai/docs/headless).

A workflow in GitHub Actions is one step: [In GitHub Actions](#in-github-actions) above.

## Where your data goes

Files, sessions and commands stay on your machine. What the model reads goes
to the connected provider. On OpenRouter every request carries
`data_collection: deny`; hosts that train on paid inputs are kept out of the
ladders. Exception: a `:free` model, pinned by you or answering for an account
with no credit. Its hosts train on what they are sent; the request says so
(`allow`) and Memdoor warns at the pin or at connect. For a trial, not private
code. On a vendor key or a company gateway nothing else is contacted.
[`docs/SECURITY.md`](docs/SECURITY.md) lists the hosts the binary can name; a
test fails the build on a new one.

## Documentation

- [Getting started](GETTING_STARTED.md)
- [Workflows](docs/features/WORKFLOWS.md) · [Scheduled checks](docs/features/cron-jobs.md) · [MCP servers](docs/features/MCP.md)
- [Providers](docs/features/PROVIDERS.md) · [Security](docs/SECURITY.md) · [Decisions](docs/features/DECIDE.md)
- [CLI reference](docs/reference/CLI.md) · [Architecture](docs/reference/ARCHITECTURE.md) · [Skills](docs/reference/SKILLS.md) · [ADRs](docs/adr/)
- The served docs: [memdoor.ai/docs](https://memdoor.ai/docs/getting-started)

## Repository layout

```
cmd/cli/       the CLI (cobra): tui, model, workflow, cron, account, …
gateway/       the local gateway: agent runtime, tools, providers, workflows
pkg/           the domain: decision, metering, plan, workflow (the mario task factory), …
web/           the memdoor.ai site and the served docs (React, Vite)
scripts/       install.sh, install.ps1, deploy.sh
docs/          features, reference, ADRs
examples/      workflows built by the coder and run, to copy into a project
```

## License

Apache License 2.0: [LICENSE](LICENSE), [NOTICE](NOTICE). The public repository is
[github.com/guregodevo/memdoor](https://github.com/guregodevo/memdoor). Issues
go there.
