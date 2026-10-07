# Examples

Workflows built by the coder from a sentence and run before they were added
here. A workflow is a folder of task files, one per step. A step is done when
its target holds: a file exists, or a command exits 0. The engine checks the
target itself, on every run, so a step is never done because a model said so.
A failed run resumes at the failed step.

## Use one

Copy a folder into your project, or into the library every project sees:

    cp -r examples/workflows/nightly-rule <your-project>/.memdoor/workflows/
    cp -r examples/workflows/nightly-rule ~/.memdoor/workflows/     # any project

Run it, or schedule it:

    memdoor workflow run nightly-rule --partition 2026-10-07
    memdoor cron add --id nightly-rule --schedule "0 1 * * *" --workflow nightly-rule --partition today

## Workflows

| Workflow | What it shows |
|---|---|
| [nightly-rule](workflows/nightly-rule/) | A nightly build → grade → file where a stale input cannot be filed: steps 2 and 3 are gated by a `test … -nt` command the engine runs |

## Add one

Describe the steps to the coder in `memdoor tui`; it writes the folder. Run
it, then open a pull request with the folder and a README that says what the
workflow proves and what you saw when it ran.
