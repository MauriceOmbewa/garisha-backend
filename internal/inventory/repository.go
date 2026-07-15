package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the inventory domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Column lists ──────────────────────────────────────────────────────────────

const itemCols = `
	id, tenant_id,
	name, sku, description, category, unit,
	quantity, reorder_level, reorder_qty,
	unit_cost, selling_price,
	is_active,
	supplier_name, supplier_phone, supplier_email,
	notes, created_at, updated_at`

const usageCols = `
	id, tenant_id, item_id,
	movement, quantity,
	service_job_id, service_job_item_id,
	unit_cost, reference, notes,
	created_by, created_at`

// ── Item filters ──────────────────────────────────────────────────────────────

// ListFilters narrows the inventory item list query.
type ListFilters struct {
	Category    *string
	IsActive    *bool
	NeedsReorder *bool   // true = only items at/below reorder level
	Search      *string  // partial match on name or SKU
}

// ── Item queries ──────────────────────────────────────────────────────────────

// ListItems returns inventory items for a tenant, optionally filtered.
func (r *Repository) ListItems(ctx context.Context, tenantID string, f ListFilters) ([]*Item, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.Category != nil {
		args = append(args, *f.Category)
		conditions = append(conditions, fmt.Sprintf("category = $%d", len(args)))
	}
	if f.IsActive != nil {
		args = append(args, *f.IsActive)
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", len(args)))
	}
	if f.NeedsReorder != nil && *f.NeedsReorder {
		conditions = append(conditions, "quantity <= reorder_level AND is_active = TRUE")
	}
	if f.Search != nil && *f.Search != "" {
		args = append(args, "%"+*f.Search+"%")
		n := len(args)
		conditions = append(conditions, fmt.Sprintf(
			"(name ILIKE $%d OR sku ILIKE $%d)", n, n,
		))
	}

	q := `SELECT ` + itemCols + `
	      FROM   inventory_items
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY name ASC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("inventory: list items: %w", err)
	}
	defer rows.Close()

	var result []*Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("inventory: list items scan: %w", err)
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

// FindItemByID returns an inventory item by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindItemByID(ctx context.Context, id string) (*Item, error) {
	q := `SELECT ` + itemCols + ` FROM inventory_items WHERE id = $1 LIMIT 1`

	item, err := scanItem(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("inventory: find item by id: %w", err)
	}

	return item, nil
}

// FindItemBySKU returns an item by SKU for a tenant.  Returns (nil, nil) when not found.
func (r *Repository) FindItemBySKU(ctx context.Context, tenantID, sku string) (*Item, error) {
	q := `SELECT ` + itemCols + `
	      FROM   inventory_items
	      WHERE  tenant_id = $1 AND sku = $2
	      LIMIT  1`

	item, err := scanItem(r.db.QueryRow(ctx, q, tenantID, sku))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("inventory: find item by sku: %w", err)
	}

	return item, nil
}

// CreateItem inserts a new inventory item.
func (r *Repository) CreateItem(ctx context.Context, p CreateItemParams) (*Item, error) {
	q := `
		INSERT INTO inventory_items (
		    tenant_id,
		    name, sku, description, category, unit,
		    quantity, reorder_level, reorder_qty,
		    unit_cost, selling_price,
		    supplier_name, supplier_phone, supplier_email,
		    notes
		) VALUES (
		    $1,
		    $2, $3, $4, $5, $6,
		    $7, $8, $9,
		    $10, $11,
		    $12, $13, $14,
		    $15
		)
		RETURNING ` + itemCols

	item, err := scanItem(r.db.QueryRow(ctx, q,
		p.TenantID,
		p.Name, p.SKU, p.Description, p.Category, p.Unit,
		p.Quantity, p.ReorderLevel, p.ReorderQty,
		p.UnitCost, p.SellingPrice,
		p.SupplierName, p.SupplierPhone, p.SupplierEmail,
		p.Notes,
	))
	if err != nil {
		return nil, fmt.Errorf("inventory: create item: %w", err)
	}

	return item, nil
}

// UpdateItem applies a partial update to an inventory item.
func (r *Repository) UpdateItem(ctx context.Context, id string, p UpdateItemParams) (*Item, error) {
	q := `
		UPDATE inventory_items
		SET    name           = COALESCE($2,  name),
		       sku            = COALESCE($3,  sku),
		       description    = COALESCE($4,  description),
		       category       = COALESCE($5,  category),
		       unit           = COALESCE($6,  unit),
		       reorder_level  = COALESCE($7,  reorder_level),
		       reorder_qty    = COALESCE($8,  reorder_qty),
		       unit_cost      = COALESCE($9,  unit_cost),
		       selling_price  = COALESCE($10, selling_price),
		       is_active      = COALESCE($11, is_active),
		       supplier_name  = COALESCE($12, supplier_name),
		       supplier_phone = COALESCE($13, supplier_phone),
		       supplier_email = COALESCE($14, supplier_email),
		       notes          = COALESCE($15, notes),
		       updated_at     = NOW()
		WHERE  id = $1
		RETURNING ` + itemCols

	item, err := scanItem(r.db.QueryRow(ctx, q,
		id,
		p.Name, p.SKU, p.Description, p.Category, p.Unit,
		p.ReorderLevel, p.ReorderQty,
		p.UnitCost, p.SellingPrice,
		p.IsActive,
		p.SupplierName, p.SupplierPhone, p.SupplierEmail,
		p.Notes,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("inventory: update item: %w", err)
	}

	return item, nil
}

// DeleteItem hard-deletes an inventory item.
func (r *Repository) DeleteItem(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM inventory_items WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("inventory: delete item: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Usage queries ─────────────────────────────────────────────────────────────

// UsageFilters narrows the usage records list query.
type UsageFilters struct {
	ItemID       *string
	Movement     *string
	ServiceJobID *string
	FromDate     *time.Time
	ToDate       *time.Time
}

// ListUsage returns usage records for a tenant, optionally filtered.
func (r *Repository) ListUsage(ctx context.Context, tenantID string, f UsageFilters) ([]*UsageRecord, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.ItemID != nil {
		args = append(args, *f.ItemID)
		conditions = append(conditions, fmt.Sprintf("item_id = $%d", len(args)))
	}
	if f.Movement != nil {
		args = append(args, *f.Movement)
		conditions = append(conditions, fmt.Sprintf("movement = $%d", len(args)))
	}
	if f.ServiceJobID != nil {
		args = append(args, *f.ServiceJobID)
		conditions = append(conditions, fmt.Sprintf("service_job_id = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}

	q := `SELECT ` + usageCols + `
	      FROM   inventory_usage
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("inventory: list usage: %w", err)
	}
	defer rows.Close()

	var result []*UsageRecord
	for rows.Next() {
		rec, err := scanUsage(rows)
		if err != nil {
			return nil, fmt.Errorf("inventory: list usage scan: %w", err)
		}
		result = append(result, rec)
	}

	return result, rows.Err()
}

// RecordMovement inserts a usage record.  The DB trigger
// (apply_inventory_movement) automatically updates inventory_items.quantity.
func (r *Repository) RecordMovement(ctx context.Context, p RecordMovementParams) (*UsageRecord, error) {
	q := `
		INSERT INTO inventory_usage (
		    tenant_id, item_id,
		    movement, quantity,
		    service_job_id, service_job_item_id,
		    unit_cost, reference, notes, created_by
		) VALUES (
		    $1, $2,
		    $3, $4,
		    $5, $6,
		    $7, $8, $9, $10
		)
		RETURNING ` + usageCols

	rec, err := scanUsage(r.db.QueryRow(ctx, q,
		p.TenantID, p.ItemID,
		p.Movement, p.Quantity,
		p.ServiceJobID, p.ServiceJobItemID,
		p.UnitCost, p.Reference, p.Notes, p.CreatedBy,
	))
	if err != nil {
		return nil, fmt.Errorf("inventory: record movement: %w", err)
	}

	return rec, nil
}

// ── Params ────────────────────────────────────────────────────────────────────

type CreateItemParams struct {
	TenantID      string
	Name          string
	SKU           *string
	Description   *string
	Category      *string
	Unit          string
	Quantity      float64
	ReorderLevel  float64
	ReorderQty    float64
	UnitCost      float64
	SellingPrice  float64
	SupplierName  *string
	SupplierPhone *string
	SupplierEmail *string
	Notes         *string
}

type UpdateItemParams struct {
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

type RecordMovementParams struct {
	TenantID         string
	ItemID           string
	Movement         MovementType
	Quantity         float64 // signed
	ServiceJobID     *string
	ServiceJobItemID *string
	UnitCost         float64
	Reference        *string
	Notes            *string
	CreatedBy        *string
}

// ── Scanners ──────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (*Item, error) {
	var item Item

	err := row.Scan(
		&item.ID, &item.TenantID,
		&item.Name, &item.SKU, &item.Description, &item.Category, &item.Unit,
		&item.Quantity, &item.ReorderLevel, &item.ReorderQty,
		&item.UnitCost, &item.SellingPrice,
		&item.IsActive,
		&item.SupplierName, &item.SupplierPhone, &item.SupplierEmail,
		&item.Notes, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &item, nil
}

func scanUsage(row rowScanner) (*UsageRecord, error) {
	var rec UsageRecord
	var movement string

	err := row.Scan(
		&rec.ID, &rec.TenantID, &rec.ItemID,
		&movement, &rec.Quantity,
		&rec.ServiceJobID, &rec.ServiceJobItemID,
		&rec.UnitCost, &rec.Reference, &rec.Notes,
		&rec.CreatedBy, &rec.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	rec.Movement = MovementType(movement)
	return &rec, nil
}
