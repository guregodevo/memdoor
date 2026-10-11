# Why Memdoor

## 1. It is the agent you can leave alone

Say the steps once and Memdoor runs them as a workflow: a graph of tasks,
each done when the file it names exists or its command exits 0, never
because the model says so. Independent steps run in parallel. A step marked
for you holds the run with the diff in front of you until you approve or
send it back with a comment. A failed run restarts at the failed step. The
workflow is files in your repository, so it runs again by hand, on a
schedule, or from CI. Memdoor's own release is one of these workflows, and
the first job an outsider gave it, a trading desk's nightly three-step, found
a real engine bug on its first replay. Every plain turn ends on a receipt
too: the files it changed, the checks it ran and what they returned.

## 2. One gateway, every door

A run is a conversation on a gateway on your machine, and every door opens
the same one: the terminal window, the phone, the editor, a schedule, a
workflow from a shell or CI, a spawned child, and `memdoor resume` into any
of them. `memdoor run` is one turn with no window, prompt in and answer out,
and its exit code is the turn's receipt, so a pipeline fails on what the
checks said rather than on whether a process survived. Other agents are a
process in a terminal; the gateway is what lets you start a run in one place
and pick it up in another. See [Headless, then attached](/docs/headless).

## 3. It stops, instead of looping to a cap

A hard call limit is the worst of both: it cuts off turns that were about to
finish, and it cannot rescue the ones going in circles. Memdoor has no cap.
After each step a decision model judges whether the turn is still making
progress and wraps it up with what it has when it is not, so a stuck turn
costs you a few calls instead of fifty.

## 4. Most of what a coding agent bills you for, the model never needed to read

A whole file when three functions mattered. Every grep hit when two did. A
whole log when ten lines answered. Each of those sits in the transcript and
is resent on every later call of the turn, so the waste compounds. The same
decision model sits in front of the one writing your code and passes on only
what the task needs: 74% fewer input tokens on a question about the
codebase, measured over three pairs of runs, and no change where the task
reads the same files either way. Every agent also has a ladder, cheapest
rung first; most turns finish there, and the footer always names the model
that answered and why it was on that rung.

## 5. Your key, your account, your bill

You hold the account at the provider you chose, or a ChatGPT Plus/Pro plan.
Requests go from your machine to that provider at their list price; nothing
of your code passes through memdoor.ai and nothing is added to what you pay.
It is also why `/model` shows real prices rather than a band or a credit
balance: you are the one paying them, so you should see them.

## 6. Measured, not claimed

Every number here comes from paired runs, the same tasks with decisions off
and on, alternating, with the answers checked rather than eyeballed: every
edit had to pass `go test` and `go vet` on its own copy of the repository.
Twenty-two of twenty-two passed each way. When something we tried showed no
saving, it was reverted and written down as not working rather than shipped;
the rows and the harness are in the repository.

## 7. Privacy is a request header, not a promise

Every request says `data_collection: deny`, and hosts that train on paid
inputs are excluded from every ladder (a `:free` model you pin is the
exception, and the pin says so). Your files, sessions, memory and the output
of every command the agent runs stay on your machine. What leaves is what a
model must read to answer, and with the decision model on, that is a
fraction of what it would otherwise be. Turn decisions off and nothing is
judged off-machine at all.

## 8. One binary

No Docker, no Python, no app, no editor plugin to keep in sync. One
command-line binary, working in the project directory you launch it from,
on macOS, Linux and Windows.

## Next

- **[What is Memdoor](/docs/what-is-memdoor)** — the short answer.
- **[Headless, then attached](/docs/headless)** — every door, with the commands.
- **[Getting started](/docs/getting-started)** — install to first turn.
