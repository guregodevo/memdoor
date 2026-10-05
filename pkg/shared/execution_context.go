package shared

import (
	"context"
	"fmt"

	sharedctx "memdoor/pkg/shared/context"
)

// ExecutionContext represents the runtime execution context for a request
// This is a domain model that captures WHO is performing an action and WHERE (workspace)
//
// Design: Value Object (immutable, passed by value)
// Pattern: Context Object (carries request-scoped data through layers)
//
// Used by:
// - Infrastructure layer: Auth middleware creates it from JWT token
// - Application layer: Services use it for authorization and data isolation
// - Domain layer: Aggregates validate against it (e.g., "user is member of channel")
//
// DDD Context: This is part of the Shared Kernel - used across all bounded contexts
type ExecutionContext struct {
	// ActorID identifies who is performing the action (human or agent)
	ActorID ActorID

	// WorkspaceID identifies the tenant/workspace boundary for multi-tenancy
	WorkspaceID string

	// WorkspaceSlug is the workspace slug (e.g. "memdoor"), resolved
	// per-request from the workspaces table or set explicitly.
	WorkspaceSlug string

	// UserEmail is optional metadata for logging/debugging
	UserEmail string

	// MentionDepth tracks agent-to-agent mention chain depth to prevent infinite loops
	// 0 = user-initiated request
	// 1 = agent mentioned by user
	// 2 = agent mentioned by another agent (max allowed)
	MentionDepth int

	// UserRole is the workspace-level role of the initiating user (admin/user)
	// Used for RBAC enforcement: tool restrictions, agent visibility, channel permissions
	UserRole string
}

// NewExecutionContext creates a new execution context with validated invariants
// Returns error if actorID or workspaceID are invalid
//
// Design: "Always Valid" pattern - validation at construction means instances are
// guaranteed to be valid. No need to call Validate() anywhere else!
func NewExecutionContext(actorID ActorID, workspaceID string) (*ExecutionContext, error) {
	// Validate actorID
	if err := actorID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid actor_id: %w", err)
	}

	// Validate workspaceID
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id cannot be empty")
	}

	return &ExecutionContext{
		ActorID:     actorID,
		WorkspaceID: workspaceID,
	}, nil
}

// NewExecutionContextWithEmail creates a new execution context with email metadata
// Returns error if validation fails
func NewExecutionContextWithEmail(actorID ActorID, workspaceID, email string) (*ExecutionContext, error) {
	// Validate actorID
	if err := actorID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid actor_id: %w", err)
	}

	// Validate workspaceID
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id cannot be empty")
	}

	return &ExecutionContext{
		ActorID:     actorID,
		WorkspaceID: workspaceID,
		UserEmail:   email,
	}, nil
}

// MustNewExecutionContext creates an ExecutionContext or panics if invalid
// Use only when you're certain the inputs are valid (e.g., constants)
func MustNewExecutionContext(actorID ActorID, workspaceID string) *ExecutionContext {
	execCtx, err := NewExecutionContext(actorID, workspaceID)
	if err != nil {
		panic(fmt.Sprintf("failed to create ExecutionContext: %v", err))
	}
	return execCtx
}

// Validate checks if the execution context is valid
//
// Deprecated: This method is unnecessary because ExecutionContext uses the
// "Always Valid" pattern - validation happens at construction. All instances
// created via NewExecutionContext() are guaranteed to be valid.
//
// This method is kept only for backward compatibility and will always return nil
// for instances created via the factory constructors.
func (ec *ExecutionContext) Validate() error {
	// Since we enforce invariants at construction, this should never fail
	// for instances created properly. But we keep the checks for safety.
	if err := ec.ActorID.Validate(); err != nil {
		return fmt.Errorf("invalid actor_id: %w", err)
	}

	if ec.WorkspaceID == "" {
		return fmt.Errorf("workspace_id cannot be empty")
	}

	return nil
}

// IsHuman returns true if the actor is a human
func (ec *ExecutionContext) IsHuman() bool {
	return ec.ActorID.IsHuman()
}

// IsAgent returns true if the actor is an agent
func (ec *ExecutionContext) IsAgent() bool {
	return ec.ActorID.IsAgent()
}

// WithIncrementedDepth creates a new ExecutionContext with MentionDepth incremented by 1
// Used when an agent mentions another agent (agent-to-agent communication)
func (ec *ExecutionContext) WithIncrementedDepth() *ExecutionContext {
	return &ExecutionContext{
		ActorID:      ec.ActorID,
		WorkspaceID:  ec.WorkspaceID,
		UserEmail:    ec.UserEmail,
		MentionDepth: ec.MentionDepth + 1,
	}
}

// CanMentionAgents returns true if this context can trigger agent mentions
// Enforces max depth limit to prevent infinite agent-to-agent chains
func (ec *ExecutionContext) CanMentionAgents() bool {
	const maxMentionDepth = 2 // 0=user, 1=agent by user, 2=agent by agent (stop)
	return ec.MentionDepth < maxMentionDepth
}

// Context Keys for storing ExecutionContext in context.Context
const (
	ExecutionContextKey sharedctx.ContextKey = "execution_context"
)

// WithExecutionContext adds an ExecutionContext to the context
func WithExecutionContext(ctx context.Context, execCtx *ExecutionContext) context.Context {
	return context.WithValue(ctx, ExecutionContextKey, execCtx)
}

// GetExecutionContext retrieves the ExecutionContext from the context
// Returns nil if not found
func GetExecutionContext(ctx context.Context) *ExecutionContext {
	execCtx, ok := ctx.Value(ExecutionContextKey).(*ExecutionContext)
	if !ok {
		return nil
	}
	return execCtx
}

// ToContextValues extracts individual values for backward compatibility
// Deprecated: Use GetExecutionContext() instead
func (ec *ExecutionContext) ToContextValues(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, sharedctx.ActorIDKey, ec.ActorID)
	ctx = context.WithValue(ctx, sharedctx.WorkspaceIDKey, ec.WorkspaceID)
	return ctx
}

// FromContextValues creates ExecutionContext from legacy context values
// Deprecated: For migration from old code that uses individual context keys
// Returns nil if no valid context values found
func FromContextValues(ctx context.Context) *ExecutionContext {
	actorID, _ := ctx.Value(sharedctx.ActorIDKey).(ActorID)
	workspaceID, _ := ctx.Value(sharedctx.WorkspaceIDKey).(string)

	if actorID == "" && workspaceID == "" {
		return nil // No context values found
	}

	// Try to create validated ExecutionContext
	execCtx, err := NewExecutionContext(actorID, workspaceID)
	if err != nil {
		// Legacy values were invalid - return nil
		return nil
	}

	return execCtx
}
