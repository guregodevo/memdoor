# Why Memdoor

## 1. Most of what a coding agent bills you for, the model never needed to read

A whole file when three functions mattered. Every grep hit when two did. A
whole log when ten lines answered. Each of those sits in the transcript
and is resent on every later call of the turn, so the waste compounds. Memdoor
puts a decision model in front of the one writing your code and passes on only
what the task needs: 74% fewer input tokens on a question about the codebase,
measured over three pairs of runs, with the same pass rate on everything else.

## 2. A cheap model does most of the work, if something decides when it cannot

Every agent has a ladder, cheapest rung first. Most turns finish there. The rungs above exist for the
turns that need them, and the footer always names the model that answered and
why it was on that rung. No opaque "auto" mode deciding for you.

## 3. It stops, instead of looping to a cap

A hard call limit is the worst of both: it cuts off turns that were about to
finish, and it cannot rescue the ones going in circles. Memdoor has no cap.
After each step the decision model judges whether the turn is still making
progress and wraps it up with what it has when it is not, so a stuck turn costs
you a few calls instead of fifty.

## 4. Your key, your account, your bill

You hold the account at the provider you chose. Requests go from your machine
to that provider at their list price; nothing of your code passes through memdoor.ai and nothing is
added to what you pay. It is also why `/model` shows real prices rather than a
band or a credit balance: you are the one paying them, so you should see them.

## 5. Measured, not claimed

Every number here comes from paired runs — the same tasks, decisions off and on,
alternating — with the answers checked rather than eyeballed: every edit had to
pass `go test` and `go vet` on its own copy of the repository. Twenty of twenty
were right. When something we tried showed no saving, it was reverted and written
down as not working rather than shipped.

## 6. Privacy is a request header, not a promise

Every request says `data_collection: deny`, and hosts that train on paid inputs
are excluded from every ladder (a `:free` model you pin is the exception, and
the pin says so). Your files, sessions, memory and the output of
every command the agent runs stay on your machine. What leaves is what a model
must read to answer — and with the decision model on, that is a fraction of what
it would otherwise be. Turn decisions off and nothing is judged off-machine at
all.

## 7. One binary

No Docker, no Python, no app, no editor plugin to keep in sync. One
command-line binary, working in the project directory you launch it from.

## Next

- **[What is Memdoor](/docs/what-is-memdoor)** — the short answer.
- **[Your key and the models](/docs/your-key)** — the ladder, pins, prices.
- **[Getting started](/docs/getting-started)** — install to first turn.
