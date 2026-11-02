---
name: roadmap
description: The roadmap is docs/ROADMAP.md: Now, Next, Not now, every item with a done-when; how an item is added, moved or closed
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: "\U0001F5FA"
    skillKey: roadmap
---

# Roadmap

One file, `docs/ROADMAP.md`, public. Three lists.

- **Now**: numbered, at most five. What is being built this week or next.
- **Next**: what follows, with the evidence that would move it to Now.
- **Not now**: what was decided against and why, so it is not re-proposed.

## An item

- One bold label, two or three sentences, a **done-when** that a stranger
  could check: a page, a command, a number.
- No quotes of the maintainer, no dates inside the text beyond the one the
  decision was made.
- A claim in an item ships with its run, like every other claim.

## Moving an item

- Into Now: a user asked, a run showed it, or a number moved. Say which, in
  the item.
- Out of Now: done (delete it; the commit and the page are the record) or
  demoted to Next with the reason.
- Into Not now: the reason in one sentence.

## Before adding

```bash
grep -n "label" docs/ROADMAP.md     # already there?
```

The roadmap is read by contributors and by the people deciding whether to
rely on Memdoor. Write it for them.
