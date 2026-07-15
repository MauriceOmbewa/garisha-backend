package notifications

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the notification endpoints onto mux.
// All routes are tenant-scoped, require authentication, and are automatically
// scoped to the authenticated user — users can only see their own notifications.
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

	// All notification endpoints require notification.view at minimum.
	canView := middleware.Authorize(rbac.PermNotificationView, log)

	base := func(h http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, canView)(h)
	}

	// ── Read / query ──────────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/notifications",               base(http.HandlerFunc(h.List)))
	mux.Handle("GET    /api/v1/notifications/unread-count",  base(http.HandlerFunc(h.UnreadCount)))

	// ── Mark read ─────────────────────────────────────────────────────────────
	mux.Handle("PATCH  /api/v1/notifications/read-all",      base(http.HandlerFunc(h.MarkAllRead)))
	mux.Handle("PATCH  /api/v1/notifications/{id}/read",     base(http.HandlerFunc(h.MarkRead)))

	// ── Delete ────────────────────────────────────────────────────────────────
	mux.Handle("DELETE /api/v1/notifications/read",          base(http.HandlerFunc(h.DeleteRead)))
	mux.Handle("DELETE /api/v1/notifications/{id}",          base(http.HandlerFunc(h.Delete)))
}
