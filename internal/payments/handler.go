package payments

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the payments domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type createManualRequest struct {
	HireBookingID *string `json:"hire_booking_id" validate:"omitempty,uuid4"`
	SaleID        *string `json:"sale_id"         validate:"omitempty,uuid4"`
	ServiceJobID  *string `json:"service_job_id"  validate:"omitempty,uuid4"`
	CustomerID    *string `json:"customer_id"     validate:"omitempty,uuid4"`

	Method   string  `json:"payment_method" validate:"required,oneof=cash bank_transfer card other"`
	Amount   float64 `json:"amount"         validate:"required,gt=0"`
	Currency string  `json:"currency"       validate:"omitempty,max=5"`

	Reference *string `json:"reference" validate:"omitempty,max=200"`
	Notes     *string `json:"notes"     validate:"omitempty,max=2000"`
}

type initiateMpesaRequest struct {
	HireBookingID *string `json:"hire_booking_id" validate:"omitempty,uuid4"`
	SaleID        *string `json:"sale_id"         validate:"omitempty,uuid4"`
	ServiceJobID  *string `json:"service_job_id"  validate:"omitempty,uuid4"`
	CustomerID    *string `json:"customer_id"     validate:"omitempty,uuid4"`

	PhoneNumber      string  `json:"phone_number"       validate:"required,min=10,max=15"`
	Amount           float64 `json:"amount"             validate:"required,gt=0"`
	AccountReference string  `json:"account_reference"  validate:"required,max=12"`
	Description      string  `json:"description"        validate:"required,max=13"`

	Notes *string `json:"notes" validate:"omitempty,max=2000"`
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type paymentDTO struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`

	HireBookingID *string `json:"hire_booking_id"`
	SaleID        *string `json:"sale_id"`
	ServiceJobID  *string `json:"service_job_id"`
	CustomerID    *string `json:"customer_id"`

	Method   string  `json:"payment_method"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Status   string  `json:"status"`

	MpesaPhone         *string `json:"mpesa_phone"`
	MpesaCheckoutReqID *string `json:"mpesa_checkout_req_id"`
	MpesaReceiptNumber *string `json:"mpesa_receipt_number"`
	MpesaResultCode    *int    `json:"mpesa_result_code"`
	MpesaResultDesc    *string `json:"mpesa_result_desc"`

	Reference     *string `json:"reference"`
	FailureReason *string `json:"failure_reason"`

	PaidAt    *string `json:"paid_at"`
	CreatedBy *string `json:"created_by"`
	Notes     *string `json:"notes"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/payments[?status=&method=&customer_id=&hire_booking_id=&sale_id=&service_job_id=&from=&to=]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f ListFilters
	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	if v := q.Get("method"); v != "" {
		f.Method = &v
	}
	if v := q.Get("customer_id"); v != "" {
		f.CustomerID = &v
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

	payments, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]paymentDTO, 0, len(payments))
	for _, p := range payments {
		dtos = append(dtos, toDTO(p))
	}

	response.Success(w, http.StatusOK, "payments retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/payments/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "payment retrieved", toDTO(p), h.log)
}

// CreateManual godoc
// POST /api/v1/payments/manual
// Records a cash, bank transfer, or card payment as immediately completed.
func (h *Handler) CreateManual(w http.ResponseWriter, r *http.Request) {
	var req createManualRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	p, err := h.svc.CreateManual(r.Context(), CreateManualInput{
		HireBookingID: req.HireBookingID,
		SaleID:        req.SaleID,
		ServiceJobID:  req.ServiceJobID,
		CustomerID:    req.CustomerID,
		Method:        req.Method,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Reference:     req.Reference,
		Notes:         req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "payment recorded", toDTO(p), h.log)
}

// InitiateMpesa godoc
// POST /api/v1/payments/mpesa
// Triggers a Lipa Na M-PESA STK Push and returns a pending payment record.
func (h *Handler) InitiateMpesa(w http.ResponseWriter, r *http.Request) {
	var req initiateMpesaRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	p, err := h.svc.InitiateMpesa(r.Context(), InitiateMpesaInput{
		HireBookingID:    req.HireBookingID,
		SaleID:           req.SaleID,
		ServiceJobID:     req.ServiceJobID,
		CustomerID:       req.CustomerID,
		PhoneNumber:      req.PhoneNumber,
		Amount:           req.Amount,
		AccountReference: req.AccountReference,
		Description:      req.Description,
		Notes:            req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "M-PESA payment initiated", toDTO(p), h.log)
}

// MpesaCallback godoc
// POST /api/v1/payments/mpesa/callback
// Public endpoint — Safaricom POSTs the STK Push result here.
// No authentication required; verified by CheckoutRequestID lookup.
func (h *Handler) MpesaCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		h.log.Error("mpesa callback: failed to read body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := h.svc.HandleMpesaCallback(r.Context(), body); err != nil {
		h.log.Error("mpesa callback: processing error", "error", err)
		// Always return 200 to Safaricom — retry policy handles real errors.
		w.WriteHeader(http.StatusOK)
		return
	}

	// Safaricom expects a 200 with this specific body on success.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ResultCode":0,"ResultDesc":"Success"}`))
}

// Cancel godoc
// PATCH /api/v1/payments/{id}/cancel
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Cancel(r.Context(), r.PathValue("id"))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "payment cancelled", toDTO(p), h.log)
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func toDTO(p *Payment) paymentDTO {
	dto := paymentDTO{
		ID:                 p.ID,
		TenantID:           p.TenantID,
		HireBookingID:      p.HireBookingID,
		SaleID:             p.SaleID,
		ServiceJobID:       p.ServiceJobID,
		CustomerID:         p.CustomerID,
		Method:             string(p.Method),
		Amount:             p.Amount,
		Currency:           p.Currency,
		Status:             string(p.Status),
		MpesaPhone:         p.MpesaPhone,
		MpesaCheckoutReqID: p.MpesaCheckoutReqID,
		MpesaReceiptNumber: p.MpesaReceiptNumber,
		MpesaResultCode:    p.MpesaResultCode,
		MpesaResultDesc:    p.MpesaResultDesc,
		Reference:          p.Reference,
		FailureReason:      p.FailureReason,
		CreatedBy:          p.CreatedBy,
		Notes:              p.Notes,
		CreatedAt:          p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}

	if p.PaidAt != nil {
		s := p.PaidAt.UTC().Format("2006-01-02T15:04:05Z")
		dto.PaidAt = &s
	}

	return dto
}
