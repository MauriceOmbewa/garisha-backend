package tenants

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the tenants domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type createTenantRequest struct {
	Name       string  `json:"name"        validate:"required,min=2,max=255"`
	Slug       string  `json:"slug"        validate:"required,min=2,max=100"`
	Email      string  `json:"email"       validate:"required,email"`
	Phone      *string `json:"phone"`
	LogoURL    *string `json:"logo_url"`
	WebsiteURL *string `json:"website_url"`
	Plan       string  `json:"plan"        validate:"omitempty,oneof=trial basic professional enterprise"`
}

type updateTenantRequest struct {
	Name       *string `json:"name"        validate:"omitempty,min=2,max=255"`
	Email      *string `json:"email"       validate:"omitempty,email"`
	Phone      *string `json:"phone"`
	LogoURL    *string `json:"logo_url"`
	WebsiteURL *string `json:"website_url"`
	Plan       *string `json:"plan"        validate:"omitempty,oneof=trial basic professional enterprise"`
	IsActive   *bool   `json:"is_active"`
}

// tenantDTO is the public representation of a tenant.
type tenantDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	Email      string  `json:"email"`
	Phone      *string `json:"phone"`
	LogoURL    *string `json:"logo_url"`
	WebsiteURL *string `json:"website_url"`
	Plan       string  `json:"plan"`
	IsActive   bool    `json:"is_active"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/admin/tenants
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	records, err := h.svc.List(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]tenantDTO, 0, len(records))
	for _, rec := range records {
		dtos = append(dtos, toDTO(rec))
	}

	response.Success(w, http.StatusOK, "tenants retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/admin/tenants/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rec, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "tenant retrieved", toDTO(rec), h.log)
}

// Create godoc
// POST /api/v1/admin/tenants
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	rec, err := h.svc.Create(r.Context(), CreateParams{
		Name:       req.Name,
		Slug:       req.Slug,
		Email:      req.Email,
		Phone:      req.Phone,
		LogoURL:    req.LogoURL,
		WebsiteURL: req.WebsiteURL,
		Plan:       req.Plan,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "tenant created", toDTO(rec), h.log)
}

// Update godoc
// PATCH /api/v1/admin/tenants/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateTenantRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	rec, err := h.svc.Update(r.Context(), id, UpdateParams{
		Name:       req.Name,
		Email:      req.Email,
		Phone:      req.Phone,
		LogoURL:    req.LogoURL,
		WebsiteURL: req.WebsiteURL,
		Plan:       req.Plan,
		IsActive:   req.IsActive,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "tenant updated", toDTO(rec), h.log)
}

// Delete godoc
// DELETE /api/v1/admin/tenants/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.svc.Delete(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toDTO(rec *tenant.Record) tenantDTO {
	return tenantDTO{
		ID:         rec.ID,
		Name:       rec.Name,
		Slug:       rec.Slug,
		Email:      rec.Email,
		Phone:      rec.Phone,
		LogoURL:    rec.LogoURL,
		WebsiteURL: rec.WebsiteURL,
		Plan:       rec.Plan,
		IsActive:   rec.IsActive,
		CreatedAt:  rec.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  rec.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
