# Contributing

Memdoor is open source under the Apache License 2.0 with one maintainer,
Gregory Desvaux. This repository is the source of truth: what is on `main`
is what memdoor.ai ships.

## What is welcome

- **Issues**: a bug with the steps to reproduce it, or a request with the
  problem it would solve.
- **Pull requests**: a fix or a change with a way to see it (a command, a
  test that fails without it). Small and single-purpose merges faster.
- **Security reports**, by email, as [SECURITY.md](.github/SECURITY.md) says.

## Pull requests

- Open an issue first for anything beyond a fix, so the design is agreed
  before the code.
- CI must pass: build, vet, formatting, the test suite and the secret scan
  (`make check`, `make scan`).
- The rules are in `AGENTS.md` and `.agents/skills/`: search before coding,
  test through the CLI, factories return interfaces, no obvious comments,
  the logger and never `fmt.Println`, docs with no AI slop.
- One squash commit per pull request. The maintainer reviews every one;
  a review may ask for a change or decline the change with the reason.

## Developer Certificate of Origin

Every commit is signed off (`git commit -s`), which adds
`Signed-off-by: Name <email>` and states that you wrote the change or have
the right to submit it under the project's licence, as the
[Developer Certificate of Origin 1.1](https://developercertificate.org)
says. No contributor licence agreement.

## Ownership and licence

The code is copyright Gregory Desvaux and licensed to you under Apache 2.0
([LICENSE](LICENSE), [NOTICE](NOTICE)). A contribution is licensed under
the same terms, as section 5 of the licence says; the contributor keeps the
copyright of what they wrote. The name and the project stay with the
maintainer.

## Building it

```bash
make build          # the binary
make test           # the whole suite
make check          # fmt, vet, test
make scan           # nothing in the tree that must not be public
gofmt -l .          # must print nothing: CI checks formatting
```

Go 1.25, Node 22, SQLite 3 (`libsqlite3-dev` on Linux). CI runs build, vet,
format check, the scan and the full test suite on every push and pull
request (`.github/workflows/lint.yml`).
