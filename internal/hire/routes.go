package hire

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the hire-booking endpoints onto mux.
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

	canView   := middleware.Authorize(rbac.PermBookingView, log)
	canCreate := middleware.Authorize(rbac.PermBookingCreate, log)
	canUpdate := middleware.Authorize(rbac.PermBookingUpdate, log)
	canDelete := middleware.Authorize(rbac.PermBookingDelete, log)

	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	// Availability check — booking.view is sufficient (read-only probe).
	mux.Handle("POST   /api/v1/hire/availability",            base(canView)(http.HandlerFunc(h.CheckAvailability)))

	// Enriched booking list (includes customer name + vehicle details).
	mux.Handle("GET    /api/v1/hire/bookings/enriched",       base(canView)(http.HandlerFunc(h.ListEnriched)))

	// Booking CRUD.
	mux.Handle("GET    /api/v1/hire/bookings",                base(canView)(http.HandlerFunc(h.List)))
	mux.Handle("POST   /api/v1/hire/bookings",                base(canCreate)(http.HandlerFunc(h.Create)))
	mux.Handle("GET    /api/v1/hire/bookings/{id}",           base(canView)(http.HandlerFunc(h.Get)))
	mux.Handle("PATCH  /api/v1/hire/bookings/{id}",           base(canUpdate)(http.HandlerFunc(h.Update)))
	mux.Handle("PATCH  /api/v1/hire/bookings/{id}/status",    base(canUpdate)(http.HandlerFunc(h.UpdateStatus)))
	mux.Handle("DELETE /api/v1/hire/bookings/{id}",           base(canDelete)(http.HandlerFunc(h.Delete)))
}
