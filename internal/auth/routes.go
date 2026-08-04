package auth

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
)

// RegisterRoutes mounts the auth endpoints onto mux.
func RegisterRoutes(mux *http.ServeMux, h *Handler, jwtManager *platformauth.Manager, log *slog.Logger) {
	// Public — no authentication required.
	mux.HandleFunc("POST /api/v1/auth/google",    h.GoogleLogin)
	mux.HandleFunc("POST /api/v1/auth/refresh",   h.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout",    h.Logout)
	mux.HandleFunc("POST /api/v1/auth/exchange",  h.Exchange)

	// Server-side OAuth2 redirect flow.
	mux.HandleFunc("GET /api/v1/auth/google/login",    h.GoogleOAuthInitiate)
	mux.HandleFunc("GET /api/v1/auth/google/callback", h.GoogleOAuthCallback)

	// Protected — valid access token required.
	authenticate := middleware.Authenticate(jwtManager, log)
	mux.Handle("GET  /api/v1/auth/me",     authenticate(http.HandlerFunc(h.Me)))
	mux.Handle("POST /api/v1/yards",       authenticate(http.HandlerFunc(h.CreateYard)))
	mux.Handle("POST /api/v1/users/invite",authenticate(http.HandlerFunc(h.InviteUser)))
}
