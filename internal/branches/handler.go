package branches

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds HTTP handlers for the branches domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ── DTOs ──────────────────────────────────────────────────────────────────────

type createBranchRequest struct {
	Name      string  `json:"name"      validate:"required,min=2,max=255"`
	Slug      string  `json:"slug"      validate:"omitempty,min=2,max=100"`
	City      *string `json:"city"`
	Address   *string `json:"address"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"     validate:"omitempty,email"`
	IsDefault bool    `json:"is_default"`
}

type updateBranchRequest struct {
	Name      *string `json:"name"      validate:"omitempty,min=2,max=255"`
	City      *string `json:"city"`
	Address   *string `json:"address"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"     validate:"omitempty,email"`
	IsActive  *bool   `json:"is_active"`
	IsDefault *bool   `json:"is_default"`
}

type branchDTO struct {
	ID        string  `json:"id"`
	TenantID  string  `json:"tenant_id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	City      *string `json:"city"`
	Address   *string `json:"address"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"`
	IsActive  bool    `json:"is_active"`
	IsDefault bool    `json:"is_default"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// List godoc — GET /api/v1/branches
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	branches, err := h.svc.List(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]branchDTO, 0, len(branches))
	for _, b := range branches {
		dtos = append(dtos, toDTO(b))
	}
	response.Success(w, http.StatusOK, "branches retrieved", dtos, h.log)
}

// Get godoc — GET /api/v1/branches/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := h.svc.Get(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}
	response.Success(w, http.StatusOK, "branch retrieved", toDTO(b), h.log)
}

// Create godoc — POST /api/v1/branches
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createBranchRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	b, err := h.svc.Create(r.Context(), CreateInput{
		Name:      req.Name,
		Slug:      req.Slug,
		City:      req.City,
		Address:   req.Address,
		Phone:     req.Phone,
		Email:     req.Email,
		IsDefault: req.IsDefault,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}
	response.Success(w, http.StatusCreated, "branch created", toDTO(b), h.log)
}

// Update godoc — PATCH /api/v1/branches/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateBranchRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	b, err := h.svc.Update(r.Context(), id, UpdateInput{
		Name:      req.Name,
		City:      req.City,
		Address:   req.Address,
		Phone:     req.Phone,
		Email:     req.Email,
		IsActive:  req.IsActive,
		IsDefault: req.IsDefault,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}
	response.Success(w, http.StatusOK, "branch updated", toDTO(b), h.log)
}

// Delete godoc — DELETE /api/v1/branches/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}
	response.NoContent(w)
}

// ── helper ────────────────────────────────────────────────────────────────────

func toDTO(b *Branch) branchDTO {
	return branchDTO{
		ID:        b.ID,
		TenantID:  b.TenantID,
		Name:      b.Name,
		Slug:      b.Slug,
		City:      b.City,
		Address:   b.Address,
		Phone:     b.Phone,
		Email:     b.Email,
		IsActive:  b.IsActive,
		IsDefault: b.IsDefault,
		CreatedAt: b.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: b.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
