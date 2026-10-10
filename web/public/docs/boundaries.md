# Boundaries

What the agent may touch is a set of rules you can read, not a mode you
toggle at every call.

## What holds by default

- **Writes stay in the project.** File tools write only under the folder
  you launched `memdoor tui` from (or the editor's project). Reads go
  anywhere, as a shell could anyway.
- **Commands start in the project** and can reach the rest of the machine.
  A short hard-coded net refuses the catastrophic ones (wiping a disk,
  recursing over `/`).
- **A secret in a tool's output is redacted** before the model or the window
  sees it: an API key printed by a script shows as a mask.

## The rules you add

A guard is a rule on a tool: a tool name (`bash`, `apply_patch`, or `*` for
any) and a regular expression over the call's input. A call that matches is
refused with your message, and the agent reads the message and works another
way instead of asking you.

```bash
memdoor guards add --tool '*'  --pattern '\.env\b'        --message 'secrets stay out of reach'
memdoor guards add --tool bash --pattern '\bgit push\b'   --message 'pushing is mine'
memdoor guards add --tool bash --pattern '\bsudo\b'
memdoor guards add --tool bash --pattern 'git checkout (main|master)\b' --message 'work on the branch you are on'
memdoor guards                 # the rules, numbered
memdoor guards remove 3
```

The rules hold for every agent in the workspace on every turn, however the
turn started: a window, a scheduled check, a workflow, an editor session.
Setting them takes an admin's sign-in; the first account on a machine is one.

What the agent sees when a rule fires:

```
⏺ Bash(git push origin main)
  Error: blocked by workspace tool guard: pushing is mine. Choose a different
  approach; if the action is truly needed, ask the user to do it or to lift the guard
```

It then finishes with the branch unpushed and says so, which is the point:
a boundary the agent can read is one it stops testing.

## Ask before every change

For a company that forbids any "yolo" agent, approval mode asks before every
command, file write and MCP tool call, in the window:

```bash
export MEMDOOR_APPROVE=changes
```

The picker offers Yes, Yes for this session, or No. No ends the call with a
result the agent acts on; no answer in five minutes is a no. Reads never ask.
The same setting is `approve` in the workspace settings.

## A gate in a run

For a multi-step job, the boundary is a step: a workflow task marked
external waits for your approval with the diff in front of you, and nothing
after it starts until you say so. See [Workflows](/docs/workflows).

## Next

- **[No babysitting](/docs/no-babysitting)** — what the agent decides for itself inside these lines.
- **[Where your data goes](/docs/security)** — every host the binary names.
- **[Configuration](/docs/configuration)** — the other workspace settings.
