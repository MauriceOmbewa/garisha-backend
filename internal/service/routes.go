package service

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the vehicle-service endpoints onto mux.
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

	canView   := middleware.Authorize(rbac.PermServiceView, log)
	canCreate := middleware.Authorize(rbac.PermServiceCreate, log)
	canUpdate := middleware.Authorize(rbac.PermServiceUpdate, log)
	canDelete := middleware.Authorize(rbac.PermServiceDelete, log)

	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	// ── Service Jobs ──────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/service/jobs",                      base(canView)(http.HandlerFunc(h.List)))
	mux.Handle("GET    /api/v1/service/jobs/enriched",             base(canView)(http.HandlerFunc(h.ListEnriched)))
	mux.Handle("POST   /api/v1/service/jobs",                      base(canCreate)(http.HandlerFunc(h.Create)))
	mux.Handle("GET    /api/v1/service/jobs/{id}",                 base(canView)(http.HandlerFunc(h.Get)))
	mux.Handle("PATCH  /api/v1/service/jobs/{id}",                 base(canUpdate)(http.HandlerFunc(h.Update)))
	mux.Handle("PATCH  /api/v1/service/jobs/{id}/status",          base(canUpdate)(http.HandlerFunc(h.UpdateStatus)))
	mux.Handle("PATCH  /api/v1/service/jobs/{id}/mechanic",        base(canUpdate)(http.HandlerFunc(h.AssignMechanic)))
	mux.Handle("DELETE /api/v1/service/jobs/{id}",                 base(canDelete)(http.HandlerFunc(h.Delete)))

	// ── Job Items ─────────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/service/jobs/{id}/items",           base(canView)(http.HandlerFunc(h.ListItems)))
	mux.Handle("POST   /api/v1/service/jobs/{id}/items",           base(canUpdate)(http.HandlerFunc(h.AddItem)))
	mux.Handle("PATCH  /api/v1/service/jobs/{id}/items/{item_id}", base(canUpdate)(http.HandlerFunc(h.UpdateItem)))
	mux.Handle("DELETE /api/v1/service/jobs/{id}/items/{item_id}", base(canUpdate)(http.HandlerFunc(h.DeleteItem)))
}
