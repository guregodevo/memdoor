# ExecutionContext: Domain Model for Request-Scoped Authentication

## Overview

`ExecutionContext` is a **domain model** (Value Object) that represents the runtime execution context for a request. It captures **WHO** is performing an action (`ActorID`) and **WHERE** (`WorkspaceID`) in a multi-tenant system.

## Design Patterns

### "Always Valid" Pattern

**ExecutionContext** uses the **"Always Valid"** pattern - validation happens at construction, so instances are guaranteed to be valid throughout their lifetime.

```go
//  Good: Factory enforces invariants
execCtx, err := shared.NewExecutionContext(actorID, workspaceID)
if err != nil {
    return err  // Invalid inputs caught here
}

//  Now execCtx is guaranteed valid - no need to check again!
workspaceID := execCtx.WorkspaceID  // Safe to use
actorID := execCtx.ActorID          // Safe to use

//  Bad: Manual construction bypasses validation
execCtx := &shared.ExecutionContext{  // DON'T DO THIS!
    ActorID: "",           // Invalid!
    WorkspaceID: "",       // Invalid!
}
```

**Benefits**:
-  **Validation happens once** - at construction
-  **No defensive checks needed** - instances are guaranteed valid
-  **Compile-time enforcement** - constructor is the only way to create instances
-  **Simpler code** - no `if err := validate()` scattered everywhere

**From Domain-Driven Design (Eric Evans)**:
> "Make illegal states unrepresentable. Push validation to the boundaries and keep your domain models always valid."

## Design Rationale

### Problem: Scattered Context Values

**Before** (anti-pattern):
```go
// Multiple individual context values scattered across code
ctx = context.WithValue(ctx, "actor_id", actorID)
ctx = context.WithValue(ctx, "workspace_id", workspaceID)
ctx = context.WithValue(ctx, "user_email", email)

// Extracting values requires knowing all the keys
actorID, _ := ctx.Value("actor_id").(shared.ActorID)
workspaceID, _ := ctx.Value("workspace_id").(string)
```

**Issues**:
-  No type safety
-  No validation
-  Easy to forget values when copying context
-  Not a domain concept - just infrastructure plumbing

### Solution: ExecutionContext as Domain Model

**After** (DDD pattern):
```go
// Single cohesive domain model
execCtx := shared.NewExecutionContext(actorID, workspaceID)
ctx = shared.WithExecutionContext(ctx, execCtx)

// Type-safe extraction
execCtx := shared.GetExecutionContext(ctx)
workspaceID := execCtx.WorkspaceID  // No type assertion needed!
```

**Benefits**:
-  Type-safe - compile-time checks
-  Validated - `execCtx.Validate()` ensures correctness
-  Cohesive - all auth context in one place
-  Domain model - expresses business concept
-  Easy to extend - add fields without changing signatures

## Architecture

### DDD Classification

**ExecutionContext** is a **Value Object** in the **Shared Kernel**:

- **Value Object**: Immutable, identified by values (not ID), passed by value
- **Shared Kernel**: Used across all bounded contexts (Team Collaboration, Multi-Agent Platform)

### Layer Responsibilities

```
┌─────────────────────────────────────────────────────┐
│ Infrastructure Layer (HTTP Middleware)              │
│ - Extracts JWT token from Authorization header     │
│ - Creates ExecutionContext from token claims        │
│ - Adds to context.Context                           │
└──────────────────┬──────────────────────────────────┘
                   │ ExecutionContext in context
┌──────────────────▼──────────────────────────────────┐
│ Application Layer (Service Layer)                   │
│ - Extracts ExecutionContext from context           │
│ - Uses for authorization checks                     │
│ - Passes to domain layer for validation            │
└──────────────────┬──────────────────────────────────┘
                   │ ExecutionContext as parameter
┌──────────────────▼──────────────────────────────────┐
│ Domain Layer (Aggregates)                           │
│ - Validates business rules using ExecutionContext  │
│ - E.g., "user is member of channel"                │
│ - Enforces invariants                               │
└─────────────────────────────────────────────────────┘
```

## Usage Examples

### 1. Infrastructure Layer: Auth Middleware

```go
// pkg/authorization/middleware.go

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract token from header
        token := extractBearerToken(r)

        // Verify and get user info
        user, err := m.authService.VerifyToken(token)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Create ExecutionContext (domain model)
        execCtx := shared.NewExecutionContextWithEmail(
            shared.ActorID(user.ID),
            user.WorkspaceID,
            user.Email,
        )

        // Add to context
        ctx := shared.WithExecutionContext(r.Context(), execCtx)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 2. Application Layer: Service

```go
// pkg/message/service.go

func (s *Service) PostMessage(ctx context.Context, req PostMessageRequest) (*Message, error) {
    // Extract ExecutionContext
    execCtx := shared.GetExecutionContext(ctx)
    if execCtx == nil {
        return nil, fmt.Errorf("authentication required")
    }

    // Use for authorization
    isMember, err := s.membershipRepo.IsMember(ctx, req.ChannelID, execCtx.ActorID)
    if !isMember {
        return nil, fmt.Errorf("actor %s not member of channel", execCtx.ActorID)
    }

    // Use workspace for multi-tenancy isolation
    sessionKey := fmt.Sprintf("workspace:%s:channel:%s",
        execCtx.WorkspaceID, req.ChannelID)

    // Continue with message creation...
}
```

### 3. Async Goroutines: Detached Context

```go
// pkg/message/service.go

func (s *Service) PostMessage(ctx context.Context, req PostMessageRequest) (*Message, error) {
    // ... save message ...

    // Trigger async agent execution
    if msg.HasMentions() {
        // IMPORTANT: Copy ExecutionContext to detached context
        // HTTP request context gets canceled after response is sent
        asyncCtx := context.Background()

        if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
            asyncCtx = shared.WithExecutionContext(asyncCtx, execCtx)
        }

        go s.handleAgentMentions(asyncCtx, msg)
    }

    return msg, nil
}
```

## Migration Strategy

### Backward Compatibility

The implementation supports **gradual migration** from legacy individual context values:

```go
// Legacy code (still works):
actorID, _ := ctx.Value(sharedctx.ActorIDKey).(shared.ActorID)
workspaceID, _ := ctx.Value(sharedctx.WorkspaceIDKey).(string)

// New code (preferred):
execCtx := shared.GetExecutionContext(ctx)
actorID := execCtx.ActorID
workspaceID := execCtx.WorkspaceID
```

### Migration Steps

1.  **Add ExecutionContext model** (`pkg/shared/execution_context.go`)
2.  **Update middleware** to create ExecutionContext + set legacy values
3.  **Update services** to prefer ExecutionContext, fallback to legacy
4. ⏳ **Remove legacy context values** (breaking change - do last)

### Deprecation Path

```go
// CURRENT STATE: Both patterns work
ctx = shared.WithExecutionContext(ctx, execCtx)              // New
ctx = context.WithValue(ctx, ActorIDKey, execCtx.ActorID)   // Legacy (for backward compat)

// FUTURE STATE: Only ExecutionContext
ctx = shared.WithExecutionContext(ctx, execCtx)              // Only this
```

## Testing

### Unit Tests

```go
func TestExecutionContext_Validate(t *testing.T) {
    // Valid context
    execCtx := shared.NewExecutionContext(
        shared.NewHumanActorID("user-123"),
        "workspace-456",
    )
    assert.NoError(t, execCtx.Validate())

    // Invalid: empty workspace
    execCtx = &shared.ExecutionContext{
        ActorID: shared.NewHumanActorID("user-123"),
        WorkspaceID: "",
    }
    assert.Error(t, execCtx.Validate())
}
```

### Integration Tests

```go
func TestMessageService_WithExecutionContext(t *testing.T) {
    ctx := context.Background()

    // Simulate middleware setting ExecutionContext
    execCtx := shared.NewExecutionContext(
        shared.NewHumanActorID("user-123"),
        "default",
    )
    ctx = shared.WithExecutionContext(ctx, execCtx)

    // Post message
    msg, err := messageService.PostMessage(ctx, req)

    // Verify workspace isolation
    assert.Contains(t, sessionPath, "workspaces/default/")
}
```

## Benefits

### 1. Type Safety

```go
// Before: Runtime error if wrong type
workspaceID := ctx.Value("workspace_id").(string) // Panics if wrong type!

// After: Compile-time checks
execCtx := shared.GetExecutionContext(ctx)
workspaceID := execCtx.WorkspaceID  // Type-safe
```

### 2. Validation

```go
execCtx := shared.NewExecutionContext(actorID, workspaceID)
if err := execCtx.Validate(); err != nil {
    return fmt.Errorf("invalid execution context: %w", err)
}
```

### 3. Extensibility

```go
// Easy to add fields without breaking existing code
type ExecutionContext struct {
    ActorID     ActorID
    WorkspaceID string
    UserEmail   string    // Added
    Permissions []string  // Added later
    Quota       *Quota    // Added later
}
```

### 4. Discoverability

```go
// Before: How do I know what context keys exist?
// Answer: Search codebase for "context.WithValue"

// After: Just look at ExecutionContext struct!
type ExecutionContext struct {
    ActorID     ActorID    // ← Discoverable
    WorkspaceID string     // ← Documented
    UserEmail   string     // ← Type-safe
}
```

## Related Patterns

### Context Object Pattern

From *Patterns of Enterprise Application Architecture* (Fowler):

> "Encapsulates a context, carrying request-scoped information that must be accessible by objects without explicit passing through all method calls."

### Value Object (DDD)

From *Domain-Driven Design* (Evans):

> "An object that represents a descriptive aspect of the domain with no conceptual identity. Two value objects with the same values can be considered equal."

## See Also

- `pkg/shared/actor_id.go` - ActorID value object
- `pkg/shared/context/keys.go` - Legacy context keys (deprecated)
- `pkg/authorization/middleware.go` - ExecutionContext creation
- `pkg/message/service.go` - ExecutionContext usage example
