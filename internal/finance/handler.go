package finance

import (
	"log/slog"
	"net/http"
	"time"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the finance domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Category request DTOs ────────────────────────────────────────────────────

type createCategoryRequest struct {
	Name        string  `json:"name"        validate:"required,min=1,max=100"`
	Type        string  `json:"type"        validate:"required,oneof=income expense"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

type updateCategoryRequest struct {
	Name        *string `json:"name"        validate:"omitempty,min=1,max=100"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	IsActive    *bool   `json:"is_active"`
}

// ─── Record request DTOs ──────────────────────────────────────────────────────

type createRecordRequest struct {
	CategoryID    string  `json:"category_id"  validate:"required,uuid4"`
	Type          string  `json:"type"         validate:"required,oneof=income expense"`
	Amount        float64 `json:"amount"       validate:"required,gt=0"`
	Description   string  `json:"description"  validate:"required,min=1,max=500"`
	TransactionDate *string `json:"transaction_date" validate:"omitempty"` // YYYY-MM-DD

	HireBookingID *string `json:"hire_booking_id" validate:"omitempty,uuid4"`
	SaleID        *string `json:"sale_id"         validate:"omitempty,uuid4"`
	ServiceJobID  *string `json:"service_job_id"  validate:"omitempty,uuid4"`

	PaymentMethod *string `json:"payment_method" validate:"omitempty,oneof=cash mpesa bank_transfer card other"`
	Reference     *string `json:"reference"      validate:"omitempty,max=200"`
	Notes         *string `json:"notes"          validate:"omitempty,max=2000"`
}

type updateRecordRequest struct {
	CategoryID    *string  `json:"category_id"      validate:"omitempty,uuid4"`
	Amount        *float64 `json:"amount"           validate:"omitempty,gt=0"`
	Description   *string  `json:"description"      validate:"omitempty,min=1,max=500"`
	TransactionDate *string `json:"transaction_date" validate:"omitempty"` // YYYY-MM-DD

	HireBookingID *string `json:"hire_booking_id" validate:"omitempty,uuid4"`
	SaleID        *string `json:"sale_id"         validate:"omitempty,uuid4"`
	ServiceJobID  *string `json:"service_job_id"  validate:"omitempty,uuid4"`

	PaymentMethod *string `json:"payment_method" validate:"omitempty,oneof=cash mpesa bank_transfer card other"`
	Reference     *string `json:"reference"      validate:"omitempty,max=200"`
	Notes         *string `json:"notes"          validate:"omitempty,max=2000"`
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

type categoryDTO struct {
	ID          string  `json:"id"`
	TenantID    string  `json:"tenant_id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description *string `json:"description"`
	IsActive    bool    `json:"is_active"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type recordDTO struct {
	ID         string  `json:"id"`
	TenantID   string  `json:"tenant_id"`
	CategoryID string  `json:"category_id"`
	Type       string  `json:"type"`
	Amount     float64 `json:"amount"`

	HireBookingID *string `json:"hire_booking_id"`
	SaleID        *string `json:"sale_id"`
	ServiceJobID  *string `json:"service_job_id"`

	Description     string  `json:"description"`
	TransactionDate string  `json:"transaction_date"`
	PaymentMethod   *string `json:"payment_method"`
	Reference       *string `json:"reference"`

	CreatedBy *string `json:"created_by"`
	Notes     *string `json:"notes"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ─── Category handlers ────────────────────────────────────────────────────────

// ListCategories godoc
// GET /api/v1/finance/categories[?type=income]
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	var t *string
	if v := r.URL.Query().Get("type"); v != "" {
		t = &v
	}

	cats, err := h.svc.ListCategories(r.Context(), t)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]categoryDTO, 0, len(cats))
	for _, c := range cats {
		dtos = append(dtos, toCategoryDTO(c))
	}

	response.Success(w, http.StatusOK, "categories retrieved", dtos, h.log)
}

// GetCategory godoc
// GET /api/v1/finance/categories/{id}
func (h *Handler) GetCategory(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.GetCategory(r.Context(), r.PathValue("id"))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "category retrieved", toCategoryDTO(c), h.log)
}

// CreateCategory godoc
// POST /api/v1/finance/categories
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req createCategoryRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	c, err := h.svc.CreateCategory(r.Context(), CreateCategoryInput{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "category created", toCategoryDTO(c), h.log)
}

// UpdateCategory godoc
// PATCH /api/v1/finance/categories/{id}
func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req updateCategoryRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	c, err := h.svc.UpdateCategory(r.Context(), r.PathValue("id"), UpdateCategoryInput{
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "category updated", toCategoryDTO(c), h.log)
}

// DeleteCategory godoc
// DELETE /api/v1/finance/categories/{id}
func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteCategory(r.Context(), r.PathValue("id")); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Record handlers ──────────────────────────────────────────────────────────

// GetSummary godoc
// GET /api/v1/finance/summary[?from=YYYY-MM-DD&to=YYYY-MM-DD]
func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	var from, to *time.Time

	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid from — expected YYYY-MM-DD"), h.log)
			return
		}
		from = &t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid to — expected YYYY-MM-DD"), h.log)
			return
		}
		to = &t
	}

	summary, err := h.svc.GetSummary(r.Context(), from, to)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "ledger summary retrieved", summary, h.log)
}

// ListRecords godoc
// GET /api/v1/finance/records[?type=income&category_id=...&from=...&to=...&payment_method=...]
func (h *Handler) ListRecords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f RecordFilters
	if v := q.Get("type"); v != "" {
		f.Type = &v
	}
	if v := q.Get("category_id"); v != "" {
		f.CategoryID = &v
	}
	if v := q.Get("payment_method"); v != "" {
		f.PaymentMethod = &v
	}
	if v := q.Get("hire_booking_id"); v != "" {
		f.HireBookingID = &v
	}
	if v := q.Get("sale_id"); v != "" {
		f.SaleID = &v
	}
	if v := q.Get("service_job_id"); v != "" {
		f.ServiceJobID = &v
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.FromDate = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.ToDate = &t
		}
	}

	records, err := h.svc.ListRecords(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]recordDTO, 0, len(records))
	for _, rec := range records {
		dtos = append(dtos, toRecordDTO(rec))
	}

	response.Success(w, http.StatusOK, "finance records retrieved", dtos, h.log)
}

// GetRecord godoc
// GET /api/v1/finance/records/{id}
func (h *Handler) GetRecord(w http.ResponseWriter, r *http.Request) {
	rec, err := h.svc.GetRecord(r.Context(), r.PathValue("id"))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "finance record retrieved", toRecordDTO(rec), h.log)
}

// CreateRecord godoc
// POST /api/v1/finance/records
func (h *Handler) CreateRecord(w http.ResponseWriter, r *http.Request) {
	var req createRecordRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	in := CreateRecordInput{
		CategoryID:    req.CategoryID,
		Type:          req.Type,
		Amount:        req.Amount,
		Description:   req.Description,
		HireBookingID: req.HireBookingID,
		SaleID:        req.SaleID,
		ServiceJobID:  req.ServiceJobID,
		PaymentMethod: req.PaymentMethod,
		Reference:     req.Reference,
		Notes:         req.Notes,
	}

	if req.TransactionDate != nil && *req.TransactionDate != "" {
		t, err := time.Parse("2006-01-02", *req.TransactionDate)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid transaction_date — expected YYYY-MM-DD"), h.log)
			return
		}
		in.TransactionDate = &t
	}

	rec, err := h.svc.CreateRecord(r.Context(), in)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "finance record created", toRecordDTO(rec), h.log)
}

// UpdateRecord godoc
// PATCH /api/v1/finance/records/{id}
func (h *Handler) UpdateRecord(w http.ResponseWriter, r *http.Request) {
	var req updateRecordRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	in := UpdateRecordInput{
		CategoryID:    req.CategoryID,
		Amount:        req.Amount,
		Description:   req.Description,
		HireBookingID: req.HireBookingID,
		SaleID:        req.SaleID,
		ServiceJobID:  req.ServiceJobID,
		PaymentMethod: req.PaymentMethod,
		Reference:     req.Reference,
		Notes:         req.Notes,
	}

	if req.TransactionDate != nil && *req.TransactionDate != "" {
		t, err := time.Parse("2006-01-02", *req.TransactionDate)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid transaction_date — expected YYYY-MM-DD"), h.log)
			return
		}
		in.TransactionDate = &t
	}

	rec, err := h.svc.UpdateRecord(r.Context(), r.PathValue("id"), in)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "finance record updated", toRecordDTO(rec), h.log)
}

// DeleteRecord godoc
// DELETE /api/v1/finance/records/{id}
func (h *Handler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteRecord(r.Context(), r.PathValue("id")); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toCategoryDTO(c *Category) categoryDTO {
	return categoryDTO{
		ID:          c.ID,
		TenantID:    c.TenantID,
		Name:        c.Name,
		Type:        string(c.Type),
		Description: c.Description,
		IsActive:    c.IsActive,
		CreatedAt:   c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   c.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toRecordDTO(rec *Record) recordDTO {
	return recordDTO{
		ID:              rec.ID,
		TenantID:        rec.TenantID,
		CategoryID:      rec.CategoryID,
		Type:            string(rec.Type),
		Amount:          rec.Amount,
		HireBookingID:   rec.HireBookingID,
		SaleID:          rec.SaleID,
		ServiceJobID:    rec.ServiceJobID,
		Description:     rec.Description,
		TransactionDate: rec.TransactionDate.UTC().Format("2006-01-02"),
		PaymentMethod:   rec.PaymentMethod,
		Reference:       rec.Reference,
		CreatedBy:       rec.CreatedBy,
		Notes:           rec.Notes,
		CreatedAt:       rec.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       rec.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
