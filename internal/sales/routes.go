package sales

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the vehicle-sales endpoints onto mux.
// All routes are tenant-scoped and require authentication + scope enforcement.
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

	canView   := middleware.Authorize(rbac.PermSaleView, log)
	canCreate := middleware.Authorize(rbac.PermSaleCreate, log)
	canUpdate := middleware.Authorize(rbac.PermSaleUpdate, log)
	canDelete := middleware.Authorize(rbac.PermSaleDelete, log)

	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	mux.Handle("GET    /api/v1/sales",              base(canView)(http.HandlerFunc(h.List)))
	mux.Handle("POST   /api/v1/sales",              base(canCreate)(http.HandlerFunc(h.Create)))
	mux.Handle("GET    /api/v1/sales/{id}",         base(canView)(http.HandlerFunc(h.Get)))
	mux.Handle("PATCH  /api/v1/sales/{id}",         base(canUpdate)(http.HandlerFunc(h.Update)))
	mux.Handle("PATCH  /api/v1/sales/{id}/status",  base(canUpdate)(http.HandlerFunc(h.UpdateStatus)))
	mux.Handle("DELETE /api/v1/sales/{id}",         base(canDelete)(http.HandlerFunc(h.Delete)))
}
