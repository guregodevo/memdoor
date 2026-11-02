verdict: FAIL

## How I judged

`test-status.txt` reads `none` — nothing mechanical could be run on this
tree (no code, tests, or samples; only `.memdoor/` and `.git`), so per the
task I judged the change against `task.md` and the Prompt directly, using
the run's own documented counting method (`wc -w`, per plan.md and the
Solver decision in notes.md).

## Findings

1. **Word count is 110, not 120 — the Done-when line "answer.md … is
   exactly 120 words" is missed.** Verified with `wc -w answer.md`, which
   prints `110 answer.md`. The plan and the Solver decision both commit to
   exactly 120 verified by command; the check line in task.md says the
   same. This alone fails the run.
2. **All three rules are present and correct.** The note states lowercase
   ("Branch names stay all lowercase, never capitals and never
   underscores"), the short type prefix (naming `fix/`, `feat/`, `docs/`
   explicitly), and hyphen-joined words ("Separate words with hyphens,
   never spaces"). No rule is missing or misstated.
3. **All three examples are present, one per prefix, hyphen-joined and
   lowercase:** `fix/login-crash`, `feat/export-csv`, `docs/api-reference`.
4. **Tone is right.** Friendly welcome ("Welcome aboard! We're really
   glad you're here") and sign-off ("That's it — welcome to the team!")
   wrap the checklist; it reads as a note for a new teammate, not a spec.
5. **Scope is clean.** `changes.diff` touches only `answer.md`; the run
   directory holds the ledger. No file outside the Scope was created or
   modified. The change does only what the task asked.

## Conclusion

The content is fully correct; the only defect is the word count
(110 where 120 is required — 10 words short). Everything else passes.

## Repair needed (for the next fresh worker)

Add exactly 10 words to `answer.md` without dropping a rule or an example,
then re-run `wc -w answer.md` until it prints `120 answer.md`. Concrete
suggestions, in decreasing order of preference:

- Expand the examples line: "Some examples of the convention in action:
  `fix/login-crash`, `feat/export-csv`, and `docs/api-reference` — three
  prefixes, three everyday cases." (+8 words vs the current line).
- Expand the hyphen bullet by one clause, e.g. "…at a glance in any
  branch list or log view." (+7 words there, then trim elsewhere).
- Preferred: adjust the examples line (+8) and add two words to the
  welcome ("Welcome aboard — we're really glad you're finally here with
  us" style filler is discouraged; prefer a second short welcoming
  clause that adds information, such as naming where branches show up).

After editing, verify mechanically: `wc -w answer.md` must print exactly
`120 answer.md`, and re-grep that `lowercase`, `hyphen`, `fix/`, `feat/`,
`docs/` and each full example still appear. Then append the new count
under Repair (second pass) in notes.md.

## Verification of this review

After writing, confirm review.md exists on disk and its first line is
`verdict: FAIL`, and that the Review and Repair sections were appended to
notes.md.
