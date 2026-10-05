# Contributing

Memdoor is open source under the Apache License 2.0 and has one maintainer,
Gregory Desvaux. For now he is also its only contributor.

## What is welcome

- **Issues**: a bug with the steps to reproduce it, or a request with the
  problem it would solve. Both are read and answered.
- **Discussions** of design, in an issue.
- **Security reports**, by email, as [SECURITY.md](.github/SECURITY.md) says.

## Pull requests

External pull requests are not merged at this stage. A pull request is still
useful as a proposal: it shows the change exactly, and the fix it suggests may
be made by the maintainer, with credit in the commit. It will be closed once
answered. When contributions open up, this file will say so and name the
terms.

## Ownership and licence

The code is copyright Gregory Desvaux and licensed to you under Apache 2.0
([LICENSE](LICENSE), [NOTICE](NOTICE)): you may use, change and ship it, and
the licence grants you a patent licence for what it contains. Ownership of
the code, the name and the project stays with the maintainer; nothing here
assigns, waives or transfers it. If contributions are accepted later, they
will come with a contributor licence agreement giving the maintainer a
perpetual, irrevocable licence to the contribution, including under any
patents it practises, and the right to relicense it.

## Building it

```bash
make build          # the binary
make test           # the whole suite
gofmt -l .          # must print nothing: CI checks formatting
```

Go 1.25, Node 22, SQLite 3 (`libsqlite3-dev` on Linux). CI runs build, vet,
format check and the full test suite on every push and pull request
(`.github/workflows/lint.yml`). Coding rules are in `AGENTS.md` and
`.agents/skills/coding-principles.md`.
