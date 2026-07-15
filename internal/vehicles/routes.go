package vehicles

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the vehicle management endpoints onto mux.
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

	canView   := middleware.Authorize(rbac.PermVehicleView, log)
	canCreate := middleware.Authorize(rbac.PermVehicleCreate, log)
	canUpdate := middleware.Authorize(rbac.PermVehicleUpdate, log)
	canDelete := middleware.Authorize(rbac.PermVehicleDelete, log)

	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	mux.Handle("GET    /api/v1/vehicles",      base(canView)(http.HandlerFunc(h.List)))
	mux.Handle("POST   /api/v1/vehicles",      base(canCreate)(http.HandlerFunc(h.Create)))
	mux.Handle("GET    /api/v1/vehicles/{id}", base(canView)(http.HandlerFunc(h.Get)))
	mux.Handle("PATCH  /api/v1/vehicles/{id}", base(canUpdate)(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /api/v1/vehicles/{id}", base(canDelete)(http.HandlerFunc(h.Delete)))
}
