package customers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the customers domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository backed by the given pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── ListFilters ───────────────────────────────────────────────────────────────

// ListFilters narrows the customer list query.  Zero-value fields are ignored.
type ListFilters struct {
	Search   *string // partial match on full_name, email, or phone
	IsActive *bool   // filter by active/inactive status
}

// ── Queries ───────────────────────────────────────────────────────────────────

// List returns customers for tenantID, optionally filtered and searched.
func (r *Repository) List(ctx context.Context, tenantID string, f ListFilters) ([]*Customer, error) {
	args := []any{tenantID}
	conditions := []string{"tenant_id = $1"}

	if f.IsActive != nil {
		args = append(args, *f.IsActive)
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", len(args)))
	}

	if f.Search != nil && *f.Search != "" {
		args = append(args, "%"+*f.Search+"%")
		n := len(args)
		conditions = append(conditions, fmt.Sprintf(
			"(full_name ILIKE $%d OR email ILIKE $%d OR phone ILIKE $%d)",
			n, n, n,
		))
	}

	q := `SELECT id, tenant_id, user_id,
	             full_name, email, phone, id_number, id_type,
	             country, city, address,
	             is_active, notes, created_at, updated_at
	      FROM   customers
	      WHERE  ` + strings.Join(conditions, " AND ") + `
	      ORDER  BY full_name ASC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("customers: list: %w", err)
	}
	defer rows.Close()

	var result []*Customer
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, fmt.Errorf("customers: list scan: %w", err)
		}
		result = append(result, c)
	}

	return result, rows.Err()
}

// FindByID returns a customer by UUID.  Returns (nil, nil) when not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Customer, error) {
	const q = `
		SELECT id, tenant_id, user_id,
		       full_name, email, phone, id_number, id_type,
		       country, city, address,
		       is_active, notes, created_at, updated_at
		FROM   customers
		WHERE  id = $1
		LIMIT  1`

	c, err := scanCustomer(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("customers: find by id: %w", err)
	}

	return c, nil
}

// FindByEmail returns the customer with the given email for a tenant.
// Returns (nil, nil) when not found.
func (r *Repository) FindByEmail(ctx context.Context, tenantID, email string) (*Customer, error) {
	const q = `
		SELECT id, tenant_id, user_id,
		       full_name, email, phone, id_number, id_type,
		       country, city, address,
		       is_active, notes, created_at, updated_at
		FROM   customers
		WHERE  tenant_id = $1
		AND    email     = $2
		LIMIT  1`

	c, err := scanCustomer(r.db.QueryRow(ctx, q, tenantID, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("customers: find by email: %w", err)
	}

	return c, nil
}

// Create inserts a new customer and returns the persisted record.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Customer, error) {
	const q = `
		INSERT INTO customers (
		    tenant_id, user_id,
		    full_name, email, phone, id_number, id_type,
		    country, city, address, notes
		) VALUES (
		    $1, $2,
		    $3, $4, $5, $6, $7,
		    $8, $9, $10, $11
		)
		RETURNING id, tenant_id, user_id,
		          full_name, email, phone, id_number, id_type,
		          country, city, address,
		          is_active, notes, created_at, updated_at`

	c, err := scanCustomer(r.db.QueryRow(ctx, q,
		p.TenantID, p.UserID,
		p.FullName, p.Email, p.Phone, p.IDNumber, p.IDType,
		p.Country, p.City, p.Address, p.Notes,
	))
	if err != nil {
		return nil, fmt.Errorf("customers: create: %w", err)
	}

	return c, nil
}

// Update applies a partial update to a customer.
func (r *Repository) Update(ctx context.Context, id string, p UpdateParams) (*Customer, error) {
	const q = `
		UPDATE customers
		SET    full_name  = COALESCE($2,  full_name),
		       email      = COALESCE($3,  email),
		       phone      = COALESCE($4,  phone),
		       id_number  = COALESCE($5,  id_number),
		       id_type    = COALESCE($6,  id_type),
		       country    = COALESCE($7,  country),
		       city       = COALESCE($8,  city),
		       address    = COALESCE($9,  address),
		       is_active  = COALESCE($10, is_active),
		       notes      = COALESCE($11, notes),
		       updated_at = NOW()
		WHERE  id = $1
		RETURNING id, tenant_id, user_id,
		          full_name, email, phone, id_number, id_type,
		          country, city, address,
		          is_active, notes, created_at, updated_at`

	c, err := scanCustomer(r.db.QueryRow(ctx, q,
		id,
		p.FullName, p.Email, p.Phone, p.IDNumber, p.IDType,
		p.Country, p.City, p.Address, p.IsActive, p.Notes,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("customers: update: %w", err)
	}

	return c, nil
}

// Delete hard-deletes a customer by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM customers WHERE id = $1`

	ct, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("customers: delete: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// ── Params ────────────────────────────────────────────────────────────────────

// CreateParams holds all fields for inserting a new customer.
type CreateParams struct {
	TenantID string
	UserID   *string
	FullName string
	Email    *string
	Phone    *string
	IDNumber *string
	IDType   *string
	Country  *string
	City     *string
	Address  *string
	Notes    *string
}

// UpdateParams holds nullable fields for a partial customer update.
type UpdateParams struct {
	FullName *string
	Email    *string
	Phone    *string
	IDNumber *string
	IDType   *string
	Country  *string
	City     *string
	Address  *string
	IsActive *bool
	Notes    *string
}

// ── Scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCustomer(row rowScanner) (*Customer, error) {
	var c Customer

	err := row.Scan(
		&c.ID,
		&c.TenantID,
		&c.UserID,
		&c.FullName,
		&c.Email,
		&c.Phone,
		&c.IDNumber,
		&c.IDType,
		&c.Country,
		&c.City,
		&c.Address,
		&c.IsActive,
		&c.Notes,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// isDuplicateError detects PostgreSQL unique constraint violations (code 23505).
func isDuplicateError(err error) bool {
	return err != nil && strings.Contains(fmt.Sprintf("%v", err), "23505")
}
