package sandbox

import "errors"

var (
	// ErrInvalidScope is returned when an invalid scope is provided
	ErrInvalidScope = errors.New("invalid sandbox scope")

	// ErrAccessDenied is returned when access to a path is denied
	ErrAccessDenied = errors.New("access denied: insufficient scope")

	// ErrInsufficientScope is returned when an agent's scope is insufficient for a tool
	ErrInsufficientScope = errors.New("insufficient scope for this operation")
)
