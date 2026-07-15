package sales

import (
	"log/slog"
	"net/http"
	"time"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the sales domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type createSaleRequest struct {
	VehicleID  string `json:"vehicle_id"  validate:"required,uuid4"`
	CustomerID string `json:"customer_id" validate:"required,uuid4"`

	AskingPrice    float64 `json:"asking_price"    validate:"gte=0"`
	AgreedPrice    float64 `json:"agreed_price"    validate:"required,gt=0"`
	DepositAmount  float64 `json:"deposit_amount"  validate:"gte=0"`
	DiscountAmount float64 `json:"discount_amount" validate:"gte=0"`

	PaymentMethod *string `json:"payment_method" validate:"omitempty,oneof=cash mpesa bank_transfer finance other"`
	PaymentTerms  *string `json:"payment_terms"  validate:"omitempty,max=200"`

	SaleDate *string `json:"sale_date" validate:"omitempty"` // "YYYY-MM-DD"

	InvoiceNumber *string `json:"invoice_number" validate:"omitempty,max=100"`
	ContractRef   *string `json:"contract_ref"   validate:"omitempty,max=100"`
	Notes         *string `json:"notes"          validate:"omitempty,max=2000"`
}

type updateSaleRequest struct {
	CustomerID *string `json:"customer_id" validate:"omitempty,uuid4"`

	AskingPrice    *float64 `json:"asking_price"    validate:"omitempty,gte=0"`
	AgreedPrice    *float64 `json:"agreed_price"    validate:"omitempty,gt=0"`
	DepositAmount  *float64 `json:"deposit_amount"  validate:"omitempty,gte=0"`
	DiscountAmount *float64 `json:"discount_amount" validate:"omitempty,gte=0"`

	PaymentMethod *string `json:"payment_method" validate:"omitempty,oneof=cash mpesa bank_transfer finance other"`
	PaymentTerms  *string `json:"payment_terms"  validate:"omitempty,max=200"`

	SaleDate   *string `json:"sale_date"   validate:"omitempty"` // "YYYY-MM-DD"
	HandoverAt *string `json:"handover_at" validate:"omitempty"` // RFC3339

	InvoiceNumber *string `json:"invoice_number" validate:"omitempty,max=100"`
	ContractRef   *string `json:"contract_ref"   validate:"omitempty,max=100"`
	Notes         *string `json:"notes"          validate:"omitempty,max=2000"`
}

type updateStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=reserved completed cancelled"`
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type saleDTO struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	VehicleID  string `json:"vehicle_id"`
	CustomerID string `json:"customer_id"`

	AskingPrice    float64 `json:"asking_price"`
	AgreedPrice    float64 `json:"agreed_price"`
	DepositAmount  float64 `json:"deposit_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`

	PaymentMethod *string `json:"payment_method"`
	PaymentTerms  *string `json:"payment_terms"`

	SaleDate   string  `json:"sale_date"`
	HandoverAt *string `json:"handover_at"`

	Status string `json:"status"`

	InvoiceNumber *string `json:"invoice_number"`
	ContractRef   *string `json:"contract_ref"`

	CreatedBy *string `json:"created_by"`
	Notes     *string `json:"notes"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/sales[?status=pending&vehicle_id=...&customer_id=...&from=YYYY-MM-DD&to=YYYY-MM-DD]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f ListFilters
	if s := q.Get("status"); s != "" {
		f.Status = &s
	}
	if v := q.Get("vehicle_id"); v != "" {
		f.VehicleID = &v
	}
	if c := q.Get("customer_id"); c != "" {
		f.CustomerID = &c
	}
	if from := q.Get("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			f.FromDate = &t
		}
	}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			f.ToDate = &t
		}
	}

	sales, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]saleDTO, 0, len(sales))
	for _, s := range sales {
		dtos = append(dtos, toDTO(s))
	}

	response.Success(w, http.StatusOK, "sales retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/sales/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "sale retrieved", toDTO(s), h.log)
}

// Create godoc
// POST /api/v1/sales
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createSaleRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	in := CreateInput{
		VehicleID:      req.VehicleID,
		CustomerID:     req.CustomerID,
		AskingPrice:    req.AskingPrice,
		AgreedPrice:    req.AgreedPrice,
		DepositAmount:  req.DepositAmount,
		DiscountAmount: req.DiscountAmount,
		PaymentMethod:  req.PaymentMethod,
		PaymentTerms:   req.PaymentTerms,
		InvoiceNumber:  req.InvoiceNumber,
		ContractRef:    req.ContractRef,
		Notes:          req.Notes,
	}

	if req.SaleDate != nil && *req.SaleDate != "" {
		t, err := time.Parse("2006-01-02", *req.SaleDate)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid sale_date — expected YYYY-MM-DD"), h.log)
			return
		}
		in.SaleDate = t
	}

	s, err := h.svc.Create(r.Context(), in)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "sale created", toDTO(s), h.log)
}

// Update godoc
// PATCH /api/v1/sales/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateSaleRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	in := UpdateInput{
		CustomerID:    req.CustomerID,
		AskingPrice:   req.AskingPrice,
		AgreedPrice:   req.AgreedPrice,
		DepositAmount: req.DepositAmount,
		DiscountAmount: req.DiscountAmount,
		PaymentMethod: req.PaymentMethod,
		PaymentTerms:  req.PaymentTerms,
		InvoiceNumber: req.InvoiceNumber,
		ContractRef:   req.ContractRef,
		Notes:         req.Notes,
	}

	if req.SaleDate != nil && *req.SaleDate != "" {
		t, err := time.Parse("2006-01-02", *req.SaleDate)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid sale_date — expected YYYY-MM-DD"), h.log)
			return
		}
		in.SaleDate = &t
	}

	if req.HandoverAt != nil && *req.HandoverAt != "" {
		t, err := time.Parse(time.RFC3339, *req.HandoverAt)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid handover_at — expected RFC3339"), h.log)
			return
		}
		in.HandoverAt = &t
	}

	s, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "sale updated", toDTO(s), h.log)
}

// UpdateStatus godoc
// PATCH /api/v1/sales/{id}/status
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateStatusRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	s, err := h.svc.UpdateStatus(r.Context(), id, SaleStatus(req.Status))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "sale status updated", toDTO(s), h.log)
}

// Delete godoc
// DELETE /api/v1/sales/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.svc.Delete(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toDTO(s *Sale) saleDTO {
	dto := saleDTO{
		ID:             s.ID,
		TenantID:       s.TenantID,
		VehicleID:      s.VehicleID,
		CustomerID:     s.CustomerID,
		AskingPrice:    s.AskingPrice,
		AgreedPrice:    s.AgreedPrice,
		DepositAmount:  s.DepositAmount,
		DiscountAmount: s.DiscountAmount,
		FinalAmount:    s.FinalAmount,
		PaymentMethod:  s.PaymentMethod,
		PaymentTerms:   s.PaymentTerms,
		SaleDate:       s.SaleDate.UTC().Format("2006-01-02"),
		Status:         string(s.Status),
		InvoiceNumber:  s.InvoiceNumber,
		ContractRef:    s.ContractRef,
		CreatedBy:      s.CreatedBy,
		Notes:          s.Notes,
		CreatedAt:      s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      s.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}

	if s.HandoverAt != nil {
		t := s.HandoverAt.UTC().Format("2006-01-02T15:04:05Z")
		dto.HandoverAt = &t
	}

	return dto
}
