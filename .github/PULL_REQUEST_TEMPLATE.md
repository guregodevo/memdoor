<!--
Opening this pull request is your agreement to the CLA (CLA.md). To decline,
say so here. CONTRIBUTING.md says what a pull request needs.
-->

## Description

<!-- What changes, and why. The problem first, then the fix. -->

## Related issue

<!-- Fixes #N. Anything beyond a fix needs an issue agreed first. -->

Fixes #

## Type of change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds behaviour, opt-in)
- [ ] Breaking change (existing behaviour or defaults change)
- [ ] Documentation only
- [ ] Refactor, tests or tooling (no behaviour change)

## How has this been tested?

### Test results

<!-- Paste the output, trimmed to what matters. A box left unticked says why. -->

- [ ] `make build`
- [ ] `make test`
- [ ] `make check` (fmt, vet)
- [ ] `make scan`
- [ ] New or changed tests cover this change

```
<test output>
```

### Validation

<!--
What you ran by hand to see it work, and what it printed: a CLI command, a
`memdoor logs query` line, a TUI capture (.agents/skills/test-tui.md).
Run the coder from a scratch dir, never from the repo.
-->

**Steps to reproduce / verify:**

1.
2.

**Result:**

## Screenshots or recordings

<!-- For a TUI or web change: before and after. Delete otherwise. -->

## Checklist

- [ ] My code follows the rules in `AGENTS.md` and `.agents/skills/`
- [ ] I searched for existing code before adding new code
- [ ] I added or updated tests that fail without this change
- [ ] I updated the docs where behaviour changed (`docs/`, `make cli-index` for `docs/reference/CLI.md`)
- [ ] Existing defaults are unchanged; new behaviour is opt-in
- [ ] The branch is one squash commit
