# Hooks

**Status: designed, not shipped** (2026-10-01). What exists today is
`tool_guards` — regex rules that *refuse* a tool call — and the agent's own
`cron` tool. Nothing yet *runs* on an event. This page is the design the
roadmap commits to (`docs/roadmap/MUST.md`, item 0), decided on the evidence
in `../internal/HOOKS_AND_CRON_2026_10.md` (removed 2026-10-03).

## What a hook is for

Instructions are suggestions; a blocked tool call is not. Every vendor now
says it in writing: the model can rationalise away a line in AGENTS.md, it
cannot rationalise away a refused call. Real usage converges on four hooks —
format after an edit, refuse one command pattern, a test gate before the turn
stops, a notification when it ends — and nobody has found one worth turning on
for a person by default. So: a small, deterministic layer, off until asked for.

## Memdoor's own, not Claude Code's

Greg, 2026-10-01: **"no hook on claude."** Hooks are configured the way the
rest of Memdoor is — a workspace setting beside `tool_guards`, read per call —
not through Claude Code's `settings.json` contract, and Memdoor never again
installs a hook *into* Claude Code (the old `memdoor hook install` is gone).
The portability a borrowed contract would buy is given up on
purpose; being downstream of a format we do not control is the cost not paid.

## The setting

`hooks` is a JSON array, one rule per entry, in the shape `tool_guards` uses:

```bash
TOKEN=$(memdoor auth token)
curl -X PUT http://localhost:18789/api/workspace/settings \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"hooks":"[
    {\"when\":\"before\",\"tool\":\"bash\",\"command\":\"./hooks/no-migrations.sh\"},
    {\"when\":\"after\",\"tool\":\"apply_patch\",\"command\":\"gofmt -w $MEMDOOR_FILE\"},
    {\"when\":\"stop\",\"command\":\"go test ./...\",\"message\":\"the tests must pass before you stop\"}
  ]"}'
```

| field | meaning |
|---|---|
| `when` | `before` a tool runs · `after` a tool succeeded · `stop`, when the turn is about to end |
| `tool` | a tool name or `*`; absent on `stop` |
| `command` | run with `bash -c`, in the turn's project directory |
| `message` | optional teaching text, shown with the refusal |
| `timeout` | seconds, default 30; a hook that overruns is killed and counts as a failure |

Clear with `{"hooks":""}`. One invalid rule never disables the rest — the
same promise `tool_guards` makes.

## The contract

The hook gets the call on **stdin** as JSON — `{"tool","input","workdir",
"session"}` for `before`/`after`, plus `"output"` for `after`; `{"workdir",
"session","summary"}` for `stop` — and `$MEMDOOR_FILE` when the input names one.

- **exit 0** — go on. For `stop`, stdout is appended to the turn as context.
- **exit 2** — refuse. For `before`, the call does not run and the agent is
  told why, in words: the hook's stderr, else `message`. For `stop`, the turn
  does not end; the reason goes to the agent as the next thing to do.
- **any other exit** — the hook failed; the call goes on, the failure is
  logged at WARN. A hook that silently does nothing is the most common bug in
  every hook system shipped so far, so a failing hook is loud, not ignored.

`before` on `bash` can refuse `makemigrations` while allowing `test`; `after`
on `apply_patch` can run the formatter; `stop` can hold the turn open until
the build is green — bounded: a `stop` hook that has refused three times in
one turn is not consulted again, because a loop needs a counter, not a better
prompt.

## Trust

A hook is a command, and a setting can arrive with a cloned repository. Hooks
live in the **workspace setting**, which only the workspace's own people can
write. If a project file (`.memdoor/hooks.json`) is ever read, it starts
nothing before the person's yes — the `.mcp.json` rule, the hole in omp's
loader and the published Claude Code exploit alike.

## Not building

- Session-end learning — "write what you learned" — measured at −3% success and
  +20% cost for LLM-written context files; Greg: "skip it entirely."
- Hooks on by default. Four tools ship zero.
- Claude Code's events, format or `settings.json`. See above.
- An in-process hook API: one Go binary, subprocess hooks, nothing to compile.

## Receipt

None yet. This page is rewritten from the live run when the first hook ships,
the way `cron-jobs.md` was.
