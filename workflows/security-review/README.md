# security-review

Reviews the change in front of it for security only — a pull request, a branch or the last commit — and fails on what an attacker could use. Not style, not general bugs.

`diff` takes the change the way pr-review's does: in a GitHub pull request, the PR against its base branch; elsewhere, the branch against the default branch, or the last commit, with uncommitted edits included. The review (one step of two, exactly one runs) looks the diff over for what an attacker could reach in the code the change adds:

- secrets or keys committed
- injection: SQL, shell, template, path traversal
- missing or broken authorization
- unsafe deserialization
- SSRF
- weak crypto or weak randomness
- user input reaching a dangerous call (eval, exec, delete, redirect)
- a dependency added with a known risk
- a CI workflow made exploitable (`pull_request_target` that checks out the PR's code, a secret exposed to a fork)

It writes `.memdoor/runs/security-review-<partition>.md`: a summary and one line per finding, **P0** (exploitable, must not merge), **P1** (exploitable under a condition the change does not create), **P2** (hardening) — each with file:line, the attack, and the fix. `verdict` fails the run when there is a P0 and names each one, so a CI job goes red on a blocker and green otherwise. It changes no file in the project; everything it writes is under `.memdoor/runs/`.

## Inputs

Nothing to set. It takes the change from git, and from the CI environment when there is one: `GITHUB_BASE_REF` for a pull request's base branch, `GITHUB_EVENT_PATH` and `GH_TOKEN` (with `gh`) for the pull request's context — whether it is from a fork, who authored it. Outside GitHub — on a laptop, in another CI — those are absent and it falls back to the branch against `origin/HEAD`, or the last commit, or the first one.

## The two ways it runs

**With a model key** (OpenRouter or a vendor's): the `review` step runs — an agent that reads the diff, opens the changed files and what they call, and may read the GitHub event payload for context. Slower, sharper.

**With no key**: set `MEMDOOR_LEAN=1` (the Memdoor GitHub Action sets it when you give it `local-model:`, a small model on the runner's CPU — an 8B runs on 4). Then `review-lean` runs instead: one model call with the diff inlined in the prompt, thinking off, no tools, no loop. A CPU reads a few thousand tokens a minute, so a big diff is slow. The two steps sit under their own `when` and write the same target file, so exactly one runs; `verdict` requires both patterns and waits for the one that ran.

Run it on `pull_request`, never `pull_request_target`: GitHub gives a fork's pull request no secrets, so a stranger's code never runs next to your key; `pull_request_target` runs their code with your secrets — the very thing this workflow looks for.

## Run it

    memdoor workflow run security-review --wait

In GitHub Actions:

```yaml
jobs:
  security:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: read
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: guregodevo/memdoor-action@v1
        with:
          workflow: security-review
          # With a key:
          # model-key: ${{ secrets.OPENROUTER_API_KEY }}
          # Or with none — a small model on the runner (sets MEMDOOR_LEAN):
          # local-model: qwen3-8b
```
