package company

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the company profile endpoints onto mux.
// All routes require tenant resolution, authentication, and tenant scope
// enforcement.  Write operations additionally require settings permission.
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	jwtManager *platformauth.Manager,
	tenantResolver middleware.TenantResolver,
	log *slog.Logger,
) {
	resolveTenant  := middleware.ResolveTenant(tenantResolver, log)
	authenticate   := middleware.Authenticate(jwtManager, log)
	enforceScope   := middleware.EnforceTenantScope(log)
	canViewSettings   := middleware.Authorize(rbac.PermSettingsView, log)
	canUpdateSettings := middleware.Authorize(rbac.PermSettingsUpdate, log)

	// GET  — any authenticated tenant member with settings.view
	mux.Handle("GET /api/v1/company/profile",
		middleware.Chain(resolveTenant, authenticate, enforceScope, canViewSettings)(
			http.HandlerFunc(h.GetProfile),
		),
	)

	// PUT  — admin-level: settings.update
	mux.Handle("PUT /api/v1/company/profile",
		middleware.Chain(resolveTenant, authenticate, enforceScope, canUpdateSettings)(
			http.HandlerFunc(h.UpdateProfile),
		),
	)
}
