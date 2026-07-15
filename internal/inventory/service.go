package inventory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"

	apperr "github.com/MauriceOmbewa/garisha-backend/internal/platform/errors"
	"github.com/MauriceOmbewa/garisha-backend/internal/platform/tenant"
)

// Service implements business logic for inventory management.
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService creates a Service.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ── Item inputs ───────────────────────────────────────────────────────────────

type CreateItemInput struct {
	Name          string
	SKU           *string
	Description   *string
	Category      *string
	Unit          string
	Quantity      float64  // opening stock
	ReorderLevel  float64
	ReorderQty    float64
	UnitCost      float64
	SellingPrice  float64
	SupplierName  *string
	SupplierPhone *string
	SupplierEmail *string
	Notes         *string
	CreatedBy     *string
}

type UpdateItemInput struct {
	Name          *string
	SKU           *string
	Description   *string
	Category      *string
	Unit          *string
	ReorderLevel  *float64
	ReorderQty    *float64
	UnitCost      *float64
	SellingPrice  *float64
	IsActive      *bool
	SupplierName  *string
	SupplierPhone *string
	SupplierEmail *string
	Notes         *string
}

// ── Movement inputs ───────────────────────────────────────────────────────────

// AdjustStockInput records a manual stock correction or supplier receipt.
type AdjustStockInput struct {
	ItemID           string
	Movement         string  // "adjustment" | "receipt"
	Quantity         float64 // positive for receipt; positive or negative for adjustment
	UnitCost         float64
	Reference        *string
	Notes            *string
	CreatedBy        *string
}

// RecordUsageInput records consumption of a part against a service job.
type RecordUsageInput struct {
	ItemID           string
	Quantity         float64 // positive — will be stored as negative movement
	ServiceJobID     *string
	ServiceJobItemID *string
	UnitCost         float64
	Notes            *string
	CreatedBy        *string
}

// ── Item service methods ──────────────────────────────────────────────────────

// ListItems returns inventory items for the tenant in ctx, optionally filtered.
func (s *Service) ListItems(ctx context.Context, f ListFilters) ([]*Item, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	items, err := s.repo.ListItems(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list inventory items", err)
	}

	return items, nil
}

// GetItem returns a single inventory item scoped to the tenant in ctx.
func (s *Service) GetItem(ctx context.Context, id string) (*Item, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	item, err := s.repo.FindItemByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get inventory item", err)
	}

	if item == nil || item.TenantID != tenantID {
		return nil, apperr.NotFound("inventory item")
	}

	return item, nil
}

// ListReorderAlerts returns active items at or below their reorder level.
func (s *Service) ListReorderAlerts(ctx context.Context) ([]*Item, error) {
	needs := true
	active := true

	return s.ListItems(ctx, ListFilters{
		NeedsReorder: &needs,
		IsActive:     &active,
	})
}

// CreateItem adds a new inventory item.  If opening stock > 0 a receipt
// movement is logged automatically for a complete audit trail.
func (s *Service) CreateItem(ctx context.Context, in CreateItemInput) (*Item, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if err := validateUnit(in.Unit); err != nil {
		return nil, err
	}

	if in.Quantity < 0 {
		return nil, apperr.BadRequest("opening quantity cannot be negative")
	}

	// Guard duplicate SKU within tenant.
	if in.SKU != nil && *in.SKU != "" {
		dup, err := s.repo.FindItemBySKU(ctx, tenantID, *in.SKU)
		if err != nil {
			return nil, apperr.Internal("failed to check for duplicate SKU", err)
		}

		if dup != nil {
			return nil, apperr.Conflict(fmt.Sprintf("an item with SKU %q already exists", *in.SKU))
		}
	}

	item, err := s.repo.CreateItem(ctx, CreateItemParams{
		TenantID:      tenantID,
		Name:          in.Name,
		SKU:           in.SKU,
		Description:   in.Description,
		Category:      in.Category,
		Unit:          in.Unit,
		Quantity:      0, // always start at 0; opening stock goes through movement
		ReorderLevel:  in.ReorderLevel,
		ReorderQty:    in.ReorderQty,
		UnitCost:      in.UnitCost,
		SellingPrice:  in.SellingPrice,
		SupplierName:  in.SupplierName,
		SupplierPhone: in.SupplierPhone,
		SupplierEmail: in.SupplierEmail,
		Notes:         in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to create inventory item", err)
	}

	// Log opening stock as a receipt movement so quantity updates via trigger.
	if in.Quantity > 0 {
		ref := "opening stock"
		if _, err := s.repo.RecordMovement(ctx, RecordMovementParams{
			TenantID:  tenantID,
			ItemID:    item.ID,
			Movement:  MovementReceipt,
			Quantity:  in.Quantity,
			UnitCost:  in.UnitCost,
			Reference: &ref,
			CreatedBy: in.CreatedBy,
		}); err != nil {
			s.log.Warn("failed to log opening stock movement",
				"item_id", item.ID, "error", err,
			)
		}

		// Reload to get the updated quantity from the trigger.
		item, err = s.repo.FindItemByID(ctx, item.ID)
		if err != nil || item == nil {
			return nil, apperr.Internal("failed to reload item after opening stock", err)
		}
	}

	s.log.Info("inventory item created",
		"item_id",   item.ID,
		"name",      item.Name,
		"tenant_id", tenantID,
	)

	return item, nil
}

// UpdateItem applies a partial update to an inventory item.
// Quantity is not directly mutable — use AdjustStock or RecordUsage.
func (s *Service) UpdateItem(ctx context.Context, id string, in UpdateItemInput) (*Item, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindItemByID(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to get inventory item", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return nil, apperr.NotFound("inventory item")
	}

	if in.Unit != nil {
		if err := validateUnit(*in.Unit); err != nil {
			return nil, err
		}
	}

	// Guard SKU rename collision.
	if in.SKU != nil && *in.SKU != "" && (existing.SKU == nil || *existing.SKU != *in.SKU) {
		dup, err := s.repo.FindItemBySKU(ctx, tenantID, *in.SKU)
		if err != nil {
			return nil, apperr.Internal("failed to check for duplicate SKU", err)
		}

		if dup != nil && dup.ID != id {
			return nil, apperr.Conflict(fmt.Sprintf("an item with SKU %q already exists", *in.SKU))
		}
	}

	item, err := s.repo.UpdateItem(ctx, id, UpdateItemParams{
		Name:          in.Name,
		SKU:           in.SKU,
		Description:   in.Description,
		Category:      in.Category,
		Unit:          in.Unit,
		ReorderLevel:  in.ReorderLevel,
		ReorderQty:    in.ReorderQty,
		UnitCost:      in.UnitCost,
		SellingPrice:  in.SellingPrice,
		IsActive:      in.IsActive,
		SupplierName:  in.SupplierName,
		SupplierPhone: in.SupplierPhone,
		SupplierEmail: in.SupplierEmail,
		Notes:         in.Notes,
	})
	if err != nil {
		return nil, apperr.Internal("failed to update inventory item", err)
	}

	if item == nil {
		return nil, apperr.NotFound("inventory item")
	}

	return item, nil
}

// DeleteItem hard-deletes an item.  Fails if it has usage history.
func (s *Service) DeleteItem(ctx context.Context, id string) error {
	tenantID := tenant.MustGetTenantID(ctx)

	existing, err := s.repo.FindItemByID(ctx, id)
	if err != nil {
		return apperr.Internal("failed to get inventory item", err)
	}

	if existing == nil || existing.TenantID != tenantID {
		return apperr.NotFound("inventory item")
	}

	if err := s.repo.DeleteItem(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("inventory item")
		}
		if isFKViolation(err) {
			return apperr.Conflict("cannot delete an item that has usage history; deactivate it instead")
		}
		return apperr.Internal("failed to delete inventory item", err)
	}

	s.log.Info("inventory item deleted", "item_id", id, "tenant_id", tenantID)
	return nil
}

// ── Movement service methods ──────────────────────────────────────────────────

// ListUsage returns stock movement records for the tenant in ctx.
func (s *Service) ListUsage(ctx context.Context, f UsageFilters) ([]*UsageRecord, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	records, err := s.repo.ListUsage(ctx, tenantID, f)
	if err != nil {
		return nil, apperr.Internal("failed to list usage records", err)
	}

	return records, nil
}

// AdjustStock records a manual stock-take correction or a supplier receipt.
// Positive quantity = stock coming in (receipt or positive correction).
// Negative quantity = write-off or negative correction.
func (s *Service) AdjustStock(ctx context.Context, in AdjustStockInput) (*UsageRecord, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if err := validateMovement(in.Movement); err != nil {
		return nil, err
	}

	if in.Movement == string(MovementUsage) {
		return nil, apperr.BadRequest("use the usage endpoint to record service consumption")
	}

	item, err := s.repo.FindItemByID(ctx, in.ItemID)
	if err != nil {
		return nil, apperr.Internal("failed to get inventory item", err)
	}

	if item == nil || item.TenantID != tenantID {
		return nil, apperr.NotFound("inventory item")
	}

	if in.Quantity == 0 {
		return nil, apperr.BadRequest("quantity must not be zero")
	}

	// For receipts, quantity must be positive.
	if in.Movement == string(MovementReceipt) && in.Quantity < 0 {
		return nil, apperr.BadRequest("receipt quantity must be positive")
	}

	// Guard against stock going negative.
	if item.Quantity+in.Quantity < 0 {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"adjustment would result in negative stock (current: %.3f)", item.Quantity,
		))
	}

	rec, err := s.repo.RecordMovement(ctx, RecordMovementParams{
		TenantID:  tenantID,
		ItemID:    in.ItemID,
		Movement:  MovementType(in.Movement),
		Quantity:  in.Quantity,
		UnitCost:  in.UnitCost,
		Reference: in.Reference,
		Notes:     in.Notes,
		CreatedBy: in.CreatedBy,
	})
	if err != nil {
		return nil, apperr.Internal("failed to record stock movement", err)
	}

	s.log.Info("stock adjusted",
		"item_id",  in.ItemID,
		"movement", in.Movement,
		"qty",      in.Quantity,
	)

	return rec, nil
}

// RecordUsage deducts stock consumed during a service job.
// Quantity is the amount used (positive); it is stored as a negative movement.
func (s *Service) RecordUsage(ctx context.Context, in RecordUsageInput) (*UsageRecord, error) {
	tenantID := tenant.MustGetTenantID(ctx)

	if in.Quantity <= 0 {
		return nil, apperr.BadRequest("usage quantity must be greater than 0")
	}

	item, err := s.repo.FindItemByID(ctx, in.ItemID)
	if err != nil {
		return nil, apperr.Internal("failed to get inventory item", err)
	}

	if item == nil || item.TenantID != tenantID {
		return nil, apperr.NotFound("inventory item")
	}

	if item.Quantity < in.Quantity {
		return nil, apperr.BadRequest(fmt.Sprintf(
			"insufficient stock: available %.3f, requested %.3f",
			item.Quantity, in.Quantity,
		))
	}

	rec, err := s.repo.RecordMovement(ctx, RecordMovementParams{
		TenantID:         tenantID,
		ItemID:           in.ItemID,
		Movement:         MovementUsage,
		Quantity:         -in.Quantity, // stored negative
		ServiceJobID:     in.ServiceJobID,
		ServiceJobItemID: in.ServiceJobItemID,
		UnitCost:         in.UnitCost,
		Notes:            in.Notes,
		CreatedBy:        in.CreatedBy,
	})
	if err != nil {
		return nil, apperr.Internal("failed to record usage", err)
	}

	s.log.Info("stock usage recorded",
		"item_id", in.ItemID,
		"qty",     in.Quantity,
	)

	return rec, nil
}

// ── Validators ────────────────────────────────────────────────────────────────

var validUnits = map[string]struct{}{
	string(UnitPiece): {}, string(UnitLitre): {}, string(UnitKg): {},
	string(UnitMetre): {}, string(UnitSet): {}, string(UnitBox): {},
	string(UnitOther): {},
}

func validateUnit(u string) error {
	if _, ok := validUnits[u]; !ok {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid unit %q — must be one of: piece, litre, kg, metre, set, box, other", u,
		))
	}
	return nil
}

var validMovements = map[string]struct{}{
	string(MovementUsage): {}, string(MovementAdjustment): {}, string(MovementReceipt): {},
}

func validateMovement(m string) error {
	if _, ok := validMovements[m]; !ok {
		return apperr.BadRequest(fmt.Sprintf(
			"invalid movement %q — must be one of: usage, adjustment, receipt", m,
		))
	}
	return nil
}

func isFKViolation(err error) bool {
	return err != nil && fmt.Sprintf("%v", err) != "" &&
		containsStr(fmt.Sprintf("%v", err), "23503")
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && strings.Contains(s, sub)
}
