package finance

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the finance endpoints onto mux.
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

	canView   := middleware.Authorize(rbac.PermFinanceView, log)
	canCreate := middleware.Authorize(rbac.PermFinanceCreate, log)
	canUpdate := middleware.Authorize(rbac.PermFinanceUpdate, log)

	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	// ── Ledger summary ────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/finance/summary",              base(canView)(http.HandlerFunc(h.GetSummary)))

	// ── Categories ────────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/finance/categories",           base(canView)(http.HandlerFunc(h.ListCategories)))
	mux.Handle("POST   /api/v1/finance/categories",           base(canCreate)(http.HandlerFunc(h.CreateCategory)))
	mux.Handle("GET    /api/v1/finance/categories/{id}",      base(canView)(http.HandlerFunc(h.GetCategory)))
	mux.Handle("PATCH  /api/v1/finance/categories/{id}",      base(canUpdate)(http.HandlerFunc(h.UpdateCategory)))
	mux.Handle("DELETE /api/v1/finance/categories/{id}",      base(canUpdate)(http.HandlerFunc(h.DeleteCategory)))

	// ── Records ───────────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/finance/records",              base(canView)(http.HandlerFunc(h.ListRecords)))
	mux.Handle("POST   /api/v1/finance/records",              base(canCreate)(http.HandlerFunc(h.CreateRecord)))
	mux.Handle("GET    /api/v1/finance/records/{id}",         base(canView)(http.HandlerFunc(h.GetRecord)))
	mux.Handle("PATCH  /api/v1/finance/records/{id}",         base(canUpdate)(http.HandlerFunc(h.UpdateRecord)))
	mux.Handle("DELETE /api/v1/finance/records/{id}",         base(canUpdate)(http.HandlerFunc(h.DeleteRecord)))
}
