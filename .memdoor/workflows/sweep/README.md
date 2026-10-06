# sweep

The truth sweep as a DAG: `sweep` reads this repository against what Memdoor is
today and writes `.memdoor/runs/SWEEP-<partition>.md` — every finding a
file:line with the stale sentence, why it is stale, the exact replacement, each
one verified by reading the file, dead `memdoor.ai` links checked with `curl`,
the whole list split into **Must fix** (a false user-visible claim or a dead
link) and **Could fix**, with counts. The run then stops at a gate for you to
read the sweep; `fix` applies every Must fix exactly, runs `go build`, `go vet`
and `go test` over `gateway pkg tools cmd` plus the web type check, and commits
with a message that says why (no trailers); `check` reruns vet, the suite and
the web type check on the result. The partition names the sweep file:
`/workflow:sweep` then `--partition today` (with no partition the file carries
the run's own stamp, so every run is fresh). Approve the gate with `a` in
`/workflow` or `memdoor workflow approve <run> approve`; a sweep with nothing
to fix is approved the same way and `fix` changes and commits nothing. Needs a
model on your own key; leaves the sweep under `.memdoor/runs/` and, when there
was something to fix, a commit.

Reusing a partition (`--partition today` twice in one day) finds the sweep file
already there, so mario counts `sweep` done and skips it without a turn: give a
new partition, or delete `.memdoor/runs/SWEEP-<partition>.md`, to sweep again.

## What it sweeps against

Memdoor today: a coding agent in the terminal on the user's own API key at any
of eight providers; a decision model (Jev by TypeSafe, a hosted model, on an
OpenRouter key or a decision key, off on a vendor key alone); workflows on mario
(one YAML per task, done = target exists, a gate = an external task, resume),
free with local cron schedules; Pro = $10 a month for remote control and the
hosted workflow state on memdoor.ai; Enterprise by invoice; public repo
`github.com/guregodevo/memdoor`, private `memdoor-private`; `aktapus.ai`
redirects to `memdoor.ai`; and there is **no** local model, GPU rental, broker,
clipper, fitness product, Mac app, hosted scheduler, device-code or `--browser`
login, heartbeat checklist.

## Running it

```
/workflow:sweep            # sweep this repo as HEAD, gate, fix, check
```

The workflow is shared by copying this directory `.memdoor/workflows/sweep/`.
