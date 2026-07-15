package users

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the user management endpoints onto mux.
//
// All routes are tenant-scoped and require authentication + tenant scope
// enforcement.  Individual operations are gated by the appropriate RBAC
// permission so callers with user.view can list/get but cannot modify.
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	jwtManager *platformauth.Manager,
	tenantResolver middleware.TenantResolver,
	log *slog.Logger,
) {
	resolveTenant := middleware.ResolveTenant(tenantResolver, log)
	authenticate  := middleware.Authenticate(jwtManager, log)
	enforceScope  := middleware.EnforceTenantScope(log)

	// Per-action permission middleware.
	canView   := middleware.Authorize(rbac.PermUserView, log)
	canUpdate := middleware.Authorize(rbac.PermUserUpdate, log)
	canDelete := middleware.Authorize(rbac.PermUserDelete, log)

	// Convenience: base chain shared by all user routes.
	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	// ── Collection ────────────────────────────────────────────────────────────
	mux.Handle("GET /api/v1/users",
		base(canView)(http.HandlerFunc(h.List)))

	// ── Single resource ───────────────────────────────────────────────────────
	mux.Handle("GET /api/v1/users/{id}",
		base(canView)(http.HandlerFunc(h.Get)))

	mux.Handle("DELETE /api/v1/users/{id}",
		base(canDelete)(http.HandlerFunc(h.Delete)))

	// ── Sub-resource actions ──────────────────────────────────────────────────
	mux.Handle("PATCH /api/v1/users/{id}/role",
		base(canUpdate)(http.HandlerFunc(h.AssignRole)))

	mux.Handle("POST /api/v1/users/{id}/activate",
		base(canUpdate)(http.HandlerFunc(h.Activate)))

	mux.Handle("POST /api/v1/users/{id}/suspend",
		base(canUpdate)(http.HandlerFunc(h.Suspend)))

	mux.Handle("PUT /api/v1/users/{id}/permissions",
		base(canUpdate)(http.HandlerFunc(h.UpdatePermissions)))
}
