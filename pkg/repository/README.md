# Repository Pattern Architecture

This package implements a clean repository pattern that **abstracts away the database**, making it trivial to switch from SQLite (single-tenant) to PostgreSQL (multi-tenant) without changing any business logic code.

## 🎯 The Problem We're Solving

**NOW**: Build the app with SQLite (fast iteration, local testing)
**LATER**: Migrate to PostgreSQL multi-tenant (when you have paying customers)

Without abstraction, you'd need to rewrite tons of code. With this architecture, you just **change config**.

---

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│         Frontend (React) / API Handlers                 │
│         • NO database knowledge                         │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│              Service Layer                              │
│  • Business logic (validation, etc.)                    │
│  • Uses repository INTERFACES only                      │
│  • pkg/service/buddy_service.go                         │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│           Repository Interfaces                         │
│  • BuddyRepository, ChannelRepository, etc.             │
│  • pkg/repository/interfaces.go                         │
└────────────┬────────────────────┬────────────────────────┘
             │                    │
┌────────────▼──────┐   ┌────────▼──────────┐
│ SQLite Impl       │   │ PostgreSQL Impl   │
│ (NOW)             │   │ (LATER)           │
│ • Single-tenant   │   │ • Multi-tenant    │
│ • No workspace    │   │ • Workspace       │
│   filtering       │   │   filtering       │
│ pkg/repository/   │   │ pkg/repository/   │
│   sqlite/         │   │   postgres/       │
└───────────────────┘   └───────────────────┘
```

---

## 📁 Directory Structure

```
pkg/
├── domain/
│   └── models.go               # Domain models (Buddy, Channel, Message, etc.)
├── repository/
│   ├── interfaces.go           # Repository contracts (database-agnostic)
│   ├── sqlite/
│   │   ├── factory.go          # SQLite factory (creates repositories)
│   │   ├── buddy_repository.go # Buddy repository implementation
│   │   └── stubs.go            # Other repositories (TODO)
│   └── postgres/
│       ├── factory.go          # PostgreSQL factory
│       └── stubs.go            # All PostgreSQL repos (not implemented yet)
└── service/
    └── buddy_service.go        # Business logic (uses interfaces only)

gateway/
├── repository/
│   └── factory.go              # Gateway repository factory (uses config)
└── config/
    ├── types.go                # Configuration types (includes DatabaseConfig)
    └── loader.go               # Configuration loader with defaults
```

---

## 🚀 How to Use

### 1. Gateway Integration (Current Implementation)

The gateway automatically creates a repository factory based on configuration:

```go
// In gateway/server.go
import (
    "chat/gateway/config"
    gatewayrepo "chat/gateway/repository"
    "chat/pkg/repository"
)

// Load config (with database defaults)
cfg, err := config.LoadConfigFromFile(configPath)

// Create repository factory from config
repoFactory, err := gatewayrepo.NewRepositoryFactory(cfg.Database)
defer repoFactory.Close()

// Store in server
server := &Server{
    repoFactory: repoFactory,
    // ...
}
```

### 2. Using Repositories in Gateway Code

```go
// Access repositories through server.repoFactory
buddyRepo := server.repoFactory.Buddies()
messageRepo := server.repoFactory.Messages()

// Use repository interface
buddy, err := buddyRepo.GetByID(ctx, id)
messages, err := messageRepo.List(ctx, channelID)
```

### 3. Create Services (Available, Not Yet Integrated)

The service layer is ready but not yet wired into the gateway API:

```go
import "chat/pkg/service"

// Create service with repository
buddyService := service.NewBuddyService(server.repoFactory.Buddies())

// Use service methods
buddy, err := buddyService.CreateBuddy(ctx, service.CreateBuddyRequest{
    Name:          "Marketing Buddy",
    Skills:        []string{"content-writing", "seo"},
    CreatedBy:     userID,
})

buddies, err := buddyService.ListBuddies(ctx)
// In SQLite: Returns ALL buddies
// In PostgreSQL: Returns only buddies in current workspace
```

**Status**: Service layer is implemented, waiting for HTTP/WebSocket API handlers.

---

## 🔄 Switching from SQLite to PostgreSQL

**Step 1**: Implement PostgreSQL repositories (copy pattern from SQLite)
**Step 2**: Run migration script (see `docs/MIGRATION_PLAN.md`)
**Step 3**: Update configuration file:

```json
// Before (SQLite) - ~/.greg/config.json
{
  "database": {
    "type": "sqlite",
    "sqlite": {
      "path": "~/.greg/data/memdoor.db"
    }
  }
}

// After (PostgreSQL) - ~/.greg/config.json
{
  "database": {
    "type": "postgres",
    "postgres": {
      "url": "postgres://user:pass@localhost:5432/cobuddy"
    }
  }
}
```

**Step 4**: Restart gateway. **That's it! No code changes needed.**

---

## 🎨 Key Design Patterns

### 1. Repository Pattern
Each repository handles data access for one domain model (Buddy, Channel, Message).

### 2. Factory Pattern
`RepositoryFactory` creates all repositories at once. Ensures consistency.

```go
type RepositoryFactory interface {
    Buddies() BuddyRepository
    Channels() ChannelRepository
    Messages() MessageRepository
    // ... etc
    Close() error
}
```

### 3. Interface Segregation
Service layer depends on **interfaces**, not concrete implementations.

```go
// ✅ Good
type BuddyService struct {
    buddyRepo repository.BuddyRepository  // Interface
}

// ❌ Bad
type BuddyService struct {
    buddyRepo *sqlite.BuddyRepository  // Concrete type - hard to swap!
}
```

### 4. Dependency Injection
Services receive repositories via constructor, not global singletons.

```go
// ✅ Good
func NewBuddyService(buddyRepo repository.BuddyRepository) *BuddyService {
    return &BuddyService{buddyRepo: buddyRepo}
}

// ❌ Bad
func NewBuddyService() *BuddyService {
    return &BuddyService{buddyRepo: globalDB.Buddies()} // Tight coupling!
}
```

---

## 🔑 Key Differences: SQLite vs PostgreSQL

| Aspect | SQLite (NOW) | PostgreSQL (LATER) |
|--------|--------------|-------------------|
| **Workspace Filtering** | None (single-tenant) | AUTO-INJECTED from context |
| **Query Example** | `SELECT * FROM buddies` | `SELECT * FROM buddies WHERE workspace_id = $1` |
| **Context Usage** | Ignored | **CRITICAL** (must have workspace_id) |
| **Tenant Isolation** | N/A (one workspace) | Enforced by middleware + queries |
| **Performance** | Fast (local file) | Slower (network), but scales better |
| **Deployment** | One instance per customer | One instance, many customers |

### SQLite Implementation
```go
func (r *BuddyRepository) List(ctx context.Context) ([]*domain.Buddy, error) {
    query := `SELECT * FROM buddies ORDER BY name`
    // Returns ALL buddies (no filtering)
    return r.db.QueryContext(ctx, query)
}
```

### PostgreSQL Implementation (LATER)
```go
func (r *BuddyRepository) List(ctx context.Context) ([]*domain.Buddy, error) {
    workspaceID := middleware.MustGetWorkspaceID(ctx) // From JWT
    query := `SELECT * FROM buddies WHERE workspace_id = $1 ORDER BY name`
    // Returns ONLY buddies in current workspace
    return r.db.QueryContext(ctx, query, workspaceID)
}
```

**SAME INTERFACE, DIFFERENT BEHAVIOR!** The service layer doesn't care.

---

## 🧪 Testing

Because we use interfaces, testing is trivial - create mock repositories!

```go
type MockBuddyRepository struct {
    buddies []*domain.Buddy
}

func (m *MockBuddyRepository) Create(ctx context.Context, buddy *domain.Buddy) error {
    m.buddies = append(m.buddies, buddy)
    return nil
}

func (m *MockBuddyRepository) List(ctx context.Context) ([]*domain.Buddy, error) {
    return m.buddies, nil
}

// Test service with mock (no database needed!)
func TestBuddyService_CreateBuddy(t *testing.T) {
    mockRepo := &MockBuddyRepository{}
    service := service.NewBuddyService(mockRepo)

    buddy, err := service.CreateBuddy(ctx, request)
    assert.NoError(t, err)
    assert.Equal(t, 1, len(mockRepo.buddies))
}
```

---

## 📋 TODO: Remaining Work

### SQLite Repositories (Priority: HIGH)
- [x] BuddyRepository (DONE - full implementation)
- [ ] ChannelRepository (stub - needs implementation)
- [ ] MessageRepository (stub - needs implementation)
- [ ] UserRepository (stub - needs implementation)
- [ ] AgentEventRepository (stub - needs implementation)
- [ ] BuddyMemoryRepository (stub - needs implementation)
- [ ] SessionRepository (stub - needs implementation)

### PostgreSQL Repositories (Priority: LATER)
- [ ] All repositories (stubs only - implement when migrating)

### Services
- [x] BuddyService (DONE)
- [ ] ChannelService
- [ ] MessageService
- [ ] UserService

---

## 🎯 Benefits of This Architecture

1. **Easy Migration**: Switch databases by changing config (just update `~/.greg/config.json`)
2. **Testable**: Mock repositories for unit tests (no real database needed)
3. **Flexible**: Add new storage backends (Redis? MongoDB?) without changing services
4. **Clear Separation**: Business logic (services) separate from data access (repositories)
5. **Type-Safe**: Compiler ensures interface contracts are followed

---

## 📚 References

- [Repository Pattern (Martin Fowler)](https://martinfowler.com/eaaCatalog/repository.html)
- [Clean Architecture (Uncle Bob)](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Dependency Injection in Go](https://google.github.io/wire/)

---

## Example: How Gateway Uses It

```go
// Gateway startup (gateway/server.go)

// 1. Load configuration with defaults
cfg, _ := config.LoadConfigFromFile(configPath)

// 2. Create repository factory
repoFactory, _ := gatewayrepo.NewRepositoryFactory(cfg.Database)

// 3. Store in server
server := &Server{
    repoFactory: repoFactory,
    // ...
}

// 4. Use repositories in handlers (when implemented)
buddyRepo := server.repoFactory.Buddies()
buddy, err := buddyRepo.GetByID(ctx, id)
```

**To switch to PostgreSQL later:**
1. Implement PostgreSQL repositories (copy SQLite pattern)
2. Update `database.type: "postgres"` in `~/.greg/config.json`
3. **Done! No code changes.**

---

**Current Status**: ✅ Architecture complete, integrated into gateway, SQLite working

**Next Steps**: Add HTTP/WebSocket API handlers using repositories