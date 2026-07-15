package reports

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the report endpoints onto mux.
// All routes require authentication, tenant scope, and report.view permission.
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

	mux.Handle("GET /api/v1/reports/hire",     base(http.HandlerFunc(h.HireReport)))
	mux.Handle("GET /api/v1/reports/sales",    base(http.HandlerFunc(h.SalesReport)))
	mux.Handle("GET /api/v1/reports/service",  base(http.HandlerFunc(h.ServiceReport)))
	mux.Handle("GET /api/v1/reports/finance",  base(http.HandlerFunc(h.FinanceReport)))
	mux.Handle("GET /api/v1/reports/payments", base(http.HandlerFunc(h.PaymentsReport)))
}
