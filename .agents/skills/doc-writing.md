---
name: doc-writing
description: How Memdoor docs, READMEs, landing copy and llms.txt are written and edited: an engineer's documentation, no AI slop (always-on for any doc change)
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: ✍️
    skillKey: doc-writing
    always: true
---

# Doc writing

Applies to web/public/docs/*.md, README.md, examples/*/README.md, llms.txt,
docs/*.md and every sentence on the landing page. Set by the maintainer on 2026-11-01
after an HN reader called the team-solve README "pure, unfiltered Claude".

## The bar

Internal engineering documentation maintained by an experienced team, written
by a competent engineer for other competent engineers. Not a polished
AI-generated blog post.

A reference, not prose. Drop the sentences, keep the facts. Label and value:
"Judged reads: search hits, file sections and log lines filtered against the
task." No narrative, no scene-setting, no "you" walkthrough unless it is a
numbered procedure. A mechanism gets one explaining sentence only where a
reader could not infer it. The exemplar is web/public/docs/features.md as
rewritten on 2026-11-01 (864 to 650 words); the maintainer called three lighter passes
"still full of slop" and this one the bar.

## Rules

### Remove AI writing patterns

- Delete filler, generic introductions, motivational language, unnecessary
  conclusions, "Key takeaways" and summaries that restate the page.
- Never: "It's important to note", "Let's dive into", "This ensures that",
  "robust and scalable", "seamlessly", "comprehensive", "powerful",
  "elegant", "flexible", "worth noting", "in other words".
- No aphorisms ("On a sample this small, 9.8 and 9.6 are the same number"),
  no "not X, but Y", no colon-then-reveal, no rhetorical questions, no
  triplets for rhythm, no closing flourishes, no em-dash asides.
- Do not explain what the audience knows. Do not say a thing twice in
  different words, within a sentence, a paragraph or a page.
- No self-congratulation, no claims about how elegant or powerful a thing is.

### Be direct and precise

- Concrete statements. Established engineering terms. What the system does,
  how it works, why the decision was made (constraints and tradeoffs, not
  benefits).
- Short, information-dense sentences. Few commas: a sentence chaining
  clauses becomes two or three sentences. Lists of three keep theirs.
- Delete every expression the sentence holds without: emphatic "every",
  "never", "its own", "itself", "your own", "still", "also", "already",
  "at all", "exactly", "just", "really", "there is/are". Keep the word
  where it states a rule, a schedule or a guarantee ("never twice at
  once", "every 20 seconds").
- Specifics over adjectives. Keep useful examples, drop decorative ones.

### House vocabulary

- "on your key", "your key"; never "on your own key" or "using your own
  provider account".
- "your machine", "your gateway"; never "a machine you own" or "a gateway
  you own".
- A workflow is the deterministic harness. Say so wherever a multi-step job
  is described. The page titled "The harness" is the agent's turn (decision
  model, guards); the workflow is the harness around many calls.
- Never "multi-step jobs kept as files in your repo" in a tagline or
  summary. A format sentence (one YAML task file per step under
  .memdoor/workflows) is fine where a doc explains the format.
- The one sentence, everywhere (README hero, site title and description, OG,
  JSON-LD, HOME_META, the landing hero, what-is, llms.txt): "The workflow harness that makes a small model beat a frontier one, on your machine."
  Under it, the second line with the benchmark. The GitHub description stays
  "A headless coding agent with multi-agent harness".

### Structure

- Prose for explanations, bullets for real enumerations, tables for real
  comparisons, code blocks for implementation details. Not every paragraph
  is a list.
- No unnecessary headings, no deep nesting, no Overview/Benefits/Summary
  templates. Merge sections that repeat.
- Headings are link anchors. Before renaming, removing or merging one,
  grep web/src, web/public, README.md, examples and docs for its slug and
  text. If anything links to it, keep it exactly. Keep every H1.
- Bold only on terms a reader scans for (option names, a list item's
  label). Not for emphasis. No emojis.

### Preserve substance

- Keep implementation details, constraints, caveats, tradeoffs, edge cases,
  code, commands, flags, paths, configuration keys, identifiers, numbers and
  link targets exactly. Never touch a fenced code block or an indented
  command block in an editing pass.
- Never invent a detail. Never silently change a technical statement's
  meaning. Flag a doubtful or unverifiable claim to the maintainer instead of
  rewriting it as a fact.
- Do not add sections, claims, examples or recommendations in an edit.
- If a passage is already clear and concise, leave it alone. Optimize for
  density and readability, not a word count.

### Mirrors

- docs/SECURITY.md and web/public/docs/security.md must be identical (the
  deploy guard stops on a difference). Edit one, copy to the other.
- index.html's description, the Open Graph tags, the JSON-LD and
  HOME_META in web/src/meta.ts carry one home description. Change all or
  none.

## An editing pass

1. Branch, never on main while a deploy runs; a worktree under
   .claude/worktrees/ keeps the deploy's tree untouched.
2. Edit by file group in parallel if large. Each editor reports words
   before and after, what it kept and why, and the claims it flagged.
3. Check per file against main: headings, fenced blocks and link targets
   identical. The checker is a few lines of Python over `git show main:`.
4. PR with the flagged claims in the body. The maintainer decides on those.
