# Why Memdoor

Four priorities: token optimization, a single binary, a headless-first CLI,
durable workflows. Runs on your machine or over SSH on another, on your key.

## 1. Token optimization

The waste: a whole file when three functions mattered, every search hit
when two did, a whole log when ten lines answered. That text stays in the
conversation and is resent on later calls.

The fix: a decision model filters search hits, file sections and logs
against the task, selects the tools the turn needs and checks progress.
A stalled turn stops instead of spending more calls.

Measured: 44 runs, 22 pairs with decisions off and on, 22 of 22 tasks
passed each way. A codebase question: 74% fewer input tokens over three
pairs. An edit with a test and ten one-file fixes: no saving (within ±11%).
[Measurements and harness](https://github.com/guregodevo/memdoor/blob/main/docs/features/DECIDE.md).

## 2. Single binary

One binary for macOS, Linux or Windows. No Docker, no Python. Runs in your
project with a small gateway in the background that keeps conversations
and jobs.

Remote: `memdoor --gateway ssh://you@box` installs Memdoor on the host if
needed, starts its gateway and opens the window on it. Work lives on the
host and continues when the laptop disconnects. See [The gateway](/docs/gateway).

## 3. CLI, headless first

`memdoor run "<prompt>"` takes a task, writes the answer to stdout and
exits with a code from its checks: 0 checked, 2 unverified or a failing
check, 3 an unanswered question, 1 an error, 130 interrupted. Usable from
shell scripts, CI and cron.

A run is a conversation on the gateway. `memdoor resume <id>` opens it in
a window; the IDE and the phone reach the same gateway. Start unattended,
join later. See [Headless, then attached](/docs/headless).

## 4. Durable workflows

A workflow is the deterministic harness: steps run as a graph, independent
steps in parallel, a step done when its target check passes, whatever the
model says. A gate waits for approval where you put one. A failed run
resumes at the failed step. Run it by hand, from CI with
`memdoor workflow run <name> --wait`, or on local cron. See [Workflows](/docs/workflows).

## Next

- **[What is Memdoor](/docs/what-is-memdoor)** — the product and how it works.
- **[Getting started](/docs/getting-started)** — install to first turn.
- **[Your key and the models](/docs/your-key)** — provider accounts and prices.
