# Why Memdoor

Memdoor is a coding and automation agent built around four priorities:
token optimization, a single binary, a headless-first CLI and durable
workflows. It runs on your machine or over SSH on another, on your key.

## 1. Token optimization

A model is often sent a whole file when three functions mattered, every
search hit when two did, or a whole log when ten lines answered. That text
stays in the conversation and is sent again on later calls.

A decision model filters search hits, file sections and logs against the
task, selects the tools the turn needs and checks whether it is
making progress. The coding model receives less irrelevant input, and a
stalled turn can stop instead of spending more calls going in circles.

The saving depends on the task. Across 44 runs
(22 pairs, decisions off and on), 22 of 22 tasks passed each way. A
codebase question used 74% fewer input tokens over three pairs; an edit
with a test and ten one-file fixes showed no saving (within ±11%). See
the [measurements and harness](https://github.com/guregodevo/memdoor/blob/main/docs/features/DECIDE.md).

## 2. Single binary

One binary for macOS, Linux or Windows, without Docker or Python. It
works in your project and runs a small gateway in the background that
keeps conversations and jobs.

Run it here, or connect over SSH to another machine with `memdoor --gateway ssh://you@box`. That command installs
Memdoor on the host if needed, starts its gateway and opens the window on
it. Work lives on that host; on a box that stays on, it keeps running when
your laptop disconnects. See [The gateway](/docs/gateway).

## 3. CLI, headless first

Automation does not require an open terminal window or an IDE.
`memdoor run "<prompt>"` accepts a task, writes its answer to stdout and
returns an exit code reflecting its checks: 0 checked, 2 unverified or a
failing check, 3 an unanswered question, 1 an error, 130 interrupted.
Shell scripts, CI and cron can use those results directly.

A run is a conversation on the gateway. `memdoor resume <id>` opens
it interactively, and your IDE or phone can reach that gateway, so a
run can start unattended and be joined later.
See [Headless, then attached](/docs/headless).

## 4. Durable workflows

A workflow is the deterministic harness. Describe the steps once. The engine runs them as a graph: independent steps run in parallel, and
a step is done when its target check passes, whatever the model says. A
gate waits for your approval where you put one.

A failed run resumes at the failed step.
Run the workflow by hand, from CI with
`memdoor workflow run <name> --wait`, or on your local cron. The checks
and the run state show what happened. See [Workflows](/docs/workflows).

## Next

- **[What is Memdoor](/docs/what-is-memdoor)** — the product and how it works.
- **[Getting started](/docs/getting-started)** — install to first turn.
- **[Your key and the models](/docs/your-key)** — provider accounts and prices.
