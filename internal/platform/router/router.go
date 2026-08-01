// Package router builds and returns the application's HTTP handler tree.
// Domain route groups are registered by passing the mux to each module's
// RegisterRoutes function, keeping this file thin.
package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MauriceOmbewa/garisha-backend/internal/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/audit"
	"github.com/MauriceOmbewa/garisha-backend/internal/branches"
	"github.com/MauriceOmbewa/garisha-backend/internal/company"
	"github.com/MauriceOmbewa/garisha-backend/internal/customers"
	"github.com/MauriceOmbewa/garisha-backend/internal/dashboard"
	"github.com/MauriceOmbewa/garisha-backend/internal/files"
	"github.com/MauriceOmbewa/garisha-backend/internal/finance"
	"github.com/MauriceOmbewa/garisha-backend/internal/hire"
	"github.com/MauriceOmbewa/garisha-backend/internal/inventory"
	"github.com/MauriceOmbewa/garisha-backend/internal/notifications"
	"github.com/MauriceOmbewa/garisha-backend/internal/payments"
	"github.com/MauriceOmbewa/garisha-backend/internal/reports"
	"github.com/MauriceOmbewa/garisha-backend/internal/sales"
	"github.com/MauriceOmbewa/garisha-backend/internal/service"
	"github.com/MauriceOmbewa/garisha-backend/internal/tenants"
	"github.com/MauriceOmbewa/garisha-backend/internal/users"
	"github.com/MauriceOmbewa/garisha-backend/internal/vehicles"
	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// Dependencies groups every cross-cutting dependency the router needs to
// wire up all domain modules.  New fields are added here as modules are built.
type Dependencies struct {
	Log             *slog.Logger
	DB              *pgxpool.Pool  // used by the health check
	JWTManager      *platformauth.Manager
	TenantResolver  middleware.TenantResolver
	AllowedOrigins  []string       // CORS + Google OAuth allowed frontend origins
	AuthHandler     *auth.Handler
	TenantsHandler  *tenants.Handler
	BranchesHandler *branches.Handler
	CompanyHandler  *company.Handler
	UsersHandler    *users.Handler
	VehiclesHandler  *vehicles.Handler
	CustomersHandler *customers.Handler
	HireHandler      *hire.Handler
	SalesHandler     *sales.Handler
	ServiceHandler   *service.Handler
	FinanceHandler   *finance.Handler
	PaymentsHandler   *payments.Handler
	InventoryHandler      *inventory.Handler
	NotificationsHandler  *notifications.Handler
	AuditHandler          *audit.Handler
	ReportsHandler        *reports.Handler
	DashboardHandler      *dashboard.Handler
	FilesHandler          *files.Handler
}

// New constructs the root http.Handler with all middleware applied and all
// domain routes registered.
func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	// ── Health (no tenant context needed) ────────────────────────────────────
	mux.HandleFunc("GET /api/v1/health", healthHandler(deps.DB, deps.Log))

	// ── Auth routes (public + protected, no tenant scope enforcement) ─────────
	// Login and refresh are public and use tenant resolution internally.
	// /auth/me only needs authentication, not a tenant header.
	auth.RegisterRoutes(mux, deps.AuthHandler, deps.JWTManager, deps.Log)

	// ── Super-admin tenant management ─────────────────────────────────────────
	tenants.RegisterRoutes(mux, deps.TenantsHandler, deps.JWTManager, deps.Log)

	// ── Branch management (tenant-scoped) ─────────────────────────────────────
	branches.RegisterRoutes(mux, deps.BranchesHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Company profile (tenant-scoped) ───────────────────────────────────────
	company.RegisterRoutes(mux, deps.CompanyHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── User management (tenant-scoped) ───────────────────────────────────────
	users.RegisterRoutes(mux, deps.UsersHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Vehicle management (tenant-scoped) ────────────────────────────────────
	vehicles.RegisterRoutes(mux, deps.VehiclesHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Customer management (tenant-scoped) ──────────────────────────────────
	customers.RegisterRoutes(mux, deps.CustomersHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Car hire / bookings (tenant-scoped) ───────────────────────────────────
	hire.RegisterRoutes(mux, deps.HireHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Vehicle sales (tenant-scoped) ─────────────────────────────────────────
	sales.RegisterRoutes(mux, deps.SalesHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Vehicle service jobs (tenant-scoped) ──────────────────────────────────
	service.RegisterRoutes(mux, deps.ServiceHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Finance ledger (tenant-scoped) ────────────────────────────────────────
	finance.RegisterRoutes(mux, deps.FinanceHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Payments (tenant-scoped + public M-PESA callback) ─────────────────────
	payments.RegisterRoutes(mux, deps.PaymentsHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Inventory (tenant-scoped) ─────────────────────────────────────────────
	inventory.RegisterRoutes(mux, deps.InventoryHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Notifications (tenant + user-scoped) ──────────────────────────────────
	notifications.RegisterRoutes(mux, deps.NotificationsHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Audit logs (tenant-scoped, read-only, admin only) ─────────────────────
	audit.RegisterRoutes(mux, deps.AuditHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Reports (tenant-scoped, report.view permission) ───────────────────────
	reports.RegisterRoutes(mux, deps.ReportsHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Dashboard / analytics (tenant-scoped) ─────────────────────────────────
	dashboard.RegisterRoutes(mux, deps.DashboardHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── File management (tenant-scoped) ───────────────────────────────────────
	files.RegisterRoutes(mux, deps.FilesHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Global middleware chain ───────────────────────────────────────────────
	// Execution order (outermost → innermost):
	//   RequestID → CORS → Recovery → Logger → mux
	corsCfg := middleware.DefaultCORSConfig()
	if len(deps.AllowedOrigins) > 0 {
		corsCfg.AllowedOrigins = deps.AllowedOrigins
	}
	handler := middleware.Logger(deps.Log)(mux)
	handler = middleware.Recovery(deps.Log)(handler)
	handler = middleware.CORS(corsCfg)(handler)
	handler = middleware.RequestID(handler)

	return handler
}

// healthHandler returns a liveness + readiness probe.
// Returns 200 when the app and DB are healthy; 503 when the DB is unreachable.
func healthHandler(db *pgxpool.Pool, log *slog.Logger) http.HandlerFunc {
	type check struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	type healthResponse struct {
		Status   string           `json:"status"`
		Checks   map[string]check `json:"checks"`
		Version  string           `json:"version"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		checks := make(map[string]check)
		overall := "ok"

		// Database ping with a 3-second timeout.
		dbStatus := check{Status: "ok"}
		if db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()

			if err := db.Ping(ctx); err != nil {
				dbStatus = check{Status: "error", Error: "database unreachable"}
				overall = "degraded"
				log.Warn("health check: db ping failed", "error", err)
			}
		}
		checks["database"] = dbStatus

		resp := healthResponse{
			Status:  overall,
			Checks:  checks,
			Version: "1.0.0",
		}

		statusCode := http.StatusOK
		if overall != "ok" {
			statusCode = http.StatusServiceUnavailable
		}

		response.Success(w, statusCode, "health check", resp, log)
	}
}
