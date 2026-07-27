package auth

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/middleware"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the auth domain.
type Handler struct {
	svc        *Service
	log        *slog.Logger
	secureCookie bool // true in production (HTTPS), false in local dev
}

// NewHandler creates a Handler.
// secureCookie should be true in production and false in local HTTP development.
func NewHandler(svc *Service, log *slog.Logger, secureCookie bool) *Handler {
	return &Handler{svc: svc, log: log, secureCookie: secureCookie}
}

// setAccessCookie writes the access token as an HttpOnly cookie.
func (h *Handler) setAccessCookie(w http.ResponseWriter, accessToken string) {
	sameSite := http.SameSiteLaxMode
	if h.secureCookie {
		// Cross-origin (frontend on Vercel, backend on Render) requires
		// SameSite=None so the browser sends the cookie on cross-origin requests.
		// SameSite=None is only valid when Secure=true (HTTPS).
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "garisha_at",
		Value:    accessToken,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: sameSite,
		Path:     "/",
		MaxAge:   15 * 60,
	})
}

// setRefreshCookie writes the refresh token as an HttpOnly cookie.
func (h *Handler) setRefreshCookie(w http.ResponseWriter, refreshToken string) {
	sameSite := http.SameSiteLaxMode
	if h.secureCookie {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "garisha_rt",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: sameSite,
		Path:     "/api/v1/auth",
		MaxAge:   7 * 24 * 60 * 60,
	})
}

// setBothCookies sets access + refresh tokens as HttpOnly cookies in one call.
func (h *Handler) setBothCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	h.setAccessCookie(w, accessToken)
	h.setRefreshCookie(w, refreshToken)
}

// clearAuthCookies removes both auth cookies.
func (h *Handler) clearAuthCookies(w http.ResponseWriter) {
	sameSite := http.SameSiteLaxMode
	if h.secureCookie {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "garisha_at",
		Value:    "",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: sameSite,
		Path:     "/",
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "garisha_rt",
		Value:    "",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: sameSite,
		Path:     "/api/v1/auth",
		MaxAge:   -1,
	})
}

// ─── Request / Response DTOs ─────────────────────────────────────────────────

type googleLoginRequest struct {
	TenantID string `json:"tenant_id"` // optional — omit for consumer logins
	IDToken  string `json:"id_token"   validate:"required"`
}

// refreshRequest is kept for mobile clients that cannot use cookies and must
// send the refresh token in the request body instead.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	User userDTO `json:"user"`
}

type userDTO struct {
	ID          string   `json:"id"`
	TenantID    *string  `json:"tenant_id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	AvatarURL   *string  `json:"avatar_url"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// GoogleLogin godoc
// POST /api/v1/auth/google
// Accepts a Google ID token (from the frontend/mobile app).
// Sets both garisha_at and garisha_rt as HttpOnly cookies.
// Returns only the user profile in the body — no tokens.
func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	var req googleLoginRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	tokens, user, err := h.svc.LoginWithGoogle(r.Context(), req.TenantID, req.IDToken)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	h.setBothCookies(w, tokens.AccessToken, tokens.RefreshToken)

	response.Success(w, http.StatusOK, "login successful", authResponse{
		User: toUserDTO(user),
	}, h.log)
}

// Refresh godoc
// POST /api/v1/auth/refresh
// Reads garisha_rt from the HttpOnly cookie (web) or request body (mobile).
// Rotates both cookies. Returns only the user profile in the body.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Prefer the HttpOnly cookie (web). Fall back to body (mobile apps).
	refreshToken := ""
	if cookie, err := r.Cookie("garisha_rt"); err == nil && cookie.Value != "" {
		refreshToken = cookie.Value
	} else {
		var req refreshRequest
		_ = validation.DecodeJSON(r, &req)
		refreshToken = req.RefreshToken
	}

	if refreshToken == "" {
		apperr.Handle(w, r, apperr.Unauthorized("no refresh token provided"), h.log)
		return
	}

	tokens, err := h.svc.RefreshTokens(r.Context(), refreshToken)
	if err != nil {
		h.clearAuthCookies(w)
		apperr.Handle(w, r, err, h.log)
		return
	}

	h.setBothCookies(w, tokens.AccessToken, tokens.RefreshToken)

	response.Success(w, http.StatusOK, "tokens refreshed", nil, h.log)
}

// Logout godoc
// POST /api/v1/auth/logout
// Clears both auth cookies. No auth required.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearAuthCookies(w)
	response.Success(w, http.StatusOK, "logged out", nil, h.log)
}

// Me godoc
// GET /api/v1/auth/me  (requires Authenticate middleware)
// Returns the currently authenticated user's profile.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.RequireClaims(r.Context(), w, h.log)
	if !ok {
		return
	}

	user, err := h.svc.Me(r.Context(), claims.UserID)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "user retrieved", toUserDTO(user), h.log)
}

// GoogleOAuthInitiate godoc
// GET /api/v1/auth/google/login?origin=<frontend-url>
// Redirects the user to Google's consent screen.
// tenant_id is optional — omit it for consumer logins.
func (h *Handler) GoogleOAuthInitiate(w http.ResponseWriter, r *http.Request) {
	origin := r.URL.Query().Get("origin")
	if origin == "" {
		apperr.Handle(w, r, apperr.BadRequest("origin query parameter is required"), h.log)
		return
	}

	authURL, err := h.svc.GoogleOAuthInitiate(origin)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	// Store optional tenant_id in a short-lived cookie so the callback can use it.
	// Empty string is fine — the service handles nil tenant gracefully.
	tenantID := r.URL.Query().Get("tenant_id")
	http.SetCookie(w, &http.Cookie{
		Name:     "tenant_id",
		Value:    tenantID,
		HttpOnly: true,
		Secure:   r.TLS != nil, // only Secure over HTTPS
		SameSite: http.SameSiteLaxMode,
		Path:     "/api/v1/auth/google/callback",
		MaxAge:   300, // 5 minutes
	})

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// GoogleOAuthCallback godoc
// GET /api/v1/auth/google/callback?code=<code>&state=<state>
// Called by Google after the user grants consent.
// Sets the refresh token as an HttpOnly cookie, then redirects to the frontend
// with only the short-lived access token in the URL.
func (h *Handler) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		apperr.Handle(w, r, apperr.BadRequest("missing code or state"), h.log)
		return
	}

	// tenant_id cookie is optional — empty string means no tenant scope.
	tenantID := ""
	if cookie, err := r.Cookie("tenant_id"); err == nil {
		tenantID = cookie.Value
	}

	tokens, origin, err := h.svc.GoogleOAuthCallback(r.Context(), tenantID, code, state)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	// Clear the tenant_id helper cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   "tenant_id",
		MaxAge: -1,
		Path:   "/api/v1/auth/google/callback",
	})

	// Set both tokens as HttpOnly cookies — nothing goes in the URL.
	h.setBothCookies(w, tokens.AccessToken, tokens.RefreshToken)

	// Redirect clean — no tokens in the URL. The frontend calls /api/v1/auth/me
	// and the browser sends garisha_at automatically via the cookie.
	http.Redirect(w, r, origin+"/my-yards", http.StatusTemporaryRedirect)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toUserDTO(u *User) userDTO {
	perms := make([]string, 0)
	if u.Permissions != nil {
		perms = u.Permissions
	}

	return userDTO{
		ID:          u.ID,
		TenantID:    u.TenantID,
		Email:       u.Email,
		Name:        u.Name,
		AvatarURL:   u.AvatarURL,
		Role:        u.Role,
		Permissions: perms,
	}
}
