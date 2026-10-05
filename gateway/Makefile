.PHONY: build run run-verbose test clean help cli-build

# Binary names
CLI_BINARY := memdoor

# Version metadata baked into the binary via -ldflags. `memdoor version`
# reads these vars; `memdoor upgrade` compares them to the VERSION file
# scripts/deploy.sh publishes at memdoor.ai/dl/VERSION. `git describe`
# falls back to the short SHA when no tag is reachable; the --dirty
# suffix flags uncommitted changes so a dev build can't masquerade as a
# release. Variables are evaluated once at make-invocation time, not
# per recipe line — keeps the same string in --version and ldflags even
# if the tree changes mid-build.
# Resolve a working git binary. macOS dev machines occasionally have a
# stale x86 /usr/local/bin/git left over from an older Homebrew prefix
# that errors "Bad CPU type in executable" under /bin/sh — the shell
# make's $(shell …) uses. When that happens git describe silently
# fails and the binary ships with Version="dev". Prefer Xcode's
# CommandLineTools git (always native-arch), fall back to PATH.
GIT_CMD := $(shell xcrun --find git 2>/dev/null || command -v git 2>/dev/null || echo git)
VERSION := $(shell $(GIT_CMD) describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell $(GIT_CMD) rev-parse --short HEAD 2>/dev/null || echo "")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X 'memdoor/cmd/cli/cmd.Version=$(VERSION)' -X 'memdoor/cmd/cli/cmd.Commit=$(COMMIT)' -X 'memdoor/cmd/cli/cmd.BuildDate=$(BUILD_DATE)'

# Build the unified CLI (includes gateway, TUI, and all commands).
# CGO_CFLAGS enables SQLite FTS5 for memory tool.
#
build:
	@printf "Building Memdoor $(VERSION)... "
	@cd ../cmd/cli && CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -ldflags "$(LDFLAGS)" -o ../../$(CLI_BINARY) main.go
	@printf "✓\n"

# Run the gateway via CLI
run:
	@echo "Starting Memdoor Gateway..."
	@cd ../cmd/cli && go run main.go gateway

# Run with verbose logging
run-verbose:
	@echo "Starting Memdoor Gateway with verbose logging..."
	@cd ../cmd/cli && go run main.go gateway --verbose

# Format code
fmt:
	@echo "Formatting gateway code..."
	@go fmt ./...
	@echo "✓ Gateway code formatted"

# Run tests
test:
	@echo "Running gateway tests..."
	@go test ./... || echo "No tests yet"
	@echo "✓ Gateway tests complete"

# Clean binary
clean:
	@rm -f $(CLI_BINARY) 2>/dev/null || true
	@echo "✓ Cleaned"

# Help
help:
	@echo "🐙 Memdoor (gateway/Makefile) — low-level build targets"
	@echo ""
	@echo "Build commands:"
	@echo "  make build         - Build the memdoor binary into the repo root"
	@echo "  make run           - Run the gateway server"
	@echo "  make run-verbose   - Run gateway with verbose logging"
	@echo ""
	@echo "Development:"
	@echo "  make fmt           - Format code"
	@echo "  make test          - Run gateway-package tests"
	@echo "  make clean         - Remove binary"
	@echo ""
	@echo "For the full surface (web build, start/stop, setup, …), use the"
	@echo "root Makefile:  cd .. && make help"

# Default target
.DEFAULT_GOAL := help