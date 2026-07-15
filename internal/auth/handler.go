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
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request / Response DTOs ─────────────────────────────────────────────────

type googleLoginRequest struct {
	TenantID string `json:"tenant_id" validate:"required,uuid4"`
	IDToken  string `json:"id_token"  validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type authResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	User         userDTO  `json:"user"`
}

type userDTO struct {
	ID        string  `json:"id"`
	TenantID  *string `json:"tenant_id"`
	Email     string  `json:"email"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
	Role      string  `json:"role"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// GoogleLogin godoc
// POST /api/v1/auth/google
// Accepts a Google ID token (from the frontend/mobile app) and returns a JWT pair.
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

	response.Success(w, http.StatusOK, "login successful", authResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         toUserDTO(user),
	}, h.log)
}

// Refresh godoc
// POST /api/v1/auth/refresh
// Accepts a refresh token and returns a new token pair.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	tokens, err := h.svc.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "tokens refreshed", map[string]string{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
	}, h.log)
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

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toUserDTO(u *User) userDTO {
	return userDTO{
		ID:        u.ID,
		TenantID:  u.TenantID,
		Email:     u.Email,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
		Role:      u.Role,
	}
}
