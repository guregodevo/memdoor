---
name: roadmap
description: The roadmap is the quarter plan in docs/ROADMAP.md; features are issues labelled now or next, each with a done-when
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: "\U0001F5FA"
    skillKey: roadmap
---

# Roadmap

Two public places, one private.

- **`docs/ROADMAP.md`**: the quarter. What it is for, how it is judged on
  its last day, what is not this quarter. Public.
- **Issues** on guregodevo/memdoor: one feature per issue. Label `now` for
  what is being built this week or next, at most five. Label `next` for
  what follows, with the evidence that would move it to `now`. An issue
  closes when its done-when holds.
- **Internal** (usage metrics, user research, pricing and sales, growth and
  listings, enterprise): memdoor-private, its issues and
  `docs/internal/ROADMAP_Q4_2026.md`. Never in this repo or its issues.

## An issue

- A title that names the feature. Two or three sentences. A **done-when** a
  stranger could check: a page, a command, a number.
- No quotes of the maintainer. A number in it has its run published.

## Moving an issue

- To `now`: a user asked, a run showed it, or a number moved. Say which in
  a comment.
- Out of `now`: closed when done, or relabelled `next` with the reason.
- Declined: closed with the reason in one sentence.

## The quarter

Changes at its end, or earlier on evidence. Link the issue that carries the
evidence from the change.

```bash
gh issue list --label now
gh issue list --label next
```
