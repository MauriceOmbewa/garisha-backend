// Package router builds and returns the application's HTTP handler tree.
// Domain route groups are registered by passing the mux to each module's
// RegisterRoutes function, keeping this file thin.
package router

import (
	"log/slog"
	"net/http"

	"github.com/MauriceOmbewa/garisha-backend/internal/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/company"
	"github.com/MauriceOmbewa/garisha-backend/internal/customers"
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
	Log            *slog.Logger
	JWTManager     *platformauth.Manager
	TenantResolver middleware.TenantResolver
	AuthHandler    *auth.Handler
	TenantsHandler *tenants.Handler
	CompanyHandler  *company.Handler
	UsersHandler    *users.Handler
	VehiclesHandler *vehicles.Handler
	CustomersHandler *customers.Handler
}

// New constructs the root http.Handler with all middleware applied and all
// domain routes registered.
func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	// ── Health (no tenant context needed) ────────────────────────────────────
	mux.HandleFunc("GET /api/v1/health", healthHandler(deps.Log))

	// ── Auth routes (public + protected, no tenant scope enforcement) ─────────
	// Login and refresh are public and use tenant resolution internally.
	// /auth/me only needs authentication, not a tenant header.
	auth.RegisterRoutes(mux, deps.AuthHandler, deps.JWTManager, deps.Log)

	// ── Super-admin tenant management (no tenant header required) ─────────────
	tenants.RegisterRoutes(mux, deps.TenantsHandler, deps.JWTManager, deps.Log)

	// ── Company profile (tenant-scoped) ───────────────────────────────────────
	company.RegisterRoutes(mux, deps.CompanyHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── User management (tenant-scoped) ───────────────────────────────────────
	users.RegisterRoutes(mux, deps.UsersHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Vehicle management (tenant-scoped) ────────────────────────────────────
	vehicles.RegisterRoutes(mux, deps.VehiclesHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Customer management (tenant-scoped) ────────────────────────────────────
	customers.RegisterRoutes(mux, deps.CustomersHandler, deps.JWTManager, deps.TenantResolver, deps.Log)

	// ── Global middleware chain ───────────────────────────────────────────────
	// Execution order (outermost → innermost):
	//   RequestID → CORS → Recovery → Logger → mux
	handler := middleware.Logger(deps.Log)(mux)
	handler = middleware.Recovery(deps.Log)(handler)
	handler = middleware.CORS(middleware.DefaultCORSConfig())(handler)
	handler = middleware.RequestID(handler)

	return handler
}

// healthHandler returns a simple liveness probe.
func healthHandler(log *slog.Logger) http.HandlerFunc {
	type healthResponse struct {
		Status string `json:"status"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, http.StatusOK, "service is healthy", healthResponse{
			Status: "ok",
		}, log)
	}
}
