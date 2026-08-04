package auth

import (
	"log/slog"
	"net/http"

	platformauth "github.com/MauriceOmbewa/garisha-backend/internal/platform/auth"
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

type membershipDTO struct {
	TenantID   string  `json:"tenant_id"`
	TenantName string  `json:"tenant_name"`
	TenantSlug string  `json:"tenant_slug"`
	BranchID   *string `json:"branch_id"`
	Role       string  `json:"role"`
	IsActive   bool    `json:"is_active"`
}

type userDTO struct {
	ID          string          `json:"id"`
	TenantID    *string         `json:"tenant_id"`
	BranchID    *string         `json:"branch_id"`
	Email       string          `json:"email"`
	Name        string          `json:"name"`
	AvatarURL   *string         `json:"avatar_url"`
	Role        string          `json:"role"`
	Permissions []string        `json:"permissions"`
	Memberships []membershipDTO `json:"memberships"`
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
// Returns the currently authenticated user's profile with all memberships.
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

	memberships, _ := h.svc.repo.FindMemberships(r.Context(), user.ID)

	response.Success(w, http.StatusOK, "user retrieved", toUserDTOWithMemberships(user, memberships), h.log)
}

// InviteUser godoc
// POST /api/v1/users/invite
// Looks up an existing Google SSO user by email and adds them to the caller's tenant.
// The invited user must have previously signed in via Google SSO.
func (h *Handler) InviteUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.RequireClaims(r.Context(), w, h.log)
	if !ok {
		return
	}

	type inviteRequest struct {
		Email    string  `json:"email"     validate:"required,email"`
		Role     string  `json:"role"      validate:"required"`
		BranchID *string `json:"branch_id"`
	}

	var req inviteRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	user, membership, err := h.svc.InviteUser(r.Context(), claims.UserID, claims.TenantID, req.Email, req.Role, req.BranchID)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dto := toUserDTO(user)
	dto.Memberships = []membershipDTO{{
		TenantID: membership.TenantID,
		Role:     membership.Role,
		BranchID: membership.BranchID,
		IsActive: membership.IsActive,
	}}

	response.Success(w, http.StatusCreated, "user invited to yard", dto, h.log)
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
//
// Browsers block Set-Cookie on cross-origin redirect responses, so we do NOT
// set cookies here. Instead we store the token pair in a server-side session
// store under an opaque one-time code (2 min TTL) and redirect the browser to
// <origin>/my-yards?code=<one-time-code>.
//
// The frontend then POSTs that code to POST /api/v1/auth/exchange, which is a
// normal CORS JSON response — Set-Cookie works correctly there.
func (h *Handler) GoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		apperr.Handle(w, r, apperr.BadRequest("missing code or state"), h.log)
		return
	}

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

	// Store tokens server-side and send an opaque code to the frontend.
	sessionCode := sessions.put(tokens.AccessToken, tokens.RefreshToken)

	// Redirect to <origin>/my-yards?code=<opaque> — no token in the URL.
	http.Redirect(w, r, origin+"/my-yards?code="+sessionCode, http.StatusTemporaryRedirect)
}

// Exchange godoc
// POST /api/v1/auth/exchange
// Called by the frontend immediately after the OAuth redirect lands on /my-yards.
// Accepts the one-time code from the URL, sets HttpOnly cookies in a normal
// CORS response (not a redirect), and returns the user profile.
func (h *Handler) Exchange(w http.ResponseWriter, r *http.Request) {
	type exchangeRequest struct {
		Code string `json:"code" validate:"required"`
	}

	var req exchangeRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	ps, ok := sessions.take(req.Code)
	if !ok {
		apperr.Handle(w, r, apperr.Unauthorized("invalid or expired session code"), h.log)
		return
	}

	// Set both cookies in this normal (non-redirect) response — browsers
	// accept Set-Cookie here without the cross-site redirect restriction.
	h.setBothCookies(w, ps.accessToken, ps.refreshToken)

	// Also fetch and return the user profile so the frontend doesn't need a
	// second round-trip.
	claims, err := h.svc.jwtManager.Verify(ps.accessToken, platformauth.TokenTypeAccess)
	if err != nil {
		apperr.Handle(w, r, apperr.Internal("failed to verify access token", err), h.log)
		return
	}

	user, err := h.svc.Me(r.Context(), claims.UserID)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "session established", authResponse{
		User: toUserDTO(user),
	}, h.log)
}

// CreateYard godoc
// POST /api/v1/yards
// Self-service yard registration. Any authenticated user without a tenant
// can call this to create a new business and become its owner.
// Returns fresh HttpOnly cookies (new tokens with tenant_id) + the user profile.
func (h *Handler) CreateYard(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.RequireClaims(r.Context(), w, h.log)
	if !ok {
		return
	}

	type createYardRequest struct {
		Name         string  `json:"name"          validate:"required,min=2,max=255"`
		Slug         string  `json:"slug"          validate:"required,min=2,max=100"`
		Email        string  `json:"email"         validate:"required,email"`
		Phone        *string `json:"phone"`
		BusinessType *string `json:"business_type" validate:"omitempty,oneof=car_yard dealership rental service_center mixed"`
	}

	var req createYardRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	tokens, user, err := h.svc.CreateYard(r.Context(), claims.UserID, CreateYardParams{
		Name:         req.Name,
		Slug:         req.Slug,
		Email:        req.Email,
		Phone:        req.Phone,
		BusinessType: req.BusinessType,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	// Issue fresh cookies with the new tenant_id baked in.
	h.setBothCookies(w, tokens.AccessToken, tokens.RefreshToken)

	response.Success(w, http.StatusCreated, "yard created", authResponse{
		User: toUserDTO(user),
	}, h.log)
}

func toUserDTO(u *User) userDTO {
	perms := make([]string, 0)
	if u.Permissions != nil {
		perms = u.Permissions
	}

	return userDTO{
		ID:          u.ID,
		TenantID:    u.TenantID,
		BranchID:    u.BranchID,
		Email:       u.Email,
		Name:        u.Name,
		AvatarURL:   u.AvatarURL,
		Role:        u.Role,
		Permissions: perms,
		Memberships: []membershipDTO{},
	}
}

func toUserDTOWithMemberships(u *User, memberships []Membership) userDTO {
	dto := toUserDTO(u)
	for _, m := range memberships {
		dto.Memberships = append(dto.Memberships, membershipDTO{
			TenantID:   m.TenantID,
			TenantName: m.TenantName,
			TenantSlug: m.TenantSlug,
			BranchID:   m.BranchID,
			Role:       m.Role,
			IsActive:   m.IsActive,
		})
	}
	return dto
}
