package domain

// Predicate represents a business rule that can be evaluated to true or false
// This is the Specification Pattern from Domain-Driven Design
// It allows composable, reusable business logic
//
// Example:
//
//	isActive := func(b *Buddy) bool { return b.IsActive }
//	isRemote := func(b *Buddy) bool { return b.ExecutionType == "remote" }
//	isActiveRemote := And(isActive, isRemote)
//	if isActiveRemote(buddy) { ... }
type Predicate[T any] func(T) bool

// =============================================================================
// Predicate-based Validation
// =============================================================================

// Specification is a named predicate with an error message
// Used for validation with clear error reporting
type Specification[T any] struct {
	Name      string       // Name of the specification (e.g., "NameRequired")
	Predicate Predicate[T] // The predicate to evaluate
	Error     error        // Error to return if predicate fails
}

// IsSatisfiedBy checks if the entity satisfies the specification
func (s Specification[T]) IsSatisfiedBy(t T) bool {
	return s.Predicate(t)
}

// Validate checks if the entity satisfies the specification and returns error if not
func (s Specification[T]) Validate(t T) error {
	if !s.IsSatisfiedBy(t) {
		return s.Error
	}
	return nil
}

// ValidateAll validates multiple specifications and returns the first error
func ValidateAll[T any](t T, specs ...Specification[T]) error {
	for _, spec := range specs {
		if err := spec.Validate(t); err != nil {
			return err
		}
	}
	return nil
}
