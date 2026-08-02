package service

import (
	"log/slog"
	"net/http"
	"time"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the service domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Request DTOs ─────────────────────────────────────────────────────────────

type createJobRequest struct {
	VehicleID  string  `json:"vehicle_id"  validate:"required,uuid4"`
	CustomerID *string `json:"customer_id" validate:"omitempty,uuid4"`
	MechanicID *string `json:"mechanic_id" validate:"omitempty,uuid4"`
	JobType    string  `json:"job_type"    validate:"required,oneof=general repair maintenance inspection bodywork electrical other"`
	ReceivedAt *string `json:"received_at" validate:"omitempty"` // RFC3339
	DueDate    *string `json:"due_date"    validate:"omitempty"` // YYYY-MM-DD
	MileageIn  *int    `json:"mileage_in"  validate:"omitempty,gte=0"`
	CustomerNotes *string `json:"customer_notes" validate:"omitempty,max=2000"`
	InternalNotes *string `json:"internal_notes" validate:"omitempty,max=2000"`
}

type updateJobRequest struct {
	CustomerID    *string  `json:"customer_id"     validate:"omitempty,uuid4"`
	MechanicID    *string  `json:"mechanic_id"     validate:"omitempty,uuid4"`
	JobType       *string  `json:"job_type"        validate:"omitempty,oneof=general repair maintenance inspection bodywork electrical other"`
	DueDate       *string  `json:"due_date"        validate:"omitempty"` // YYYY-MM-DD
	MileageIn     *int     `json:"mileage_in"      validate:"omitempty,gte=0"`
	DiscountAmount *float64 `json:"discount_amount" validate:"omitempty,gte=0"`
	CustomerNotes *string  `json:"customer_notes"  validate:"omitempty,max=2000"`
	InternalNotes *string  `json:"internal_notes"  validate:"omitempty,max=2000"`
}

type updateStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=in_progress awaiting_parts completed cancelled"`
}

type assignMechanicRequest struct {
	MechanicID string `json:"mechanic_id" validate:"required,uuid4"`
}

type addItemRequest struct {
	ItemType    string  `json:"item_type"    validate:"required,oneof=labour part consumable other"`
	Description string  `json:"description"  validate:"required,min=1,max=500"`
	Quantity    float64 `json:"quantity"     validate:"required,gt=0"`
	UnitPrice   float64 `json:"unit_price"   validate:"gte=0"`
	PartNumber  *string `json:"part_number"  validate:"omitempty,max=100"`
}

type updateItemRequest struct {
	ItemType    *string  `json:"item_type"    validate:"omitempty,oneof=labour part consumable other"`
	Description *string  `json:"description"  validate:"omitempty,min=1,max=500"`
	Quantity    *float64 `json:"quantity"     validate:"omitempty,gt=0"`
	UnitPrice   *float64 `json:"unit_price"   validate:"omitempty,gte=0"`
	PartNumber  *string  `json:"part_number"  validate:"omitempty,max=100"`
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

type jobDTO struct {
	ID         string  `json:"id"`
	TenantID   string  `json:"tenant_id"`
	VehicleID  string  `json:"vehicle_id"`
	CustomerID *string `json:"customer_id"`
	MechanicID *string `json:"mechanic_id"`

	JobType string `json:"job_type"`
	Status  string `json:"status"`

	ReceivedAt  string  `json:"received_at"`
	DueDate     *string `json:"due_date"`
	CompletedAt *string `json:"completed_at"`

	MileageIn *int `json:"mileage_in"`

	LabourTotal    float64 `json:"labour_total"`
	PartsTotal     float64 `json:"parts_total"`
	TotalAmount    float64 `json:"total_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`

	CreatedBy     *string `json:"created_by"`
	CustomerNotes *string `json:"customer_notes"`
	InternalNotes *string `json:"internal_notes"`

	Items []itemDTO `json:"items"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type itemDTO struct {
	ID          string  `json:"id"`
	JobID       string  `json:"job_id"`
	ItemType    string  `json:"item_type"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
	PartNumber  *string `json:"part_number"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type jobEnrichedDTO struct {
	jobDTO
	CustomerName string  `json:"customer_name"`
	VehicleMake  string  `json:"vehicle_make"`
	VehicleModel string  `json:"vehicle_model"`
	VehiclePlate *string `json:"vehicle_plate"`
	VehicleType  string  `json:"vehicle_type"`
}

// ListEnriched godoc
// GET /api/v1/service/jobs/enriched
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
	if m := q.Get("mechanic_id"); m != "" {
		f.MechanicID = &m
	}
	if jt := q.Get("job_type"); jt != "" {
		f.JobType = &jt
	}
	if from := q.Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			f.FromDate = &t
		}
	}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			f.ToDate = &t
		}
	}

	jobs, err := h.svc.ListEnriched(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]jobEnrichedDTO, 0, len(jobs))
	for _, j := range jobs {
		dtos = append(dtos, jobEnrichedDTO{
			jobDTO:       toJobDTO(&j.Job),
			CustomerName: j.CustomerName,
			VehicleMake:  j.VehicleMake,
			VehicleModel: j.VehicleModel,
			VehiclePlate: j.VehiclePlate,
			VehicleType:  j.VehicleType,
		})
	}
	response.Success(w, http.StatusOK, "service jobs retrieved", dtos, h.log)
}
// GET /api/v1/service/jobs[?status=pending&vehicle_id=...&customer_id=...&mechanic_id=...&job_type=...&from=...&to=...]
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
	if m := q.Get("mechanic_id"); m != "" {
		f.MechanicID = &m
	}
	if jt := q.Get("job_type"); jt != "" {
		f.JobType = &jt
	}
	if from := q.Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			f.FromDate = &t
		}
	}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			f.ToDate = &t
		}
	}

	jobs, err := h.svc.List(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]jobDTO, 0, len(jobs))
	for _, j := range jobs {
		dtos = append(dtos, toJobDTO(j))
	}

	response.Success(w, http.StatusOK, "service jobs retrieved", dtos, h.log)
}

// Get godoc
// GET /api/v1/service/jobs/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	j, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "service job retrieved", toJobDTO(j), h.log)
}

// Create godoc
// POST /api/v1/service/jobs
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	in := CreateJobInput{
		VehicleID:     req.VehicleID,
		CustomerID:    req.CustomerID,
		MechanicID:    req.MechanicID,
		JobType:       req.JobType,
		MileageIn:     req.MileageIn,
		CustomerNotes: req.CustomerNotes,
		InternalNotes: req.InternalNotes,
	}

	if req.ReceivedAt != nil && *req.ReceivedAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ReceivedAt)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid received_at — expected RFC3339"), h.log)
			return
		}
		in.ReceivedAt = &t
	}

	if req.DueDate != nil && *req.DueDate != "" {
		t, err := time.Parse("2006-01-02", *req.DueDate)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid due_date — expected YYYY-MM-DD"), h.log)
			return
		}
		in.DueDate = &t
	}

	j, err := h.svc.CreateJob(r.Context(), in)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "service job created", toJobDTO(j), h.log)
}

// Update godoc
// PATCH /api/v1/service/jobs/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateJobRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	in := UpdateJobInput{
		CustomerID:    req.CustomerID,
		MechanicID:    req.MechanicID,
		JobType:       req.JobType,
		MileageIn:     req.MileageIn,
		DiscountAmount: req.DiscountAmount,
		CustomerNotes: req.CustomerNotes,
		InternalNotes: req.InternalNotes,
	}

	if req.DueDate != nil && *req.DueDate != "" {
		t, err := time.Parse("2006-01-02", *req.DueDate)
		if err != nil {
			apperr.Handle(w, r, apperr.BadRequest("invalid due_date — expected YYYY-MM-DD"), h.log)
			return
		}
		in.DueDate = &t
	}

	j, err := h.svc.UpdateJob(r.Context(), id, in)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "service job updated", toJobDTO(j), h.log)
}

// UpdateStatus godoc
// PATCH /api/v1/service/jobs/{id}/status
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateStatusRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	j, err := h.svc.UpdateStatus(r.Context(), id, JobStatus(req.Status))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "service job status updated", toJobDTO(j), h.log)
}

// AssignMechanic godoc
// PATCH /api/v1/service/jobs/{id}/mechanic
func (h *Handler) AssignMechanic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req assignMechanicRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	j, err := h.svc.AssignMechanic(r.Context(), id, req.MechanicID)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "mechanic assigned", toJobDTO(j), h.log)
}

// Delete godoc
// DELETE /api/v1/service/jobs/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.svc.DeleteJob(r.Context(), id); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Item handlers ────────────────────────────────────────────────────────────

// ListItems godoc
// GET /api/v1/service/jobs/{id}/items
func (h *Handler) ListItems(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	// Reuse GetByID which loads items.
	j, err := h.svc.GetByID(r.Context(), jobID)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]itemDTO, 0, len(j.Items))
	for _, item := range j.Items {
		dtos = append(dtos, toItemDTO(item))
	}

	response.Success(w, http.StatusOK, "job items retrieved", dtos, h.log)
}

// AddItem godoc
// POST /api/v1/service/jobs/{id}/items
func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	var req addItemRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	item, err := h.svc.AddItem(r.Context(), jobID, AddItemInput{
		ItemType:    req.ItemType,
		Description: req.Description,
		Quantity:    req.Quantity,
		UnitPrice:   req.UnitPrice,
		PartNumber:  req.PartNumber,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "item added", toItemDTO(item), h.log)
}

// UpdateItem godoc
// PATCH /api/v1/service/jobs/{id}/items/{item_id}
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	jobID  := r.PathValue("id")
	itemID := r.PathValue("item_id")

	var req updateItemRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	item, err := h.svc.UpdateItem(r.Context(), jobID, itemID, UpdateItemInput{
		ItemType:    req.ItemType,
		Description: req.Description,
		Quantity:    req.Quantity,
		UnitPrice:   req.UnitPrice,
		PartNumber:  req.PartNumber,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "item updated", toItemDTO(item), h.log)
}

// DeleteItem godoc
// DELETE /api/v1/service/jobs/{id}/items/{item_id}
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	jobID  := r.PathValue("id")
	itemID := r.PathValue("item_id")

	if err := h.svc.DeleteItem(r.Context(), jobID, itemID); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toJobDTO(j *Job) jobDTO {
	dto := jobDTO{
		ID:             j.ID,
		TenantID:       j.TenantID,
		VehicleID:      j.VehicleID,
		CustomerID:     j.CustomerID,
		MechanicID:     j.MechanicID,
		JobType:        string(j.JobType),
		Status:         string(j.Status),
		ReceivedAt:     j.ReceivedAt.UTC().Format("2006-01-02T15:04:05Z"),
		MileageIn:      j.MileageIn,
		LabourTotal:    j.LabourTotal,
		PartsTotal:     j.PartsTotal,
		TotalAmount:    j.TotalAmount,
		DiscountAmount: j.DiscountAmount,
		FinalAmount:    j.FinalAmount,
		CreatedBy:      j.CreatedBy,
		CustomerNotes:  j.CustomerNotes,
		InternalNotes:  j.InternalNotes,
		CreatedAt:      j.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      j.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}

	if j.DueDate != nil {
		s := j.DueDate.UTC().Format("2006-01-02")
		dto.DueDate = &s
	}

	if j.CompletedAt != nil {
		s := j.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
		dto.CompletedAt = &s
	}

	dto.Items = make([]itemDTO, 0, len(j.Items))
	for _, item := range j.Items {
		dto.Items = append(dto.Items, toItemDTO(item))
	}

	return dto
}

func toItemDTO(item *JobItem) itemDTO {
	return itemDTO{
		ID:          item.ID,
		JobID:       item.JobID,
		ItemType:    string(item.ItemType),
		Description: item.Description,
		Quantity:    item.Quantity,
		UnitPrice:   item.UnitPrice,
		TotalPrice:  item.TotalPrice,
		PartNumber:  item.PartNumber,
		CreatedAt:   item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
