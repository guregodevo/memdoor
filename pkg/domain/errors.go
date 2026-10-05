package domain

import (
	"fmt"
)

// DomainError is the base interface for all domain-specific errors
// Following DDD principles, errors are part of the ubiquitous language
type DomainError interface {
	error
	Code() string    // Machine-readable error code
	Message() string // Human-readable message
	Unwrap() error   // Support error wrapping
}

// =============================================================================
// NotFoundError - Entity Not Found
// =============================================================================

// NotFoundError indicates an entity was not found
// Pattern: Type-safe error with context
type NotFoundError struct {
	EntityType string // "Buddy", "Channel", "Message", etc.
	EntityID   string // ID or name of the entity
	cause      error  // Optional underlying error
}

func (e *NotFoundError) Error() string {
	if e.EntityID != "" {
		return fmt.Sprintf("%s not found: %s", e.EntityType, e.EntityID)
	}
	return fmt.Sprintf("%s not found", e.EntityType)
}

func (e *NotFoundError) Code() string {
	return "NOT_FOUND"
}

func (e *NotFoundError) Message() string {
	return e.Error()
}

func (e *NotFoundError) Unwrap() error {
	return e.cause
}

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(entityType, entityID string) *NotFoundError {
	return &NotFoundError{
		EntityType: entityType,
		EntityID:   entityID,
	}
}

// =============================================================================
// ValidationError - Business Rule Violation
// =============================================================================

// ValidationError indicates a validation failure
// Used for business rule violations and input validation
type ValidationError struct {
	Field   string            // Field that failed validation (optional)
	Message string            // Human-readable error message
	Details map[string]string // Additional validation details
	cause   error
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

func (e *ValidationError) Code() string {
	return "VALIDATION_ERROR"
}

func (e *ValidationError) Unwrap() error {
	return e.cause
}

// NewValidationError creates a new ValidationError
func NewValidationError(message string) *ValidationError {
	return &ValidationError{
		Message: message,
		Details: make(map[string]string),
	}
}

// NewFieldValidationError creates a validation error for a specific field
func NewFieldValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Details: make(map[string]string),
	}
}

// WithDetail adds additional context to the validation error
func (e *ValidationError) WithDetail(key, value string) *ValidationError {
	e.Details[key] = value
	return e
}

// =============================================================================
// AuthorizationError - Permission Denied
// =============================================================================

// AuthorizationError indicates insufficient permissions
type AuthorizationError struct {
	Action   string // Action that was denied (e.g., "delete buddy", "send message")
	Resource string // Resource being accessed
	UserID   string // User who attempted the action
	Message  string // Human-readable message
}

// =============================================================================
// ConflictError - Duplicate or Constraint Violation
// =============================================================================

// ConflictError indicates a conflict with existing data
// Used for duplicate keys, unique constraint violations, optimistic locking failures
type ConflictError struct {
	EntityType string // Type of entity with conflict
	Field      string // Field that caused the conflict (e.g., "name", "email")
	Value      string // Conflicting value
	Message    string // Human-readable message
}

// =============================================================================
// InternalError - System/Infrastructure Errors
// =============================================================================

// InternalError indicates an unexpected system error
// Used for database failures, external service failures, etc.
type InternalError struct {
	Operation string // Operation that failed (e.g., "database query", "file read")
	Message   string // Human-readable message
}

// =============================================================================
// Helper Functions
// =============================================================================
