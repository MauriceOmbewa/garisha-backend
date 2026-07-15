// Package router builds and returns the application's HTTP handler tree.
// Domain route groups are registered by passing the mux to each module's
// RegisterRoutes function, keeping this file thin.
package router

import (
	"log/slog"
	"net/http"

	"github.com/MauriceOmbewa/garisha-backend/internal/auth"
	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// Dependencies groups every cross-cutting dependency the router needs to
// wire up all domain modules.  New fields are added here as modules are built.
type Dependencies struct {
	Log        *slog.Logger
	JWTManager *platformauth.Manager
	AuthHandler *auth.Handler
}

// New constructs the root http.Handler with all middleware applied and all
// domain routes registered.
func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	// ── Health ───────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/v1/health", healthHandler(deps.Log))

	// ── Domain routes ────────────────────────────────────────────────────────
	auth.RegisterRoutes(mux, deps.AuthHandler, deps.JWTManager, deps.Log)

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
