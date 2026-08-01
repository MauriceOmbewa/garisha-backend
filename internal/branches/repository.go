package branches

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all DB operations for the branches domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ListByTenant returns all branches for a tenant ordered by name.
func (r *Repository) ListByTenant(ctx context.Context, tenantID string) ([]*Branch, error) {
	const q = `
		SELECT id, tenant_id, name, slug, city, address, phone, email,
		       is_active, is_default, created_at, updated_at
		FROM   branches
		WHERE  tenant_id = $1
		ORDER  BY is_default DESC, name ASC`

	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("branches: list: %w", err)
	}
	defer rows.Close()

	var out []*Branch
	for rows.Next() {
		b, err := scanBranch(rows)
		if err != nil {
			return nil, fmt.Errorf("branches: list scan: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// FindByID returns a single branch or nil if not found.
func (r *Repository) FindByID(ctx context.Context, id string) (*Branch, error) {
	const q = `
		SELECT id, tenant_id, name, slug, city, address, phone, email,
		       is_active, is_default, created_at, updated_at
		FROM   branches WHERE id = $1 LIMIT 1`

	b, err := scanBranch(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("branches: find by id: %w", err)
	}
	return b, nil
}

// CreateParams holds the fields for creating a branch.
type CreateParams struct {
	TenantID  string
	Name      string
	Slug      string
	City      *string
	Address   *string
	Phone     *string
	Email     *string
	IsDefault bool
}

// Create inserts a new branch.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Branch, error) {
	const q = `
		INSERT INTO branches (tenant_id, name, slug, city, address, phone, email, is_default)
		VALUES               ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, name, slug, city, address, phone, email,
		          is_active, is_default, created_at, updated_at`

	b, err := scanBranch(r.db.QueryRow(ctx, q,
		p.TenantID, p.Name, p.Slug, p.City, p.Address, p.Phone, p.Email, p.IsDefault,
	))
	if err != nil {
		return nil, fmt.Errorf("branches: create: %w", err)
	}
	return b, nil
}

// UpdateParams holds nullable update fields.
type UpdateParams struct {
	Name      *string
	City      *string
	Address   *string
	Phone     *string
	Email     *string
	IsActive  *bool
	IsDefault *bool
}

// Update modifies mutable fields on a branch.
func (r *Repository) Update(ctx context.Context, id string, p UpdateParams) (*Branch, error) {
	const q = `
		UPDATE branches
		SET    name       = COALESCE($2, name),
		       city       = COALESCE($3, city),
		       address    = COALESCE($4, address),
		       phone      = COALESCE($5, phone),
		       email      = COALESCE($6, email),
		       is_active  = COALESCE($7, is_active),
		       is_default = COALESCE($8, is_default),
		       updated_at = NOW()
		WHERE  id = $1
		RETURNING id, tenant_id, name, slug, city, address, phone, email,
		          is_active, is_default, created_at, updated_at`

	b, err := scanBranch(r.db.QueryRow(ctx, q,
		id, p.Name, p.City, p.Address, p.Phone, p.Email, p.IsActive, p.IsDefault,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("branches: update: %w", err)
	}
	return b, nil
}

// Delete hard-deletes a branch. branch_id FKs on other tables are SET NULL.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM branches WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("branches: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ── scanner ───────────────────────────────────────────────────────────────────

type scanner interface{ Scan(dest ...any) error }

func scanBranch(row scanner) (*Branch, error) {
	var b Branch
	err := row.Scan(
		&b.ID, &b.TenantID, &b.Name, &b.Slug,
		&b.City, &b.Address, &b.Phone, &b.Email,
		&b.IsActive, &b.IsDefault, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
