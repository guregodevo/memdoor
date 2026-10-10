# No babysitting

A coding agent you have to watch is a coding agent you have to pay twice:
once in tokens, once in your attention. Memdoor checks the agent's work so
you don't have to sit there.

## Who decides: the decision model

Behind every check on this page is a second, small model whose only job is
to decide. It does not write code or chat. It is asked one yes-or-no question
at a time, with the evidence attached, and answers with a probability:

- Is this turn finished, given what its tool calls show?
- Could the agent answer this question itself, by reading or running something?
- Is this run going in circles?
- Did this command fail, even though it exited 0?

Memdoor acts only when the probability crosses a threshold you can change,
for example 0.7 to send a turn back. The coding model does the work; the
decision model decides whether the work needs you. It runs on your own key,
and you connect one with
`memdoor connect typesafe`.

## Every turn ends on a receipt

When a turn changes code, its last line says what actually happened, taken
from the tool calls, not from the model's summary:

```
✓ checked: 2 files changed · `go test ./...` PASS
⚠ unverified: 1 file changed · nothing was built, tested or run after it
⚠ failing after the change: `go build ./...` FAIL
```

A command that exits 0 but prints a failure (a test piped into `tail`)
counts as failing. Reading files is not a check.

## Unfinished work goes back, not to you

If the receipt says the turn is not done, say a file changed and nothing
ran, the agent gets one more round with that reason at the top of its
prompt, before the turn ends. You see the result, not the excuse.

## Questions it can answer itself don't reach you

Before the agent asks you something, Memdoor asks whether it could find out
by reading, running or checking. If yes, the question goes back to the agent
with "find out yourself". Ask the same thing twice and it gets through.

## A stuck run ends with a summary, not a spin

Repeating the same call, or a test that keeps failing without getting
closer, ends the turn with what is done, what failed and what to try next.

## Let it run: a token budget

Set `turn_token_budget` (a number of tokens) and a turn keeps going on its
own: each round continues from where the last one said it would go next,
until the receipt shows it done or the budget is spent. It asks its
questions first, then works.

```bash
TOKEN=$(memdoor auth token)
curl -X PUT http://localhost:18789/api/workspace/settings \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"turn_token_budget":"300000"}'
```

## For longer work, a workflow

A turn is one task. When the work has steps, run it as a
[workflow](/docs/workflows): each step is done only when its target exists,
a failed run resumes where it stopped, and you are asked only at the gates
you put in.

Without a decision model, the receipts still show and nothing is judged:
you are back to reading them yourself.

## Next

- **[Headless, then attached](/docs/headless)** — leave a run alone and come back into it live.
