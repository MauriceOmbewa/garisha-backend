package users

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the users domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request / Response DTOs ─────────────────────────────────────────────────

type assignRoleRequest struct {
	Role string `json:"role" validate:"required"`
}

type assignBranchRequest struct {
	BranchID string `json:"branch_id"` // empty string = remove branch assignment
}

type updatePermissionsRequest struct {
	// Permissions is the complete replacement set of permission overrides.
	// Send an empty array to clear all overrides.
	Permissions []string `json:"permissions" validate:"required"`
}

// userDTO is the public representation of a user.
type userDTO struct {
	ID          string   `json:"id"`
	TenantID    *string  `json:"tenant_id"`
	BranchID    *string  `json:"branch_id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	AvatarURL   *string  `json:"avatar_url"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	IsActive    bool     `json:"is_active"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/users
// Returns all users for the resolved tenant.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]userDTO, 0, len(users))
	for _, u := range users {
		dtos = append(dtos, toDTO(u))
	}

	response.Success(w, http.StatusOK, "users retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/users/{id}
// Returns a single user scoped to the resolved tenant.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	u, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "user retrieved", toDTO(u), h.log)
}

// AssignBranch godoc
// PATCH /api/v1/users/{id}/branch
// Sets (or clears) the branch for a user.
func (h *Handler) AssignBranch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req assignBranchRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	u, err := h.svc.AssignBranch(r.Context(), id, req.BranchID)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "user branch updated", toDTO(u), h.log)
}

// AssignRole godoc
// PATCH /api/v1/users/{id}/role
// Assigns a new role to a user.  Requires user.update permission.
func (h *Handler) AssignRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req assignRoleRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	u, err := h.svc.AssignRole(r.Context(), id, req.Role)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "user role updated", toDTO(u), h.log)
}

// Activate godoc
// POST /api/v1/users/{id}/activate
// Re-enables a suspended user account.  Requires user.update permission.
func (h *Handler) Activate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	u, err := h.svc.Activate(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "user activated", toDTO(u), h.log)
}

// Suspend godoc
// POST /api/v1/users/{id}/suspend
// Suspends a user account preventing login.  Requires user.update permission.
func (h *Handler) Suspend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	u, err := h.svc.Suspend(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "user suspended", toDTO(u), h.log)
}

// UpdatePermissions godoc
// PUT /api/v1/users/{id}/permissions
// Replaces the per-user permission overrides.  Requires user.update permission.
func (h *Handler) UpdatePermissions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updatePermissionsRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	u, err := h.svc.UpdatePermissions(r.Context(), id, req.Permissions)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "user permissions updated", toDTO(u), h.log)
}

// Delete godoc
// DELETE /api/v1/users/{id}
// Hard-deletes a user.  Requires user.delete permission.
// Prefer Suspend for reversible deactivation.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.svc.Delete(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toDTO(u *User) userDTO {
	perms := u.Permissions
	if perms == nil {
		perms = []string{}
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
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   u.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
