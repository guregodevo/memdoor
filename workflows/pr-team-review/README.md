# pr-team-review

Reviews the change in front of it — a pull request, a branch or the last commit — as a small team of fresh contexts that meet only through files, built for a model that is small or free: one model call per role, no tools, no loop.

`diff` takes the change the way pr-review's does, `trim` cuts it to what a small model can read, and `intent` keeps the commit message, so a change is not faulted for the bug it fixes. `reviewer-a` (bugs in the new code) and `reviewer-b` (what the change puts at risk: a security hole, data loss, a broken build or test, a path left untested) read the same trimmed diff in parallel and each write their own findings. `merge`, the manager, reads both and the message, drops what the change fixes or removes, merges duplicates and writes `.memdoor/runs/pr-team-review-<partition>.md` in pr-review's shape: **P0** (must not merge), **P1**, **P2**. Each of its first three P0s gets pr-review's second look (`p0-list`, `p0-check-1..3`: a fault of the code after the change, or of the code before it?) and `verdict` demotes one answered BEFORE. A P0 is advisory here: it is in the review and the job's summary, and `verdict` passes. It changes no file in the project.

Run it with `memdoor workflow run pr-team-review --model <id> --wait`. Needs a model on your key; `cohere/north-mini-code:free` on OpenRouter does it at no cost.

One check so far, on one change: `guregodevo/memdoor` #20, a correct fix, got "No findings"; the same change with its auth check inverted got the P0, from both reviewers, and the re-check kept it. That is two runs, not a benchmark. The agent version of this team (team-solve's steps with a review prompt) did not finish on that model: its review step wrote no file.
