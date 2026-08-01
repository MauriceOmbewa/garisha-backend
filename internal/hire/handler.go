package hire

import (
	"log/slog"
	"net/http"
	"time"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the hire domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type createBookingRequest struct {
	VehicleID  string  `json:"vehicle_id"   validate:"required,uuid4"`
	CustomerID string  `json:"customer_id"  validate:"required,uuid4"`
	StartDate  string  `json:"start_date"   validate:"required"` // "YYYY-MM-DD"
	EndDate    string  `json:"end_date"     validate:"required"` // "YYYY-MM-DD"
	PickupTime *string `json:"pickup_time"  validate:"omitempty"`
	ReturnTime *string `json:"return_time"  validate:"omitempty"`

	DailyRate      float64 `json:"daily_rate"       validate:"required,gt=0"`
	DepositAmount  float64 `json:"deposit_amount"   validate:"gte=0"`
	DiscountAmount float64 `json:"discount_amount"  validate:"gte=0"`

	PickupLocation *string `json:"pickup_location"  validate:"omitempty,max=255"`
	ReturnLocation *string `json:"return_location"  validate:"omitempty,max=255"`
	Notes          *string `json:"notes"            validate:"omitempty,max=2000"`
}

type updateBookingRequest struct {
	StartDate  *string  `json:"start_date"       validate:"omitempty"`
	EndDate    *string  `json:"end_date"         validate:"omitempty"`
	PickupTime *string  `json:"pickup_time"      validate:"omitempty"`
	ReturnTime *string  `json:"return_time"      validate:"omitempty"`
	ActualStart *string `json:"actual_start"     validate:"omitempty"`
	ActualEnd   *string `json:"actual_end"       validate:"omitempty"`

	DailyRate      *float64 `json:"daily_rate"       validate:"omitempty,gt=0"`
	DepositAmount  *float64 `json:"deposit_amount"   validate:"omitempty,gte=0"`
	DiscountAmount *float64 `json:"discount_amount"  validate:"omitempty,gte=0"`

	PickupLocation *string `json:"pickup_location"  validate:"omitempty,max=255"`
	ReturnLocation *string `json:"return_location"  validate:"omitempty,max=255"`
	MileageOut     *int    `json:"mileage_out"      validate:"omitempty,gte=0"`
	MileageIn      *int    `json:"mileage_in"       validate:"omitempty,gte=0"`
	Notes          *string `json:"notes"            validate:"omitempty,max=2000"`
}

type updateStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=confirmed active completed cancelled"`
}

type availabilityRequest struct {
	VehicleID string `json:"vehicle_id" validate:"required,uuid4"`
	StartDate string `json:"start_date" validate:"required"`
	EndDate   string `json:"end_date"   validate:"required"`
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type bookingDTO struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	VehicleID  string `json:"vehicle_id"`
	CustomerID string `json:"customer_id"`

	StartDate  string  `json:"start_date"`
	EndDate    string  `json:"end_date"`
	PickupTime *string `json:"pickup_time"`
	ReturnTime *string `json:"return_time"`

	ActualStart *string `json:"actual_start"`
	ActualEnd   *string `json:"actual_end"`

	DailyRate      float64 `json:"daily_rate"`
	TotalDays      int     `json:"total_days"`
	TotalAmount    float64 `json:"total_amount"`
	DepositAmount  float64 `json:"deposit_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`

	Status string `json:"status"`

	PickupLocation *string `json:"pickup_location"`
	ReturnLocation *string `json:"return_location"`
	MileageOut     *int    `json:"mileage_out"`
	MileageIn      *int    `json:"mileage_in"`

	CreatedBy *string `json:"created_by"`
	Notes     *string `json:"notes"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type availabilityDTO struct {
	VehicleID string `json:"vehicle_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Available bool   `json:"available"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/hire/bookings[?status=pending&vehicle_id=...&customer_id=...&from=YYYY-MM-DD&to=YYYY-MM-DD]
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
		t, err := time.Parse("2006-01-02", from)
		if err == nil {
			f.FromDate = &t
		}
	}
	if to := q.Get("to"); to != "" {
		t, err := time.Parse("2006-01-02", to)
		if err == nil {
			f.ToDate = &t
		}
	}

	bookings, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]bookingDTO, 0, len(bookings))
	for _, b := range bookings {
		dtos = append(dtos, toDTO(b))
	}

	response.Success(w, http.StatusOK, "bookings retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/hire/bookings/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	b, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "booking retrieved", toDTO(b), h.log)
}

type bookingEnrichedDTO struct {
	bookingDTO
	CustomerName string  `json:"customer_name"`
	VehicleMake  string  `json:"vehicle_make"`
	VehicleModel string  `json:"vehicle_model"`
	VehiclePlate *string `json:"vehicle_plate"`
	VehicleType  string  `json:"vehicle_type"`
}

// ListEnriched godoc
// GET /api/v1/hire/bookings/enriched[?status=&from=&to=]
// Returns bookings with customer name and vehicle details joined in.
func (h *Handler) ListEnriched(w http.ResponseWriter, r *http.Request) {
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
		t, err := time.Parse("2006-01-02", from)
		if err == nil {
			f.FromDate = &t
		}
	}
	if to := q.Get("to"); to != "" {
		t, err := time.Parse("2006-01-02", to)
		if err == nil {
			f.ToDate = &t
		}
	}

	bookings, err := h.svc.ListEnriched(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]bookingEnrichedDTO, 0, len(bookings))
	for _, b := range bookings {
		dtos = append(dtos, bookingEnrichedDTO{
			bookingDTO:   toDTO(&b.Booking),
			CustomerName: b.CustomerName,
			VehicleMake:  b.VehicleMake,
			VehicleModel: b.VehicleModel,
			VehiclePlate: b.VehiclePlate,
			VehicleType:  b.VehicleType,
		})
	}
	response.Success(w, http.StatusOK, "bookings retrieved", dtos, h.log)
}
// POST /api/v1/hire/availability
// Returns whether the vehicle is free for the given date range.
func (h *Handler) CheckAvailability(w http.ResponseWriter, r *http.Request) {
	var req availabilityRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	start, end, err := parseDateRange(req.StartDate, req.EndDate)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	available, err := h.svc.CheckAvailability(r.Context(), req.VehicleID, start, end)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "availability checked", availabilityDTO{
		VehicleID: req.VehicleID,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Available: available,
	}, h.log)
}

// Create godoc
// POST /api/v1/hire/bookings
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	start, end, err := parseDateRange(req.StartDate, req.EndDate)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	b, err := h.svc.Create(r.Context(), CreateInput{
		VehicleID:      req.VehicleID,
		CustomerID:     req.CustomerID,
		StartDate:      start,
		EndDate:        end,
		PickupTime:     req.PickupTime,
		ReturnTime:     req.ReturnTime,
		DailyRate:      req.DailyRate,
		DepositAmount:  req.DepositAmount,
		DiscountAmount: req.DiscountAmount,
		PickupLocation: req.PickupLocation,
		ReturnLocation: req.ReturnLocation,
		Notes:          req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "booking created", toDTO(b), h.log)
}

// Update godoc
// PATCH /api/v1/hire/bookings/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateBookingRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	in := UpdateInput{
		PickupTime:     req.PickupTime,
		ReturnTime:     req.ReturnTime,
		DailyRate:      req.DailyRate,
		DepositAmount:  req.DepositAmount,
		DiscountAmount: req.DiscountAmount,
		PickupLocation: req.PickupLocation,
		ReturnLocation: req.ReturnLocation,
		MileageOut:     req.MileageOut,
		MileageIn:      req.MileageIn,
		Notes:          req.Notes,
	}

	if req.StartDate != nil {
		t, err := time.Parse("2006-01-02", *req.StartDate)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid start_date format — expected YYYY-MM-DD"), h.log)
			return
		}
		in.StartDate = &t
	}

	if req.EndDate != nil {
		t, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid end_date format — expected YYYY-MM-DD"), h.log)
			return
		}
		in.EndDate = &t
	}

	if req.ActualStart != nil {
		t, err := time.Parse(time.RFC3339, *req.ActualStart)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid actual_start format — expected RFC3339"), h.log)
			return
		}
		in.ActualStart = &t
	}

	if req.ActualEnd != nil {
		t, err := time.Parse(time.RFC3339, *req.ActualEnd)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid actual_end format — expected RFC3339"), h.log)
			return
		}
		in.ActualEnd = &t
	}

	b, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "booking updated", toDTO(b), h.log)
}

// UpdateStatus godoc
// PATCH /api/v1/hire/bookings/{id}/status
// Transitions the booking through its lifecycle.
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateStatusRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	b, err := h.svc.UpdateStatus(r.Context(), id, BookingStatus(req.Status))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "booking status updated", toDTO(b), h.log)
}

// Delete godoc
// DELETE /api/v1/hire/bookings/{id}
// Only pending or cancelled bookings may be hard-deleted.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.svc.Delete(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toDTO(b *Booking) bookingDTO {
	dto := bookingDTO{
		ID:             b.ID,
		TenantID:       b.TenantID,
		VehicleID:      b.VehicleID,
		CustomerID:     b.CustomerID,
		StartDate:      b.StartDate.UTC().Format("2006-01-02"),
		EndDate:        b.EndDate.UTC().Format("2006-01-02"),
		PickupTime:     b.PickupTime,
		ReturnTime:     b.ReturnTime,
		DailyRate:      b.DailyRate,
		TotalDays:      b.TotalDays,
		TotalAmount:    b.TotalAmount,
		DepositAmount:  b.DepositAmount,
		DiscountAmount: b.DiscountAmount,
		FinalAmount:    b.FinalAmount,
		Status:         string(b.Status),
		PickupLocation: b.PickupLocation,
		ReturnLocation: b.ReturnLocation,
		MileageOut:     b.MileageOut,
		MileageIn:      b.MileageIn,
		CreatedBy:      b.CreatedBy,
		Notes:          b.Notes,
		CreatedAt:      b.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      b.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}

	if b.ActualStart != nil {
		s := b.ActualStart.UTC().Format("2006-01-02T15:04:05Z")
		dto.ActualStart = &s
	}

	if b.ActualEnd != nil {
		e := b.ActualEnd.UTC().Format("2006-01-02T15:04:05Z")
		dto.ActualEnd = &e
	}

	return dto
}

// parseDateRange parses two "YYYY-MM-DD" strings into time.Time values.
func parseDateRange(startStr, endStr string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return time.Time{}, time.Time{}, apperr.BadRequest("invalid start_date format — expected YYYY-MM-DD")
	}

	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return time.Time{}, time.Time{}, apperr.BadRequest("invalid end_date format — expected YYYY-MM-DD")
	}

	return start, end, nil
}
