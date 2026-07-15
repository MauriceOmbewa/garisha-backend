package audit

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the audit log endpoints onto mux.
// Audit logs are read-only via the API and restricted to users with
// the audit.view permission (admin and super-admin roles only).
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
	canView       := middleware.Authorize(rbac.PermAuditView, log)

	base := func(h http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, canView)(h)
	}

	mux.Handle("GET /api/v1/audit/logs",      base(http.HandlerFunc(h.List)))
	mux.Handle("GET /api/v1/audit/logs/{id}", base(http.HandlerFunc(h.Get)))
}
