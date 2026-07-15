package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the sales domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── ListFilters ───────────────────────────────────────────────────────────────

// ListFilters narrows the sales list query.  Zero/nil fields are ignored.
type ListFilters struct {
	Status     *string
	VehicleID  *string
	CustomerID *string
	FromDate   *time.Time // sale_date >= FromDate
	ToDate     *time.Time // sale_date <= ToDate
}

// ── Queries ───────────────────────────────────────────────────────────────────

const selectCols = `
	id, tenant_id, vehicle_id, customer_id,
	asking_price, agreed_price, deposit_amount, discount_amount, final_amount,
	payment_method, payment_terms,
	sale_date, handover_at,
	status,
	invoice_number, contract_ref,
	created_by, notes, created_at, updated_at`

// List returns sales for tenantID, optionally filtered.
func (r *Repository) List(ctx context.Context, tenantID string, f ListFilters) ([]*Sale, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.Status != nil {
		args = append(args, *f.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.VehicleID != nil {
		args = append(args, *f.VehicleID)
		conditions = append(conditions, fmt.Sprintf("vehicle_id = $%d", len(args)))
	}
	if f.CustomerID != nil {
		args = append(args, *f.CustomerID)
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conditions = append(conditions, fmt.Sprintf("sale_date >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conditions = append(conditions, fmt.Sprintf("sale_date <= $%d", len(args)))
	}

	q := `SELECT ` + selectCols + `
	      FROM   vehicle_sales
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY sale_date DESC, created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sales: list: %w", err)
	}
	defer rows.Close()

	var result []*Sale
	for rows.Next() {
		s, err := scanSale(rows)
		if err != nil {
			return nil, fmt.Errorf("sales: list scan: %w", err)
		}
		result = append(result, s)
	}

	return result, rows.Err()
}

// FindByID returns a sale by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Sale, error) {
	q := `SELECT ` + selectCols + `
	      FROM   vehicle_sales
	      WHERE  id = $1
	      LIMIT  1`

	s, err := scanSale(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("sales: find by id: %w", err)
	}

	return s, nil
}

// HasActiveSale reports whether the vehicle already has a non-cancelled /
// non-completed sale record (i.e. it is effectively reserved/pending for
// another buyer).
func (r *Repository) HasActiveSale(ctx context.Context, vehicleID string, excludeID *string) (bool, error) {
	args := []any{vehicleID}
	excludeClause := ""

	if excludeID != nil {
		args = append(args, *excludeID)
		excludeClause = fmt.Sprintf("AND id <> $%d", len(args))
	}

	q := fmt.Sprintf(`
		SELECT EXISTS (
		    SELECT 1 FROM vehicle_sales
		    WHERE  vehicle_id = $1
		    AND    status NOT IN ('cancelled', 'completed')
		    %s
		)`, excludeClause)

	var exists bool
	if err := r.db.QueryRow(ctx, q, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("sales: active sale check: %w", err)
	}

	return exists, nil
}

// Create inserts a new sale record and returns the persisted record.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Sale, error) {
	const q = `
		INSERT INTO vehicle_sales (
		    tenant_id, vehicle_id, customer_id,
		    asking_price, agreed_price, deposit_amount, discount_amount, final_amount,
		    payment_method, payment_terms,
		    sale_date,
		    status,
		    invoice_number, contract_ref,
		    created_by, notes
		) VALUES (
		    $1,  $2,  $3,
		    $4,  $5,  $6,  $7,  $8,
		    $9,  $10,
		    $11,
		    $12,
		    $13, $14,
		    $15, $16
		)
		RETURNING ` + selectCols

	s, err := scanSale(r.db.QueryRow(ctx, q,
		p.TenantID, p.VehicleID, p.CustomerID,
		p.AskingPrice, p.AgreedPrice, p.DepositAmount, p.DiscountAmount, p.FinalAmount,
		p.PaymentMethod, p.PaymentTerms,
		p.SaleDate,
		p.Status,
		p.InvoiceNumber, p.ContractRef,
		p.CreatedBy, p.Notes,
	))
	if err != nil {
		return nil, fmt.Errorf("sales: create: %w", err)
	}

	return s, nil
}

// Update applies a partial update to a sale record.
func (r *Repository) Update(ctx context.Context, id string, p UpdateParams) (*Sale, error) {
	const q = `
		UPDATE vehicle_sales
		SET    customer_id     = COALESCE($2,  customer_id),
		       asking_price    = COALESCE($3,  asking_price),
		       agreed_price    = COALESCE($4,  agreed_price),
		       deposit_amount  = COALESCE($5,  deposit_amount),
		       discount_amount = COALESCE($6,  discount_amount),
		       final_amount    = COALESCE($7,  final_amount),
		       payment_method  = COALESCE($8,  payment_method),
		       payment_terms   = COALESCE($9,  payment_terms),
		       sale_date       = COALESCE($10, sale_date),
		       handover_at     = COALESCE($11, handover_at),
		       status          = COALESCE($12, status),
		       invoice_number  = COALESCE($13, invoice_number),
		       contract_ref    = COALESCE($14, contract_ref),
		       notes           = COALESCE($15, notes),
		       updated_at      = NOW()
		WHERE  id = $1
		RETURNING ` + selectCols

	s, err := scanSale(r.db.QueryRow(ctx, q,
		id,
		p.CustomerID,
		p.AskingPrice, p.AgreedPrice, p.DepositAmount, p.DiscountAmount, p.FinalAmount,
		p.PaymentMethod, p.PaymentTerms,
		p.SaleDate,
		p.HandoverAt,
		p.Status,
		p.InvoiceNumber, p.ContractRef,
		p.Notes,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("sales: update: %w", err)
	}

	return s, nil
}

// Delete hard-deletes a sale record.
// Only pending or cancelled records should be deleted in practice.
func (r *Repository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM vehicle_sales WHERE id = $1`

	ct, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("sales: delete: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Params ────────────────────────────────────────────────────────────────────

// CreateParams holds all fields required to insert a new sale.
type CreateParams struct {
	TenantID   string
	VehicleID  string
	CustomerID string

	AskingPrice    float64
	AgreedPrice    float64
	DepositAmount  float64
	DiscountAmount float64
	FinalAmount    float64

	PaymentMethod *string
	PaymentTerms  *string

	SaleDate time.Time
	Status   SaleStatus

	InvoiceNumber *string
	ContractRef   *string

	CreatedBy *string
	Notes     *string
}

// UpdateParams holds nullable fields for a partial sale update.
type UpdateParams struct {
	CustomerID *string

	AskingPrice    *float64
	AgreedPrice    *float64
	DepositAmount  *float64
	DiscountAmount *float64
	FinalAmount    *float64

	PaymentMethod *string
	PaymentTerms  *string

	SaleDate   *time.Time
	HandoverAt *time.Time

	Status *SaleStatus

	InvoiceNumber *string
	ContractRef   *string

	Notes *string
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSale(row rowScanner) (*Sale, error) {
	var s Sale
	var status string

	err := row.Scan(
		&s.ID,
		&s.TenantID,
		&s.VehicleID,
		&s.CustomerID,
		&s.AskingPrice,
		&s.AgreedPrice,
		&s.DepositAmount,
		&s.DiscountAmount,
		&s.FinalAmount,
		&s.PaymentMethod,
		&s.PaymentTerms,
		&s.SaleDate,
		&s.HandoverAt,
		&status,
		&s.InvoiceNumber,
		&s.ContractRef,
		&s.CreatedBy,
		&s.Notes,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	s.Status = SaleStatus(status)
	return &s, nil
}
