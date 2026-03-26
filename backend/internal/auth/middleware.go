package auth

import (
	"context"
	"net/http"
	"strings"

	"salusdomi.com/api/internal/common"
	"salusdomi.com/api/internal/config"
)

type contextKey string

const (
	contextKeyUserID contextKey = "user_id"
	contextKeyRole   contextKey = "role"
)

// RequireAuth returns middleware that validates the Bearer JWT on every request.
// On success it injects the user's ID and role into the request context.
func RequireAuth(cfg config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				common.WriteError(w, r, http.StatusUnauthorized, "MISSING_TOKEN", "authorization header with Bearer token is required")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := validateJWT(tokenStr, cfg.JWTSecret)
			if err != nil {
				common.WriteError(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired access token")
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyUserID, claims.Subject)
			ctx = context.WithValue(ctx, contextKeyRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that further restricts an authenticated route
// to users with one of the allowed roles.
// Must be chained after RequireAuth.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := RoleFromContext(r.Context())
			if _, ok := allowed[role]; !ok {
				common.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "you do not have permission to access this resource")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserIDFromContext extracts the authenticated user's ID from the request context.
// Returns an empty string if the context was not populated by RequireAuth.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKeyUserID).(string)
	return id
}

// RoleFromContext extracts the authenticated user's role from the request context.
func RoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(contextKeyRole).(string)
	return role
}
