# hn-watch

Watches Hacker News for threads where a comment from Memdoor's author would add
something. The `search` step queries the Algolia API — one query each for
"coding agent", "agent skills", "agent workflows", "claude code", "codex cli",
"openrouter", "api key cost", "terminal agent" — for stories of the last 36
hours with more than 20 points, merges the hits by story id and writes them,
sorted by points, as a markdown table into `.memdoor/runs/HN-<partition>.md`
(points, comments, age in hours, title, the `news.ycombinator.com/item?id=…`
link). The `rank` step then reads that table and appends the section **Where a
comment adds something**: at most five threads where a comment about workflows
versus skills, running on your own API key, or what agents cost would be
relevant to the thread's topic — each the link plus one line on the angle and
nothing else; threads that are only tangentially related are left out, and when
none fits it says so. It writes no comment text.

The partition is the day, so a re-run of the same day finishes what was left
instead of starting over: run `/workflow:hn-watch --partition today` (or
`memdoor workflow run hn-watch --partition today`). The search step needs
`curl` and `python3` and no key; the rank step needs a model on your key. It
leaves one file, `.memdoor/runs/HN-<partition>.md`, and touches nothing else.

Every morning at 8:30, on your own machine (free — no seat):

```bash
memdoor cron add --id hn-watch --workflow hn-watch --partition today --schedule '30 8 * * *'
```
