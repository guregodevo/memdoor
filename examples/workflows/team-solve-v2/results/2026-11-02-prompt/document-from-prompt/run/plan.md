# Plan — team-solve-v2 run 2026-11-02T203357

## Task in one line

Write `answer.md`: a 120-word onboarding note teaching a new teammate the
team's git branch naming convention — lowercase, a short type prefix
(`fix/`, `feat/`, `docs/`), words joined by hyphens — with one example of
each.

## Decomposition

1. Draft the three rules in plain language (rule source: the Prompt itself;
   the repo is empty, so nothing more to discover).
2. Write one example branch name per prefix: `fix/…`, `feat/…`, `docs/…`.
3. Assemble into a friendly note addressed to a new teammate.
4. Trim or expand the wording until the word count is exactly 120
   (count with a word-count command before handing over).
5. Reviewer reads it for clarity and tone; judge checks the hard facts.

## Candidate approaches

### Proposal A — rules-first checklist

A compact intro line, then three bullet rules (lowercase; short type
prefix `fix/`/`feat/`/`docs/`; hyphen-joined words), each bullet followed
by its example, and a one-line sign-off. Tradeoffs: very easy for the
judge to verify (all rules and examples are greppable), the most
information per word; slightly drier in tone than a narrative note, and
the bullets cost words.

### Proposal B — narrative memo

Flowing paragraphs: why the convention exists, then the rules woven into
prose, examples inline in backticks. Tradeoffs: reads most naturally for
a new teammate; hardest to keep at exactly 120 words and the easiest place
to accidentally drop one rule, which the judge would catch.

### Proposal C — memo + cheat-sheet footer

Short prose intro, then a mini table/cheat sheet of prefix → example.
Tradeoffs: most scannable long-term, but the highest word overhead; risk
of exceeding 120 words or having the intro and sheet drift out of sync.

## Recommended

Proposal A with a warm one-line intro and sign-off so it still reads as a
note, not a spec. Highest judge-pass probability per word.

## Check run on the current tree

`judge` — nothing can be run: the tree has no code, no tests, no samples
(only `.memdoor/` and `.git`). The solver should still verify the word
count mechanically before finishing (e.g. count the words of answer.md)
and record the count in notes.md under Solver decision.

## Instructions — independent ideators

Two ideators each propose a full draft outline (not the final text) under
their marker in notes.md: the structure, the three example branch names,
and how they will land on exactly 120 words. Do not read the other
marker before writing. Keep proposals small patches inserted at your own
marker.

## Instructions — solver

Read both proposals and the Prompt. Pick one (or a blend) and write
`answer.md` at the repo root: exactly 120 words, all three rules, one
example per prefix (`fix/`, `feat/`, `docs/`), hyphen-joined lowercase
words, tone for a new teammate. Verify the word count with a command and
log the decision + count in notes.md under Solver decision. Do not touch
any file outside `answer.md` and the run directory.

## Instructions — reviewer

Check `answer.md` against the Prompt: exactly 120 words; lowercase; a
short type prefix rule; hyphen-joined words; one example for each of
`fix/`, `feat/`, `docs/`; reads as a note for a new teammate, not a spec.
Log the verdict under Review in notes.md. If it fails, state precisely
what to repair; Repair is logged under Repair.
