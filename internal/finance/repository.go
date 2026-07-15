package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the finance domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Column constants ──────────────────────────────────────────────────────────

const categoryCols = `
	id, tenant_id, name, type, description, is_active, created_at, updated_at`

const recordCols = `
	id, tenant_id, category_id,
	type, amount,
	hire_booking_id, sale_id, service_job_id,
	description, transaction_date,
	payment_method, reference,
	created_by, notes,
	created_at, updated_at`

// ── Category queries ──────────────────────────────────────────────────────────

// ListCategories returns all categories for a tenant, optionally filtered by type.
func (r *Repository) ListCategories(ctx context.Context, tenantID string, entryType *string) ([]*Category, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if entryType != nil {
		args = append(args, *entryType)
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)))
	}

	q := `SELECT ` + categoryCols + `
	      FROM   finance_categories
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY type, name`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("finance: list categories: %w", err)
	}
	defer rows.Close()

	var result []*Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("finance: list categories scan: %w", err)
		}
		result = append(result, c)
	}

	return result, rows.Err()
}

// FindCategoryByID returns a category by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindCategoryByID(ctx context.Context, id string) (*Category, error) {
	q := `SELECT ` + categoryCols + ` FROM finance_categories WHERE id = $1 LIMIT 1`

	c, err := scanCategory(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finance: find category by id: %w", err)
	}

	return c, nil
}

// FindCategoryByName returns a category matching name+type for a tenant.
// Returns (nil, nil) when not found.
func (r *Repository) FindCategoryByName(ctx context.Context, tenantID, name, entryType string) (*Category, error) {
	q := `SELECT ` + categoryCols + `
	      FROM   finance_categories
	      WHERE  tenant_id = $1 AND name = $2 AND type = $3
	      LIMIT  1`

	c, err := scanCategory(r.db.QueryRow(ctx, q, tenantID, name, entryType))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finance: find category by name: %w", err)
	}

	return c, nil
}

// CreateCategory inserts a new finance category.
func (r *Repository) CreateCategory(ctx context.Context, p CreateCategoryParams) (*Category, error) {
	q := `INSERT INTO finance_categories (tenant_id, name, type, description)
	      VALUES ($1, $2, $3, $4)
	      RETURNING ` + categoryCols

	c, err := scanCategory(r.db.QueryRow(ctx, q, p.TenantID, p.Name, p.Type, p.Description))
	if err != nil {
		return nil, fmt.Errorf("finance: create category: %w", err)
	}

	return c, nil
}

// UpdateCategory applies a partial update to a category.
func (r *Repository) UpdateCategory(ctx context.Context, id string, p UpdateCategoryParams) (*Category, error) {
	q := `UPDATE finance_categories
	      SET    name        = COALESCE($2, name),
	             description = COALESCE($3, description),
	             is_active   = COALESCE($4, is_active),
	             updated_at  = NOW()
	      WHERE  id = $1
	      RETURNING ` + categoryCols

	c, err := scanCategory(r.db.QueryRow(ctx, q, id, p.Name, p.Description, p.IsActive))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finance: update category: %w", err)
	}

	return c, nil
}

// DeleteCategory hard-deletes a category.
func (r *Repository) DeleteCategory(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM finance_categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("finance: delete category: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Record filters ────────────────────────────────────────────────────────────

// RecordFilters narrows the finance records list query.
type RecordFilters struct {
	Type           *string    // 'income' | 'expense'
	CategoryID     *string
	FromDate       *time.Time // transaction_date >= FromDate
	ToDate         *time.Time // transaction_date <= ToDate
	PaymentMethod  *string
	HireBookingID  *string
	SaleID         *string
	ServiceJobID   *string
}

// ── Record queries ────────────────────────────────────────────────────────────

// ListRecords returns finance records for a tenant, optionally filtered.
func (r *Repository) ListRecords(ctx context.Context, tenantID string, f RecordFilters) ([]*Record, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.Type != nil {
		args = append(args, *f.Type)
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)))
	}
	if f.CategoryID != nil {
		args = append(args, *f.CategoryID)
		conditions = append(conditions, fmt.Sprintf("category_id = $%d", len(args)))
	}
	if f.FromDate != nil {
		args = append(args, *f.FromDate)
		conditions = append(conditions, fmt.Sprintf("transaction_date >= $%d", len(args)))
	}
	if f.ToDate != nil {
		args = append(args, *f.ToDate)
		conditions = append(conditions, fmt.Sprintf("transaction_date <= $%d", len(args)))
	}
	if f.PaymentMethod != nil {
		args = append(args, *f.PaymentMethod)
		conditions = append(conditions, fmt.Sprintf("payment_method = $%d", len(args)))
	}
	if f.HireBookingID != nil {
		args = append(args, *f.HireBookingID)
		conditions = append(conditions, fmt.Sprintf("hire_booking_id = $%d", len(args)))
	}
	if f.SaleID != nil {
		args = append(args, *f.SaleID)
		conditions = append(conditions, fmt.Sprintf("sale_id = $%d", len(args)))
	}
	if f.ServiceJobID != nil {
		args = append(args, *f.ServiceJobID)
		conditions = append(conditions, fmt.Sprintf("service_job_id = $%d", len(args)))
	}

	q := `SELECT ` + recordCols + `
	      FROM   finance_records
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY transaction_date DESC, created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("finance: list records: %w", err)
	}
	defer rows.Close()

	var result []*Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("finance: list records scan: %w", err)
		}
		result = append(result, rec)
	}

	return result, rows.Err()
}

// FindRecordByID returns a finance record by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindRecordByID(ctx context.Context, id string) (*Record, error) {
	q := `SELECT ` + recordCols + ` FROM finance_records WHERE id = $1 LIMIT 1`

	rec, err := scanRecord(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finance: find record by id: %w", err)
	}

	return rec, nil
}

// GetSummary returns aggregated income/expense totals for a tenant,
// optionally restricted to a date range.
func (r *Repository) GetSummary(ctx context.Context, tenantID string, from, to *time.Time) (LedgerSummary, error) {
	args := []any{tenantID}
	dateClause := ""

	if from != nil {
		args = append(args, *from)
		dateClause += fmt.Sprintf(" AND transaction_date >= $%d", len(args))
	}
	if to != nil {
		args = append(args, *to)
		dateClause += fmt.Sprintf(" AND transaction_date <= $%d", len(args))
	}

	q := fmt.Sprintf(`
		SELECT
		    COALESCE(SUM(amount) FILTER (WHERE type = 'income'),  0) AS total_income,
		    COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0) AS total_expenses
		FROM  finance_records
		WHERE tenant_id = $1%s`, dateClause)

	var summary LedgerSummary
	err := r.db.QueryRow(ctx, q, args...).Scan(&summary.TotalIncome, &summary.TotalExpenses)
	if err != nil {
		return LedgerSummary{}, fmt.Errorf("finance: get summary: %w", err)
	}

	summary.NetBalance = summary.TotalIncome - summary.TotalExpenses
	return summary, nil
}

// CreateRecord inserts a new finance record.
func (r *Repository) CreateRecord(ctx context.Context, p CreateRecordParams) (*Record, error) {
	q := `INSERT INTO finance_records (
	          tenant_id, category_id,
	          type, amount,
	          hire_booking_id, sale_id, service_job_id,
	          description, transaction_date,
	          payment_method, reference,
	          created_by, notes
	      ) VALUES (
	          $1, $2,
	          $3, $4,
	          $5, $6, $7,
	          $8, $9,
	          $10, $11,
	          $12, $13
	      )
	      RETURNING ` + recordCols

	rec, err := scanRecord(r.db.QueryRow(ctx, q,
		p.TenantID, p.CategoryID,
		p.Type, p.Amount,
		p.HireBookingID, p.SaleID, p.ServiceJobID,
		p.Description, p.TransactionDate,
		p.PaymentMethod, p.Reference,
		p.CreatedBy, p.Notes,
	))
	if err != nil {
		return nil, fmt.Errorf("finance: create record: %w", err)
	}

	return rec, nil
}

// UpdateRecord applies a partial update to a finance record.
func (r *Repository) UpdateRecord(ctx context.Context, id string, p UpdateRecordParams) (*Record, error) {
	q := `UPDATE finance_records
	      SET    category_id      = COALESCE($2,  category_id),
	             amount           = COALESCE($3,  amount),
	             hire_booking_id  = COALESCE($4,  hire_booking_id),
	             sale_id          = COALESCE($5,  sale_id),
	             service_job_id   = COALESCE($6,  service_job_id),
	             description      = COALESCE($7,  description),
	             transaction_date = COALESCE($8,  transaction_date),
	             payment_method   = COALESCE($9,  payment_method),
	             reference        = COALESCE($10, reference),
	             notes            = COALESCE($11, notes),
	             updated_at       = NOW()
	      WHERE  id = $1
	      RETURNING ` + recordCols

	rec, err := scanRecord(r.db.QueryRow(ctx, q,
		id,
		p.CategoryID, p.Amount,
		p.HireBookingID, p.SaleID, p.ServiceJobID,
		p.Description, p.TransactionDate,
		p.PaymentMethod, p.Reference,
		p.Notes,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finance: update record: %w", err)
	}

	return rec, nil
}

// DeleteRecord hard-deletes a finance record.
func (r *Repository) DeleteRecord(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM finance_records WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("finance: delete record: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Params ────────────────────────────────────────────────────────────────────

type CreateCategoryParams struct {
	TenantID    string
	Name        string
	Type        EntryType
	Description *string
}

type UpdateCategoryParams struct {
	Name        *string
	Description *string
	IsActive    *bool
}

type CreateRecordParams struct {
	TenantID       string
	CategoryID     string
	Type           EntryType
	Amount         float64
	HireBookingID  *string
	SaleID         *string
	ServiceJobID   *string
	Description    string
	TransactionDate time.Time
	PaymentMethod  *string
	Reference      *string
	CreatedBy      *string
	Notes          *string
}

type UpdateRecordParams struct {
	CategoryID      *string
	Amount          *float64
	HireBookingID   *string
	SaleID          *string
	ServiceJobID    *string
	Description     *string
	TransactionDate *time.Time
	PaymentMethod   *string
	Reference       *string
	Notes           *string
}

// ── Scanners ──────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCategory(row rowScanner) (*Category, error) {
	var c Category
	var t string

	err := row.Scan(
		&c.ID, &c.TenantID, &c.Name, &t, &c.Description,
		&c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	c.Type = EntryType(t)
	return &c, nil
}

func scanRecord(row rowScanner) (*Record, error) {
	var rec Record
	var t string

	err := row.Scan(
		&rec.ID, &rec.TenantID, &rec.CategoryID,
		&t, &rec.Amount,
		&rec.HireBookingID, &rec.SaleID, &rec.ServiceJobID,
		&rec.Description, &rec.TransactionDate,
		&rec.PaymentMethod, &rec.Reference,
		&rec.CreatedBy, &rec.Notes,
		&rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	rec.Type = EntryType(t)
	return &rec, nil
}
