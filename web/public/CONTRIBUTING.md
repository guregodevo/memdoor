# Contributing to Memdoor

Memdoor is open source under the Apache License 2.0 and has one maintainer, Gregory Desvaux. Issues are welcome: a bug with the steps to reproduce it, or a request with the problem it would solve. External pull requests are not merged at this stage; a PR is still useful as a proposal (see `CONTRIBUTING.md`).

---

## Development Setup

### Prerequisites

- **Go 1.25** - Backend and CLI
- **Node.js 22** - Web frontend
- **SQLite 3** - Default database
- **Make** - Build automation

### Quick Start

```bash
# Build the project
make build

# Initialize workspace
make setup

# Start Memdoor (gateway + web UI)
make start

# View logs
./memdoor logs query --limit 20
```

Access the web UI at http://localhost:5173

---

## Project Structure

```
memdoor/
├── cmd/cli/           # CLI commands and entry point
├── gateway/           # WebSocket gateway server
├── pkg/               # Domain-Driven Design (DDD) entities
│   ├── message/       # Message domain
│   ├── channel/       # Channel management
│   ├── auth/          # Authentication
│   └── domain/        # Core domain models
├── tools/             # Agent tools
├── web/               # React + TypeScript frontend
└── docs/              # Documentation
```

### Build Commands

```bash
make build              # Build binary
make start              # Start gateway + web UI
make stop               # Stop all services
make clean              # Clean binaries
make test               # Run all tests
```

---

## Code Guidelines

Key points:
1. Search before coding (avoid duplicates)
2. Test via CLI (never direct DB)
3. Factory pattern (fail fast, return interface)
4. Duck typing for circular dependencies
5. Use logger (never fmt.Println)
6. Use constants (no magic strings)
7. Follow DDD (entities in `pkg/`, infrastructure in `gateway/`)

---

## Git Workflow

- Feature branches off `main`
- Squash commits before merging
- Tag releases as `vX.YZ`

---

## License

Apache License 2.0 — see LICENSE. By contributing you agree your contribution
is licensed under it.
