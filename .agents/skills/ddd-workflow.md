---
name: ddd-workflow
description: Domain-Driven Design workflow for Memdoor development
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: 🏛️
    skillKey: ddd-workflow
    always: false
    os:
      - darwin
      - linux
---

# DDD Workflow Skill

Domain-Driven Design patterns and workflows for Memdoor development.

## Core Principles

### Layer Separation

```
pkg/          → Domain Layer (entities, value objects, domain logic)
gateway/      → Infrastructure Layer (HTTP, WebSocket, database, external APIs)
```

**CRITICAL RULE:** `pkg/` never imports `gateway/`. The gateway composes
domain services, repositories and value objects (it imports `pkg/` freely);
it must not put business rules in handlers or bypass a domain constructor.
Duck typing is for the one circular case: a domain package calling BACK into
the gateway declares the interface it needs in the consumer.

### Quick Check

```bash
# Verify the domain never imports the gateway (excluding tests)
grep -rl '"memdoor/gateway/' pkg | grep -v "_test.go"

# Must be empty
```

---

## DDD Patterns

### Pattern 1: Duck Typing for Circular Dependencies

**Problem:** Gateway needs to call domain service, but domain shouldn't depend on infrastructure.

**Solution:** Define interface in infrastructure, implement in domain.

```go
// ❌ WRONG: Gateway imports domain
// gateway/handler.go
import "memdoor/pkg/message"  // BAD!

func (h *Handler) Process() {
    h.messageService.Post(...)  // Direct coupling
}

// ✅ CORRECT: Duck typing
// gateway/interfaces.go (infrastructure defines need)
type MessagePoster interface {
    PostMessage(channelID, content string) error
}

// gateway/handler.go (infrastructure uses interface)
type Handler struct {
    poster MessagePoster  // No domain import!
}

func NewHandler(poster MessagePoster) *Handler {
    return &Handler{poster: poster}
}

// pkg/message/service.go (domain implements interface)
type MessageService struct {
    // domain logic
}

func (s *MessageService) PostMessage(channelID, content string) error {
    // Implementation satisfies gateway interface without import
}

// cmd/main.go (wiring layer)
messageService := message.NewMessageService(...)
handler := gateway.NewHandler(messageService)  // Dependency injection
```

**Example from codebase:**
- `SystemAnnouncementPoster` interface in gateway
- Implemented by message service in pkg/

### Pattern 2: Factory Returns Interface

**Rule:** Factories MUST return interfaces, not concrete types

```go
// ❌ WRONG: Returns concrete type
func NewSessionManager(logger *slog.Logger) *sessionManager {
    return &sessionManager{logger: logger}
}

// ✅ CORRECT: Returns interface
type SessionManager interface {
    Create(key string) (*Session, error)
    Get(key string) (*Session, error)
    Delete(key string) error
}

type sessionManager struct {
    logger   *slog.Logger
    sessions map[string]*Session
}

func NewSessionManager(logger *slog.Logger) SessionManager {
    if logger == nil {
        panic("logger is required")  // Fail fast!
    }
    return &sessionManager{
        logger:   logger,
        sessions: make(map[string]*Session),
    }
}
```

**Benefits:**
- Easy to mock for testing
- Can swap implementations
- Hides internal details
- Clear contract

### Pattern 3: Fail Fast in Constructors

**Rule:** Panic on nil dependencies in constructors

```go
func NewService(logger *slog.Logger, db *sql.DB) Service {
    if logger == nil {
        panic("logger is required")  // Fail fast!
    }
    if db == nil {
        panic("database is required")  // Fail fast!
    }

    return &service{
        logger: logger,
        db:     db,
    }
}
```

**Why:**
- Catches configuration errors immediately
- Prevents nil pointer panics later
- Clear error messages at startup

### Pattern 4: Predicate Pattern

**Use for:** Complex validation and filtering logic

```go
// Define predicate interface
type Specification interface {
    IsSatisfiedBy(entity Entity) bool
}

// Implement specific predicates
type IsActiveSpec struct{}

func (s IsActiveSpec) IsSatisfiedBy(entity Entity) bool {
    return entity.Status == StatusActive
}

type HasPermissionSpec struct {
    permission string
}

func (s HasPermissionSpec) IsSatisfiedBy(entity Entity) bool {
    return entity.HasPermission(s.permission)
}

// Compose predicates
type AndSpec struct {
    specs []Specification
}

func (s AndSpec) IsSatisfiedBy(entity Entity) bool {
    for _, spec := range s.specs {
        if !spec.IsSatisfiedBy(entity) {
            return false
        }
    }
    return true
}

// Usage
spec := AndSpec{
    specs: []Specification{
        IsActiveSpec{},
        HasPermissionSpec{permission: "edit"},
    },
}

if spec.IsSatisfiedBy(user) {
    // Allow action
}
```

**Example from codebase:**
- Mention validation using Specification Pattern
- Performance: `Perf: Optimize mention validation with Specification Pattern`

### Pattern 5: Constants Instead of Magic Strings

**Rule:** Always extract string literals to constants

```go
// ❌ WRONG: Magic strings
if channelType == "public" {
    log.Info("channel created", "type", "public")
}

// ✅ CORRECT: Constants
// pkg/channel/types.go
const (
    ChannelTypePublic  = "public"
    ChannelTypePrivate = "private"
    ChannelTypeDM      = "dm"
)

// pkg/constants/log_components.go
const (
    ComponentGateway = "Gateway"
    ComponentAgent   = "Agent"
    ComponentChannel = "Channel"
)

// Usage
if channelType == channel.ChannelTypePublic {
    log.Info("channel created", "component", constants.ComponentChannel, "type", channel.ChannelTypePublic)
}
```

**Benefits:**
- Type safety
- Autocomplete
- Easy refactoring
- No typos

---

## DDD Workflow

### Adding New Feature

**Step 1: Identify Layer**

```bash
# Ask: Is this domain logic or infrastructure?

# Domain logic → pkg/
# - Business rules
# - Entities (User, Channel, Message)
# - Value objects
# - Domain services

# Infrastructure → gateway/
# - HTTP handlers
# - WebSocket connections
# - Database access
# - External API calls
```

**Step 2: Check for Existing Code**

```bash
# Search for similar functionality
grep -r "SimilarFeature" pkg/
grep -r "SimilarFeature" gateway/

# Find existing factories
grep -r "New.*Factory" pkg/
grep -r "func New" pkg/

# Find existing interfaces
grep -r "type.*interface" pkg/
```

**Step 3: Design Interfaces First**

```go
// 1. Define domain interface (if needed)
// pkg/user/user.go
type UserRepository interface {
    Create(user *User) error
    FindByID(id string) (*User, error)
    FindByEmail(email string) (*User, error)
}

// 2. Define infrastructure needs
// gateway/interfaces.go
type UserService interface {
    RegisterUser(email, password string) error
    AuthenticateUser(email, password string) (string, error)
}

// 3. Implement domain logic
// pkg/user/service.go
type userService struct {
    repo UserRepository
}

func NewUserService(repo UserRepository) UserService {
    if repo == nil {
        panic("repository is required")
    }
    return &userService{repo: repo}
}

// 4. Implement infrastructure
// gateway/user_handler.go
type UserHandler struct {
    userService UserService
}

func NewUserHandler(userService UserService) *UserHandler {
    return &UserHandler{userService: userService}
}
```

**Step 4: Wire Dependencies**

```go
// cmd/main.go or gateway/server.go
func main() {
    // Infrastructure
    db := initDatabase()
    logger := initLogger()

    // Domain repositories (infrastructure implementations)
    userRepo := persistence.NewUserRepository(db)

    // Domain services
    userService := user.NewUserService(userRepo)

    // Infrastructure handlers
    userHandler := gateway.NewUserHandler(userService)

    // Server
    server := gateway.NewServer(userHandler)
    server.Start()
}
```

---

## DDD Checklist

### Before Coding
- [ ] Identified correct layer (domain vs infrastructure)
- [ ] Searched for existing similar code
- [ ] Designed interfaces first
- [ ] Planned dependency injection

### During Coding
- [ ] No pkg/ imports in gateway/ (except tests)
- [ ] Using duck typing for circular deps
- [ ] Factories return interfaces
- [ ] Failing fast on nil dependencies
- [ ] Using constants instead of strings
- [ ] Following naming conventions

### After Coding
- [ ] No domain imports in infrastructure
- [ ] All factories return interfaces
- [ ] Clear separation of concerns
- [ ] Tests written for domain logic
- [ ] Integration tests for infrastructure

---

## Verification Commands

### Check DDD Violations

```bash
# 1. Domain imports in infrastructure
grep -r "\"memdoor/pkg/" gateway/ | grep -v "_test.go"

# 2. Concrete types returned from factories
grep -r "func New.*\*" pkg/ | grep -v "error"

# 3. Magic strings (find candidates)
grep -r "\"public\"" . | grep -v "const"
grep -r "\"private\"" . | grep -v "const"

# 4. Missing interfaces
# Look for: func New* that return concrete types
grep -r "func New" pkg/

# 5. Missing fail-fast checks
grep -r "func New" pkg/ -A 5 | grep -v "panic"
```

### Fix Common Violations

```bash
# Found domain import in gateway?
# → Use duck typing pattern

# Found concrete type return?
# → Extract interface, return interface

# Found magic string?
# → Extract to constants file

# Found missing panic check?
# → Add fail-fast validation
```

---

## Common Scenarios

### Scenario 1: Gateway Needs Domain Service

**❌ Wrong Approach:**
```go
// gateway/handler.go
import "memdoor/pkg/message"  // NO!

type Handler struct {
    messageService *message.MessageService
}
```

**✅ Correct Approach:**
```go
// gateway/interfaces.go
type MessagePoster interface {
    PostMessage(channelID, content string) error
}

// gateway/handler.go
type Handler struct {
    poster MessagePoster
}

func NewHandler(poster MessagePoster) *Handler {
    return &Handler{poster: poster}
}

// pkg/message/service.go (implements interface without import)
func (s *MessageService) PostMessage(channelID, content string) error {
    // Implementation
}
```

### Scenario 2: Domain Needs Infrastructure

**Problem:** Domain service needs to send HTTP request

**❌ Wrong Approach:**
```go
// pkg/user/service.go
import "net/http"  // Domain depending on infrastructure!

func (s *UserService) NotifyExternal(user *User) error {
    resp, err := http.Post(...)  // Direct HTTP call
}
```

**✅ Correct Approach:**
```go
// pkg/user/service.go (domain defines need)
type ExternalNotifier interface {
    Notify(userID, message string) error
}

type UserService struct {
    notifier ExternalNotifier
}

// gateway/notifier.go (infrastructure implements)
type httpNotifier struct {
    client *http.Client
}

func (n *httpNotifier) Notify(userID, message string) error {
    // HTTP implementation
}
```

### Scenario 3: Circular Dependency

**Problem:** Package A needs B, B needs A

**Solution:** Define interface in consuming package

```go
// Package A needs B
// pkg/a/service.go
type BInterface interface {
    DoSomething() error
}

type ServiceA struct {
    b BInterface
}

// Package B implements interface
// pkg/b/service.go
type ServiceB struct {}

func (s *ServiceB) DoSomething() error {
    // Implementation satisfies A's interface
}
```

---

## Best Practices

### DO

✅ **Define interfaces in consumer** (duck typing)
✅ **Factory returns interface**
✅ **Fail fast on nil dependencies**
✅ **Use constants for strings**
✅ **Separate domain from infrastructure**
✅ **Inject dependencies**
✅ **Test domain logic in isolation**

### DON'T

❌ **Import gateway/ from pkg/**
❌ **Return concrete types from factories**
❌ **Use magic strings**
❌ **Mix domain and infrastructure logic**
❌ **Create circular dependencies**
❌ **Skip nil checks in constructors**

---

## Related Documentation

- `AGENTS.md` - Architecture patterns section
- `docs/architecture/ALWAYS_VALID_PATTERN.md` - DDD example (fail-fast constructors)
- `docs/reference/ARCHITECTURE.md` - System architecture
- `.agents/skills/refactoring.md` - Refactoring to DDD patterns
