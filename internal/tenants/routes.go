package tenants

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the tenants admin endpoints onto mux.
// All tenant management routes are super-admin only — they are protected
// by RequireRole(RoleSuperAdmin) in addition to Authenticate.
func RegisterRoutes(mux *http.ServeMux, h *Handler, jwtManager *platformauth.Manager, log *slog.Logger) {
	authenticate  := middleware.Authenticate(jwtManager, log)
	superAdminOnly := middleware.RequireRole(log, rbac.RoleSuperAdmin)

	guard := middleware.Chain(authenticate, superAdminOnly)

	mux.Handle("GET    /api/v1/admin/tenants",     guard(http.HandlerFunc(h.List)))
	mux.Handle("POST   /api/v1/admin/tenants",     guard(http.HandlerFunc(h.Create)))
	mux.Handle("GET    /api/v1/admin/tenants/{id}", guard(http.HandlerFunc(h.Get)))
	mux.Handle("PATCH  /api/v1/admin/tenants/{id}", guard(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /api/v1/admin/tenants/{id}", guard(http.HandlerFunc(h.Delete)))
}
