# Task

## Prompt

Write answer.md: a 120-word note for a new teammate on how to name a git branch in this team: lowercase, a short type prefix (fix/, feat/, docs/), words joined by hyphens, one example of each.

## Deliverable

The new file `answer.md` (a 120-word note for a new teammate on team branch naming). The tree has no code, tests, or README — only `.memdoor/` — so the run leaves exactly this one file at the repository root.

## Check

`judge` — the deliverable is prose with a word-count constraint and the tree contains no test suite, build, or samples that could verify it mechanically. The judge checks: answer.md exists, is exactly 120 words, and states the three rules plus one example of each.

## Scope

Only `answer.md` (created at the repository root) and the run ledger `.memdoor/runs/team-solve-v2-2026-11-02T203357/`. No other file, the `.git` directory, and the rest of `.memdoor/` stay untouched.

## Done when

Reviewer: answer.md reads as a clear onboarding note — lowercase names, short type prefix (fix/, feat/, docs/), hyphen-joined words — with one example per prefix, in a friendly tone for a new teammate. Judge: answer.md exists at the repo root, is exactly 120 words, contains all three rules and all three examples, and matches the Prompt's wording requirements.
