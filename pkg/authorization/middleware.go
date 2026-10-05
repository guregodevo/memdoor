package authorization

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"memdoor/pkg/auth"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
)

// Logger is the duck-typed interface for structured logging.
// Satisfied by both gateway/logs.EventLogger and *slog.Logger.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

var logger Logger = slog.Default().With("component", "HTTP")

// SetLogger injects the package-level logger (called by infrastructure layer)
func SetLogger(l Logger) {
	logger = l
}

// Re-export context keys from shared package for backward compatibility
const (
	ActorIDKey     = sharedctx.ActorIDKey
	WorkspaceIDKey = sharedctx.WorkspaceIDKey
)

// ContextKey is re-exported from shared package
type ContextKey = sharedctx.ContextKey

// AuthMiddleware extracts the authenticated user from the Authorization header
// and adds it to the request context
type AuthMiddleware struct {
	authService  AuthService
	slugResolver WorkspaceSlugResolver
}

// AuthService provides authentication verification
type AuthService interface {
	VerifyToken(token string) (*auth.User, error)
}

// WorkspaceSlugResolver resolves a workspace UUID to its slug.
type WorkspaceSlugResolver interface {
	ResolveSlug(workspaceID string) string
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(authService AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}
}

// SetSlugResolver injects a workspace slug resolver.
func (m *AuthMiddleware) SetSlugResolver(r WorkspaceSlugResolver) {
	m.slugResolver = r
}

// Handler wraps an HTTP handler with authentication
func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Internal service calls (tools running inside the gateway process)
		// Trust X-Internal-Service header only from loopback addresses
		if r.Header.Get("X-Internal-Service") != "" && isLoopback(r.RemoteAddr) {
			execCtx := &shared.ExecutionContext{
				ActorID:     "system:internal",
				WorkspaceID: "default",
				UserEmail:   "internal@system",
			}
			ctx := shared.WithExecutionContext(r.Context(), execCtx)
			ctx = context.WithValue(ctx, ActorIDKey, execCtx.ActorID)
			ctx = context.WithValue(ctx, WorkspaceIDKey, execCtx.WorkspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No auth header - allow unauthenticated requests to pass through
			// Individual endpoints can require authentication
			next.ServeHTTP(w, r)
			return
		}

		// Parse Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error": "Invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}
		token := parts[1]

		// Verify token
		user, err := m.authService.VerifyToken(token)
		if err != nil {
			// Present-but-unverifiable token — expired, or issued by a
			// DIFFERENT gateway (e.g. someone logged into their own memdoor
			// and pointed at a remote one). Treat it as
			// unauthenticated and fall through exactly like the no-header
			// case above: public reads then succeed anonymously, while
			// protected endpoints still reject via their own requireAuth/
			// requireAdmin checks (no actor is set on the context). This is
			// no weaker than the no-token path — an attacker could just omit
			// the token to reach the same anonymous state.
			next.ServeHTTP(w, r)
			return
		}

		// Create ExecutionContext (domain model for request-scoped auth context)
		if user.WorkspaceID == "" {
			// This should never happen after migration - indicates a data integrity issue
			logger.Error("User has empty workspace_id",
				slog.String("user_id", user.ID),
				slog.String("email", user.Email))
			http.Error(w, `{"error": "User workspace not configured. Please contact support."}`, http.StatusInternalServerError)
			return
		}

		execCtx, err := shared.NewExecutionContextWithEmail(
			shared.ActorID(user.ID),
			user.WorkspaceID,
			user.Email,
		)
		if err == nil {
			execCtx.UserRole = string(user.Role)
			// The workspace slug, resolved from the id.
			if m.slugResolver != nil {
				execCtx.WorkspaceSlug = strings.TrimSpace(m.slugResolver.ResolveSlug(user.WorkspaceID))
			}
		}
		if err != nil {
			// This should never happen if token verification succeeded
			logger.Error("Failed to create ExecutionContext",
				slog.String("error", err.Error()),
				slog.String("user_id", user.ID),
				slog.String("workspace_id", user.WorkspaceID))
			http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
			return
		}

		// Add ExecutionContext to context
		ctx := shared.WithExecutionContext(r.Context(), execCtx)

		// Also add individual values for backward compatibility (deprecated)
		ctx = context.WithValue(ctx, ActorIDKey, execCtx.ActorID)
		ctx = context.WithValue(ctx, WorkspaceIDKey, execCtx.WorkspaceID)

		// Debug, not Info: this fires on EVERY authenticated request, so at Info
		// it drowns the log in plumbing — 74 lines in a 20-minute window that
		// held ten actual agent turns.
		logger.Debug("Set execution context",
			slog.String("actor_id", execCtx.ActorID.String()),
			slog.String("workspace_id", execCtx.WorkspaceID),
			slog.String("user_email", execCtx.UserEmail))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetActorID retrieves the authenticated actor ID from the context
func GetActorID(ctx context.Context) shared.ActorID {
	actorID, ok := ctx.Value(ActorIDKey).(shared.ActorID)
	if !ok {
		return ""
	}
	return actorID
}

// WithActorID adds an actor ID to the context (for testing)
func WithActorID(ctx context.Context, actorID shared.ActorID) context.Context {
	return context.WithValue(ctx, ActorIDKey, actorID)
}

// isLoopback checks if a remote address is a loopback/localhost address
func isLoopback(remoteAddr string) bool {
	host := remoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	// Strip brackets from IPv6
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}
