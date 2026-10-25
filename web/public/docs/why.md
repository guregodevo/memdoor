# Why Memdoor

Memdoor is a lightweight coding and automation agent built around four
priorities: token optimization, a single binary, a headless-first CLI and
durable workflows. Run it locally or connect over SSH to another machine,
on a gateway you own, using your own provider account.

## 1. Token optimization

A whole file when three functions mattered. Every search hit when two did.
A whole log when ten lines answered. That text stays in the conversation
and can be sent again on later calls, so unnecessary input adds up.

A decision model filters search hits, file sections and logs against the
task, selects the tools the turn needs and checks whether it is still
making progress. The coding model receives less irrelevant input, and a
stalled turn can stop instead of spending more calls going in circles.

The savings are task-dependent, not a blanket discount. Across 44 runs
(22 pairs, decisions off and on), 22 of 22 tasks passed each way. A
codebase question used 74% fewer input tokens over three pairs; an edit
with a test and ten one-file fixes showed no saving (within ±11%). See
the [measurements and harness](https://github.com/guregodevo/memdoor/blob/main/docs/features/DECIDE.md).

## 2. Single binary

Lightweight means low deployment overhead: one CLI binary for macOS,
Linux or Windows, without Docker or Python. It works in your project and
runs a small gateway in the background to keep conversations and jobs.

Use it on your own machine, or connect over SSH to a peer machine or a
remote host with `memdoor --gateway ssh://you@box`. That command installs
Memdoor on the host if needed, starts its gateway and opens the window on
it. Work lives on that host; on a box that stays on, it keeps running when
your laptop disconnects. See [The gateway](/docs/gateway).

## 3. CLI, headless first

Automation does not require an open terminal window or an IDE.
`memdoor run "<prompt>"` accepts a task, writes its answer to stdout and
returns an exit code reflecting its checks: 0 checked, 2 unverified or a
failing check, 3 an unanswered question, 1 an error, 130 interrupted.
Shell scripts, CI and cron can use those results directly.

Every run is a conversation on the gateway, not a disposable terminal
process. `memdoor resume <id>` opens it interactively; your IDE or phone
can reach the same gateway. Start unattended, then step in when needed.
See [Headless, then attached](/docs/headless).

## 4. Durable workflows

Describe the steps once and keep them as task files in your repository.
The engine runs them as a graph: independent steps run in parallel, and
each step is done when its target check passes, not when the model says so.
A gate waits for your approval where you require it.

A failed run resumes at the failed step without repeating completed work.
Run the same workflow by hand, from CI with
`memdoor workflow run <name> --wait`, or on your local cron. The task
files make the automation repeatable and shareable; the checks and run
state make its outcome inspectable. See [Workflows](/docs/workflows).

## Next

- **[What is Memdoor](/docs/what-is-memdoor)** — the product and how it works.
- **[Getting started](/docs/getting-started)** — install to first turn.
- **[Your key and the models](/docs/your-key)** — provider accounts and prices.
