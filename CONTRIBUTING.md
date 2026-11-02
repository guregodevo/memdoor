# Contributing

Memdoor is open source under the Apache License 2.0 with one maintainer,
guregodevo. This repository is the source of truth: what is on `main`
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
- One squash commit per pull request, signed CLA. The maintainer reviews every one;
  a review may ask for a change or decline the change with the reason.

## Contributor Licence Agreement

Before a first pull request is merged, its author signs the
[CLA](CLA.md), once: the copyright in the contribution is assigned to the
maintainer (an exclusive, irrevocable licence where the law allows no
assignment), with a patent licence for what it practises; the author gets a
licence back to reuse their own work. The CLA check on the pull request
asks for it.

## Ownership and licence

The code is copyright the maintainer, guregodevo, and licensed to you under Apache 2.0
([LICENSE](LICENSE), [NOTICE](NOTICE)). Every merged contribution is his
copyright too, under the CLA, so the project is one work with one owner,
who can relicense it. The name and the project stay with him.

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
