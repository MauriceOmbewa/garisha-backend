package auth

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
)

// RegisterRoutes mounts the auth endpoints onto mux.
// It receives the JWT manager so the Authenticate middleware can be applied
// to protected routes in this module.
func RegisterRoutes(mux *http.ServeMux, h *Handler, jwtManager *platformauth.Manager, log *slog.Logger) {
	// Public — no authentication required.
	mux.HandleFunc("POST /api/v1/auth/google",  h.GoogleLogin)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.Refresh)

	// Protected — valid access token required.
	authenticate := middleware.Authenticate(jwtManager, log)
	mux.Handle("GET /api/v1/auth/me", authenticate(http.HandlerFunc(h.Me)))
}
