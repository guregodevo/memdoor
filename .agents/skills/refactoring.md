---
name: refactoring
description: Systematic refactoring workflow for Memdoor codebase following established patterns
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: ♻️
    skillKey: refactoring
    always: false
    os:
      - darwin
      - linux
---

# Refactoring Skill

Systematic refactoring workflow following Memdoor's established patterns and conventions.

## Before Refactoring

### 1. Check for Existing Code

**ALWAYS search before adding new code:**

```bash
# Search for similar functionality
grep -r "FunctionName" .
grep -r "similar_pattern" pkg/
grep -r "duplicate_logic" gateway/

# Find factories
grep -r "New.*Factory" .
grep -r "func New" pkg/

# Find interfaces
grep -r "type.*interface" pkg/
grep -r "type.*interface" gateway/

# Find constants
grep -r "const.*=" pkg/
```

### 2. Read Architecture Patterns

**Review before refactoring:**
- Read `AGENTS.md` DDD design section
- Check `docs/reference/ARCHITECTURE.md`
- Review similar code in codebase

### 3. Identify Refactoring Type

**Common Memdoor refactoring patterns:**
- **Eliminate duplication** - Extract common code
- **Separation of concerns** - Domain vs infrastructure
- **Duck typing** - Resolve circular dependencies
- **Factory pattern** - Use revealing interfaces

---

## Refactoring Patterns

### Pattern 1: Eliminate Duplication

**Commit pattern:** `Refactor: Extract common X to eliminate duplication`

**Example from codebase:**
```
dd8b670 Refactor: Eliminate duplicate directory creation logic in path resolution
127cf74 Refactor: Extract common session persistence code to eliminate duplication
```

**Workflow:**

1. **Identify duplication:**
```bash
# Find repeated code patterns
grep -r "createDirectory" .
grep -r "ensurePathExists" .
```

2. **Extract to common function:**
```go
// Before: Duplicated in multiple places
func handlerA() {
    if err := os.MkdirAll(path, 0755); err != nil {
        return err
    }
}

func handlerB() {
    if err := os.MkdirAll(path, 0755); err != nil {
        return err
    }
}

// After: Extracted to utility
func ensureDirectory(path string) error {
    return os.MkdirAll(path, 0755)
}
```

3. **Use factory if creating objects:**
```go
// Extract to factory
func NewSessionManager(logger *slog.Logger) SessionManager {
    if logger == nil {
        panic("logger is required") // Fail fast
    }
    return &sessionManager{
        logger: logger,
        sessions: make(map[string]*Session),
    }
}
```

### Pattern 2: Separation of Concerns

**Commit pattern:** `Refactor: Improve separation of concerns in X`

**Example from codebase:**
```
bb093ba Refactor: Further improve separation of concerns in session persistence
```

**DDD Rules:**
- `pkg/` = Domain layer (entities, value objects)
- `gateway/` = Infrastructure layer
- **`pkg/` never imports `gateway/`**; the gateway composes domain services and must not re-implement domain rules in handlers

**Workflow:**

1. **Check for violations:**
```bash
# Look for pkg/ imports in gateway/
grep -r "\"memdoor/pkg/" gateway/ | grep -v "_test.go"
```

2. **Use duck typing for callbacks:**
```go
// gateway/interface.go (infrastructure defines interface)
type SystemAnnouncementPoster interface {
    PostSystemAnnouncement(channelID, message string) error
}

// pkg/message/service.go (domain implements interface)
type MessageService struct {
    // ...
}

func (s *MessageService) PostSystemAnnouncement(channelID, message string) error {
    // Domain logic here
}

// gateway/handler.go (infrastructure uses interface)
type Handler struct {
    announcer SystemAnnouncementPoster
}
```

### Pattern 3: Duck Typing for Circular Dependencies

**When to use:**
- Gateway needs to call domain service
- Domain service needs gateway functionality
- Circular import would occur

**Pattern:**
```go
// 1. Define interface in the CALLING layer (gateway)
// gateway/announcer.go
type Announcer interface {
    Announce(msg string) error
}

// 2. Implement in domain layer (pkg)
// pkg/message/service.go
type MessageService struct {}

func (s *MessageService) Announce(msg string) error {
    // Implementation
}

// 3. Use in gateway
// gateway/handler.go
func NewHandler(announcer Announcer) *Handler {
    return &Handler{announcer: announcer}
}
```

### Pattern 4: Factory with Revealing Interface

**Commit pattern:** `Refactor: Use factory pattern for X`

**Rules:**
- Use factory to create structs
- Fail fast on missing dependencies (panic if nil)
- Factory returns interface, not concrete type
- **ALWAYS initialize all fields to stable state**
- Initialize maps, slices, channels
- Set default values for primitives

**Pattern:**
```go
// Define interface (what callers need)
type SessionManager interface {
    Create(key string) (*Session, error)
    Get(key string) (*Session, error)
    Delete(key string) error
}

// Concrete implementation (private)
type sessionManager struct {
    logger   *slog.Logger
    sessions map[string]*Session  // Must be initialized!
    mu       sync.RWMutex
}

// Factory returns interface (public)
func NewSessionManager(logger *slog.Logger) SessionManager {
    // Fail fast on nil dependencies
    if logger == nil {
        panic("logger is required")
    }

    // Return with ALL fields initialized to stable state
    return &sessionManager{
        logger:   logger,
        sessions: make(map[string]*Session), // CRITICAL: Initialize map
        mu:       sync.RWMutex{},            // Explicit zero value (optional but clear)
    }
}
```

**Why initialize everything:**
```go
// ❌ BAD: Uninitialized map causes panic
type service struct {
    cache map[string]string  // nil map!
}

func NewService() *service {
    return &service{}  // cache is nil
}

func (s *service) Store(key, val string) {
    s.cache[key] = val  // PANIC: assignment to nil map
}

// ✅ GOOD: All fields initialized
type service struct {
    cache   map[string]string
    queue   []string
    done    chan bool
    counter int
}

func NewService() *service {
    return &service{
        cache:   make(map[string]string),     // Initialize map
        queue:   make([]string, 0),           // Initialize slice
        done:    make(chan bool),             // Initialize channel
        counter: 0,                           // Explicit zero (optional)
    }
}
```

**Stable State Checklist:**
- [ ] Maps initialized with `make()`
- [ ] Slices initialized with `make()` or `[]T{}`
- [ ] Channels initialized with `make()`
- [ ] Pointers to structs initialized with `&T{}`
- [ ] Interface fields set (or left nil if optional)
- [ ] Primitives set to sensible defaults
- [ ] Mutexes explicitly initialized (optional but clear)

### Pattern 5: Constants Instead of Strings

**Commit pattern:** `Refactor: Use constants for X`

**Before:**
```go
if channelType == "public" {
    // ...
}

log.Info("starting", "component", "gateway")
```

**After:**
```go
// pkg/channel/types.go
const (
    ChannelTypePublic  = "public"
    ChannelTypePrivate = "private"
    ChannelTypeDM      = "dm"
)

// pkg/constants/components.go
const (
    ComponentGateway = "gateway"
    ComponentAgent   = "agent"
)

// Usage
if channelType == channel.ChannelTypePublic {
    // ...
}

log.Info("starting", "component", constants.ComponentGateway)
```

---

## Refactoring Workflow

### Step 1: Identify Target

```bash
# Find code smells
grep -r "TODO" .                # Marked technical debt
grep -r "FIXME" .               # Known issues
grep -r "Duplicate" .           # Explicit duplicates

# Find long functions (>50 lines)
# Find deep nesting (>3 levels)
# Find large files (>500 lines)
```

### Step 2: Plan Refactoring

**Questions to ask:**
- [ ] Does similar code exist elsewhere?
- [ ] Is this domain or infrastructure?
- [ ] Can I extract a factory?
- [ ] Should I use duck typing?
- [ ] Are there magic strings to extract?

### Step 3: Execute Refactoring

**Small incremental changes:**

1. **Make one change**
2. **Run tests:** `make test`
3. **Build:** `make build`
4. **Commit:** Small, focused commit
5. **Repeat**

### Step 4: Verify

**Testing after refactoring:**

```bash
# Build
make build

# Unit tests
make test

# Critical paths
./memdoor agent --message "test" --channel test2 --agent-id writer
./memdoor messages --channel test2 --limit 5

# Check logs
./memdoor logs query --limit 20
./memdoor logs errors --since 5m
```

---

## Refactoring Checklist

### Before Refactoring
- [ ] Searched for existing similar code
- [ ] Read relevant architecture docs
- [ ] Identified refactoring pattern
- [ ] Planned small incremental steps
- [ ] Tests exist for code to refactor

### During Refactoring
- [ ] Making small changes (one pattern at a time)
- [ ] Tests pass after each change
- [ ] Build succeeds after each change
- [ ] Committing frequently
- [ ] Following DDD boundaries

### After Refactoring
- [ ] All tests pass: `make test`
- [ ] Build succeeds: `make build`
- [ ] Critical paths tested manually
- [ ] Logs clean (no new errors)
- [ ] Code review (self-review)
- [ ] Documentation updated if needed

---

## Common Refactoring Scenarios

### Scenario 1: Found Duplicate Code

```bash
# 1. Search for all instances
grep -r "duplicatePattern" .

# 2. Extract to common location
# - If domain logic → pkg/
# - If infrastructure → gateway/
# - If utility → pkg/util/ or gateway/util/

# 3. Use factory pattern
func NewCommonService(deps) CommonService {
    // Fail fast on nil dependencies
    return &commonService{...}
}

# 4. Update all call sites
# 5. Test: make test
# 6. Commit: "Refactor: Extract common X to eliminate duplication"
```

### Scenario 2: Gateway Importing Domain

```bash
# 1. Find violation
grep -r "\"memdoor/pkg/" gateway/

# 2. Apply duck typing
# - Define interface in gateway/
# - Implement in pkg/
# - Pass implementation to gateway

# 3. Test: make test
# 4. Commit: "Refactor: Remove domain dependency from gateway using duck typing"
```

### Scenario 3: Magic Strings Everywhere

```bash
# 1. Find magic strings
grep -r "\"public\"" . | grep -v "const"
grep -r "\"private\"" . | grep -v "const"

# 2. Create constants file
# pkg/channel/types.go or similar

# 3. Replace all uses
# 4. Test: make test
# 5. Commit: "Refactor: Use constants instead of magic strings for channel types"
```

### Scenario 4: Long Function (>50 lines)

```bash
# 1. Identify logical sections
# 2. Extract each section to helper function
# 3. Keep helpers private (lowercase) unless needed elsewhere
# 4. Test after each extraction
# 5. Commit: "Refactor: Extract helpers from X for clarity"
```

---

## Best Practices

### DO

✅ **Search for duplicates first:** `grep -r "pattern" .`
✅ **Follow DDD boundaries:** pkg/ = domain, gateway/ = infrastructure
✅ **Use duck typing for circular deps**
✅ **Factories return interfaces**
✅ **Fail fast in constructors**
✅ **Extract constants from strings**
✅ **Small incremental changes**
✅ **Test after each change**
✅ **Commit frequently**

### DON'T

❌ **Don't refactor without tests**
❌ **Don't import pkg/ from gateway/**
❌ **Don't return concrete types from factories**
❌ **Don't use magic strings**
❌ **Don't make large changes at once**
❌ **Don't skip testing**
❌ **Don't add comments for obvious code**

---

## Git Workflow for Refactoring

### Commit Messages

**Pattern:** `Refactor: <Action> <Target> <Reason>`

**Examples:**
```
Refactor: Eliminate duplicate directory creation logic in path resolution
Refactor: Extract common session persistence code to eliminate duplication
Refactor: Further improve separation of concerns in session persistence
Refactor: Move browser tool to tools/ package for consistency
Refactor: Remove dead/unused code to improve maintainability
```

### Squashing Commits

**Before push:**
```bash
# Interactive rebase to squash
git rebase -i HEAD~5

# Mark commits to squash (s)
pick abc1234 Refactor: Step 1
s def5678 Refactor: Step 2
s ghi9012 Refactor: Step 3

# Result: Single logical commit
Refactor: Extract common session persistence code to eliminate duplication
```

---

## Related Documentation

- `AGENTS.md` - DDD design patterns, architecture
- `docs/reference/ARCHITECTURE.md` - System architecture
- `docs/architecture/ALWAYS_VALID_PATTERN.md` - DDD example (fail-fast constructors)
