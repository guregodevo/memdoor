# Contributing to Memdoor

Memdoor is open source under the Apache License 2.0. Issues and pull requests are welcome: say what you changed and why, with a test that fails without it.

---

## Development Setup

### Prerequisites

- **Go 1.22+** - Backend and CLI
- **Node.js 18+** - Web frontend
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
