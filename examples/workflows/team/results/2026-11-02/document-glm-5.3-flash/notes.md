# notes.md — team-solve-v2, run 2026-11-02T194923

Task: 150-word friendly note to a new colleague explaining what a pull
request is and why we use them. Deliverable: answer.md (see plan.md).

Ledger rules: append at your own heading via a small patch; never overwrite
another section; the two HTML comment markers below are reserved anchors
for the two ideators and must each appear exactly once in this file.

## Proposal A
<!-- proposal-a -->

**Proposal A draft: conversation-first, one extended analogy.**

Word count: exactly 150 (verified with `wc -w` on the note body; the draft
below is the entire body — salutation and sign-off included in the count).

Jargon self-check: contains only "branch", "review", and "pull request"
(the subject, allowed). No "diff", "merge", "CI", "approve", "PR",
"commit", "main", or any other term of art. The one mild figure of speech,
"two pairs of eyes", is plain language, not jargon.

Both halves covered: (a) what — a way to say "look at my work before we
use it", done on a branch with a teammate's review; (b) why — extra eyes
catch bugs early, the shared copy stays dependable, and knowledge of the
change spreads. Self-contained, friendly, addressed to a new colleague.

--- begin draft ---
Hi, and welcome!

Here's the friendliest way I can think of to explain it. A pull request is like saying, "I've done some work — could you look at it before we use it?" You do your work on a branch, which is a private copy where mistakes can't hurt anyone. When it's ready, you ask for a review: a teammate reads what you changed, asks questions, and catches problems while they're still small.

Why do it this way? Two reasons. First, two pairs of eyes beat one — the silly bug you stared past all afternoon is obvious to a fresh reader. Second, it keeps our shared copy dependable, because only looked-at work ends up in it. It's also how knowledge travels: after the review, someone else understands your change too.

That's the whole idea. Ask me anything — I'm happy to sit with you through your first one!
--- end draft ---

Note for the solver: the em-dashes inside sentences are plain text, not
word separators that could confuse a naive counter — `wc -w` (which splits
on whitespace) already counts this draft as exactly 150, which matches the
plan's stated counting rule (whitespace-separated tokens).

(placeholder — ideator A appends here)










## Proposal B
**Alternative: a short conversation-first note built around one central
idea — "nothing joins the shared work unseen" — rather than a formal
definition.** This differs from a two-part what-then-why structure by
leading with a single plain sentence a newcomer can repeat back, then
unfolding branch and review as the two roles in that idea, and closing on
team safety. The why is carried by the framing (mistakes caught early,
knowledge shared, main version trustworthy) rather than by a bulleted list,
keeping the tone personal.

Draft (word count of the note body: 131 by wc -w; solver can expand to
exactly 150 by adding one or two plain-language sentences, e.g. a line on
questions being welcome or on changes staying easy to fix while small):

---
Hi, and welcome aboard!

Here's the short version of pull requests, which you'll see every day here.

A pull request is a way of saying: "here are changes I made on a branch —
please look at them before they become part of our shared work." A branch is
just your own private copy where you can build and test safely, without
touching what everyone else relies on.

Why we use them: nobody's code goes in unseen. A teammate reads your changes
while they're still easy to fix, catches mistakes early, and learns what
you've been working on. It also keeps our main version trustworthy, so anyone
can build on it without fear.

It's not about formality — it's how we look out for each other. Ask questions
anytime!

— Sam
---

Self-checks:
- Jargon budget: uses only "branch", "review"-adjacent plain words ("look at
  them", "reads"), and "pull request" itself. Words avoided entirely: diff,
  merge, CI, approve, PR. Every non-allowed term is explained in the same
  breath (branch = "your own private copy...").
- Both halves covered: the quoted sentence defines what a pull request is;
  the "Why we use them" paragraph answers why. Neither half is a fragment.
- Tone: friendly, addressed to the new colleague, ends with an open door.
- Risks of this angle: the closing sentence ("It's not about formality") is
  opinion-flavored; if the reviewer wants strictly neutral tone, swap it for
  a sentence restating the safety benefit. The word count is short of 150 by
  design (131) so the solver polishes to exact; expansions should go into
  the why paragraph to keep both halves balanced.
- Verification command for the solver/reviewer: `wc -w` on the note body
  (excluding any heading line); must report exactly 150.
<!-- proposal-b -->

(placeholder — ideator B appends here)










## Solver decision

**Choice: Proposal B as the base, with Proposal A's warm closing line folded
in.** B's what-then-why structure guaranteed both required halves (what a
pull request is / why we use them), and its own draft flagged its weak spots
(131 words, opinion-flavored closer), which made trimming/expanding
mechanical. A's draft was exactly 150 but carried more analogy mass per
word; B was the safer scaffold. Proposal C was not needed — a worked example
would have blown the jargon and word budget together.

**Final word count: 150 exactly** (verified by `wc -w` on the note body of
answer.md, whitespace-token rule per plan.md; the heading-free body is the
entire file). The whole file is the note: salutation, body, sign-off
included in the count.

Trims and additions made from B's 131-word draft (recorded so the reviewer
can trace them):
- Added the knowledge-sharing sentence ("And it's how knowledge travels:
  someone else understands your change too") to cover the third reason
  plan.md names for the why.
- Added the warm open-door closer from Proposal A ("Questions are welcome
  anytime — I'm happy to sit with you through your very first one!") to
  fix B's flagged opinion-flavored ending and land the friendly tone.
- Compacted B's branch sentence ("is just your own private copy" → "is
  your own private copy, where you build and test safely") and the
  teammate sentence to pay for the additions.
- Swapped B's sign-off "— Sam" (kept; it is 2 words and gives the note a
  human sender).

Jargon self-check against the plan's list: no diff, merge, CI, approve,
commit, or PR anywhere in answer.md (grep-verified). Terms of art used:
"pull request" (the subject, allowed), "branch" (allowed, and explained
in plain words in the same breath), "review"-adjacent plain wording
("look at them", "reads your changes"). "Main version" is plain English
(the main/shared copy), not the jargon word "main" as a branch name.

Content check: half (a) what — the quoted "here are changes I made on a
branch — please look at them before they become part of our shared work"
definition plus the branch explanation; half (b) why — catches mistakes
early, keeps the shared copy trustworthy, spreads knowledge. Both present.

No tests/ directory exists in this task, and the deliverable is answer.md,
not a program — nothing to execute.










## Review

**Verdict: PASS** (fresh-context reviewer; full write-up in review.md).

Independently verified, not taken from the solver's claims:
- Word count: `wc -w answer.md` and Python split() both report **exactly
  150** tokens (the whole file is the note body; salutation and sign-off
  count, as plan.md item 7 allows). Requirement 2 met.
- No tests/ directory exists; test-status.txt is "none", so I judged the
  deliverable against every requirement in plan.md and task.md directly.
- Jargon budget (req 4): term-of-art scan for diff/merge/CI/approve/PR/
  commit/rebase/repo/repository/git/staging/deploy/lint/pipeline — none
  present. "Branch" appears twice and is explained in the same breath
  ("your own private copy, where you build and test safely"). "Review"
  appears 0 times — it is allowed, not required, so its absence is fine.
  "Main version" is plain English (the shared copy), not the branch-name
  jargon sense; the plan's example of banned "main" is the jargon word
  for the primary branch, and here it is a common adjective on
  "version". The word "review" as a concept is still delivered in plain
  wording ("a teammate reads your changes", "look at them").
- Both halves (req 5): (a) what — the quoted definition "here are
  changes I made on a branch — please look at them before they become
  part of our shared work" plus the branch explanation; (b) why — the
  "Why we use them" paragraph covers mistakes caught early, the shared
  copy staying trustworthy, and knowledge spreading. Both complete.
- Tone (req 3): friendly, addressed to a new colleague ("welcome
  aboard", "happy to sit with you through your very first one"); no
  condescension or boilerplate.
- Self-contained (req 6) and plain markdown (req 7): the note reads
  fully on its own; only minimal salutation and "— Sam" sign-off.

Non-blocking nit: the plan lists "review" as an allowed term and the
note never uses it, relying on plain-language wording instead. That is
within budget (allowed ≠ required); no repair needed.

All requirements met. No repair round required.











## Repair

(placeholder — repair notes, if any, go here)
Repair worker: review.md verdict is PASS and test-status.txt is "none"
(no tests exist for this task), so answer.md was left unchanged — no
repair was needed. The reviewer independently verified all plan.md
requirements (word count 150, jargon budget, both content halves, tone,
self-contained, format) and found no real defects; nothing to correct.
