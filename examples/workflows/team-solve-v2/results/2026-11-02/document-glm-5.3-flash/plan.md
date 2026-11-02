deliverable: answer.md

# Task
Write a 150-word note to a new colleague explaining what a pull request is
and why we use them. Friendly tone. No jargon beyond the words "branch"
and "review" — those two words are allowed, everything else must be plain
language or must be explained in the note itself.

## Requirements the deliverable must meet
1. Deliverable file is answer.md (no tests/N.in,N.out exist, and the task
   asks for a written note, not a program).
2. Length: 150 words. Interpret this as the note body being exactly 150
   words (word = whitespace-separated token, counting the words of the
   note itself, not any heading the file might carry). Prefer exact;
   149-151 is a hard fail per the strictest reading, so aim for exactly
   150. State the final count in notes.md so the reviewer can re-verify.
3. Tone: friendly, addressed to a new colleague. No condescension, no
   corporate boilerplate.
4. Jargon budget: only "branch" and "review" may be used as terms of art.
   Everything else (e.g. "diff", "merge", "CI", "approve", "PR") is out
   unless the note itself explains it in plain words in the same breath.
   "Pull request" itself is the subject of the note, so it is fine.
5. Content must answer both halves: (a) what a pull request IS,
   (b) WHY we use them. An explanation that covers only one half is
   incomplete.
6. Self-contained: answer.md alone must satisfy the task; the reader
   should not need plan.md or notes.md.
7. Format: plain markdown, a short salutation and a sign-off are fine
   and count toward the word budget unless kept minimal.

## Candidate approaches

### Proposal A: conversation-first, one extended analogy
Open with the "look at my work before it goes in" idea, then ground it in
branch/review vocabulary, ending on why it makes the team's work safer
and shared. Risk: analogy may eat words needed for the "why".

### Proposal B: two-part structure, what-then-why
One short paragraph on the what (a way to show your changes on a branch
and get review before they're joined into the main work), one on the why
(mistakes caught early, knowledge shared, main copy stays trustworthy).
Lowest risk of missing a required half; the cost is a slightly less
conversational flow.

### Proposal C: story-first (a tiny worked example)
Walk a mini-example ("you finish your fix on a branch, open the request,
a teammate reads it...") and draw the moral at the end. Most engaging,
highest risk of blowing the word budget or slipping in extra jargon.

## Decomposition
- Ideators (2): independently draft the note under their own marker in
  notes.md, following Proposal A and Proposal B respectively (Proposal C
  is optional exploration). Each must state the exact word count of
  their draft and self-check the jargon budget.
- Solver: pick the better draft (default: B for structural safety,
  unless A reads notably better), polish it to exactly 150 words, write
  it to answer.md, and record the decision and word count in notes.md.
- Reviewer: recount words, scan for jargon violations against the list
  in the requirements, check friendliness and both halves of the content,
  and log findings in notes.md. Any fix goes under Repair.

## Instructions for ideators
Work only from plan.md and task.md. Do not read each other's entries.
Append at your own marker (<!-- proposal-a --> / <!-- proposal-b -->)
with a small patch; never overwrite the ledger. Include your draft and
a one-line word count.

## Instructions for the solver
1. Read both proposals in notes.md.
2. Write the final note to answer.md (self-contained, exactly 150 words
   by whitespace-token count of the note body).
3. Verify the count with: wc -w on the note body.
4. Record choice, final count, and any trims in notes.md under Solver
   decision.
