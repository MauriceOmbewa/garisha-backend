package inventory

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the inventory endpoints onto mux.
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

	canView   := middleware.Authorize(rbac.PermInventoryView, log)
	canCreate := middleware.Authorize(rbac.PermInventoryCreate, log)
	canUpdate := middleware.Authorize(rbac.PermInventoryUpdate, log)
	canDelete := middleware.Authorize(rbac.PermInventoryDelete, log)

	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	// ── Item catalogue ────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/inventory/items",              base(canView)(http.HandlerFunc(h.ListItems)))
	mux.Handle("POST   /api/v1/inventory/items",              base(canCreate)(http.HandlerFunc(h.CreateItem)))
	mux.Handle("GET    /api/v1/inventory/items/{id}",         base(canView)(http.HandlerFunc(h.GetItem)))
	mux.Handle("PATCH  /api/v1/inventory/items/{id}",         base(canUpdate)(http.HandlerFunc(h.UpdateItem)))
	mux.Handle("DELETE /api/v1/inventory/items/{id}",         base(canDelete)(http.HandlerFunc(h.DeleteItem)))

	// ── Reorder alerts ────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/inventory/reorder-alerts",     base(canView)(http.HandlerFunc(h.ReorderAlerts)))

	// ── Stock movements ───────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/inventory/usage",              base(canView)(http.HandlerFunc(h.ListUsage)))
	mux.Handle("POST   /api/v1/inventory/usage",              base(canCreate)(http.HandlerFunc(h.RecordUsage)))
	mux.Handle("POST   /api/v1/inventory/items/{id}/adjust",  base(canUpdate)(http.HandlerFunc(h.AdjustStock)))
}
