package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// TenantResolver is the interface the middleware uses to load a tenant.
// The tenants domain repository satisfies this interface, but it can also
// be swapped for a mock in tests.
type TenantResolver interface {
	FindByID(ctx context.Context, id string) (*tenant.Record, error)
	FindBySlug(ctx context.Context, slug string) (*tenant.Record, error)
}

// ResolveTenant reads the tenant identifier from the request headers and
// loads the matching tenant from the database, storing it in the context.
//
// Header priority:
//  1. X-Tenant-ID   (UUID)   — preferred for server-to-server calls
//  2. X-Tenant-Slug (string) — preferred for browser/frontend calls
//
// Returns 400 if neither header is present, 404 if the tenant does not exist,
// and 403 if the tenant is suspended.
//
// Routes that do not belong to a tenant (e.g. super-admin, public health)
// should not be wrapped by this middleware.
func ResolveTenant(resolver TenantResolver, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			record, err := resolveTenantFromHeaders(r.Context(), r, resolver)
			if err != nil {
				switch {
				case errors.Is(err, tenant.ErrNotFound):
					response.Error(w, http.StatusNotFound, "tenant not found", nil, log)
				case errors.Is(err, tenant.ErrInactive):
					response.Error(w, http.StatusForbidden, "this account has been suspended", nil, log)
				default:
					log.Error("failed to resolve tenant", "error", err, "request_id", GetRequestID(r.Context()))
					response.Error(w, http.StatusInternalServerError, "an unexpected error occurred", nil, log)
				}
				return
			}

			if record == nil {
				response.Error(w, http.StatusBadRequest,
					"tenant identifier required: provide X-Tenant-ID or X-Tenant-Slug header", nil, log)
				return
			}

			t := &tenant.Tenant{
				ID:       record.ID,
				Slug:     record.Slug,
				Name:     record.Name,
				Plan:     record.Plan,
				IsActive: record.IsActive,
			}

			ctx := tenant.SetTenant(r.Context(), t)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// EnforceTenantScope cross-checks the tenant in context against the tenant
// ID embedded in the authenticated user's JWT claims.  This prevents a
// valid token issued for tenant A from being used against tenant B's routes.
//
// Must be chained after both ResolveTenant and Authenticate.
func EnforceTenantScope(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := tenant.GetTenant(r.Context())
			if t == nil {
				// Programming error — ResolveTenant was not applied.
				log.Error("enforce tenant scope called without resolved tenant",
					"path", r.URL.Path)
				response.Error(w, http.StatusInternalServerError, "an unexpected error occurred", nil, log)
				return
			}

			claims := GetClaims(r.Context())
			if claims == nil {
				// Programming error — Authenticate was not applied.
				log.Error("enforce tenant scope called without authentication",
					"path", r.URL.Path)
				response.Error(w, http.StatusUnauthorized, "authentication required", nil, log)
				return
			}

			// Super-admins have an empty TenantID in their JWT and may access
			// any tenant's data.
			if claims.TenantID == "" {
				next.ServeHTTP(w, r)
				return
			}

			if claims.TenantID != t.ID {
				log.Warn("tenant scope violation",
					"claims_tenant", claims.TenantID,
					"request_tenant", t.ID,
					"user_id",       claims.UserID,
					"request_id",    GetRequestID(r.Context()),
				)
				response.Error(w, http.StatusForbidden, "access to this tenant is not allowed", nil, log)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// resolveTenantFromHeaders reads headers and delegates to the resolver.
// Returns (nil, nil) when no identifier header is present so the caller
// can return a descriptive 400.
func resolveTenantFromHeaders(
	ctx context.Context,
	r *http.Request,
	resolver TenantResolver,
) (*tenant.Record, error) {
	if id := r.Header.Get("X-Tenant-ID"); id != "" {
		record, err := resolver.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if record == nil {
			return nil, tenant.ErrNotFound
		}
		if !record.IsActive {
			return nil, tenant.ErrInactive
		}
		return record, nil
	}

	if slug := r.Header.Get("X-Tenant-Slug"); slug != "" {
		record, err := resolver.FindBySlug(ctx, slug)
		if err != nil {
			return nil, err
		}
		if record == nil {
			return nil, tenant.ErrNotFound
		}
		if !record.IsActive {
			return nil, tenant.ErrInactive
		}
		return record, nil
	}

	return nil, nil
}
