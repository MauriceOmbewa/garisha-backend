package middleware

import (
	"log/slog"
	"net/http"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// Authorize returns a middleware that requires the authenticated user to hold
// the given permission.  It must be chained after Authenticate so that claims
// are already present in the context.
//
// Usage in a route file:
//
//	auth   := middleware.Authenticate(jwtManager, log)
//	create := middleware.Authorize(rbac.PermVehicleCreate, log)
//	mux.Handle("POST /api/v1/vehicles", chain(auth, create)(handler))
func Authorize(perm rbac.Permission, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				// Authenticate middleware was not applied — programming error.
				log.Error("authorize called without authenticate middleware",
					"path",       r.URL.Path,
					"permission", perm,
				)
				response.Error(w, http.StatusUnauthorized, "authentication required", nil, log)
				return
			}

			role := rbac.Role(claims.Role)

			if !rbac.Has(role, perm) {
				log.Debug("permission denied",
					"user_id",    claims.UserID,
					"role",       claims.Role,
					"permission", perm,
					"request_id", GetRequestID(r.Context()),
				)
				response.Error(w, http.StatusForbidden, "you do not have permission to perform this action", nil, log)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns a middleware that requires the authenticated user to
// hold one of the specified roles.  Use Authorize for permission-based checks
// wherever possible; reserve RequireRole for administrative boundaries (e.g.
// super-admin-only routes).
func RequireRole(log *slog.Logger, roles ...rbac.Role) func(http.Handler) http.Handler {
	allowed := make(map[rbac.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				response.Error(w, http.StatusUnauthorized, "authentication required", nil, log)
				return
			}

			if _, ok := allowed[rbac.Role(claims.Role)]; !ok {
				log.Debug("role denied",
					"user_id",    claims.UserID,
					"role",       claims.Role,
					"request_id", GetRequestID(r.Context()),
				)
				response.Error(w, http.StatusForbidden, "you do not have permission to perform this action", nil, log)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Chain composes middleware in left-to-right execution order.
// chain(a, b, c)(handler) executes: a → b → c → handler
//
// This is a convenience helper for route registration where multiple
// middleware need to be applied to a single handler.
func Chain(middleware ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(middleware) - 1; i >= 0; i-- {
			final = middleware[i](final)
		}
		return final
	}
}
