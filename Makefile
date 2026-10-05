.PHONY: up build web-build fmt check check-fmt clean test test-verbose test-a2a help gateway gateway-verbose cowork start stop stop-gateway deploy setup quickstart all skills-sync

# Ports
GATEWAY_PORT ?= 18789
# The gateway `make stop` may kill: the one on GATEWAY_PORT (or with no
# --port, which means it). A gateway on another port — a test gateway, a
# second checkout — is someone else's (2026-09-29: make stop killed a test
# gateway mid-benchmark, twice).
GATEWAY_MATCH = memdoor gateway( --port $(GATEWAY_PORT))?$$
VITE_PORT    ?= 5173

# Build the memdoor binary. Depends on web-build so the embedded
# React bundle (//go:embed web/dist via web/embed.go) is fresh —
# without this the binary published at /dl/ would ship a stale UI
# even after a clean Go rebuild. The actual go build call lives in
# gateway/Makefile so version/commit/date ldflags stay in one place.
build: web-build
	@echo "🔨 Building Memdoor..."
	@# build does NOT stop the gateway (2026-09-28): a coder checking "does it
	@# build?" ran make build and killed the gateway serving it and three other
	@# sessions. Replacing a running binary is safe on macOS and Linux; `make up`
	@# is what restarts.
	@echo "  → Cleaning old binary..."
	@rm -f memdoor 2>/dev/null || true
	@cd gateway && $(MAKE) clean 2>/dev/null || true
	@echo "  → Building memdoor (embeds web/dist via //go:embed)..."
	@cd gateway && $(MAKE) build
	@echo "✅ Memdoor built: ./memdoor"
	@echo "   make up — put it on your PATH and restart the gateway on it"

# THE LOCAL UPDATE IN ONE COMMAND (Greg, 2026-09-27: "this is in make right? it
# should be"). Build, put that build on your PATH, and restart the gateway on
# it — the three steps every change needed by hand. The restart is graceful
# (SIGTERM, the six-phase teardown drains) via stop-gateway.
#
# ~/.local/bin/memdoor becomes a SYMLINK to ./memdoor, so every later build is
# what `memdoor` runs, with no copy to forget. (The installer writes a file
# there instead; this puts the symlink back.)
#
# The gateway needs provider keys in its environment (any of the providers
# `memdoor providers` lists, or a company AI gateway). This target sources
# ./.envrc itself, as deploy does, so a shell without direnv starts the same
# gateway as one with it (2026-10-03: a `make up` from a bare shell came up
# with zero providers). Once up, it prints the providers the gateway actually
# connected — the binary is the truth, no key name is repeated here — and
# warns only when there is none.
up: build stop-gateway
	@mkdir -p $(HOME)/.local/bin
	@ln -sf $(CURDIR)/memdoor $(HOME)/.local/bin/memdoor
	@echo "  → $(HOME)/.local/bin/memdoor -> $(CURDIR)/memdoor"
	@set -a; [ ! -f .envrc ] || . ./.envrc; set +a; \
	 echo "=== gateway start $$(date) ===" >> gateway.log; nohup ./memdoor gateway --port $(GATEWAY_PORT) >> gateway.log 2>&1 &
	@for i in $$(seq 1 30); do curl -sf -m 1 http://127.0.0.1:$(GATEWAY_PORT)/v1/health >/dev/null && break; sleep 1; done
	@curl -sf -m 2 http://127.0.0.1:$(GATEWAY_PORT)/v1/health >/dev/null \
		&& echo "✅ gateway up on :$(GATEWAY_PORT) — $$(./memdoor --version | head -1)" \
		|| echo "⚠️  the gateway did not answer on :$(GATEWAY_PORT) — see gateway.log"
	@./memdoor providers 2>/dev/null | grep -E "connected \(" \
		|| echo "  ⚠ no provider connected: the gateway has no key — put one in ./.envrc or run: memdoor connect <kind>"

# THE PRODUCTION UPDATE IN ONE COMMAND. scripts/deploy.sh is the whole ritual:
# rsync source, build ON the VPS, systemd restart, nginx/TLS, publish the /dl/
# binaries + VERSION. It sources .envrc itself (VPS_HOST, DOMAIN), so a bare
# `make deploy` needs nothing else; override the host with
# `make deploy VPS_HOST=root@host`. It takes no other flags any more — the
# data-push flag went with the CLI it called on 2026-10-03.
deploy:
	@./scripts/deploy.sh $(VPS_HOST) $(ARGS)

# cli-index rewrites the command index in docs/reference/CLI.md from the
# binary's own --help (hidden commands included), so the index cannot drift
# from the commands. Run it after adding, removing or renaming a command.
# oss-tree builds the PUBLIC tree at /tmp/memdoor-oss — fresh history, the
# private docs and ops scripts left out, secret scan run. Prepares; never
# publishes (2026-10-04: "keep it private for now and prepare it for opensource").
oss-tree:
	@./scripts/oss-tree.sh

# oss-publish puts the current release on the public repo (guregodevo/memdoor-oss):
# the oss-tree laid over a clone of it, one commit named after the version, a tag,
# pushed (scripts/oss-publish.sh). Private commit messages never travel.
oss-publish:
	@./scripts/oss-publish.sh

# github-protect sets the repository's rules on GitHub: squash-only, main behind
# a PR that CI passed and the code owner approved, no force push, fork PR runs
# wait for approval. Run after the repo is public (scripts/github-protect.sh).
github-protect:
	@./scripts/github-protect.sh

# metrics prints adoption from the prod access log (installs, active installs,
# landing visitors), accounts and seats, GitHub stars/views/clones and the HN
# post, and saves a snapshot under .memdoor/metrics/.
metrics:
	@mkdir -p .memdoor/metrics && ./scripts/metrics.sh | tee .memdoor/metrics/$$(date +%Y-%m-%dT%H%M).txt

# rollback puts the previous release back on memdoor.ai (scripts/rollback.sh);
# releases lists what the deploys kept.
rollback:
	@./scripts/rollback.sh
releases:
	@. ./.envrc && ssh $$VPS_HOST 'ls -1t /var/lib/memdoor/releases 2>/dev/null || echo none'

# rotate-admin rotates the prod admin password through the API (owner-run:
# needs your admin session; see scripts/rotate-admin.sh).
rotate-admin:
	@./scripts/rotate-admin.sh

# workflow-selftest runs this project's own selftest workflow
# (.memdoor/workflows/workflow-selftest, committed with the repo) here, against
# the running gateway: positive and negative workflow scenarios in a scratch
# project, DeepSeek Flash on the model steps, a PASS/FAIL report written to
# SELFTEST-<partition>.md in this folder. Fails if any scenario failed.
workflow-selftest:
	@out=$$(memdoor workflow run workflow-selftest --dir . 2>&1); echo "$$out" | head -1; \
	  id=$$(echo "$$out" | grep -oE "workflow-selftest-[0-9]{8}-[0-9]{6}" | head -1); [ -n "$$id" ] || exit 1; \
	  for i in $$(seq 1 200); do memdoor workflow status $$id 2>&1 | head -1 | grep -qE "· (done|failed|stopped) ·" && break; sleep 5; done; \
	  f=$$(ls -t SELFTEST-*.md 2>/dev/null | head -1); [ -n "$$f" ] && cat $$f || memdoor workflow status $$id; \
	  memdoor workflow status $$id 2>&1 | head -1 | grep -q "· done ·"

cli-index: build
	@./scripts/cli-index.sh ./memdoor

# web-build runs `vite build` so web/dist/ is fresh before //go:embed
# captures it into the binary. Idempotent — skipped when web/dist/
# is newer than every file under web/src/. Without the .stamp gate
# every `make build` would re-run vite (~12s) for back-to-back
# Go-only edits.
WEB_SOURCES := $(shell find web/src web/public web/index.html web/package.json web/vite.config.ts web/tsconfig.json 2>/dev/null -type f)
web-build: web/dist/.stamp

web/dist/.stamp: $(WEB_SOURCES)
	@echo "🌐 Building web/dist (vite)..."
	@if [ ! -d web/node_modules ]; then \
		echo "  → web/node_modules missing — running npm install..."; \
		cd web && npm install; \
	fi
	@cd web && npm run build
	@touch web/dist/.stamp
	@echo "✓ web/dist built"

# Format all Go files
fmt:
	@echo "Formatting Go files..."
	@go fmt ./...
	@echo "✓ Files formatted"

# Check if files need formatting (for CI)
check-fmt:
	@echo "Checking Go file formatting..."
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "❌ The following files need formatting:"; \
		echo "$$unformatted"; \
		echo ""; \
		echo "Run 'make fmt' to fix"; \
		exit 1; \
	fi
	@echo "✓ All files properly formatted"

# Lint + vet
check:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Running go mod tidy..."
	@go mod tidy
	@echo "✓ Checks passed"

# Run tests
test:
	@echo "Running tests..."
	@go test ./...
	@echo "✓ Tests passed"

test-verbose:
	@echo "Running tests (verbose)..."
	@go test ./... -v

# Agent-to-agent regression suite — its own target because A2A bugs
# tend to surface as flakes elsewhere and this suite is the canonical
# place to repro them in isolation.
test-a2a:
	@echo "Running A2A messaging tests..."
	@go test ./gateway/a2a -v

# Quickstart: clean build + setup workspace
quickstart: clean build
	@./memdoor setup

# Run the gateway server (foreground)
gateway:
	@echo "Stopping any existing gateway..."
	@lsof -ti:$(GATEWAY_PORT) -sTCP:LISTEN | xargs kill -9 2>/dev/null || true
	@cd gateway && $(MAKE) run

# Run the gateway with verbose logging (foreground)
gateway-verbose:
	@echo "Stopping any existing gateway..."
	@lsof -ti:$(GATEWAY_PORT) -sTCP:LISTEN | xargs kill -9 2>/dev/null || true
	@cd gateway && $(MAKE) run-verbose

# Launch the Cowork TUI (rebuilds first to pick up any changes)
cowork: build
	@pkill -f "memdoor cowork" 2>/dev/null || true
	@./memdoor cowork

# Start Memdoor (gateway + vite dev server) in the background.
# Vite runs the React dev server with HMR; the gateway serves the API
# + the embedded production bundle. Local dev usually wants vite for
# fast iteration; production install runs the gateway alone.
start: build
	@echo "🚀 Starting Memdoor..."
	@cd web && npm install --silent 2>/dev/null || true
	@lsof -ti:$(GATEWAY_PORT) -sTCP:LISTEN | xargs kill -9 2>/dev/null || true
	@lsof -ti:$(VITE_PORT)    -sTCP:LISTEN | xargs kill -9 2>/dev/null || true
	@pkill -f "vite" 2>/dev/null || true
	@sleep 1
	@echo "=== gateway start $$(date) ===" >> gateway.log; ./memdoor gateway --port $(GATEWAY_PORT) >> gateway.log 2>&1 &
	@sleep 2
	@cd web && VITE_GATEWAY_PORT=$(GATEWAY_PORT) nohup npm run dev -- --port $(VITE_PORT) > ../web.log 2>&1 &
	@sleep 3
	@echo "✅ Memdoor running"
	@echo "   🌐 http://localhost:$(VITE_PORT)"
	@echo "   🛑 make stop"

# Stop-gateway stops ONLY the gateway, gracefully. Shared by `make up`
# (which restarts it on the fresh binary) and `make stop` (which then
# also takes down vite).
#
# -sTCP:LISTEN: kill only the LISTENER — a bare lsof -ti:PORT also lists
# CLIENTS connected to the port (every running TUI died on every restart).
# WAIT for the graceful shutdown instead of racing it. The gateway runs a
# six-phase teardown (server_lifecycle.go): phase 3 alone allows 30s to
# drain queues, and phases 4-6 still have to persist sessions, flush the
# subagent registry and close the logger. A flat
# `sleep 2` before SIGKILL meant that path never once ran to completion --
# every restart cut it off mid-drain, which is how a turn in flight and its
# ledger row got lost. Poll for the listener to go away, and only then
# resort to -9.
stop-gateway:
	@lsof -ti:$(GATEWAY_PORT) -sTCP:LISTEN | xargs kill -TERM 2>/dev/null || true
	@for i in $$(seq 1 40); do \
		lsof -ti:$(GATEWAY_PORT) -sTCP:LISTEN >/dev/null 2>&1 || break; \
		sleep 1; \
	done
	@lsof -ti:$(GATEWAY_PORT) -sTCP:LISTEN >/dev/null 2>&1 && \
		echo "  gateway did not exit in 40s -- forcing" || true
	@lsof -ti:$(GATEWAY_PORT) -sTCP:LISTEN | xargs kill -9 2>/dev/null || true
	@pkill -f "$(GATEWAY_MATCH)" 2>/dev/null || true
	@# WAIT for actual death: a gateway stuck in uninterruptible sleep (a
	@# disk syscall, state UN) survives even SIGKILL until the syscall
	@# returns — "Stopped" used to print while a zombie lingered, and the next
	@# make start then ran TWO gateways at once (live failure, twice). Poll
	@# up to 15s, escalate to -9, and refuse to claim success while one lives.
	@i=0; while pgrep -f "$(GATEWAY_MATCH)" >/dev/null 2>&1 && [ $$i -lt 20 ]; do \
		sleep 0.5; i=$$((i+1)); \
		[ $$i -eq 6 ] && pkill -9 -f "$(GATEWAY_MATCH)" 2>/dev/null; \
	done; true
	@if pgrep -f "$(GATEWAY_MATCH)" >/dev/null 2>&1; then \
		echo "⚠️  a gateway is still dying (uninterruptible sleep) — wait a few seconds before make start"; \
	else \
		echo "✓ gateway stopped"; \
	fi

# Stop Memdoor: the gateway (gracefully, stop-gateway) plus the vite dev
# server and whatever else is left on the two dev ports.
stop:
	@$(MAKE) --no-print-directory stop-gateway
	@lsof -ti:$(VITE_PORT)    -sTCP:LISTEN | xargs kill -9 2>/dev/null || true
	@pkill -f "vite"            2>/dev/null || true
	@echo "✅ Stopped"

# Initialize workspace via the just-built CLI. The build target
# already produces ./memdoor at the repo root, so reuse it instead
# of building a second binary into ./bin/.
setup: build
	@./memdoor setup

# Clean binaries (does NOT delete database or config under ~/.memdoor)
# The skills in this repo ARE the source of truth; ~/.memdoor/skills is
# the runtime copy the gateway reads (disk-first). Syncing lets a skill
# edit take effect without a rebuild — the seeder adopts a file that
# matches the shipped one, so this never looks like a local edit.
skills-sync:
	@mkdir -p $(HOME)/.memdoor/skills
	@cp skills/*.md $(HOME)/.memdoor/skills/
	@echo "✓ $$(ls skills/*.md | wc -l | tr -d ' ') skills synced to ~/.memdoor/skills"

clean:
	@rm -f memdoor 2>/dev/null || true
	@cd gateway && $(MAKE) clean

# Build + checks
all: fmt check build

help:
	@echo "🐙 Memdoor — a coding agent that cuts your model bill"
	@echo ""
	@echo "  make build              - Build the memdoor binary"
	@echo "  make start              - Start gateway + vite dev server (background)"
	@echo "  make stop               - Stop Memdoor"
	@echo "  make setup              - Build + initialize workspace"
	@echo "  make quickstart         - Clean build + setup in one shot"
	@echo ""
	@echo "=== Run modes ==="
	@echo "  make gateway            - Run gateway in foreground"
	@echo "  make gateway-verbose    - Run gateway in foreground (verbose logs)"
	@echo "  make cowork             - Launch the Cowork TUI"
	@echo ""
	@echo "=== Development ==="
	@echo "  make test               - Run tests"
	@echo "  make test-a2a           - A2A messaging regression suite"
	@echo "  make fmt                - Format Go files"
	@echo "  make check              - go vet + go mod tidy"
	@echo "  make check-fmt          - Verify formatting (CI)"
	@echo "  make clean              - Remove binary"

# Default target
.DEFAULT_GOAL := help
