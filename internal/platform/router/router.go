// Package router builds and returns the application's HTTP handler tree.
// It wires the middleware chain and registers all route groups under the
// /api/v1 prefix.  Domain-specific route registration is delegated to each
// internal module so this file stays thin.
package router

import (
	"log/slog"
	"net/http"

	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
)

// New constructs the root http.Handler with all middleware applied and
// routes registered.  Additional domain handlers will be mounted here in
// later phases.
func New(log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// ── Health & readiness ───────────────────────────────────────────────────
	mux.HandleFunc("GET /api/v1/health", healthHandler(log))

	// ── Global middleware chain ──────────────────────────────────────────────
	// Applied outermost-first so execution order is:
	//   RequestID → CORS → Recovery → Logger → mux
	handler := middleware.Logger(log)(mux)
	handler = middleware.Recovery(log)(handler)
	handler = middleware.CORS(middleware.DefaultCORSConfig())(handler)
	handler = middleware.RequestID(handler)

	return handler
}

// healthHandler returns a simple liveness probe used by load balancers and
// container orchestrators to verify the process is up.
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
