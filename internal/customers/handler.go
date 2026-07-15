package customers

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the customers domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type createCustomerRequest struct {
	FullName string  `json:"full_name" validate:"required,min=1,max=255"`
	Email    *string `json:"email"     validate:"omitempty,email"`
	Phone    *string `json:"phone"     validate:"omitempty,max=50"`
	IDNumber *string `json:"id_number" validate:"omitempty,max=100"`
	IDType   *string `json:"id_type"   validate:"omitempty,oneof=national_id passport driving_license other"`
	Country  *string `json:"country"   validate:"omitempty,max=100"`
	City     *string `json:"city"      validate:"omitempty,max=100"`
	Address  *string `json:"address"`
	Notes    *string `json:"notes"     validate:"omitempty,max=2000"`
}

type updateCustomerRequest struct {
	FullName *string `json:"full_name" validate:"omitempty,min=1,max=255"`
	Email    *string `json:"email"     validate:"omitempty,email"`
	Phone    *string `json:"phone"     validate:"omitempty,max=50"`
	IDNumber *string `json:"id_number" validate:"omitempty,max=100"`
	IDType   *string `json:"id_type"   validate:"omitempty,oneof=national_id passport driving_license other"`
	Country  *string `json:"country"   validate:"omitempty,max=100"`
	City     *string `json:"city"      validate:"omitempty,max=100"`
	Address  *string `json:"address"`
	IsActive *bool   `json:"is_active"`
	Notes    *string `json:"notes"     validate:"omitempty,max=2000"`
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type customerDTO struct {
	ID       string  `json:"id"`
	TenantID string  `json:"tenant_id"`
	UserID   *string `json:"user_id"`

	FullName string  `json:"full_name"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
	IDNumber *string `json:"id_number"`
	IDType   *string `json:"id_type"`

	Country  *string `json:"country"`
	City     *string `json:"city"`
	Address  *string `json:"address"`

	IsActive  bool    `json:"is_active"`
	Notes     *string `json:"notes"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/customers[?search=john&active=true]
// Returns the tenant's customer list with optional search and active filter.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f ListFilters
	if s := q.Get("search"); s != "" {
		f.Search = &s
	}

	if a := q.Get("active"); a != "" {
		active := a == "true"
		f.IsActive = &active
	}

	customers, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]customerDTO, 0, len(customers))
	for _, c := range customers {
		dtos = append(dtos, toDTO(c))
	}

	response.Success(w, http.StatusOK, "customers retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/customers/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	c, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "customer retrieved", toDTO(c), h.log)
}

// Create godoc
// POST /api/v1/customers
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createCustomerRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	c, err := h.svc.Create(r.Context(), CreateInput{
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		IDNumber: req.IDNumber,
		IDType:   req.IDType,
		Country:  req.Country,
		City:     req.City,
		Address:  req.Address,
		Notes:    req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "customer created", toDTO(c), h.log)
}

// Update godoc
// PATCH /api/v1/customers/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateCustomerRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	c, err := h.svc.Update(r.Context(), id, UpdateInput{
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		IDNumber: req.IDNumber,
		IDType:   req.IDType,
		Country:  req.Country,
		City:     req.City,
		Address:  req.Address,
		IsActive: req.IsActive,
		Notes:    req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "customer updated", toDTO(c), h.log)
}

// Delete godoc
// DELETE /api/v1/customers/{id}
// Hard-deletes a customer.  Prefer PATCH is_active=false for customers with
// existing transaction history.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.svc.Delete(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toDTO(c *Customer) customerDTO {
	return customerDTO{
		ID:        c.ID,
		TenantID:  c.TenantID,
		UserID:    c.UserID,
		FullName:  c.FullName,
		Email:     c.Email,
		Phone:     c.Phone,
		IDNumber:  c.IDNumber,
		IDType:    c.IDType,
		Country:   c.Country,
		City:      c.City,
		Address:   c.Address,
		IsActive:  c.IsActive,
		Notes:     c.Notes,
		CreatedAt: c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: c.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
