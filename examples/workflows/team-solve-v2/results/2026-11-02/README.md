# team-solve-v2 runs, 2026-11-02

Four runs of the built-in on a scratch gateway with a fresh HOME (the seed
alone provided the workflow), each folder holding every file the run left:
`task.md`, `plan.md`, `notes.md` (the ledger), the deliverable, `review.md`,
`verdict.md` where the judge ran, the check logs and `run.log` (the progress
lines). The model was named with `--model` for the glm runs; the mini runs
went through an OpenAI vendor key, which the meter does not price.

| run | model | steps | wall time | check | deliverable |
|---|---|---|---|---|---|
| program-gpt-4.1-mini | gpt-4.1-mini | 10 of 10 | 1 min 33 s | 5 samples of 5 pass | `solution.py` |
| document-gpt-4.1-mini | gpt-4.1-mini | 10 of 10 | 2 min 03 s | judge: PASS | `answer.md`, 223 words |
| program-glm-5.3-flash | z-ai/glm-5.3-flash | 10 of 10 | 8 min 19 s | 5 samples of 5 pass | `solution.py` |
| document-glm-5.3-flash | z-ai/glm-5.3-flash | 10 of 10 | 10 min 37 s | judge: PASS | `answer.md`, 150 words |

The program is the sample problem an empty folder gets. The document task:
"Write a 150-word note to a new colleague explaining what a pull request is
and why we use them" (`document-glm-5.3-flash/task.md`), and for the mini run
"why the sky is blue, for a ten-year-old, under 300 words".

The two glm runs together, from `meter-glm-5.3-flash.jsonl` (they share one
partition, so the rows are not split): 114 calls, 1.40 M input tokens, 31.6 K
output tokens, $0.046 at OpenRouter's list price.

Not a benchmark: one problem, one prompt, one pass each. The published
benchmark of the method is the `team-solve` example beside this folder.
