package branches

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the branch endpoints onto mux.
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
	canView       := middleware.Authorize(rbac.PermSettingsView, log)
	canWrite      := middleware.Authorize(rbac.PermSettingsUpdate, log)

	view  := func(h http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, canView)(h)
	}
	write := func(h http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, canWrite)(h)
	}

	mux.Handle("GET    /api/v1/branches",      view(http.HandlerFunc(h.List)))
	mux.Handle("GET    /api/v1/branches/{id}", view(http.HandlerFunc(h.Get)))
	mux.Handle("POST   /api/v1/branches",      write(http.HandlerFunc(h.Create)))
	mux.Handle("PATCH  /api/v1/branches/{id}", write(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /api/v1/branches/{id}", write(http.HandlerFunc(h.Delete)))
}
