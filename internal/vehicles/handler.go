package vehicles

import (
	"log/slog"
	"net/http"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the vehicles domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type createVehicleRequest struct {
	Make        string   `json:"make"         validate:"required,min=1,max=100"`
	Model       string   `json:"model"        validate:"required,min=1,max=100"`
	Year        int      `json:"year"         validate:"required,gt=1899"`
	Color       *string  `json:"color"        validate:"omitempty,max=50"`
	VIN         *string  `json:"vin"          validate:"omitempty,max=50"`
	PlateNo     *string  `json:"plate_no"     validate:"omitempty,max=30"`
	VehicleType string   `json:"vehicle_type" validate:"required,oneof=sedan suv truck van bus pickup motorcycle other"`
	Status      string   `json:"status"       validate:"omitempty,oneof=available hired sold under_service inactive"`
	Mileage     *int     `json:"mileage"      validate:"omitempty,gte=0"`
	FuelType    *string  `json:"fuel_type"    validate:"omitempty,oneof=petrol diesel electric hybrid"`
	DailyRate   *float64 `json:"daily_rate"   validate:"omitempty,gte=0"`
	SalePrice   *float64 `json:"sale_price"   validate:"omitempty,gte=0"`
	Images      []string `json:"images"`
	Notes       *string  `json:"notes"        validate:"omitempty,max=2000"`
}

type updateVehicleRequest struct {
	Make        *string  `json:"make"         validate:"omitempty,min=1,max=100"`
	Model       *string  `json:"model"        validate:"omitempty,min=1,max=100"`
	Year        *int     `json:"year"         validate:"omitempty,gt=1899"`
	Color       *string  `json:"color"        validate:"omitempty,max=50"`
	VIN         *string  `json:"vin"          validate:"omitempty,max=50"`
	PlateNo     *string  `json:"plate_no"     validate:"omitempty,max=30"`
	VehicleType *string  `json:"vehicle_type" validate:"omitempty,oneof=sedan suv truck van bus pickup motorcycle other"`
	Status      *string  `json:"status"       validate:"omitempty,oneof=available hired sold under_service inactive"`
	Mileage     *int     `json:"mileage"      validate:"omitempty,gte=0"`
	FuelType    *string  `json:"fuel_type"    validate:"omitempty,oneof=petrol diesel electric hybrid"`
	DailyRate   *float64 `json:"daily_rate"   validate:"omitempty,gte=0"`
	SalePrice   *float64 `json:"sale_price"   validate:"omitempty,gte=0"`
	Images      []string `json:"images"`
	Notes       *string  `json:"notes"        validate:"omitempty,max=2000"`
}

// ─── Response DTO ─────────────────────────────────────────────────────────────

type vehicleDTO struct {
	ID          string   `json:"id"`
	TenantID    string   `json:"tenant_id"`
	Make        string   `json:"make"`
	Model       string   `json:"model"`
	Year        int      `json:"year"`
	Color       *string  `json:"color"`
	VIN         *string  `json:"vin"`
	PlateNo     *string  `json:"plate_no"`
	VehicleType string   `json:"vehicle_type"`
	Status      string   `json:"status"`
	Mileage     *int     `json:"mileage"`
	FuelType    *string  `json:"fuel_type"`
	DailyRate   *float64 `json:"daily_rate"`
	SalePrice   *float64 `json:"sale_price"`
	Images      []string `json:"images"`
	Notes       *string  `json:"notes"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// List godoc
// GET /api/v1/vehicles[?status=available&type=suv]
// Returns the tenant's vehicle inventory with optional filters.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f ListFilters
	if s := q.Get("status"); s != "" {
		f.Status = &s
	}
	if t := q.Get("type"); t != "" {
		f.VehicleType = &t
	}

	vehicles, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]vehicleDTO, 0, len(vehicles))
	for _, v := range vehicles {
		dtos = append(dtos, toDTO(v))
	}

	response.Success(w, http.StatusOK, "vehicles retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/vehicles/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	v, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "vehicle retrieved", toDTO(v), h.log)
}

// Create godoc
// POST /api/v1/vehicles
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createVehicleRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	status := req.Status
	if status == "" {
		status = string(VehicleStatusAvailable)
	}

	v, err := h.svc.Create(r.Context(), CreateInput{
		Make:        req.Make,
		Model:       req.Model,
		Year:        req.Year,
		Color:       req.Color,
		VIN:         req.VIN,
		PlateNo:     req.PlateNo,
		VehicleType: req.VehicleType,
		Status:      status,
		Mileage:     req.Mileage,
		FuelType:    req.FuelType,
		DailyRate:   req.DailyRate,
		SalePrice:   req.SalePrice,
		Images:      req.Images,
		Notes:       req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "vehicle created", toDTO(v), h.log)
}

// Update godoc
// PATCH /api/v1/vehicles/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateVehicleRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	v, err := h.svc.Update(r.Context(), id, UpdateInput{
		Make:        req.Make,
		Model:       req.Model,
		Year:        req.Year,
		Color:       req.Color,
		VIN:         req.VIN,
		PlateNo:     req.PlateNo,
		VehicleType: req.VehicleType,
		Status:      req.Status,
		Mileage:     req.Mileage,
		FuelType:    req.FuelType,
		DailyRate:   req.DailyRate,
		SalePrice:   req.SalePrice,
		Images:      req.Images,
		Notes:       req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "vehicle updated", toDTO(v), h.log)
}

// Delete godoc
// DELETE /api/v1/vehicles/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.svc.Delete(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toDTO(v *Vehicle) vehicleDTO {
	images := v.Images
	if images == nil {
		images = []string{}
	}

	return vehicleDTO{
		ID:          v.ID,
		TenantID:    v.TenantID,
		Make:        v.Make,
		Model:       v.Model,
		Year:        v.Year,
		Color:       v.Color,
		VIN:         v.VIN,
		PlateNo:     v.PlateNo,
		VehicleType: string(v.VehicleType),
		Status:      string(v.Status),
		Mileage:     v.Mileage,
		FuelType:    v.FuelType,
		DailyRate:   v.DailyRate,
		SalePrice:   v.SalePrice,
		Images:      images,
		Notes:       v.Notes,
		CreatedAt:   v.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   v.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
