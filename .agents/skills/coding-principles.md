---
name: coding-principles
description: Memdoor-specific coding principles and best practices
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: 📐
    skillKey: coding-principles
    always: true
    os:
      - darwin
      - linux
---

# Memdoor Coding Principles

Core coding principles specific to Memdoor development. These override general coding guidelines.

## Golden Rules

### 1. Avoid Duplicate Code

**ALWAYS search before adding code:**

```bash
# Search for similar functionality
grep -r "FunctionName" .
grep -r "similar_pattern" pkg/
grep -r "duplicate_logic" gateway/

# Find existing factories
grep -r "func New" pkg/

# Find existing interfaces
grep -r "type.*interface" pkg/
```

**If duplicate exists:**
- Use existing code
- Or extract common code to factory/utility
- See `.agents/skills/refactoring.md` for patterns

### 2. Run CLI to Test

**Test via CLI, not direct code:**

```bash
# Good: Test via CLI
./memdoor agent --message "test" --channel test2 --agent-id writer
./memdoor messages --channel test2 --limit 5

# Bad: Direct database access
sqlite3 ~/.memdoor/data/memdoor.db "SELECT * FROM messages"

# Bad: Direct HTTP calls
curl -X POST http://localhost:18789/...
```

**If CLI command missing:**
- Implement the CLI command first
- Then use CLI for testing
- See `docs/reference/CLI.md`

### 3. Use Factory to Create Structs

**Always use factory pattern:**

```go
// ❌ BAD: Direct struct creation
manager := &sessionManager{
    logger: logger,
    sessions: make(map[string]*Session),
}

// ✅ GOOD: Factory pattern
manager := NewSessionManager(logger)
```

**Factory must:**
- Fail fast on nil dependencies (panic)
- Initialize ALL fields to stable state
- Return interface, not concrete type

```go
// Complete factory example
type SessionManager interface {
    Create(key string) (*Session, error)
    Get(key string) (*Session, error)
}

type sessionManager struct {
    logger   *slog.Logger
    sessions map[string]*Session
    mu       sync.RWMutex
}

func NewSessionManager(logger *slog.Logger) SessionManager {
    // Fail fast
    if logger == nil {
        panic("logger is required")
    }

    // Initialize ALL fields
    return &sessionManager{
        logger:   logger,
        sessions: make(map[string]*Session), // CRITICAL!
        mu:       sync.RWMutex{},
    }
}
```

**Why fail fast:**
- Catches configuration errors at startup
- Clear error messages
- Prevents nil pointer panics later

**See:** `.agents/skills/refactoring.md` Pattern 4 for details

### 4. Factory Returns Revealing Interface

**NEVER return concrete types from factories:**

```go
// ❌ BAD: Returns concrete type
func NewSessionManager(logger *slog.Logger) *sessionManager {
    return &sessionManager{...}
}

// ✅ GOOD: Returns interface
func NewSessionManager(logger *slog.Logger) SessionManager {
    return &sessionManager{...}
}
```

**Benefits:**
- Easy to mock for testing
- Can swap implementations
- Hides internal details
- Clear contract for callers

**Never use concrete class directly:**
```go
// ❌ BAD: Using concrete type
var manager *sessionManager = NewSessionManager(logger)

// ✅ GOOD: Using interface
var manager SessionManager = NewSessionManager(logger)
```

**See:** `.agents/skills/ddd-workflow.md` for complete examples

### 5. Duck Typing for Circular References

**When to use:**
- Gateway needs to call domain service
- Circular import would occur

**Pattern:**
```go
// 1. Define interface in CALLING layer (gateway)
// gateway/interfaces.go
type MessagePoster interface {
    PostMessage(channelID, content string) error
}

// 2. Gateway uses interface (no pkg/ import!)
// gateway/handler.go
type Handler struct {
    poster MessagePoster  // Interface, not concrete type
}

func NewHandler(poster MessagePoster) *Handler {
    if poster == nil {
        panic("poster is required")
    }
    return &Handler{poster: poster}
}

// 3. Domain implements interface (no import needed!)
// pkg/message/service.go
type MessageService struct {
    // domain fields
}

func (s *MessageService) PostMessage(channelID, content string) error {
    // Implementation satisfies gateway interface
}

// 4. Wire together in main
// cmd/main.go or gateway/server.go
messageService := message.NewMessageService(...)
handler := gateway.NewHandler(messageService)  // Works!
```

**Why duck typing:**
- No circular imports
- Clean layer separation
- Domain doesn't depend on infrastructure

**See:** `.agents/skills/ddd-workflow.md` Pattern 1 for details

### 6. No Comments That Are Explicit in Code

**Don't comment obvious code:**

```go
// ❌ BAD: Obvious comment
x = x + 1  // Increment x
logger.Info("starting")  // Log info message

// ✅ GOOD: Self-documenting code
userIndex++  // No comment needed
logger.Info("gateway started", "port", port)  // Clear from context
```

**When to comment:**
- Why, not what
- Non-obvious business logic
- Workarounds for known issues
- Complex algorithms

```go
// ✅ GOOD: Explains why
x = x + 1  // Account for zero-based indexing

// ✅ GOOD: Explains business logic
// We cache for 5 minutes because the upstream API rate limits at 12 req/min
cache.Set(key, value, 5*time.Minute)

// ✅ GOOD: Workaround explanation
// TODO: Remove this when upstream bug #1234 is fixed
time.Sleep(100 * time.Millisecond)
```

### 7. Write Gateway Logs (Do Not Use fmt)

**Use structured logger, not fmt:**

```go
// ❌ BAD: Using fmt
fmt.Println("Processing message")
fmt.Printf("User: %s\n", userID)

// ✅ GOOD: Using logger
log := logger.New(logger.ComponentGateway)
log.Info("processing message")
log.Info("message received", "user", userID, "channel", channelID)
```

**Logger initialization:**
```go
// Always initialize logger in factory
func NewHandler(logger *slog.Logger) *Handler {
    if logger == nil {
        panic("logger is required")  // Fail fast
    }
    return &Handler{logger: logger}
}
```

**Structured logging:**
```go
// Use key-value pairs for context
log.Info("event", "key1", value1, "key2", value2)
log.Error("failed", "error", err, "user", userID)
log.Debug("state", "session", sessionID, "count", msgCount)
```

**Log levels:**
- `Debug` - Development/troubleshooting
- `Info` - Important events
- `Warn` - Unexpected but handled
- `Error` - Errors that need attention

**See:** `docs/reference/LOGS.md` for log system details

### 8. Use Constants Instead of "Strings"

**Extract magic strings to constants:**

```go
// ❌ BAD: Magic strings
if channelType == "public" {
    log.Info("creating channel", "type", "public")
}

// ✅ GOOD: Constants
// pkg/channel/types.go
const (
    ChannelTypePublic  = "public"
    ChannelTypePrivate = "private"
    ChannelTypeDM      = "dm"
)

// Usage
if channelType == channel.ChannelTypePublic {
    log.Info("creating channel", "type", channel.ChannelTypePublic)
}
```

**Where to put constants:**
```
pkg/channel/types.go       # Domain constants
pkg/constants/components.go # Shared constants
gateway/constants.go       # Infrastructure constants
```

**Benefits:**
- Type safety
- Autocomplete in IDE
- Easy refactoring
- No typos

**See:** `.agents/skills/refactoring.md` Pattern 5

### 9. Read DDD Design Section

**CRITICAL: Follow DDD boundaries:**

```
pkg/          → Domain (entities, value objects, services, repositories)
gateway/      → Infrastructure + composition (HTTP, WebSocket, DB, engines, billing)
```

**Rule 1 — `pkg/` never imports `gateway/`.** The domain does not know the
transport. This is the rule a grep enforces:

```bash
# Must be empty (it is, as of 2026-08-28)
grep -rl '"memdoor/gateway/' pkg | grep -v "_test.go"
```

**Rule 2 — `gateway/` composes `pkg/`; it does not re-implement it.** Gateway
code imports domain services, repositories and value objects freely (about
half of `gateway/` does — that is its job). What it must not do: put business
rules in handlers, mutate entity internals, or bypass a domain constructor.
When a domain package needs to call BACK into the gateway (a circular
dependency), the interface is declared in the consumer — duck typing, e.g.
`SystemAnnouncementPoster` in `gateway/server.go`, satisfied by
`pkg/message/service.go` (see §5).

**See:**
- `.agents/skills/ddd-workflow.md` - Complete DDD guide
- `AGENTS.md` - DDD design section
- `docs/architecture/ALWAYS_VALID_PATTERN.md` - DDD example (fail-fast constructors)

### 10. Model Identities as Value Objects, Not Bare Strings

**A domain identity (a slug, a workspace id, a channel id) is a TYPE, not a `string`.** Bare strings let a caller swap arguments and the compiler stays silent; a named type makes the swap a compile error.

```go
// ❌ BAD: two swappable strings — nothing stops a caller passing them in the
//         wrong order, and the membership query then asks about the wrong one.
func ListByChannel(workspaceID, channelID string) ([]Member, error)
func IsMember(actorID, channelID string) (bool, error)  // got the actor → always false

// ✅ GOOD: distinct named types; the swap won't compile
type WorkspaceID string
type ChannelID string
func ListByChannel(p WorkspaceID, c ChannelID) ([]Member, error)
```

This is not theoretical: a bad identity in a key column is a zero-row query that
no test notices — the handler just renders empty.

**Put the behavior ON the value object** — computation, not just data:

```go
// All channel-id behavior lives on ChannelID — callers never hand-build the
// string (pkg/channel/channel_id.go is the live example).
func (id ChannelID) IsZero() bool
func (id ChannelID) Validate() error
func (id ChannelID) String() string
```

**Two constructors, two contracts** (ties to Rule 3 — fail fast):

```go
// Derive from untrusted free-form input → infallible, normalizes.
func NewSlug(title string) Slug              // "Héllo Wörld!" → "hello-world"

// Assert an existing value IS valid → fail-fast, returns error.
func ParseSlug(s string) (Slug, error)       // rejects "Has Space", "" at the boundary
func ParseWorkspaceID(s string) (WorkspaceID, error)
```

Validate at the boundary (CLI arg, HTTP field, lint finding) with the `Parse*` constructor so a malformed identity is rejected immediately instead of flowing inward as a bad filename or a zero-row query. Add `IsEmpty()` to the type and use it instead of repeating `== ""`.

Named string types serialize identically (DB columns, JSON, on-disk paths unchanged) and pass straight to `database/sql` — adoption is a pure refactor.

**See:** `pkg/channel/channel_id.go` and `pkg/shared/session_id.go` for the canonical examples.

---

## Quick Reference

### Before Coding
- [ ] Search for duplicates: `grep -r "pattern" .`
- [ ] Check for existing factories/interfaces
- [ ] Read DDD section if adding domain logic
- [ ] Plan factory with interface return

### While Coding
- [ ] Use factory pattern (fail fast, return interface)
- [ ] Identities are value objects, not bare `string` (Slug, WorkspaceID, ChannelID)
- [ ] Behavior on the value object; validate at the boundary with a fail-fast `Parse*`
- [ ] Initialize ALL fields in factory
- [ ] Use duck typing for circular refs
- [ ] Use logger, not fmt
- [ ] Extract constants from strings
- [ ] No obvious comments

### After Coding
- [ ] Test via CLI (not direct DB/HTTP)
- [ ] Verify no pkg/ imports in gateway/
- [ ] Check factory returns interface
- [ ] Run `.agents/skills/quick-test.md`

---

## Checklist for Factory Creation

When creating a new factory:

- [ ] Factory name: `New<Type>(dependencies) Interface`
- [ ] Fail fast: Panic if dependencies nil
- [ ] Return type: Interface, not concrete struct
- [ ] Initialize maps: `make(map[K]V)`
- [ ] Initialize slices: `make([]T, 0)` or `[]T{}`
- [ ] Initialize channels: `make(chan T)`
- [ ] Set logger if needed
- [ ] Set mutexes if needed
- [ ] All fields have stable state

**Example:**
```go
type Service interface {
    DoWork() error
}

type service struct {
    logger *slog.Logger
    cache  map[string]string
    queue  []Task
    mu     sync.RWMutex
}

func NewService(logger *slog.Logger) Service {
    if logger == nil {
        panic("logger is required")
    }

    return &service{
        logger: logger,
        cache:  make(map[string]string),
        queue:  make([]Task, 0),
        mu:     sync.RWMutex{},
    }
}
```

---

## Common Violations

### ❌ Using fmt Instead of Logger

```go
// BAD
fmt.Println("Starting gateway")
fmt.Printf("Error: %v\n", err)

// GOOD
log.Info("starting gateway")
log.Error("operation failed", "error", err)
```

### ❌ Magic Strings

```go
// BAD
if role == "admin" {
    log.Info("user role", "role", "admin")
}

// GOOD
if role == constants.RoleAdmin {
    log.Info("user role", "role", constants.RoleAdmin)
}
```

### ❌ Direct Struct Creation

```go
// BAD
manager := &sessionManager{
    logger: logger,
    sessions: make(map[string]*Session),
}

// GOOD
manager := NewSessionManager(logger)
```

### ❌ Gateway Importing Domain

```go
// BAD - gateway/handler.go
import "memdoor/pkg/message"

type Handler struct {
    messageService *message.MessageService  // Direct coupling!
}

// GOOD - gateway/interfaces.go
type MessagePoster interface {
    PostMessage(channelID, content string) error
}

type Handler struct {
    poster MessagePoster  // Duck typing!
}
```

### ❌ Uninitialized Map/Slice

```go
// BAD
type service struct {
    cache map[string]string  // nil!
}

func NewService() *service {
    return &service{}  // cache is nil, will panic on use
}

// GOOD
func NewService() Service {
    return &service{
        cache: make(map[string]string),  // Initialized!
    }
}
```

---

## Related Skills

- `.agents/skills/refactoring.md` - Refactoring patterns
- `.agents/skills/ddd-workflow.md` - DDD patterns and examples
- `.agents/skills/quick-test.md` - Testing workflow
- `AGENTS.md` - Complete development guide

---

## Summary

**Remember the 10 principles:**

1. ✅ **Search first** - Avoid duplicates (code reuse)
2. ✅ **Test via CLI** - Not direct DB/HTTP
3. ✅ **Use factories** - Fail fast initialization
4. ✅ **Return interface** - Never concrete types
5. ✅ **Duck typing** - For circular refs
6. ✅ **No obvious comments** - Self-documenting code
7. ✅ **Use logger** - Not fmt
8. ✅ **Use constants** - Not magic strings
9. ✅ **Follow DDD** - Read design section
10. ✅ **Value objects, not bare strings** - Type-safe identities; behavior on the type; fail-fast `Parse*` at the boundary

**The three that keep biting if ignored: type-safety (10), fail-fast constructors (3), code reuse (1).**

**These are Memdoor-specific rules that enhance general best practices!**