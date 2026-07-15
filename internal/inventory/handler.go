package inventory

import (
	"log/slog"
	"net/http"
	"time"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/response"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/validation"
)

// Handler holds the HTTP handlers for the inventory domain.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ─── Item request DTOs ────────────────────────────────────────────────────────

type createItemRequest struct {
	Name          string   `json:"name"           validate:"required,min=1,max=255"`
	SKU           *string  `json:"sku"            validate:"omitempty,max=100"`
	Description   *string  `json:"description"    validate:"omitempty,max=1000"`
	Category      *string  `json:"category"       validate:"omitempty,max=100"`
	Unit          string   `json:"unit"           validate:"required,oneof=piece litre kg metre set box other"`
	Quantity      float64  `json:"quantity"       validate:"gte=0"`
	ReorderLevel  float64  `json:"reorder_level"  validate:"gte=0"`
	ReorderQty    float64  `json:"reorder_qty"    validate:"gte=0"`
	UnitCost      float64  `json:"unit_cost"      validate:"gte=0"`
	SellingPrice  float64  `json:"selling_price"  validate:"gte=0"`
	SupplierName  *string  `json:"supplier_name"  validate:"omitempty,max=255"`
	SupplierPhone *string  `json:"supplier_phone" validate:"omitempty,max=50"`
	SupplierEmail *string  `json:"supplier_email" validate:"omitempty,email"`
	Notes         *string  `json:"notes"          validate:"omitempty,max=2000"`
}

type updateItemRequest struct {
	Name          *string  `json:"name"           validate:"omitempty,min=1,max=255"`
	SKU           *string  `json:"sku"            validate:"omitempty,max=100"`
	Description   *string  `json:"description"    validate:"omitempty,max=1000"`
	Category      *string  `json:"category"       validate:"omitempty,max=100"`
	Unit          *string  `json:"unit"           validate:"omitempty,oneof=piece litre kg metre set box other"`
	ReorderLevel  *float64 `json:"reorder_level"  validate:"omitempty,gte=0"`
	ReorderQty    *float64 `json:"reorder_qty"    validate:"omitempty,gte=0"`
	UnitCost      *float64 `json:"unit_cost"      validate:"omitempty,gte=0"`
	SellingPrice  *float64 `json:"selling_price"  validate:"omitempty,gte=0"`
	IsActive      *bool    `json:"is_active"`
	SupplierName  *string  `json:"supplier_name"  validate:"omitempty,max=255"`
	SupplierPhone *string  `json:"supplier_phone" validate:"omitempty,max=50"`
	SupplierEmail *string  `json:"supplier_email" validate:"omitempty,email"`
	Notes         *string  `json:"notes"          validate:"omitempty,max=2000"`
}

// ─── Movement request DTOs ────────────────────────────────────────────────────

type adjustStockRequest struct {
	Movement  string  `json:"movement"   validate:"required,oneof=adjustment receipt"`
	Quantity  float64 `json:"quantity"   validate:"required"`
	UnitCost  float64 `json:"unit_cost"  validate:"gte=0"`
	Reference *string `json:"reference"  validate:"omitempty,max=200"`
	Notes     *string `json:"notes"      validate:"omitempty,max=2000"`
}

type recordUsageRequest struct {
	ItemID           string  `json:"item_id"            validate:"required,uuid4"`
	Quantity         float64 `json:"quantity"           validate:"required,gt=0"`
	ServiceJobID     *string `json:"service_job_id"     validate:"omitempty,uuid4"`
	ServiceJobItemID *string `json:"service_job_item_id" validate:"omitempty,uuid4"`
	UnitCost         float64 `json:"unit_cost"          validate:"gte=0"`
	Notes            *string `json:"notes"              validate:"omitempty,max=2000"`
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

type itemDTO struct {
	ID           string   `json:"id"`
	TenantID     string   `json:"tenant_id"`
	Name         string   `json:"name"`
	SKU          *string  `json:"sku"`
	Description  *string  `json:"description"`
	Category     *string  `json:"category"`
	Unit         string   `json:"unit"`
	Quantity     float64  `json:"quantity"`
	ReorderLevel float64  `json:"reorder_level"`
	ReorderQty   float64  `json:"reorder_qty"`
	NeedsReorder bool     `json:"needs_reorder"`
	UnitCost     float64  `json:"unit_cost"`
	SellingPrice float64  `json:"selling_price"`
	IsActive     bool     `json:"is_active"`
	SupplierName  *string `json:"supplier_name"`
	SupplierPhone *string `json:"supplier_phone"`
	SupplierEmail *string `json:"supplier_email"`
	Notes        *string  `json:"notes"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type usageDTO struct {
	ID               string  `json:"id"`
	TenantID         string  `json:"tenant_id"`
	ItemID           string  `json:"item_id"`
	Movement         string  `json:"movement"`
	Quantity         float64 `json:"quantity"`
	ServiceJobID     *string `json:"service_job_id"`
	ServiceJobItemID *string `json:"service_job_item_id"`
	UnitCost         float64 `json:"unit_cost"`
	Reference        *string `json:"reference"`
	Notes            *string `json:"notes"`
	CreatedBy        *string `json:"created_by"`
	CreatedAt        string  `json:"created_at"`
}

// ─── Item handlers ────────────────────────────────────────────────────────────

// ListItems godoc
// GET /api/v1/inventory/items[?category=&is_active=&needs_reorder=true&search=]
func (h *Handler) ListItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f ListFilters
	if v := q.Get("category"); v != "" {
		f.Category = &v
	}
	if v := q.Get("search"); v != "" {
		f.Search = &v
	}
	if v := q.Get("is_active"); v != "" {
		b := v == "true"
		f.IsActive = &b
	}
	if v := q.Get("needs_reorder"); v == "true" {
		t := true
		f.NeedsReorder = &t
	}

	items, err := h.svc.ListItems(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]itemDTO, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, toItemDTO(item))
	}

	response.Success(w, http.StatusOK, "inventory items retrieved", dtos, h.log)
}

// GetItem godoc
// GET /api/v1/inventory/items/{id}
func (h *Handler) GetItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.svc.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "inventory item retrieved", toItemDTO(item), h.log)
}

// ReorderAlerts godoc
// GET /api/v1/inventory/reorder-alerts
func (h *Handler) ReorderAlerts(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListReorderAlerts(r.Context())
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]itemDTO, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, toItemDTO(item))
	}

	response.Success(w, http.StatusOK, "reorder alerts retrieved", dtos, h.log)
}

// CreateItem godoc
// POST /api/v1/inventory/items
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	var req createItemRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	item, err := h.svc.CreateItem(r.Context(), CreateItemInput{
		Name:          req.Name,
		SKU:           req.SKU,
		Description:   req.Description,
		Category:      req.Category,
		Unit:          req.Unit,
		Quantity:      req.Quantity,
		ReorderLevel:  req.ReorderLevel,
		ReorderQty:    req.ReorderQty,
		UnitCost:      req.UnitCost,
		SellingPrice:  req.SellingPrice,
		SupplierName:  req.SupplierName,
		SupplierPhone: req.SupplierPhone,
		SupplierEmail: req.SupplierEmail,
		Notes:         req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "inventory item created", toItemDTO(item), h.log)
}

// UpdateItem godoc
// PATCH /api/v1/inventory/items/{id}
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	var req updateItemRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	item, err := h.svc.UpdateItem(r.Context(), r.PathValue("id"), UpdateItemInput{
		Name:          req.Name,
		SKU:           req.SKU,
		Description:   req.Description,
		Category:      req.Category,
		Unit:          req.Unit,
		ReorderLevel:  req.ReorderLevel,
		ReorderQty:    req.ReorderQty,
		UnitCost:      req.UnitCost,
		SellingPrice:  req.SellingPrice,
		IsActive:      req.IsActive,
		SupplierName:  req.SupplierName,
		SupplierPhone: req.SupplierPhone,
		SupplierEmail: req.SupplierEmail,
		Notes:         req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusOK, "inventory item updated", toItemDTO(item), h.log)
}

// DeleteItem godoc
// DELETE /api/v1/inventory/items/{id}
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteItem(r.Context(), r.PathValue("id")); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.NoContent(w)
}

// ─── Movement handlers ────────────────────────────────────────────────────────

// ListUsage godoc
// GET /api/v1/inventory/usage[?item_id=&movement=&service_job_id=&from=&to=]
func (h *Handler) ListUsage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var f UsageFilters
	if v := q.Get("item_id"); v != "" {
		f.ItemID = &v
	}
	if v := q.Get("movement"); v != "" {
		f.Movement = &v
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

	records, err := h.svc.ListUsage(r.Context(), f)
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	dtos := make([]usageDTO, 0, len(records))
	for _, rec := range records {
		dtos = append(dtos, toUsageDTO(rec))
	}

	response.Success(w, http.StatusOK, "usage records retrieved", dtos, h.log)
}

// AdjustStock godoc
// POST /api/v1/inventory/items/{id}/adjust
func (h *Handler) AdjustStock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req adjustStockRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	rec, err := h.svc.AdjustStock(r.Context(), AdjustStockInput{
		ItemID:    id,
		Movement:  req.Movement,
		Quantity:  req.Quantity,
		UnitCost:  req.UnitCost,
		Reference: req.Reference,
		Notes:     req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "stock adjusted", toUsageDTO(rec), h.log)
}

// RecordUsage godoc
// POST /api/v1/inventory/usage
func (h *Handler) RecordUsage(w http.ResponseWriter, r *http.Request) {
	var req recordUsageRequest
	if err := validation.DecodeJSON(r, &req); err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	rec, err := h.svc.RecordUsage(r.Context(), RecordUsageInput{
		ItemID:           req.ItemID,
		Quantity:         req.Quantity,
		ServiceJobID:     req.ServiceJobID,
		ServiceJobItemID: req.ServiceJobItemID,
		UnitCost:         req.UnitCost,
		Notes:            req.Notes,
	})
	if err != nil {
		apperr.Handle(w, r, err, h.log)
		return
	}

	response.Success(w, http.StatusCreated, "usage recorded", toUsageDTO(rec), h.log)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toItemDTO(item *Item) itemDTO {
	return itemDTO{
		ID:            item.ID,
		TenantID:      item.TenantID,
		Name:          item.Name,
		SKU:           item.SKU,
		Description:   item.Description,
		Category:      item.Category,
		Unit:          item.Unit,
		Quantity:      item.Quantity,
		ReorderLevel:  item.ReorderLevel,
		ReorderQty:    item.ReorderQty,
		NeedsReorder:  item.NeedsReorder(),
		UnitCost:      item.UnitCost,
		SellingPrice:  item.SellingPrice,
		IsActive:      item.IsActive,
		SupplierName:  item.SupplierName,
		SupplierPhone: item.SupplierPhone,
		SupplierEmail: item.SupplierEmail,
		Notes:         item.Notes,
		CreatedAt:     item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toUsageDTO(rec *UsageRecord) usageDTO {
	return usageDTO{
		ID:               rec.ID,
		TenantID:         rec.TenantID,
		ItemID:           rec.ItemID,
		Movement:         string(rec.Movement),
		Quantity:         rec.Quantity,
		ServiceJobID:     rec.ServiceJobID,
		ServiceJobItemID: rec.ServiceJobItemID,
		UnitCost:         rec.UnitCost,
		Reference:        rec.Reference,
		Notes:            rec.Notes,
		CreatedBy:        rec.CreatedBy,
		CreatedAt:        rec.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
