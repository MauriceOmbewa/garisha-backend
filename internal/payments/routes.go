package payments

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/rbac"
)

// RegisterRoutes mounts the payment endpoints onto mux.
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

	canView   := middleware.Authorize(rbac.PermPaymentView, log)
	canCreate := middleware.Authorize(rbac.PermPaymentCreate, log)

	base := func(perm func(http.Handler) http.Handler) func(http.Handler) http.Handler {
		return middleware.Chain(resolveTenant, authenticate, enforceScope, perm)
	}

	// ── Payment queries ───────────────────────────────────────────────────────
	mux.Handle("GET    /api/v1/payments",               base(canView)(http.HandlerFunc(h.List)))
	mux.Handle("GET    /api/v1/payments/{id}",          base(canView)(http.HandlerFunc(h.Get)))

	// ── Manual payment recording ──────────────────────────────────────────────
	mux.Handle("POST   /api/v1/payments/manual",        base(canCreate)(http.HandlerFunc(h.CreateManual)))

	// ── M-PESA STK Push ───────────────────────────────────────────────────────
	mux.Handle("POST   /api/v1/payments/mpesa",         base(canCreate)(http.HandlerFunc(h.InitiateMpesa)))

	// ── M-PESA callback (public — no auth, Safaricom POSTs here) ─────────────
	// Registered without the auth/tenant middleware chain.
	mux.Handle("POST   /api/v1/payments/mpesa/callback", http.HandlerFunc(h.MpesaCallback))

	// ── Cancel ────────────────────────────────────────────────────────────────
	mux.Handle("PATCH  /api/v1/payments/{id}/cancel",   base(canCreate)(http.HandlerFunc(h.Cancel)))
}
