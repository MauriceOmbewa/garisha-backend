package files

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the file management endpoints onto mux.
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

	// Re-use vehicle permissions as a proxy for file access:
	// anyone who can view/create/delete vehicles can also manage their files.
	// Adjust to a dedicated files permission if the RBAC model is extended.
	canView   := middleware.Authorize(rbac.PermVehicleView, log)
	canCreate := middleware.Authorize(rbac.PermVehicleCreate, log)
	canDelete := middleware.Authorize(rbac.PermVehicleDelete, log)

	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	// ── File catalogue ────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/files",             base(canView)(http.HandlerFunc(h.List)))
	mux.Handle("GET    /api/v1/files/{id}",        base(canView)(http.HandlerFunc(h.Get)))
	mux.Handle("GET    /api/v1/files/{id}/url",    base(canView)(http.HandlerFunc(h.GetDownloadURL)))
	mux.Handle("DELETE /api/v1/files/{id}",        base(canDelete)(http.HandlerFunc(h.Delete)))

	// ── Upload flow ───────────────────────────────────────────────────────────
	mux.Handle("POST   /api/v1/files/presign",     base(canCreate)(http.HandlerFunc(h.Presign)))
	mux.Handle("POST   /api/v1/files/confirm",     base(canCreate)(http.HandlerFunc(h.Confirm)))
}
