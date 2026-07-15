package dashboard

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the dashboard endpoints onto mux.
// All routes require authentication + tenant scope.
// The report.view permission gates access — same as the reports module.
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
	canView       := middleware.Authorize(rbac.PermReportView, log)

	base := func(h http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, canView)(h)
	}

	mux.Handle("GET /api/v1/dashboard/summary",  base(http.HandlerFunc(h.Summary)))
	mux.Handle("GET /api/v1/dashboard/charts",   base(http.HandlerFunc(h.Charts)))
	mux.Handle("GET /api/v1/dashboard/activity", base(http.HandlerFunc(h.Activity)))
}
