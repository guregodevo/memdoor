# "Always Valid" Pattern in Memdoor

## Overview

The **"Always Valid"** pattern is a DDD design principle where domain objects are validated at construction and remain valid throughout their lifetime. This eliminates defensive validation checks scattered throughout the codebase.

## Core Principle

> **"Make illegal states unrepresentable"** - Yaron Minsky

If an object exists, it is valid. Period.

## Implementation

### Factory Constructor Pattern

```go
//  CORRECT: Factory enforces invariants
func NewExecutionContext(actorID ActorID, workspaceID string) (*ExecutionContext, error) {
    // Validation happens HERE - at construction
    if err := actorID.Validate(); err != nil {
        return nil, fmt.Errorf("invalid actor_id: %w", err)
    }

    if workspaceID == "" {
        return nil, fmt.Errorf("workspace_id cannot be empty")
    }

    // If we reach here, all invariants are satisfied
    return &ExecutionContext{
        ActorID:     actorID,
        WorkspaceID: workspaceID,
    }, nil
}

// Usage
execCtx, err := NewExecutionContext(actorID, workspaceID)
if err != nil {
    return err  // Invalid inputs rejected at boundary
}

// From this point on, execCtx is GUARANTEED to be valid
// No need for defensive checks!
workspaceID := execCtx.WorkspaceID  // Safe
actorID := execCtx.ActorID          // Safe
```

### What NOT to Do

```go
//  WRONG: Direct struct construction bypasses validation
execCtx := &ExecutionContext{
    ActorID:     "",  // Invalid!
    WorkspaceID: "",  // Invalid!
}

//  WRONG: Validation scattered throughout code
func ProcessMessage(execCtx *ExecutionContext) error {
    // Defensive validation - shouldn't be necessary!
    if err := execCtx.Validate(); err != nil {
        return err
    }

    // ... business logic ...
}
```

## Benefits

### 1. Validation Happens Once

**Before** (defensive programming):
```go
func CreateMessage(ctx ExecutionContext) error {
    if err := ctx.Validate(); err != nil {  // Check 1
        return err
    }
    // ... logic ...
}

func SaveMessage(ctx ExecutionContext) error {
    if err := ctx.Validate(); err != nil {  // Check 2 (redundant!)
        return err
    }
    // ... logic ...
}

func BroadcastMessage(ctx ExecutionContext) error {
    if err := ctx.Validate(); err != nil {  // Check 3 (redundant!)
        return err
    }
    // ... logic ...
}
```

**After** (always valid):
```go
func CreateMessage(ctx ExecutionContext) error {
    // No validation needed - ctx is guaranteed valid!
    // ... logic ...
}

func SaveMessage(ctx ExecutionContext) error {
    // No validation needed - ctx is guaranteed valid!
    // ... logic ...
}

func BroadcastMessage(ctx ExecutionContext) error {
    // No validation needed - ctx is guaranteed valid!
    // ... logic ...
}
```

### 2. Simpler Code

- **Less boilerplate** - no `Validate()` calls everywhere
- **Easier to read** - focus on business logic, not defensive checks
- **Fewer bugs** - can't forget to validate

### 3. Type System Enforces Correctness

```go
// Compile-time guarantee: if you have an ExecutionContext, it's valid
func ProcessRequest(execCtx *ExecutionContext) {
    // No need to check - type system guarantees validity!
    workspace := execCtx.WorkspaceID  //  Safe
}
```

### 4. Clear Error Boundaries

```go
// Errors happen at boundaries (construction)
execCtx, err := NewExecutionContext(actorID, workspaceID)
if err != nil {
    // Handle invalid input from external source
    logger.Error("Invalid execution context from request",
        slog.String("error", err.Error()))
    return http.StatusBadRequest
}

// From here on, no error handling needed for validation
processMessage(execCtx)  //  Can't fail due to invalid context
saveMessage(execCtx)     //  Can't fail due to invalid context
```

## When to Use This Pattern

###  Use "Always Valid" For:

- **Value Objects** (e.g., `ExecutionContext`, `Email`, `Money`)
- **Entities with invariants** (e.g., `Order` must have at least one item)
- **Domain primitives** (e.g., `PositiveInteger`, `NonEmptyString`)

###  Don't Use For:

- **DTOs/API payloads** - validation happens separately
- **Database models** - may be partially loaded
- **Mutable objects** - can become invalid after construction

## Examples in Memdoor

### ExecutionContext

```go
// pkg/shared/execution_context.go

// Factory constructor enforces invariants
func NewExecutionContext(actorID ActorID, workspaceID string) (*ExecutionContext, error) {
    if err := actorID.Validate(); err != nil {
        return nil, fmt.Errorf("invalid actor_id: %w", err)
    }

    if workspaceID == "" {
        return nil, fmt.Errorf("workspace_id cannot be empty")
    }

    return &ExecutionContext{
        ActorID:     actorID,
        WorkspaceID: workspaceID,
    }, nil
}

// Validate() method is DEPRECATED - not needed!
func (ec *ExecutionContext) Validate() error {
    // Always returns nil for instances created via constructor
    return nil
}
```

### ActorID

```go
// pkg/shared/actor_id.go

type ActorID string

// Factory enforces format
func NewHumanActorID(identityID string) ActorID {
    return ActorID(fmt.Sprintf("human:%s", identityID))
}

func NewAgentActorID(agentID string) ActorID {
    return ActorID(fmt.Sprintf("agent:%s", agentID))
}

// Usage
actorID := shared.NewHumanActorID("user-123")  //  Valid by construction
// No need to validate - format is guaranteed correct!
```

## Comparison with Other Patterns

### vs. Defensive Programming

**Defensive Programming**:
```go
func Process(x *Foo) error {
    if x == nil {
        return errors.New("x is nil")
    }
    if err := x.Validate(); err != nil {
        return err
    }
    // ... business logic ...
}
```

**Always Valid**:
```go
func Process(x Foo) {  // Non-pointer - can't be nil
    // x is guaranteed valid - no checks needed
    // ... business logic ...
}
```

### vs. Builder Pattern

**Builder Pattern** (allows invalid intermediate states):
```go
builder := NewExecutionContextBuilder()
builder.SetActorID(actorID)  // Incomplete! WorkspaceID missing
ctx := builder.Build()  //  Could return invalid object
```

**Always Valid** (no invalid states possible):
```go
ctx, err := NewExecutionContext(actorID, workspaceID)  //  Both required
// If err == nil, ctx is complete and valid
```

## Testing

### Unit Tests Focus on Boundaries

```go
func TestNewExecutionContext_Valid(t *testing.T) {
    // Test valid construction
    execCtx, err := shared.NewExecutionContext(
        shared.NewHumanActorID("user-123"),
        "workspace-456",
    )

    require.NoError(t, err)
    assert.NotNil(t, execCtx)
    // No need to call Validate() - guaranteed valid!
}

func TestNewExecutionContext_InvalidActorID(t *testing.T) {
    // Test rejection at boundary
    _, err := shared.NewExecutionContext(
        shared.ActorID(""),  // Invalid!
        "workspace-456",
    )

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "actor_id")
}

func TestNewExecutionContext_EmptyWorkspace(t *testing.T) {
    // Test rejection at boundary
    _, err := shared.NewExecutionContext(
        shared.NewHumanActorID("user-123"),
        "",  // Invalid!
    )

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "workspace_id")
}
```

### Business Logic Tests Don't Need Validation

```go
func TestMessageService_PostMessage(t *testing.T) {
    // Create valid ExecutionContext (once)
    execCtx := shared.MustNewExecutionContext(
        shared.NewHumanActorID("user-123"),
        "default",
    )
    ctx := shared.WithExecutionContext(context.Background(), execCtx)

    // Test business logic - no validation checks needed!
    msg, err := messageService.PostMessage(ctx, req)

    assert.NoError(t, err)
    assert.NotNil(t, msg)
}
```

## References

### Books

- **Domain-Driven Design** (Eric Evans) - Chapter on "Always Valid Aggregates"
- **Implementing Domain-Driven Design** (Vaughn Vernon) - Value Object patterns

### Articles

- [Making Illegal States Unrepresentable](https://fsharpforfunandprofit.com/posts/designing-with-types-making-illegal-states-unrepresentable/) - F# for Fun and Profit
- [Parse, Don't Validate](https://lexi-lambda.github.io/blog/2019/11/05/parse-don-t-validate/) - Alexis King

### Similar Patterns in Other Languages

**F#**:
```fsharp
type EmailAddress = private EmailAddress of string

module EmailAddress =
    let create (s: string) =
        if s.Contains("@") then
            Some (EmailAddress s)
        else
            None

// Can't construct invalid EmailAddress!
```

**Rust**:
```rust
pub struct PositiveInteger(i32);

impl PositiveInteger {
    pub fn new(value: i32) -> Result<Self, String> {
        if value > 0 {
            Ok(PositiveInteger(value))
        } else {
            Err("Value must be positive".to_string())
        }
    }
}

// Can't construct invalid PositiveInteger!
```

**TypeScript**:
```typescript
class Email {
    private constructor(private value: string) {}

    static create(value: string): Email | Error {
        if (!value.includes('@')) {
            return new Error('Invalid email');
        }
        return new Email(value);
    }
}

// Can't construct invalid Email!
```

## Conclusion

The **"Always Valid"** pattern is a cornerstone of robust domain modeling:

1.  Validate at construction
2.  Make constructors the only way to create instances
3.  Eliminate defensive validation throughout codebase
4.  Let the type system enforce correctness

**Result**: Simpler, safer, more maintainable code that's impossible to use incorrectly.
