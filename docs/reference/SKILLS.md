# Skills

A skill is a reusable workflow written as markdown. Agents load one by name with
the `skill` tool, and you can invoke one directly as a slash command —
`/skill:<name> [args]` in the TUI (tab-completion lists them) injects the
skill's content as the turn's task. You write or edit skills as plain files on
disk — changes take effect on the next call, with no rebuild and no gateway
restart.

## Where skills live

Resolution order — first hit wins:

| Tier | Path | Use |
|------|------|-----|
| 1 | `<project>/.agents/skills/` | project-local; the coder can write here itself |
| 2 | gateway-cwd `./skills/` | dev checkout (repo work) |
| 3 | `~/.memdoor/workspace/skills/` | workspace bootstrap dir |
| 4 | `~/.memdoor/skills/` | **managed — the one to edit** (seeded at boot) |
| — | embedded in the binary | fallback for never-booted machines |

A skill is a flat `<name>.md` file; invoke it as `<name>`.

## Editing skills

Edit any file in `~/.memdoor/skills/` — the running gateway picks the change up
on the agent's next `skill` call. Your edits are permanent: the boot seeder
never overwrites a file you have changed (it tracks shipped-content hashes in
`.seed-manifest.json`). If a later release ships a new version of a skill you
edited, a WARN in the logs notes the divergence; diff against the repo copy if
you want to merge.

Deleting an edited file restores the shipped version on the next boot.

## Seeding

At every boot the gateway seeds the built-in skill library into
`~/.memdoor/skills/`:

- **missing file** → written
- **present and unedited** → refreshed when the shipped version changed
- **edited by you** → left alone, forever


## Self-extension: agents writing their own skills

`<project>/.agents/skills/` sits inside the coder's working-directory
confinement, so the coder can save a workflow it has worked out as a skill with
a normal file write — and load it by name from then on:

```
> Save this migration procedure as a skill named db-migrate, then run it.
```

Project skills travel with the repo (commit `.agents/skills/` if the workflow
belongs to the project). The same directory convention is used by other agent
harnesses (agentskills.io), so shared skill repos work here too — this is the
project's skills directory, agent-agnostic.

## Listing and troubleshooting

- An unknown skill name returns the full list of available skills — the
  cheapest way to see what resolves.
- The tool result shows the skill's content verbatim; if an edit doesn't seem
  to take, check tier order above — a same-named file in a higher tier wins.
- Seeding activity is logged: `memdoor logs query --regex "Skill seed"`.
