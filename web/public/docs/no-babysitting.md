# The harness

Memdoor checks the agent's work so you do not watch it.

## Who decides: the decision model

A second, small model. It writes no code. It gets one yes-or-no question
at a time with the evidence attached and answers with a probability:

- Is this turn finished, given what its tool calls show?
- Could the agent answer this question itself, by reading or running something?
- Is this run going in circles?
- Did this command fail, even though it exited 0?

Memdoor acts when the probability crosses a threshold you can change,
for example 0.7 to send a turn back. Key: a decision key
(`memdoor connect typesafe`), else the OpenRouter key you code with. On a
vendor key, a company's AI gateway or a ChatGPT plan alone: off until a
decision key is set.

## Every turn ends on a receipt

The last line of a turn that changes code, taken from the tool calls, not
from the model's summary:

```
✓ checked: 2 files changed · `go test ./...` PASS
⚠ unverified: 1 file changed · nothing was built, tested or run after it
⚠ failing after the change: `go build ./...` FAIL
```

A command that exits 0 but prints a failure (a test piped into `tail`)
counts as failing. Reading files is not a check.

## Unfinished work goes back, not to you

Receipt not done (a file changed, nothing ran): the agent gets one more
round with that reason at the top of its prompt. Then the turn ends.

## Questions it can answer itself don't reach you

Before a question reaches you, the decision model is asked whether the
agent could find out by reading, running or checking. If yes, the question
goes back with "find out yourself". A question asked twice gets through.

## A stuck run ends with a summary, not a spin

The same call repeated, or a test failing without getting closer: the turn
ends with what is done, what failed and what to try next.

## Let it run: a token budget

`turn_token_budget` (tokens): the turn continues round after round, each
from where the last said it would go, until the receipt shows done or the
budget is spent. Questions first, then work.

```bash
TOKEN=$(memdoor auth token)
curl -X PUT http://localhost:18789/api/workspace/settings \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"turn_token_budget":"300000"}'
```

## For longer work, a workflow

A turn is one task. Work with steps runs as a [workflow](/docs/workflows),
the deterministic harness: a step is done when its target exists, a failed
run resumes where it stopped, you are asked at the gates you put in.

Without a decision model: receipts show, nothing is judged.

## Next

- **[Headless, then attached](/docs/headless)** — leave a run alone and come back into it live.
