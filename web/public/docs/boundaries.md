# Boundaries

What the agent may touch, as rules you can read.

## What holds by default

- **Writes stay in the project.** File tools write only under the folder
  `memdoor tui` was launched from (or the editor's project). Reads go
  anywhere, as a shell could.
- **Commands start in the project** and can reach the rest of the machine.
  A short hard-coded net refuses the catastrophic ones (wiping a disk,
  recursing over `/`).
- **Secrets in tool output are redacted** before the model or the window
  sees them: an API key printed by a script shows as a mask.

## The rules you add

A guard: a tool name (`bash`, `apply_patch`, or `*`) and a regular
expression over the call's input. A matching call is refused with your
message. The agent reads it and works another way.

```bash
memdoor guards add --tool '*'  --pattern '\.env\b'        --message 'secrets stay out of reach'
memdoor guards add --tool bash --pattern '\bgit push\b'   --message 'pushing is mine'
memdoor guards add --tool bash --pattern '\bsudo\b'
memdoor guards add --tool bash --pattern 'git checkout (main|master)\b' --message 'work on the branch you are on'
memdoor guards                 # the rules, numbered
memdoor guards remove 3
```

Scope: every agent in the workspace, every turn, from a window, a scheduled
check, a workflow or an editor session. Setting them takes an admin
sign-in; the first account on a machine is one.

What the agent sees when a rule fires:

```
⏺ Bash(git push origin main)
  Error: blocked by workspace tool guard: pushing is mine. Choose a different
  approach; if the action is truly needed, ask the user to do it or to lift the guard
```

It finishes with the branch unpushed and says so.

## Ask before every change

Approval mode asks in the window before every command, file write and MCP
tool call:

```bash
export MEMDOOR_APPROVE=changes
```

Picker: Yes, Yes for this session, No. No ends the call with a result the
agent acts on. No answer in five minutes is a no. Reads never ask. Workspace
setting: `approve`.

## A gate in a run

In a workflow, the deterministic harness, the boundary is a step: a task
marked external waits for your approval with the diff in front of you.
Nothing after it starts until you say so. See [Workflows](/docs/workflows).

## Next

- **[The harness](/docs/no-babysitting)** — what the agent decides for itself inside these lines.
- **[Where your data goes](/docs/security)** — the hosts the binary names.
- **[Configuration](/docs/configuration)** — the other workspace settings.
