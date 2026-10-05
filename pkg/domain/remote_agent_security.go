package domain

import (
	"errors"
)

// Security constants for remote agent validation
const (
	// MaxResponseSizeBytes is the maximum allowed response size from remote endpoints (10MB)
	MaxResponseSizeBytes = 10 * 1024 * 1024

	// MaxTokensPerRequest is the maximum tokens allowed in a single request (safety limit)
	MaxTokensPerRequest = 100000

	// DefaultRequestTimeoutSeconds is the default timeout for remote agent requests
	DefaultRequestTimeoutSeconds = 180

	// MaxRequestTimeoutSeconds is the maximum allowed timeout
	MaxRequestTimeoutSeconds = 600 // 10 minutes

	// DefaultMaxRetries is the default number of retries for failed requests
	DefaultMaxRetries = 3

	// MaxRetries is the maximum allowed retries
	MaxRetries = 5
)

// Common security errors
var (
	ErrInvalidEndpoint    = errors.New("invalid endpoint URL")
	ErrInsecureEndpoint   = errors.New("endpoint must use HTTPS (except localhost)")
	ErrEmptyEndpoint      = errors.New("endpoint cannot be empty")
	ErrEmptyModel         = errors.New("model name cannot be empty")
	ErrTimeoutTooLarge    = errors.New("timeout exceeds maximum allowed value")
	ErrTooManyRetries     = errors.New("max retries exceeds maximum allowed value")
	ErrEndpointBlocked    = errors.New("endpoint is blocked")
	ErrPrivateNetwork     = errors.New("endpoint resolves to a private/internal network address")
	ErrSuspiciousEndpoint = errors.New("endpoint appears suspicious")
)
